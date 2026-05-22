package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type snapshotRepository struct {
	db *gorm.DB
}

func NewSnapshotRepository(db *gorm.DB) SnapshotRepository {
	return &snapshotRepository{db: db}
}

func (r *snapshotRepository) Create(ctx context.Context, snapshot *model.Snapshot) error {
	return r.db.WithContext(ctx).Create(snapshot).Error
}

func (r *snapshotRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Snapshot, error) {
	var snapshot model.Snapshot
	if err := r.db.WithContext(ctx).First(&snapshot, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *snapshotRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.Snapshot, error) {
	var snapshots []model.Snapshot
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&snapshots).Error
	return snapshots, err
}

func (r *snapshotRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Snapshot{}, "id = ?", id).Error
}

func (r *snapshotRepository) CountByProjectID(ctx context.Context, projectID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Snapshot{}).Where("project_id = ?", projectID).Count(&count).Error
	return count, err
}

func (r *snapshotRepository) FindOldestByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]model.Snapshot, error) {
	var snapshots []model.Snapshot
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at ASC").
		Limit(limit).
		Find(&snapshots).Error
	return snapshots, err
}
