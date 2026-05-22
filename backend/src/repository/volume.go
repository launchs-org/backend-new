package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type volumeRepository struct {
	db *gorm.DB
}

func NewVolumeRepository(db *gorm.DB) VolumeRepository {
	return &volumeRepository{db: db}
}

func (r *volumeRepository) Create(ctx context.Context, volume *model.Volume) error {
	return r.db.WithContext(ctx).Create(volume).Error
}

func (r *volumeRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Volume, error) {
	var volume model.Volume
	if err := r.db.WithContext(ctx).Preload("Mounts").First(&volume, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &volume, nil
}

func (r *volumeRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.Volume, error) {
	var volumes []model.Volume
	err := r.db.WithContext(ctx).Preload("Mounts").Where("project_id = ?", projectID).Find(&volumes).Error
	return volumes, err
}

func (r *volumeRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).Model(&model.Volume{}).Where("id = ?", id).Update("status", status).Error
}

func (r *volumeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Volume{}, "id = ?", id).Error
}

func (r *volumeRepository) CreateMount(ctx context.Context, mount *model.VolumeMount) error {
	return r.db.WithContext(ctx).Create(mount).Error
}

func (r *volumeRepository) DeleteMount(ctx context.Context, volumeID, containerID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("volume_id = ? AND container_id = ?", volumeID, containerID).Delete(&model.VolumeMount{}).Error
}

func (r *volumeRepository) FindMountsByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.VolumeMount, error) {
	var mounts []model.VolumeMount
	err := r.db.WithContext(ctx).Where("container_id = ?", containerID).Find(&mounts).Error
	return mounts, err
}

func (r *volumeRepository) FindMountsByVolumeID(ctx context.Context, volumeID uuid.UUID) ([]model.VolumeMount, error) {
	var mounts []model.VolumeMount
	err := r.db.WithContext(ctx).Where("volume_id = ?", volumeID).Find(&mounts).Error
	return mounts, err
}
