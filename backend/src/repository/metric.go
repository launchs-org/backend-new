package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type metricRepository struct {
	db *gorm.DB
}

func NewMetricRepository(db *gorm.DB) MetricRepository {
	return &metricRepository{db: db}
}

func (r *metricRepository) BulkInsert(ctx context.Context, metrics []model.ContainerMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(metrics, 500).Error
}

func (r *metricRepository) FindByRange(ctx context.Context, containerID uuid.UUID, from, to time.Time) ([]model.ContainerMetric, error) {
	var metrics []model.ContainerMetric
	err := r.db.WithContext(ctx).
		Where("container_id = ? AND timestamp BETWEEN ? AND ?", containerID, from, to).
		Order("timestamp ASC").
		Find(&metrics).Error
	return metrics, err
}

func (r *metricRepository) DeleteOlderThan(ctx context.Context, t time.Time) error {
	return r.db.WithContext(ctx).Where("timestamp < ?", t).Delete(&model.ContainerMetric{}).Error
}
