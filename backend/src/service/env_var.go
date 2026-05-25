package service

import (
	"context"

	apperrors "launchs/shared/errors"
	"launchs/shared/model"

	"github.com/google/uuid"

	"backend/repository"
)

type envVarService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	envVarRepo    repository.EnvVarRepository
}

// NewEnvVarService は EnvVarService の実装を返します。
func NewEnvVarService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	envVarRepo repository.EnvVarRepository,
) EnvVarService {
	return &envVarService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		envVarRepo:    envVarRepo,
	}
}

func (s *envVarService) ListProject(ctx context.Context, userID string, projectID uuid.UUID) ([]model.ProjectEnvVar, error) {
	if err := s.checkProjectAccess(ctx, userID, projectID); err != nil {
		return nil, err
	}
	return s.envVarRepo.FindByProjectID(ctx, projectID)
}

func (s *envVarService) UpsertProject(ctx context.Context, userID string, projectID uuid.UUID, vars []EnvVarInput) error {
	if err := s.checkProjectAccess(ctx, userID, projectID); err != nil {
		return err
	}

	envVars := make([]model.ProjectEnvVar, len(vars))
	for i, v := range vars {
		envVars[i] = model.ProjectEnvVar{
			ID:        uuid.New(),
			ProjectID: projectID,
			Key:       v.Key,
			Value:     v.Value,
		}
	}
	return s.envVarRepo.UpsertProjectEnvVars(ctx, projectID, envVars)
}

func (s *envVarService) DeleteProject(ctx context.Context, userID string, projectID uuid.UUID, key string) error {
	if err := s.checkProjectAccess(ctx, userID, projectID); err != nil {
		return err
	}
	return s.envVarRepo.DeleteProjectEnvVar(ctx, projectID, key)
}

func (s *envVarService) ListContainer(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]model.ContainerEnvVar, error) {
	if err := s.checkContainerAccess(ctx, userID, projectID, containerID); err != nil {
		return nil, err
	}
	return s.envVarRepo.FindByContainerID(ctx, containerID)
}

func (s *envVarService) UpsertContainer(ctx context.Context, userID string, projectID, containerID uuid.UUID, vars []EnvVarInput) error {
	if err := s.checkContainerAccess(ctx, userID, projectID, containerID); err != nil {
		return err
	}

	envVars := make([]model.ContainerEnvVar, len(vars))
	for i, v := range vars {
		envVars[i] = model.ContainerEnvVar{
			ID:          uuid.New(),
			ContainerID: containerID,
			Key:         v.Key,
			Value:       v.Value,
		}
	}
	return s.envVarRepo.UpsertContainerEnvVars(ctx, containerID, envVars)
}

func (s *envVarService) DeleteContainer(ctx context.Context, userID string, projectID, containerID uuid.UUID, key string) error {
	if err := s.checkContainerAccess(ctx, userID, projectID, containerID); err != nil {
		return err
	}
	return s.envVarRepo.DeleteContainerEnvVar(ctx, containerID, key)
}

func (s *envVarService) GetSelectedProjectEnvVarKeys(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]string, error) {
	if err := s.checkContainerAccess(ctx, userID, projectID, containerID); err != nil {
		return nil, err
	}
	return s.envVarRepo.FindSelectedProjectEnvVarKeys(ctx, containerID)
}

func (s *envVarService) SetSelectedProjectEnvVarKeys(ctx context.Context, userID string, projectID, containerID uuid.UUID, keys []string) error {
	if err := s.checkContainerAccess(ctx, userID, projectID, containerID); err != nil {
		return err
	}
	return s.envVarRepo.SetSelectedProjectEnvVarKeys(ctx, containerID, keys)
}

func (s *envVarService) checkProjectAccess(ctx context.Context, userID string, projectID uuid.UUID) error {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}
	return nil
}

func (s *envVarService) checkContainerAccess(ctx context.Context, userID string, projectID, containerID uuid.UUID) error {
	if err := s.checkProjectAccess(ctx, userID, projectID); err != nil {
		return err
	}
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}
	return nil
}
