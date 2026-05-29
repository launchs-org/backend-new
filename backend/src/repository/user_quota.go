package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type userQuotaRepository struct {
	db *gorm.DB
}

func NewUserQuotaRepository(db *gorm.DB) UserQuotaRepository {
	return &userQuotaRepository{db: db}
}

func (r *userQuotaRepository) FindByUserID(ctx context.Context, userID string) (*model.UserQuota, error) {
	var quota model.UserQuota
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&quota).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &quota, nil
}

func (r *userQuotaRepository) Upsert(ctx context.Context, quota *model.UserQuota) error {
	if quota.ID == uuid.Nil {
		quota.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Save(quota).Error
}
