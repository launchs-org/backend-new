package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type buildJobRepository struct {
	db *gorm.DB
}

func NewBuildJobRepository(db *gorm.DB) BuildJobRepository {
	return &buildJobRepository{db: db}
}

func (r *buildJobRepository) Create(ctx context.Context, job *model.BuildJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *buildJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.BuildJob, error) {
	var job model.BuildJob
	if err := r.db.WithContext(ctx).First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *buildJobRepository) FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.BuildJob, error) {
	var jobs []model.BuildJob
	err := r.db.WithContext(ctx).
		Where("container_id = ?", containerID).
		Order("created_at DESC").
		Find(&jobs).Error
	return jobs, err
}

func (r *buildJobRepository) FindActiveByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.BuildJob, error) {
	var jobs []model.BuildJob
	err := r.db.WithContext(ctx).Model(&model.BuildJob{}).
		Where("container_id = ? AND status IN ?", containerID, []string{
			string(model.BuildJobStatusPending),
			string(model.BuildJobStatusRunning),
		}).
		Find(&jobs).Error
	return jobs, err
}

func (r *buildJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).Model(&model.BuildJob{}).Where("id = ?", id).Update("status", status).Error
}

func (r *buildJobRepository) UpdateWorkflowID(ctx context.Context, id uuid.UUID, workflowID string) error {
	return r.db.WithContext(ctx).Model(&model.BuildJob{}).Where("id = ?", id).Update("temporal_workflow_id", workflowID).Error
}

func (r *buildJobRepository) UpdateFinishedAt(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.BuildJob{}).Where("id = ?", id).Update("finished_at", &now).Error
}

func (r *buildJobRepository) UpdateImageID(ctx context.Context, id uuid.UUID, imageID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.BuildJob{}).Where("id = ?", id).Update("image_id", imageID).Error
}

func (r *buildJobRepository) DeleteByContainerID(ctx context.Context, containerID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("container_id = ?", containerID).Delete(&model.BuildJob{}).Error
}
