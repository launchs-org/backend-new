package activity

import (
	"context"
	"fmt"

	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
)

// DBUpsertWorkflowRun は WorkflowRun レコードを作成または更新します。
func (a *BuildActivity) DBUpsertWorkflowRun(
	ctx context.Context,
	projectID uuid.UUID,
	workflowID string,
	workflowType string,
	status string,
	containerID *uuid.UUID,
	label *string,
	log *string,
) error {
	db := database.DB.WithContext(ctx)

	var run model.WorkflowRun
	err := db.Where("workflow_id = ?", workflowID).First(&run).Error
	if err != nil {
		run = model.WorkflowRun{
			ID:           uuid.New(),
			ProjectID:    projectID,
			WorkflowID:   workflowID,
			WorkflowType: model.WorkflowRunType(workflowType),
			Status:       model.WorkflowRunStatus(status),
			ContainerID:  containerID,
			Label:        label,
			Log:          log,
		}
		if createErr := db.Create(&run).Error; createErr != nil {
			return fmt.Errorf("WorkflowRun 作成エラー: %w", createErr)
		}
		event := model.WorkflowRunEvent{
			ID:            uuid.New(),
			WorkflowRunID: run.ID,
			Status:        model.WorkflowRunStatus(status),
			Message:       nil,
		}
		_ = db.Create(&event).Error
		return nil
	}

	updates := map[string]interface{}{
		"status": status,
	}
	if log != nil {
		updates["log"] = log
	}
	if err := db.Model(&run).Updates(updates).Error; err != nil {
		return fmt.Errorf("WorkflowRun 更新エラー: %w", err)
	}
	event := model.WorkflowRunEvent{
		ID:            uuid.New(),
		WorkflowRunID: run.ID,
		Status:        model.WorkflowRunStatus(status),
		Message:       log,
	}
	_ = db.Create(&event).Error
	return nil
}
