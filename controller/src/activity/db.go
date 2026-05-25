package activity

import (
	"context"
	"fmt"

	"launchs/shared/config"
	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
)

// DBActivity はコントローラーが行う DB 更新操作を担当します。
type DBActivity struct{}

// DBUpdateContainerStatus はコンテナのステータスを更新します。
func (a *DBActivity) DBUpdateContainerStatus(ctx context.Context, containerID uuid.UUID, status string) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("コンテナステータス更新エラー containerID=%s: %w", containerID, result.Error)
	}
	return nil
}

// DBUpdateContainerImage はコンテナの現在イメージID を更新します。
func (a *DBActivity) DBUpdateContainerImage(ctx context.Context, containerID, imageID uuid.UUID) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("current_image_id", imageID)
	if result.Error != nil {
		return fmt.Errorf("コンテナイメージ更新エラー: %w", result.Error)
	}
	return nil
}

// DBUpdateContainerReplicas はコンテナのレプリカ数を更新します。
func (a *DBActivity) DBUpdateContainerReplicas(ctx context.Context, containerID uuid.UUID, replicas int) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("replicas", replicas)
	if result.Error != nil {
		return fmt.Errorf("コンテナレプリカ数更新エラー: %w", result.Error)
	}
	return nil
}

// DBUpdateProjectHarbor はプロジェクトの Harbor 認証情報を更新します。
func (a *DBActivity) DBUpdateProjectHarbor(ctx context.Context, projectID uuid.UUID, projectName, username, password string) error {
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

// DBCreateDeploymentRecord は Deployment レコードを DB に作成します。
func (a *DBActivity) DBCreateDeploymentRecord(ctx context.Context, containerID uuid.UUID, imageRef string, replicas int) error {
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

// DBClearContainerWorkflowID は完了したワークフローIDをクリアします。
func (a *DBActivity) DBClearContainerWorkflowID(ctx context.Context, containerID uuid.UUID) error {
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

// DBBuildDeploySpec はコンテナIDから DB を参照して DeploymentSpec を組み立てます。
func (a *DBActivity) DBBuildDeploySpec(ctx context.Context, containerID uuid.UUID, namespace string) (DeploymentSpec, error) {
	var container model.Container
	result := database.DB.WithContext(ctx).
		Preload("EnvVars").
		Preload("Ports").
		First(&container, "id = ?", containerID)
	if result.Error != nil {
		return DeploymentSpec{}, fmt.Errorf("コンテナ取得エラー: %w", result.Error)
	}

	// 現在のボリュームマウントを取得
	var mounts []model.VolumeMount
	database.DB.WithContext(ctx).
		Where("container_id = ?", containerID).
		Find(&mounts)

	// 現在のイメージ参照を取得
	imageRef := ""
	if container.CurrentImageID != nil {
		var img model.Image
		if err := database.DB.WithContext(ctx).First(&img, "id = ?", container.CurrentImageID).Error; err == nil {
			imageRef = img.ImageRef
		}
	}

	// ResourceSize からリソース設定を解決
	sizes := config.ResourceSizes()
	size, ok := sizes[container.ResourceSize]
	if !ok {
		size = sizes["small"]
	}

	envVars := make([]EnvVar, 0, len(container.EnvVars))
	for _, e := range container.EnvVars {
		envVars = append(envVars, EnvVar{Key: e.Key, Value: e.Value})
	}

	ports := make([]Port, 0, len(container.Ports))
	for _, p := range container.Ports {
		ports = append(ports, Port{Port: p.Port, Protocol: p.Protocol})
	}

	// マウントされている各ボリューム名を取得して PVC 名を組み立てる
	volumeMounts := make([]VolumeMount, 0)
	for _, m := range mounts {
		var vol model.Volume
		if err := database.DB.WithContext(ctx).First(&vol, "id = ?", m.VolumeID).Error; err != nil {
			continue
		}
		pvcName := fmt.Sprintf("%s-%s", vol.Name, m.VolumeID.String())
		volumeMounts = append(volumeMounts, VolumeMount{PVCName: pvcName, MountPath: m.MountPath})
	}

	return DeploymentSpec{
		Namespace:     namespace,
		Name:          fmt.Sprintf("%s-%s", container.Name, containerID.String()),
		Image:         imageRef,
		Replicas:      container.Replicas,
		CPURequest:    size.CPURequest,
		CPULimit:      size.CPULimit,
		MemoryRequest: size.MemoryRequest,
		MemoryLimit:   size.MemoryLimit,
		EnvVars:       envVars,
		Ports:         ports,
		VolumeMounts:  volumeMounts,
		Labels: map[string]string{
			"launchs-managed": "true",
			"container-id":    containerID.String(),
		},
	}, nil
}

// プロジェクトを削除するアクティビティ
func (a *DBActivity) DBDeleteProject(ctx context.Context, projectID string) error {
	// 関連するコンテナを削除
	result := database.DB.WithContext(ctx).Delete(&model.Container{}, "project_id = ?", projectID)
	if result.Error != nil {
		return fmt.Errorf("コンテナ削除エラー: %w", result.Error)
	}

	// 関連するボリュームを削除
	result = database.DB.WithContext(ctx).Delete(&model.Project{}, "id = ?", projectID)
	if result.Error != nil {
		return fmt.Errorf("プロジェクト削除エラー: %w", result.Error)
	}
	return nil
}

// プロジェクトの状態を更新するアクティビティ
func (a *DBActivity) DBUpdateProjectStatus(ctx context.Context, projectID string, status string) error {
	result := database.DB.WithContext(ctx).Model(&model.Project{}).
		Where("id = ?", projectID).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("プロジェクト状態更新エラー: %w", result.Error)
	}
	return nil
}

// ボリュームのステータスを更新する関数
func (a *DBActivity) DBUpdateVolumeStatus(ctx context.Context, volumeID uuid.UUID, status string) error {
	result := database.DB.WithContext(ctx).Model(&model.Volume{}).
		Where("id = ?", volumeID).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("ボリュームステータス更新エラー: %w", result.Error)
	}
	return nil
}
