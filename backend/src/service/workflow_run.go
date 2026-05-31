package service

import (
	"context"

	"backend/repository"
	apperrors "launchs/shared/errors"
	"launchs/shared/model"

	"github.com/google/uuid"
)

type workflowRunService struct {
	projectRepo     repository.ProjectRepository
	workflowRunRepo repository.WorkflowRunRepository
}

func NewWorkflowRunService(
	projectRepo repository.ProjectRepository,
	workflowRunRepo repository.WorkflowRunRepository,
) WorkflowRunService {
	return &workflowRunService{
		projectRepo:     projectRepo,
		workflowRunRepo: workflowRunRepo,
	}
}

func (s *workflowRunService) List(ctx context.Context, userID string, projectID uuid.UUID, limit int) ([]model.WorkflowRun, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return s.workflowRunRepo.FindByProjectID(ctx, projectID, limit)
}

func (s *workflowRunService) GetEvents(ctx context.Context, userID string, projectID uuid.UUID, runID uuid.UUID) ([]model.WorkflowRunEvent, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return s.workflowRunRepo.FindEventsByRunID(ctx, runID)
}
