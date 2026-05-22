package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type logRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) LogRepository {
	return &logRepository{db: db}
}

func (r *logRepository) BulkInsert(ctx context.Context, logs []model.ContainerLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(logs, 500).Error
}

func (r *logRepository) FindAfterCursor(ctx context.Context, containerID uuid.UUID, cursor time.Time, limit int, podName string) ([]model.ContainerLog, error) {
	var logs []model.ContainerLog
	q := r.db.WithContext(ctx).
		Where("container_id = ? AND timestamp > ? AND (pod_name IS NULL OR pod_name NOT LIKE 'build:%')", containerID, cursor).
		Order("timestamp ASC").
		Limit(limit)

	if podName != "" {
		q = q.Where("pod_name = ?", podName)
	}

	return logs, q.Find(&logs).Error
}

func (r *logRepository) FindBuildJobLogs(ctx context.Context, buildJobID uuid.UUID, cursor time.Time, limit int) ([]model.ContainerLog, error) {
	var logs []model.ContainerLog
	// ビルドジョブログは pod_name = "build:{build_job_id}" で識別します
	podName := fmt.Sprintf("build:%s", buildJobID.String())
	err := r.db.WithContext(ctx).
		Where("pod_name = ? AND timestamp > ?", podName, cursor).
		Order("timestamp ASC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
