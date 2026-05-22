package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type networkRouteRepository struct {
	db *gorm.DB
}

func NewNetworkRouteRepository(db *gorm.DB) NetworkRouteRepository {
	return &networkRouteRepository{db: db}
}

func (r *networkRouteRepository) Create(ctx context.Context, route *model.NetworkRoute) error {
	return r.db.WithContext(ctx).Create(route).Error
}

func (r *networkRouteRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.NetworkRoute, error) {
	var route model.NetworkRoute
	if err := r.db.WithContext(ctx).First(&route, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *networkRouteRepository) FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.NetworkRoute, error) {
	var routes []model.NetworkRoute
	err := r.db.WithContext(ctx).Where("container_id = ?", containerID).Find(&routes).Error
	return routes, err
}

func (r *networkRouteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.NetworkRoute{}, "id = ?", id).Error
}
