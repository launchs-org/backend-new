package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
type BuildDeployWorkflowInput struct {
	ContainerID  string `json:"container_id"`
	ProjectID    string `json:"project_id"`
	GitRepo      string `json:"git_repo"`
	GitBranch    string `json:"git_branch"`
	GitCommit    string `json:"git_commit"`
	Subdir       string `json:"subdir"`
	ResourceSize string `json:"resource_size"`
	Replicas     int    `json:"replicas"`
}

// ScaleWorkflowInput は ScaleWorkflow の入力です。
type ScaleWorkflowInput struct {
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	Replicas    int    `json:"replicas"`
}

// RedeployWorkflowInput は RedeployWorkflow の入力です。
type RedeployWorkflowInput struct {
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
}

// DeleteContainerWorkflowInput は DeleteContainerWorkflow の入力です。
type DeleteContainerWorkflowInput struct {
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	Namespace   string `json:"namespace"`
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
	temporal      client.Client
}

// NewContainerService は ContainerService の実装を返します。
func NewContainerService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	envVarRepo repository.EnvVarRepository,
	portRepo repository.PortRepository,
	temporalClient client.Client,
) ContainerService {
	return &containerService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		envVarRepo:    envVarRepo,
		portRepo:      portRepo,
		temporal:      temporalClient,
	}
}

func (s *containerService) BuildDeploy(ctx context.Context, projectID uuid.UUID, req BuildDeployRequest) (string, string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	_ = project

	containerID := uuid.New()
	resourceSize := req.ResourceSize
	if resourceSize == "" {
		resourceSize = "small"
	}
	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	container := &model.Container{
		ID:           containerID,
		ProjectID:    projectID,
		Name:         req.Name,
		Status:       string(model.ContainerStatusPending),
		Replicas:     replicas,
		ResourceSize: resourceSize,
		GitRepo:      &req.GitRepo,
		GitBranch:    &req.GitBranch,
		GitSubdir:    strPtr(req.Subdir),
	}
	if err := s.containerRepo.Create(ctx, container); err != nil {
		return "", "", fmt.Errorf("failed to create container: %w", err)
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
			return "", "", fmt.Errorf("failed to save env vars: %w", err)
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
				return "", "", fmt.Errorf("failed to save port: %w", err)
			}
		}
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("build-deploy-%s", containerID.String()),
		TaskQueue: temporal.BuilderQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowBuildDeploy, BuildDeployWorkflowInput{
		ContainerID:  containerID.String(),
		ProjectID:    projectID.String(),
		GitRepo:      req.GitRepo,
		GitBranch:    req.GitBranch,
		GitCommit:    req.GitCommit,
		Subdir:       req.Subdir,
		ResourceSize: resourceSize,
		Replicas:     replicas,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to start BuildDeployWorkflow: %w", err)
	}

	// ワークフロー ID を DB に保存します
	workflowID := we.GetID()
	if err := s.containerRepo.UpdateActiveDeployWorkflowID(ctx, containerID, &workflowID); err != nil {
		return "", "", fmt.Errorf("failed to update workflow id: %w", err)
	}

	return containerID.String(), workflowID, nil
}

func (s *containerService) DeployFromTemplate(ctx context.Context, projectID uuid.UUID, req TemplateDeployRequest) (string, string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	_ = project

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
		return "", "", fmt.Errorf("failed to create container: %w", err)
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
		return "", "", fmt.Errorf("failed to start DeployWorkflow: %w", err)
	}

	workflowID := we.GetID()
	if err := s.containerRepo.UpdateActiveDeployWorkflowID(ctx, containerID, &workflowID); err != nil {
		return "", "", fmt.Errorf("failed to update workflow id: %w", err)
	}

	return containerID.String(), workflowID, nil
}

func (s *containerService) Scale(ctx context.Context, projectID, containerID uuid.UUID, replicas int) (string, error) {
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("scale-%s-%d", containerID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowScale, ScaleWorkflowInput{
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
		Replicas:    replicas,
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

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("redeploy-%s-%d", containerID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowRedeploy, RedeployWorkflowInput{
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
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

	if err := s.containerRepo.Delete(ctx, containerID); err != nil {
		return "", fmt.Errorf("failed to delete container: %w", err)
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("delete-container-%s-%d", containerID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeleteContainer, DeleteContainerWorkflowInput{
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
		Namespace:   project.Namespace,
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
	_, err = s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowRedeploy, RedeployWorkflowInput{
		ContainerID: container.ID.String(),
		ProjectID:   container.ProjectID.String(),
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
