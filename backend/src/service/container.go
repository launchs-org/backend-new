package service

import (
	"backend/repository"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"launchs/shared/model"
	"launchs/shared/temporal"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	apperrors "launchs/shared/errors"
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

// DeployWorkflowInput は DeployWorkflow への入力です。
type DeployWorkflowInput struct {
	ContainerID    string              `json:"ContainerID"`
	Namespace      string              `json:"Namespace"`
	DeploymentName string              `json:"DeploymentName"`
	ImageRef       string              `json:"ImageRef"`
	Replicas       int                 `json:"Replicas"`
	ResourceSize   string              `json:"ResourceSize"`
	EnvVars        []EnvVarWorkflow    `json:"EnvVars"`
	Ports          []PortWorkflow      `json:"Ports"`
	VolumeMounts   []VolumeMountWorkflow `json:"VolumeMounts"`
}

// EnvVarWorkflow はワークフロー用環境変数です。
type EnvVarWorkflow struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// PortWorkflow はワークフロー用ポートです。
type PortWorkflow struct {
	Port     int    `json:"Port"`
	Protocol string `json:"Protocol"`
}

// VolumeMountWorkflow はワークフロー用ボリュームマウントです。
type VolumeMountWorkflow struct {
	PVCName   string `json:"PVCName"`
	MountPath string `json:"MountPath"`
}

// VolumeRecordWorkflow はワークフローに渡すボリューム DB レコード情報です。
type VolumeRecordWorkflow struct {
	ID     string `json:"ID"`
	Name   string `json:"Name"`
	SizeMB int    `json:"SizeMB"`
}

// RouteRecordWorkflow はワークフローに渡す Route DB レコード情報です。
type RouteRecordWorkflow struct {
	ID       string `json:"ID"`
	Port     int    `json:"Port"`
	Protocol string `json:"Protocol"`
}

// DeployTemplateWorkflowInput は DeployTemplateWorkflow への入力です。
type DeployTemplateWorkflowInput struct {
	ContainerID     string                `json:"ContainerID"`
	Namespace       string                `json:"Namespace"`
	DeploymentName  string                `json:"DeploymentName"`
	ImageRef        string                `json:"ImageRef"`
	ResourceSize    string                `json:"ResourceSize"`
	Replicas        int                   `json:"Replicas"`
	EnvVars         []EnvVarWorkflow      `json:"EnvVars"`
	// VolumeRecord は新規作成するボリュームの情報（nil の場合は作成しない）
	VolumeRecord    *VolumeRecordWorkflow `json:"VolumeRecord"`
	VolumeMountPath string                `json:"VolumeMountPath"`
	// ExistingVolumeMounts は既存ボリュームのマウント情報（PVC 作成不要）
	ExistingVolumeMounts []VolumeMountWorkflow `json:"ExistingVolumeMounts"`
	RouteRecords    []RouteRecordWorkflow `json:"RouteRecords"`
}

type containerService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	envVarRepo    repository.EnvVarRepository
	portRepo      repository.PortRepository
	buildJobRepo  repository.BuildJobRepository
	volumeRepo    repository.VolumeRepository
	routeRepo     repository.NetworkRouteRepository
	templateSvc   TemplateService
	temporal      client.Client
}

// NewContainerService は ContainerService の実装を返します。
func NewContainerService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	envVarRepo repository.EnvVarRepository,
	portRepo repository.PortRepository,
	buildJobRepo repository.BuildJobRepository,
	volumeRepo repository.VolumeRepository,
	routeRepo repository.NetworkRouteRepository,
	templateSvc TemplateService,
	temporalClient client.Client,
) ContainerService {
	return &containerService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		envVarRepo:    envVarRepo,
		portRepo:      portRepo,
		buildJobRepo:  buildJobRepo,
		volumeRepo:    volumeRepo,
		routeRepo:     routeRepo,
		templateSvc:   templateSvc,
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

	// Harbor 情報がまだない場合は エラーを返す
	if project.HarborRobotUsername == "" {
		return nil, "", fmt.Errorf("harbor robot username is empty")
	}

	// コンテナを作成します
	containerID := uuid.New()

	// リソースサイズを決めます
	resourceSize := req.ResourceSize
	if resourceSize == "" {
		resourceSize = "small"
	}

	// リプリカ数を決めます
	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	// Git リポジトリを正規化します
	gitRepo := normalizeGitRepo(req.GitRepo)

	// コンテナを作成します
	container := &model.Container{
		ID:           containerID,
		ProjectID:    projectID,
		Name:         req.Name,
		Status:       model.ContainerStatusPending,
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
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}

	tmpl, err := s.templateSvc.Get(ctx, req.TemplateName)
	if err != nil {
		return nil, "", fmt.Errorf("template not found: %s", req.TemplateName)
	}

	containerID := uuid.New()

	// resource_size と replicas はリクエスト値を優先し、未指定ならテンプレート YAML の spec 値を使用
	resourceSize := req.ResourceSize
	if resourceSize == "" && tmpl.Spec != nil && tmpl.Spec.ResourceSize != "" {
		resourceSize = tmpl.Spec.ResourceSize
	}
	if resourceSize == "" {
		resourceSize = "small"
	}

	replicas := req.Replicas
	if replicas <= 0 && tmpl.Spec != nil && tmpl.Spec.Replicas > 0 {
		replicas = tmpl.Spec.Replicas
	}
	if replicas <= 0 {
		replicas = 1
	}

	container := &model.Container{
		ID:           containerID,
		ProjectID:    projectID,
		Name:         req.Name,
		Status:       model.ContainerStatusPending,
		Replicas:     replicas,
		ResourceSize: resourceSize,
		IsTemplate:   true,
	}
	if err := s.containerRepo.Create(ctx, container); err != nil {
		return nil, "", fmt.Errorf("failed to create container: %w", err)
	}

	// 環境変数をテンプレートデフォルト + ユーザー指定 + auto_generate で組み立て
	envVars := make([]EnvVarWorkflow, 0, len(tmpl.EnvVars))
	for _, ev := range tmpl.EnvVars {
		val := ev.Default
		if v, ok := req.Params[ev.Key]; ok && v != "" {
			val = v
		} else if ev.AutoGenerate && val == "" {
			if generated, genErr := generateToken(12); genErr == nil {
				val = generated
			}
		}
		envVars = append(envVars, EnvVarWorkflow{Key: ev.Key, Value: val})
	}

	// ボリューム DB レコードを作成（ワークフローが PVC を実際に作成する）
	var volumeRecord *VolumeRecordWorkflow
	volumeMountPath := ""
	if req.CreateVolume && tmpl.Volume != nil {
		sizeMB := req.VolumeSize
		if sizeMB == 0 {
			sizeMB = tmpl.Volume.DefaultSizeMB
		}
		volName := fmt.Sprintf("%s-data", req.Name)
		vol := &model.Volume{
			ID:           uuid.New(),
			ProjectID:    projectID,
			Name:         volName,
			SizeMB:       sizeMB,
			StorageClass: "default",
			Status:       model.VolumeStatusPending,
		}
		if err := s.volumeRepo.Create(ctx, vol); err != nil {
			fmt.Printf("[warn] failed to create volume record: %v\n", err)
		} else {
			volumeRecord = &VolumeRecordWorkflow{
				ID:     vol.ID.String(),
				Name:   vol.Name,
				SizeMB: vol.SizeMB,
			}
			volumeMountPath = tmpl.Volume.MountPath
		}
	}

	// ユーザーが既存ボリュームを指定した場合は PVC 作成不要（ExistingVolumeMounts に入れる）
	existingVolumeMounts := make([]VolumeMountWorkflow, 0)
	if req.VolumeID != nil {
		mountPath := ""
		if req.MountPath != nil {
			mountPath = *req.MountPath
		}
		if mountPath == "" && tmpl.Volume != nil {
			mountPath = tmpl.Volume.MountPath
		}
		vol, volErr := s.volumeRepo.FindByID(ctx, *req.VolumeID)
		if volErr == nil {
			pvcName := fmt.Sprintf("%s-%s", vol.Name, vol.ID.String())
			existingVolumeMounts = append(existingVolumeMounts, VolumeMountWorkflow{
				PVCName:   pvcName,
				MountPath: mountPath,
			})
		}
	}

	// Service 用 Route DB レコードを作成（ワークフローが K8s Service を実際に作成する）
	routeRecords := make([]RouteRecordWorkflow, 0, len(tmpl.Ports))
	for _, p := range tmpl.Ports {
		protocol := p.Protocol
		if protocol == "" {
			protocol = "TCP"
		}
		route := &model.NetworkRoute{
			ID:          uuid.New(),
			ContainerID: containerID,
			Type:        string(model.NetworkRouteTypeService),
			Port:        p.Port,
			Protocol:    protocol,
		}
		if err := s.routeRepo.Create(ctx, route); err != nil {
			fmt.Printf("[warn] failed to create route record (port=%d): %v\n", p.Port, err)
			continue
		}
		routeRecords = append(routeRecords, RouteRecordWorkflow{
			ID:       route.ID.String(),
			Port:     route.Port,
			Protocol: route.Protocol,
		})
	}

	// テンプレートの env vars と接続情報をプロジェクト環境変数として自動登録する
	if err := s.injectTemplateProjectEnvVars(ctx, projectID, req.Name, tmpl.EnvVars, envVars); err != nil {
		fmt.Printf("[warn] failed to inject template project env vars: %v\n", err)
	}

	input := DeployTemplateWorkflowInput{
		ContainerID:          containerID.String(),
		Namespace:            project.Namespace,
		DeploymentName:       model.GetDeploymentName(containerID),
		ImageRef:             tmpl.Image,
		ResourceSize:         resourceSize,
		Replicas:             replicas,
		EnvVars:              envVars,
		VolumeRecord:         volumeRecord,
		VolumeMountPath:      volumeMountPath,
		ExistingVolumeMounts: existingVolumeMounts,
		RouteRecords:         routeRecords,
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("deploy-template-%s", containerID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeployTemplate, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start DeployTemplateWorkflow: %w", err)
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
		DeploymentName: fmt.Sprintf("%s-%s", "container", containerID.String()),
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
		DeploymentName: fmt.Sprintf("%s-%s", "container", containerID.String()),
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

	if err := s.cancelTimedOutBuildJobs(ctx, containerID); err != nil {
		return "", fmt.Errorf("failed to cancel timed-out build jobs: %w", err)
	}

	activeJobs, err := s.buildJobRepo.FindActiveByContainerID(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to check active build jobs: %w", err)
	}
	if len(activeJobs) > 0 {
		return "", &apperrors.ConflictError{Resource: "build job", ID: containerID.String()}
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
		DeploymentName: fmt.Sprintf("%s-%s", "container", containerID.String()),
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
		DeploymentName: fmt.Sprintf("%s-%s", "container", container.ID.String()),
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

const buildJobTimeout = 10 * time.Minute

// cancelTimedOutBuildJobs は pending/running 状態で開始から 10 分以上経過したビルドジョブを
// Temporal ワークフローごとキャンセルして failed にします。
func (s *containerService) cancelTimedOutBuildJobs(ctx context.Context, containerID uuid.UUID) error {
	jobs, err := s.buildJobRepo.FindActiveByContainerID(ctx, containerID)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, job := range jobs {
		var since time.Duration
		if job.StartedAt != nil {
			since = now.Sub(*job.StartedAt)
		} else {
			since = now.Sub(job.CreatedAt)
		}
		if since < buildJobTimeout {
			continue
		}
		if job.TemporalWorkflowID != nil {
			// エラーは無視：ワークフローが既に終了していても DB を failed に更新する
			_ = s.temporal.CancelWorkflow(ctx, *job.TemporalWorkflowID, "")
		}
		if err := s.buildJobRepo.UpdateStatus(ctx, job.ID, string(model.BuildJobStatusFailed)); err != nil {
			return fmt.Errorf("failed to mark timed-out build job as failed: %w", err)
		}
	}
	return nil
}

// injectTemplateProjectEnvVars はテンプレートデプロイ時にプロジェクト環境変数を自動登録します。
// 登録するキーは以下の規則で生成します:
//   - テンプレートの各 env var: {PREFIX}_{KEY}  (PREFIX はコンテナ名を大文字スネークケースに変換)
//   - ホスト名: {PREFIX}_HOST = <コンテナ名> (K8s Service 名と一致させる)
//
// 既存のキーは上書きしません（OnConflict DoUpdates は value を上書きするため、
// 既存値を保持したい場合は事前に確認が必要だが、初回追加時のみ呼ばれる想定）。
func (s *containerService) injectTemplateProjectEnvVars(
	ctx context.Context,
	projectID uuid.UUID,
	containerName string,
	defs []TemplateEnvVarDef,
	resolved []EnvVarWorkflow,
) error {
	// コンテナ名を PREFIX に変換: 英数字以外を _ に、全て大文字
	prefix := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(containerName))

	resolvedMap := make(map[string]string, len(resolved))
	for _, ev := range resolved {
		resolvedMap[ev.Key] = ev.Value
	}

	vars := make([]model.ProjectEnvVar, 0, len(defs)+1)

	// ホスト名: K8s Service は コンテナ名と同名で作られる想定
	vars = append(vars, model.ProjectEnvVar{
		ID:        uuid.New(),
		ProjectID: projectID,
		Key:       prefix + "_HOST",
		Value:     containerName,
	})

	// テンプレートの各 env var
	for _, def := range defs {
		val, ok := resolvedMap[def.Key]
		if !ok {
			val = def.Default
		}
		vars = append(vars, model.ProjectEnvVar{
			ID:        uuid.New(),
			ProjectID: projectID,
			Key:       prefix + "_" + def.Key,
			Value:     val,
		})
	}

	return s.envVarRepo.UpsertProjectEnvVars(ctx, projectID, vars)
}

// normalizeGitRepo は "owner/repo" または GitHub URL を "https://github.com/owner/repo" に正規化します。
func normalizeGitRepo(raw string) string {
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		return strings.TrimSuffix(raw, ".git")
	}
	return "https://github.com/" + strings.TrimSuffix(raw, ".git")
}
