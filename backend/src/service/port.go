package service

import (
	"context"
	"fmt"

	apperrors "launchs/shared/errors"
	"launchs/shared/model"
	"backend/repository"

	"github.com/google/uuid"
)

type portService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	portRepo      repository.PortRepository
}

// NewPortService は PortService の実装を返します。
func NewPortService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	portRepo repository.PortRepository,
) PortService {
	return &portService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		portRepo:      portRepo,
	}
}

func (s *portService) List(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]model.Port, error) {
	if err := s.checkAccess(ctx, userID, projectID, containerID); err != nil {
		return nil, err
	}
	return s.portRepo.FindByContainerID(ctx, containerID)
}

func (s *portService) Create(ctx context.Context, userID string, projectID, containerID uuid.UUID, port int, protocol string) (*model.Port, error) {
	if err := s.checkAccess(ctx, userID, projectID, containerID); err != nil {
		return nil, err
	}

	p := &model.Port{
		ID:          uuid.New(),
		ContainerID: containerID,
		Port:        port,
		Protocol:    protocol,
	}
	if err := s.portRepo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to create port: %w", err)
	}
	return p, nil
}

func (s *portService) Delete(ctx context.Context, userID string, projectID, containerID, portID uuid.UUID) error {
	if err := s.checkAccess(ctx, userID, projectID, containerID); err != nil {
		return err
	}
	return s.portRepo.Delete(ctx, portID)
}

func (s *portService) checkAccess(ctx context.Context, userID string, projectID, containerID uuid.UUID) error {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return &apperrors.ForbiddenError{Message: "access denied"}
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
