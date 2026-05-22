package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type containerRepository struct {
	db *gorm.DB
}

func NewContainerRepository(db *gorm.DB) ContainerRepository {
	return &containerRepository{db: db}
}

func (r *containerRepository) Create(ctx context.Context, container *model.Container) error {
	return r.db.WithContext(ctx).Create(container).Error
}

func (r *containerRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Container, error) {
	var container model.Container
	if err := r.db.WithContext(ctx).First(&container, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &container, nil
}

func (r *containerRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.Container, error) {
	var containers []model.Container
	err := r.db.WithContext(ctx).
		Preload("PodStatuses").
		Where("project_id = ?", projectID).
		Find(&containers).Error
	return containers, err
}

func (r *containerRepository) FindByIDWithDetails(ctx context.Context, id uuid.UUID) (*model.Container, error) {
	var container model.Container
	err := r.db.WithContext(ctx).
		Preload("EnvVars").
		Preload("Ports").
		Preload("Routes").
		Preload("PodStatuses").
		First(&container, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &container, nil
}

func (r *containerRepository) FindByWebhookToken(ctx context.Context, token string) (*model.Container, error) {
	var container model.Container
	if err := r.db.WithContext(ctx).Where("webhook_token = ?", token).First(&container).Error; err != nil {
		return nil, err
	}
	return &container, nil
}

func (r *containerRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).Model(&model.Container{}).Where("id = ?", id).Update("status", status).Error
}

func (r *containerRepository) UpdateReplicas(ctx context.Context, id uuid.UUID, ready, failed int) error {
	return r.db.WithContext(ctx).Model(&model.Container{}).Where("id = ?", id).Updates(map[string]interface{}{
		"ready_replicas":  ready,
		"failed_replicas": failed,
	}).Error
}

func (r *containerRepository) UpdateActiveDeployWorkflowID(ctx context.Context, id uuid.UUID, workflowID *string) error {
	return r.db.WithContext(ctx).Model(&model.Container{}).Where("id = ?", id).Update("active_deploy_workflow_id", workflowID).Error
}

func (r *containerRepository) UpdateActiveScaleWorkflowID(ctx context.Context, id uuid.UUID, workflowID *string) error {
	return r.db.WithContext(ctx).Model(&model.Container{}).Where("id = ?", id).Update("active_scale_workflow_id", workflowID).Error
}

func (r *containerRepository) UpdateCurrentImageID(ctx context.Context, id uuid.UUID, imageID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.Container{}).Where("id = ?", id).Update("current_image_id", imageID).Error
}

func (r *containerRepository) UpdateWebhookToken(ctx context.Context, id uuid.UUID, token *string) error {
	return r.db.WithContext(ctx).Model(&model.Container{}).Where("id = ?", id).Update("webhook_token", token).Error
}

func (r *containerRepository) Update(ctx context.Context, container *model.Container) error {
	return r.db.WithContext(ctx).Save(container).Error
}

func (r *containerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Container{}, "id = ?", id).Error
}
