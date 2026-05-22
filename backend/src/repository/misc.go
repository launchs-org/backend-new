package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

// --- ContainerStatusHistory ---

type containerStatusHistoryRepository struct {
	db *gorm.DB
}

func NewContainerStatusHistoryRepository(db *gorm.DB) ContainerStatusHistoryRepository {
	return &containerStatusHistoryRepository{db: db}
}

func (r *containerStatusHistoryRepository) Insert(ctx context.Context, history *model.ContainerStatusHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

func (r *containerStatusHistoryRepository) FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.ContainerStatusHistory, error) {
	var histories []model.ContainerStatusHistory
	err := r.db.WithContext(ctx).
		Where("container_id = ?", containerID).
		Order("created_at DESC").
		Find(&histories).Error
	return histories, err
}

// DeleteOldRecords は最新 keepCount 件を超える古いレコードを削除します。
// サブクエリで残すべき ID を特定してから削除します。
func (r *containerStatusHistoryRepository) DeleteOldRecords(ctx context.Context, containerID uuid.UUID, keepCount int) error {
	subQuery := r.db.Model(&model.ContainerStatusHistory{}).
		Select("id").
		Where("container_id = ?", containerID).
		Order("created_at DESC").
		Limit(keepCount)

	return r.db.WithContext(ctx).
		Where("container_id = ? AND id NOT IN (?)", containerID, subQuery).
		Delete(&model.ContainerStatusHistory{}).Error
}

// --- PodStatus ---

type podStatusRepository struct {
	db *gorm.DB
}

func NewPodStatusRepository(db *gorm.DB) PodStatusRepository {
	return &podStatusRepository{db: db}
}

func (r *podStatusRepository) Upsert(ctx context.Context, pod *model.PodStatus) error {
	return r.db.WithContext(ctx).Save(pod).Error
}

func (r *podStatusRepository) DeleteByPodName(ctx context.Context, podName string) error {
	return r.db.WithContext(ctx).Where("pod_name = ?", podName).Delete(&model.PodStatus{}).Error
}

func (r *podStatusRepository) FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.PodStatus, error) {
	var pods []model.PodStatus
	err := r.db.WithContext(ctx).Where("container_id = ?", containerID).Find(&pods).Error
	return pods, err
}

// --- ServiceConnection ---

type serviceConnectionRepository struct {
	db *gorm.DB
}

func NewServiceConnectionRepository(db *gorm.DB) ServiceConnectionRepository {
	return &serviceConnectionRepository{db: db}
}

func (r *serviceConnectionRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.ServiceConnection, error) {
	var conns []model.ServiceConnection
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&conns).Error
	return conns, err
}

// --- Image ---

type imageRepository struct {
	db *gorm.DB
}

func NewImageRepository(db *gorm.DB) ImageRepository {
	return &imageRepository{db: db}
}

func (r *imageRepository) Create(ctx context.Context, image *model.Image) error {
	return r.db.WithContext(ctx).Create(image).Error
}

func (r *imageRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Image, error) {
	var image model.Image
	if err := r.db.WithContext(ctx).First(&image, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &image, nil
}

func (r *imageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).Model(&model.Image{}).Where("id = ?", id).Update("status", status).Error
}

// --- Deployment ---

type deploymentRepository struct {
	db *gorm.DB
}

func NewDeploymentRepository(db *gorm.DB) DeploymentRepository {
	return &deploymentRepository{db: db}
}

func (r *deploymentRepository) Create(ctx context.Context, deployment *model.Deployment) error {
	return r.db.WithContext(ctx).Create(deployment).Error
}

func (r *deploymentRepository) FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.Deployment, error) {
	var deployments []model.Deployment
	err := r.db.WithContext(ctx).
		Where("container_id = ?", containerID).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}
