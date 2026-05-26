// Package collector は Kubernetes リソースの監視・収集を担当します。
package collector

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// StatusCollector は launchs-managed=true ラベルの付いた Pod をウォッチし、
// pod_statuses テーブルと containers テーブルを更新します。
type StatusCollector struct{}

// Run は Pod の Watch を開始します。ctx キャンセルで停止します。
// launchs-managed=true ラベルのある全 Pod を監視対象にします。
func (c *StatusCollector) Run(ctx context.Context) error {
	k8s := database.K8sClientset
	fmt.Println("[status-collector] starting pod watch...")

	// 全 Namespace を対象に watch （Namespace 横断監視）
	watcher, err := k8s.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
		LabelSelector: "launchs-managed=true",
	})
	if err != nil {
		return fmt.Errorf("Pod Watch 開始エラー: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				// チャンネルが閉じたら再起動
				return fmt.Errorf("Pod Watch チャンネルが閉じられました")
			}
			c.handleEvent(ctx, event)
		}
	}
}

// handleEvent は Pod イベントを処理します。
func (c *StatusCollector) handleEvent(ctx context.Context, event watch.Event) {
	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		return
	}

	// container-id ラベルから ContainerID を取得
	containerIDStr := pod.Labels["container-id"]
	if containerIDStr == "" {
		return
	}
	containerID, err := uuid.Parse(containerIDStr)
	if err != nil {
		return
	}

	switch event.Type {
	case watch.Added, watch.Modified:
		c.upsertPodStatus(ctx, pod, containerID)
		c.updateContainerReplicas(ctx, containerID)
		c.recordStatusHistory(ctx, containerID)
	case watch.Deleted:
		c.deletePodStatus(ctx, pod.Name)
		c.updateContainerReplicas(ctx, containerID)
	}
}

// upsertPodStatus は pod_statuses テーブルを UPSERT します。
func (c *StatusCollector) upsertPodStatus(ctx context.Context, pod *corev1.Pod, containerID uuid.UUID) {
	status := podPhaseToStatus(pod.Status.Phase)
	ready := isPodReady(pod)
	restartCount := getPodRestartCount(pod)

	now := time.Now()
	var startedAt *time.Time
	if pod.Status.StartTime != nil {
		t := pod.Status.StartTime.Time
		startedAt = &t
	}

	// 既存レコードを pod_name で検索し、あれば UPDATE、なければ INSERT
	var existing model.PodStatus
	err := database.DB.WithContext(ctx).
		Where("pod_name = ?", pod.Name).
		First(&existing).Error

	if err != nil {
		// 存在しない → INSERT
		database.DB.WithContext(ctx).Create(&model.PodStatus{
			ID:           uuid.New(),
			ContainerID:  containerID,
			PodName:      pod.Name,
			Status:       status,
			Ready:        ready,
			RestartCount: restartCount,
			NodeName:     pod.Spec.NodeName,
			StartedAt:    startedAt,
			UpdatedAt:    now,
		})
	} else {
		// 存在する → UPDATE
		database.DB.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"container_id":  containerID,
			"status":        status,
			"ready":         ready,
			"restart_count": restartCount,
			"node_name":     pod.Spec.NodeName,
			"started_at":    startedAt,
			"updated_at":    now,
		})
	}
}

// deletePodStatus は pod_statuses テーブルから削除します。
func (c *StatusCollector) deletePodStatus(ctx context.Context, podName string) {
	database.DB.WithContext(ctx).
		Where("pod_name = ?", podName).
		Delete(&model.PodStatus{})
}

// updateContainerReplicas は pod_statuses から集計して containers を更新します。
func (c *StatusCollector) updateContainerReplicas(ctx context.Context, containerID uuid.UUID) {
	var readyCount, failedCount, runningCount int64
	database.DB.WithContext(ctx).Model(&model.PodStatus{}).
		Where("container_id = ? AND ready = ?", containerID, true).
		Count(&readyCount)
	database.DB.WithContext(ctx).Model(&model.PodStatus{}).
		Where("container_id = ? AND status = ?", containerID, "failed").
		Count(&failedCount)
	database.DB.WithContext(ctx).Model(&model.PodStatus{}).
		Where("container_id = ? AND status = ?", containerID, "running").
		Count(&runningCount)

	updates := map[string]interface{}{
		"ready_replicas":  readyCount,
		"failed_replicas": failedCount,
	}

	// Pod の状態からコンテナステータスを導出する
	var container model.Container
	if err := database.DB.WithContext(ctx).Select("status", "replicas").Where("id = ?", containerID).First(&container).Error; err == nil {
		switch container.Status {
		case model.ContainerStatusBuilding, model.ContainerStatusDeploying, model.ContainerStatusScaling:
			// ワークフローが管理中のため上書きしない

		case model.ContainerStatusApplying:
			// K8s apply 完了後、Pod が desired replicas 分 Ready になったら running に遷移。
			// failed Pod がある場合は即 failed にする。
			// rollout restart 中は古い Pod がまだ running で残るため、
			// readyCount だけを判定基準にして applying を維持する。
			if failedCount > 0 && readyCount == 0 {
				updates["status"] = string(model.ContainerStatusFailed)
			} else if readyCount >= int64(container.Replicas) && container.Replicas > 0 {
				updates["status"] = string(model.ContainerStatusRunning)
			}
			// まだ Ready 数が足りない場合は applying のまま維持

		default:
			// running / pending / failed など通常の Pod ベース管理
			if failedCount > 0 && runningCount == 0 {
				updates["status"] = string(model.ContainerStatusFailed)
			} else if runningCount > 0 {
				updates["status"] = string(model.ContainerStatusRunning)
			} else {
				updates["status"] = string(model.ContainerStatusPending)
			}
		}
	}

	database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Updates(updates)
}

// recordStatusHistory はステータスが前回から変化した時だけ INSERT します。
// 1週間を超えた古いレコードは削除します。
func (c *StatusCollector) recordStatusHistory(ctx context.Context, containerID uuid.UUID) {
	var container model.Container
	if err := database.DB.WithContext(ctx).Where("id = ?", containerID).First(&container).Error; err != nil {
		return
	}

	// 直前のレコードと比較して変化がなければスキップ
	var latest model.ContainerStatusHistory
	err := database.DB.WithContext(ctx).
		Where("container_id = ?", containerID).
		Order("created_at DESC").
		First(&latest).Error
	if err == nil &&
		latest.Status == string(container.Status) &&
		latest.ReadyReplicas == container.ReadyReplicas &&
		latest.FailedReplicas == container.FailedReplicas {
		return
	}

	database.DB.WithContext(ctx).Create(&model.ContainerStatusHistory{
		ID:             uuid.New(),
		ContainerID:    containerID,
		Status:         string(container.Status),
		Replicas:       container.Replicas,
		ReadyReplicas:  container.ReadyReplicas,
		FailedReplicas: container.FailedReplicas,
		CreatedAt:      time.Now(),
	})

	// 1週間を超えた古いレコードを削除
	database.DB.WithContext(ctx).
		Where("container_id = ? AND created_at < ?", containerID, time.Now().AddDate(0, 0, -7)).
		Delete(&model.ContainerStatusHistory{})
}

// podPhaseToStatus は K8s Pod Phase をアプリのステータス文字列に変換します。
func podPhaseToStatus(phase corev1.PodPhase) string {
	switch phase {
	case corev1.PodRunning:
		return "running"
	case corev1.PodPending:
		return "pending"
	case corev1.PodFailed:
		return "failed"
	case corev1.PodSucceeded:
		return "stopped"
	default:
		return "unknown"
	}
}

// isPodReady は Pod の Ready condition が True かどうかを返します。
func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// getPodRestartCount は Pod の全コンテナのリスタート合計回数を返します。
func getPodRestartCount(pod *corev1.Pod) int {
	total := 0
	for _, cs := range pod.Status.ContainerStatuses {
		total += int(cs.RestartCount)
	}
	return total
}
