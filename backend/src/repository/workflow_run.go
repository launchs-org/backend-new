package repository

import (
	"context"

	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
)

type workflowRunRepository struct{}

func NewWorkflowRunRepository() WorkflowRunRepository {
	return &workflowRunRepository{}
}

func (r *workflowRunRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]model.WorkflowRun, error) {
	if limit <= 0 {
		limit = 50
	}
	var runs []model.WorkflowRun
	err := database.DB.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Limit(limit).
		Find(&runs).Error
	return runs, err
}

func (r *workflowRunRepository) FindEventsByRunID(ctx context.Context, runID uuid.UUID) ([]model.WorkflowRunEvent, error) {
	var events []model.WorkflowRunEvent
	err := database.DB.WithContext(ctx).
		Where("workflow_run_id = ?", runID).
		Order("created_at ASC").
		Find(&events).Error
	return events, err
}
