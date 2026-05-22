package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type portRepository struct {
	db *gorm.DB
}

func NewPortRepository(db *gorm.DB) PortRepository {
	return &portRepository{db: db}
}

func (r *portRepository) Create(ctx context.Context, port *model.Port) error {
	return r.db.WithContext(ctx).Create(port).Error
}

func (r *portRepository) FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.Port, error) {
	var ports []model.Port
	err := r.db.WithContext(ctx).Where("container_id = ?", containerID).Find(&ports).Error
	return ports, err
}

func (r *portRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Port{}, "id = ?", id).Error
}
