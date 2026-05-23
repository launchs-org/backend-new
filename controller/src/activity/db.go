package activity

import (
	"context"
	"fmt"

	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
)

// DBActivity はコントローラーが行う DB 更新操作を担当します。
type DBActivity struct{}

// UpdateContainerStatus はコンテナのステータスを更新します。
func (a *DBActivity) UpdateContainerStatus(ctx context.Context, containerID uuid.UUID, status string) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("コンテナステータス更新エラー containerID=%s: %w", containerID, result.Error)
	}
	return nil
}

// UpdateContainerImage はコンテナの現在イメージID を更新します。
func (a *DBActivity) UpdateContainerImage(ctx context.Context, containerID, imageID uuid.UUID) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("current_image_id", imageID)
	if result.Error != nil {
		return fmt.Errorf("コンテナイメージ更新エラー: %w", result.Error)
	}
	return nil
}

// UpdateContainerReplicas はコンテナのレプリカ数を更新します。
func (a *DBActivity) UpdateContainerReplicas(ctx context.Context, containerID uuid.UUID, replicas int) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("replicas", replicas)
	if result.Error != nil {
		return fmt.Errorf("コンテナレプリカ数更新エラー: %w", result.Error)
	}
	return nil
}

// UpdateProjectHarbor はプロジェクトの Harbor 認証情報を更新します。
func (a *DBActivity) UpdateProjectHarbor(ctx context.Context, projectID uuid.UUID, projectName, username, password string) error {
	updates := map[string]interface{}{
		"harbor_project_name":   projectName,
		"harbor_robot_username": username,
		"harbor_robot_password": password,
	}
	result := database.DB.WithContext(ctx).Model(&model.Project{}).
		Where("id = ?", projectID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("プロジェクト Harbor 情報更新エラー: %w", result.Error)
	}
	return nil
}

// CreateDeploymentRecord は Deployment レコードを DB に作成します。
func (a *DBActivity) CreateDeploymentRecord(ctx context.Context, containerID uuid.UUID, imageRef string, replicas int) error {
	deployment := &model.Deployment{
		ID:          uuid.New(),
		ContainerID: containerID,
		WorkflowID:  imageRef, // ここでは imageRef を記録（WorkflowID として流用）
		Status:      "running",
	}
	result := database.DB.WithContext(ctx).Create(deployment)
	if result.Error != nil {
		return fmt.Errorf("Deployment レコード作成エラー: %w", result.Error)
	}
	return nil
}

// ClearContainerWorkflowID は完了したワークフローIDをクリアします。
func (a *DBActivity) ClearContainerWorkflowID(ctx context.Context, containerID uuid.UUID) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Updates(map[string]interface{}{
			"active_deploy_workflow_id": nil,
		})
	if result.Error != nil {
		return fmt.Errorf("ワークフローID クリアエラー: %w", result.Error)
	}
	return nil
}
