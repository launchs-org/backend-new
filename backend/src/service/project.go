package service

import (
	"context"
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

// CreateProjectInput は CreateProjectWorkflow の入力です。
type CreateProjectInput struct {
	ProjectID string `json:"project_id"`
	Namespace string `json:"namespace"`
}

// DeleteProjectInput は DeleteProjectWorkflow の入力です。
type DeleteProjectInput struct {
	ProjectID         string `json:"project_id"`
	Namespace         string `json:"namespace"`
	HarborProjectName string `json:"harbor_project_name"`
}

// DeployProjectInput は DeployProjectWorkflow の入力です。
type DeployProjectInput struct {
	ProjectID string `json:"project_id"`
}

type projectService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	snapshotRepo  repository.SnapshotRepository
	temporal      client.Client
}

// NewProjectService は ProjectService の実装を返します。
func NewProjectService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	snapshotRepo repository.SnapshotRepository,
	temporalClient client.Client,
) ProjectService {
	return &projectService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		snapshotRepo:  snapshotRepo,
		temporal:      temporalClient,
	}
}

func (s *projectService) Create(ctx context.Context, userID, name string) (*model.Project, string, error) {
	id := uuid.New()
	slug := slugify(name)
	namespace := fmt.Sprintf("project-%s", id.String())

	project := &model.Project{
		ID:        id,
		UserID:    userID,
		Name:      name,
		Slug:      slug,
		Namespace: namespace,
	}

	if err := s.projectRepo.Create(ctx, project); err != nil {
		return nil, "", fmt.Errorf("failed to create project: %w", err)
	}

	// Temporal ワークフローで Namespace と Harbor プロジェクトを作成します
	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-project-%s", id.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowCreateProject, CreateProjectInput{
		ProjectID: id.String(),
		Namespace: namespace,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start CreateProjectWorkflow: %w", err)
	}

	return project, we.GetID(), nil
}

func (s *projectService) List(ctx context.Context, userID string) ([]model.Project, error) {
	return s.projectRepo.FindByUserID(ctx, userID)
}

func (s *projectService) Get(ctx context.Context, userID string, id uuid.UUID) (*model.Project, error) {
	project, err := s.projectRepo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: id.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return project, nil
}

func (s *projectService) Delete(ctx context.Context, userID string, id uuid.UUID) (string, error) {
	project, err := s.projectRepo.FindByID(ctx, id)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: id.String()}
	}
	if project.UserID != userID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	if err := s.projectRepo.Delete(ctx, id); err != nil {
		return "", fmt.Errorf("failed to delete project: %w", err)
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("delete-project-%s-%d", id.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeleteProject, DeleteProjectInput{
		ProjectID:         id.String(),
		Namespace:         project.Namespace,
		HarborProjectName: project.HarborProjectName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start DeleteProjectWorkflow: %w", err)
	}

	return we.GetID(), nil
}

func (s *projectService) DeployAll(ctx context.Context, userID string, projectID uuid.UUID) (string, []string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", nil, &apperrors.ForbiddenError{Message: "access denied"}
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("deploy-project-%s-%d", projectID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeployProject, DeployProjectInput{
		ProjectID: projectID.String(),
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to start DeployProjectWorkflow: %w", err)
	}

	return we.GetID(), []string{we.GetID()}, nil
}

// slugify は name を URL フレンドリーな小文字スラッグに変換します。
func slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	return slug
}
