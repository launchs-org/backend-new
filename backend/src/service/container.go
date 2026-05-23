package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"launchs/shared/model"
	"launchs/shared/temporal"
	apperrors "launchs/shared/errors"
	"backend/repository"
)

// ---- Temporal ワークフロー入力型 ----

// BuildDeployWorkflowInput は BuildDeployWorkflow の入力です。
// builder の BuildWorkflowInput と JSON フィールド名を一致させます。
type BuildDeployWorkflowInput struct {
	ContainerID         string `json:"ContainerID"`
	BuildJobID          string `json:"BuildJobID"`
	ProjectID           string `json:"ProjectID"`
	Namespace           string `json:"Namespace"`
	GitRepo             string `json:"GitRepo"`
	GitBranch           string `json:"GitBranch"`
	GitCommit           string `json:"GitCommit"`
	GitSubdir           string `json:"GitSubdir"`
	HarborProjectName   string `json:"HarborProjectName"`
	HarborRobotUsername string `json:"HarborRobotUsername"`
	HarborRobotPassword string `json:"HarborRobotPassword"`
	ResourceSize        string `json:"ResourceSize"`
	Replicas            int    `json:"Replicas"`
}

// ScaleWorkflowInput は ScaleWorkflow の入力です。
type ScaleWorkflowInput struct {
	ContainerID    string `json:"ContainerID"`
	Namespace      string `json:"Namespace"`
	DeploymentName string `json:"DeploymentName"`
	Replicas       int    `json:"Replicas"`
}

// RedeployWorkflowInput は RedeployWorkflow の入力です。
type RedeployWorkflowInput struct {
	ContainerID    string `json:"ContainerID"`
	Namespace      string `json:"Namespace"`
	DeploymentName string `json:"DeploymentName"`
}

// DeleteContainerWorkflowInput は DeleteContainerWorkflow の入力です。
type DeleteContainerWorkflowInput struct {
	ContainerID    string `json:"ContainerID"`
	ProjectID      string `json:"ProjectID"`
	Namespace      string `json:"Namespace"`
	DeploymentName string `json:"DeploymentName"`
}

// DeployTemplateWorkflowInput はテンプレートデプロイの入力です。
type DeployTemplateWorkflowInput struct {
	ContainerID  string            `json:"container_id"`
	ProjectID    string            `json:"project_id"`
	TemplateName string            `json:"template_name"`
	ResourceSize string            `json:"resource_size"`
	Params       map[string]string `json:"params"`
	VolumeID     *string           `json:"volume_id"`
	MountPath    *string           `json:"mount_path"`
}

type containerService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	envVarRepo    repository.EnvVarRepository
	portRepo      repository.PortRepository
	buildJobRepo  repository.BuildJobRepository
	temporal      client.Client
}

// NewContainerService は ContainerService の実装を返します。
func NewContainerService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	envVarRepo repository.EnvVarRepository,
	portRepo repository.PortRepository,
	buildJobRepo repository.BuildJobRepository,
	temporalClient client.Client,
) ContainerService {
	return &containerService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		envVarRepo:    envVarRepo,
		portRepo:      portRepo,
		buildJobRepo:  buildJobRepo,
		temporal:      temporalClient,
	}
}

func (s *containerService) List(ctx context.Context, projectID uuid.UUID) ([]model.Container, error) {
	return s.containerRepo.FindByProjectID(ctx, projectID)
}

func (s *containerService) Get(ctx context.Context, projectID, containerID uuid.UUID) (*model.Container, error) {
	container, err := s.containerRepo.FindByIDWithDetails(ctx, containerID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return container, nil
}

func (s *containerService) BuildDeploy(ctx context.Context, projectID uuid.UUID, req BuildDeployRequest) (*model.Container, string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}

	// Harbor 情報がまだない場合は CreateProjectWorkflow の完了を待ちます。
	// プロジェクト作成直後にデプロイした場合、ワークフローがまだ実行中の可能性があります。
	if project.HarborRobotUsername == "" {
		createWorkflowID := fmt.Sprintf("create-project-%s", projectID.String())
		fmt.Printf("[INFO] Harbor 未設定、CreateProjectWorkflow 完了待ち: workflowID=%s\n", createWorkflowID)
		we := s.temporal.GetWorkflow(ctx, createWorkflowID, "")
		if waitErr := we.Get(ctx, nil); waitErr != nil {
			fmt.Printf("[ERROR] CreateProjectWorkflow 失敗: workflowID=%s err=%v\n", createWorkflowID, waitErr)
			return nil, "", fmt.Errorf("Harbor 初期化エラー: %w", waitErr)
		}
		// ワークフロー完了後に Harbor 情報を再取得
		project, err = s.projectRepo.FindByID(ctx, projectID)
		if err != nil {
			return nil, "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
		}
		if project.HarborRobotUsername == "" {
			fmt.Printf("[ERROR] CreateProjectWorkflow 完了後も Harbor 情報が空: projectID=%s\n", projectID.String())
			return nil, "", fmt.Errorf("Harbor 初期化失敗: CreateProjectWorkflow は完了しましたが Harbor 認証情報が保存されていません")
		}
		fmt.Printf("[INFO] Harbor 情報取得完了: projectID=%s username=%s\n", projectID.String(), project.HarborRobotUsername)
	}

	containerID := uuid.New()
	resourceSize := req.ResourceSize
	if resourceSize == "" {
		resourceSize = "small"
	}
	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	gitRepo := normalizeGitRepo(req.GitRepo)

	container := &model.Container{
		ID:           containerID,
		ProjectID:    projectID,
		Name:         req.Name,
		Status:       string(model.ContainerStatusPending),
		Replicas:     replicas,
		ResourceSize: resourceSize,
		GitRepo:      &gitRepo,
		GitBranch:    &req.GitBranch,
		GitSubdir:    strPtr(req.Subdir),
	}
	if err := s.containerRepo.Create(ctx, container); err != nil {
		return nil, "", fmt.Errorf("failed to create container: %w", err)
	}

	// 環境変数を保存します
	if len(req.EnvVars) > 0 {
		envVars := make([]model.ContainerEnvVar, len(req.EnvVars))
		for i, v := range req.EnvVars {
			envVars[i] = model.ContainerEnvVar{
				ID:          uuid.New(),
				ContainerID: containerID,
				Key:         v.Key,
				Value:       v.Value,
			}
		}
		if err := s.envVarRepo.UpsertContainerEnvVars(ctx, containerID, envVars); err != nil {
			return nil, "", fmt.Errorf("failed to save env vars: %w", err)
		}
	}

	// ポートを保存します
	if len(req.Ports) > 0 {
		for _, p := range req.Ports {
			port := &model.Port{
				ID:          uuid.New(),
				ContainerID: containerID,
				Port:        p.Port,
				Protocol:    p.Protocol,
			}
			if err := s.portRepo.Create(ctx, port); err != nil {
				return nil, "", fmt.Errorf("failed to save port: %w", err)
			}
		}
	}

	// BuildJob レコードを作成します
	buildJobID := uuid.New()
	subdir := req.Subdir
	if subdir == "" {
		subdir = "."
	}
	buildJob := &model.BuildJob{
		ID:          buildJobID,
		ContainerID: containerID,
		GitRepo:     gitRepo,
		GitBranch:   req.GitBranch,
		GitCommit:   req.GitCommit,
		Subdir:      subdir,
		Status:      string(model.BuildJobStatusPending),
	}
	if err := s.buildJobRepo.Create(ctx, buildJob); err != nil {
		return nil, "", fmt.Errorf("failed to create build job: %w", err)
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("build-deploy-%s", containerID.String()),
		TaskQueue: temporal.BuilderQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowBuildDeploy, BuildDeployWorkflowInput{
		ContainerID:         containerID.String(),
		BuildJobID:          buildJobID.String(),
		ProjectID:           projectID.String(),
		Namespace:           project.Namespace,
		GitRepo:             gitRepo,
		GitBranch:           req.GitBranch,
		GitCommit:           req.GitCommit,
		GitSubdir:           req.Subdir,
		HarborProjectName:   project.HarborProjectName,
		HarborRobotUsername: project.HarborRobotUsername,
		HarborRobotPassword: project.HarborRobotPassword,
		ResourceSize:        resourceSize,
		Replicas:            replicas,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start BuildDeployWorkflow: %w", err)
	}

	workflowID := we.GetID()
	if err := s.containerRepo.UpdateActiveDeployWorkflowID(ctx, containerID, &workflowID); err != nil {
		return nil, "", fmt.Errorf("failed to update workflow id: %w", err)
	}
	if err := s.buildJobRepo.UpdateWorkflowID(ctx, buildJobID, workflowID); err != nil {
		return nil, "", fmt.Errorf("failed to update build job workflow id: %w", err)
	}

	return container, workflowID, nil
}

func (s *containerService) DeployFromTemplate(ctx context.Context, projectID uuid.UUID, req TemplateDeployRequest) (*model.Container, string, error) {
	_, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}

	containerID := uuid.New()
	resourceSize := req.ResourceSize
	if resourceSize == "" {
		resourceSize = "small"
	}

	container := &model.Container{
		ID:           containerID,
		ProjectID:    projectID,
		Name:         req.Name,
		Status:       string(model.ContainerStatusPending),
		Replicas:     1,
		ResourceSize: resourceSize,
	}
	if err := s.containerRepo.Create(ctx, container); err != nil {
		return nil, "", fmt.Errorf("failed to create container: %w", err)
	}

	input := DeployTemplateWorkflowInput{
		ContainerID:  containerID.String(),
		ProjectID:    projectID.String(),
		TemplateName: req.TemplateName,
		ResourceSize: resourceSize,
		Params:       req.Params,
	}
	if req.VolumeID != nil {
		s := req.VolumeID.String()
		input.VolumeID = &s
	}
	if req.MountPath != nil {
		input.MountPath = req.MountPath
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("deploy-template-%s", containerID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeploy, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start DeployWorkflow: %w", err)
	}

	workflowID := we.GetID()
	if err := s.containerRepo.UpdateActiveDeployWorkflowID(ctx, containerID, &workflowID); err != nil {
		return nil, "", fmt.Errorf("failed to update workflow id: %w", err)
	}

	return container, workflowID, nil
}

func (s *containerService) Scale(ctx context.Context, projectID, containerID uuid.UUID, replicas int) (string, error) {
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("scale-%s-%d", containerID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowScale, ScaleWorkflowInput{
		ContainerID:    containerID.String(),
		Namespace:      project.Namespace,
		DeploymentName: fmt.Sprintf("%s-%s", container.Name, containerID.String()[:8]),
		Replicas:       replicas,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start ScaleWorkflow: %w", err)
	}

	workflowID := we.GetID()
	if err := s.containerRepo.UpdateActiveScaleWorkflowID(ctx, containerID, &workflowID); err != nil {
		return "", fmt.Errorf("failed to update scale workflow id: %w", err)
	}

	return workflowID, nil
}

func (s *containerService) Redeploy(ctx context.Context, projectID, containerID uuid.UUID) (string, error) {
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("redeploy-%s-%d", containerID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowRedeploy, RedeployWorkflowInput{
		ContainerID:    containerID.String(),
		Namespace:      project.Namespace,
		DeploymentName: fmt.Sprintf("%s-%s", container.Name, containerID.String()[:8]),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start RedeployWorkflow: %w", err)
	}

	workflowID := we.GetID()
	if err := s.containerRepo.UpdateActiveDeployWorkflowID(ctx, containerID, &workflowID); err != nil {
		return "", fmt.Errorf("failed to update deploy workflow id: %w", err)
	}

	return workflowID, nil
}

func (s *containerService) Rebuild(ctx context.Context, projectID, containerID uuid.UUID) (string, error) {
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}
	if container.GitRepo == nil || *container.GitRepo == "" {
		return "", fmt.Errorf("container is not a GitHub deploy container")
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}

	buildJobID := uuid.New()
	gitBranch := ""
	if container.GitBranch != nil {
		gitBranch = *container.GitBranch
	}
	subdir := "."
	if container.GitSubdir != nil && *container.GitSubdir != "" {
		subdir = *container.GitSubdir
	}
	gitRepo := normalizeGitRepo(*container.GitRepo)
	buildJob := &model.BuildJob{
		ID:          buildJobID,
		ContainerID: containerID,
		GitRepo:     gitRepo,
		GitBranch:   gitBranch,
		Subdir:      subdir,
		Status:      string(model.BuildJobStatusPending),
	}
	if err := s.buildJobRepo.Create(ctx, buildJob); err != nil {
		return "", fmt.Errorf("failed to create build job: %w", err)
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("build-deploy-%s-%d", containerID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.BuilderQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowBuildDeploy, BuildDeployWorkflowInput{
		ContainerID:         containerID.String(),
		BuildJobID:          buildJobID.String(),
		ProjectID:           projectID.String(),
		Namespace:           project.Namespace,
		GitRepo:             gitRepo,
		GitBranch:           gitBranch,
		GitSubdir:           subdir,
		HarborProjectName:   project.HarborProjectName,
		HarborRobotUsername: project.HarborRobotUsername,
		HarborRobotPassword: project.HarborRobotPassword,
		ResourceSize:        container.ResourceSize,
		Replicas:            container.Replicas,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start BuildDeployWorkflow: %w", err)
	}

	workflowID := we.GetID()
	if err := s.containerRepo.UpdateActiveDeployWorkflowID(ctx, containerID, &workflowID); err != nil {
		return "", fmt.Errorf("failed to update workflow id: %w", err)
	}
	if err := s.buildJobRepo.UpdateWorkflowID(ctx, buildJobID, workflowID); err != nil {
		return "", fmt.Errorf("failed to update build job workflow id: %w", err)
	}

	return workflowID, nil
}

func (s *containerService) Delete(ctx context.Context, projectID, containerID uuid.UUID) (string, error) {
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}

	// 外部キー制約のある関連テーブルを先にまとめて削除
	if err := s.buildJobRepo.DeleteByContainerID(ctx, containerID); err != nil {
		return "", fmt.Errorf("failed to delete build jobs: %w", err)
	}
	if err := s.containerRepo.DeleteRelated(ctx, containerID); err != nil {
		return "", fmt.Errorf("failed to delete related records: %w", err)
	}
	if err := s.containerRepo.Delete(ctx, containerID); err != nil {
		return "", fmt.Errorf("failed to delete container: %w", err)
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("delete-container-%s-%d", containerID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeleteContainer, DeleteContainerWorkflowInput{
		ContainerID:    containerID.String(),
		ProjectID:      projectID.String(),
		Namespace:      project.Namespace,
		DeploymentName: fmt.Sprintf("%s-%s", container.Name, containerID.String()[:8]),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start DeleteContainerWorkflow: %w", err)
	}

	return we.GetID(), nil
}

func (s *containerService) Update(ctx context.Context, projectID, containerID uuid.UUID, resourceSize string) error {
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}

	container.ResourceSize = resourceSize
	return s.containerRepo.Update(ctx, container)
}

func (s *containerService) CreateWebhook(ctx context.Context, projectID, containerID uuid.UUID) (string, string, error) {
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return "", "", &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return "", "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	token, err := generateToken(32)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate token: %w", err)
	}

	if err := s.containerRepo.UpdateWebhookToken(ctx, containerID, &token); err != nil {
		return "", "", fmt.Errorf("failed to save webhook token: %w", err)
	}

	webhookURL := fmt.Sprintf("https://api.launchs.org/api/v1/webhooks/%s", token)
	return webhookURL, token, nil
}

func (s *containerService) HandleWebhook(ctx context.Context, token string) error {
	container, err := s.containerRepo.FindByWebhookToken(ctx, token)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "webhook", ID: token}
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("redeploy-%s-%d", container.ID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	project, err := s.projectRepo.FindByID(ctx, container.ProjectID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "project", ID: container.ProjectID.String()}
	}

	_, err = s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowRedeploy, RedeployWorkflowInput{
		ContainerID:    container.ID.String(),
		Namespace:      project.Namespace,
		DeploymentName: fmt.Sprintf("%s-%s", container.Name, container.ID.String()[:8]),
	})
	return err
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func strPtr(s string) *string {
	return &s
}

// normalizeGitRepo は "owner/repo" または GitHub URL を "https://github.com/owner/repo" に正規化します。
func normalizeGitRepo(raw string) string {
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		return strings.TrimSuffix(raw, ".git")
	}
	return "https://github.com/" + strings.TrimSuffix(raw, ".git")
}
