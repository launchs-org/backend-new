package service

import (
	"context"

	"backend/repository"
	apperrors "launchs/shared/errors"
	"launchs/shared/config"
	"launchs/shared/model"

	"github.com/google/uuid"
)

// QuotaInfo はユーザーのクォータ情報（使用量と上限）です。
type QuotaInfo struct {
	Usage  map[string]int `json:"usage"`
	Limits map[string]int `json:"limits"`
}

// QuotaService はユーザーごとのリソースクォータを管理します。
type QuotaService interface {
	GetQuota(ctx context.Context, userID string) (*QuotaInfo, error)
	CheckQuota(ctx context.Context, userID, resourceSize string) error
	// CheckQuotaForUpdate はサイズ変更時にチェックします。旧サイズ分は除外して計算します。
	CheckQuotaForUpdate(ctx context.Context, userID, oldSize, newSize string) error
	CheckStorageQuota(ctx context.Context, userID string, requestedMB int) error
	SetQuota(ctx context.Context, userID string, maxSmall, maxMedium, maxLarge, maxStorageMB int) error
}

type quotaService struct {
	quotaRepo     repository.UserQuotaRepository
	containerRepo repository.ContainerRepository
	volumeRepo    repository.VolumeRepository
}

func NewQuotaService(quotaRepo repository.UserQuotaRepository, containerRepo repository.ContainerRepository, volumeRepo repository.VolumeRepository) QuotaService {
	return &quotaService{quotaRepo: quotaRepo, containerRepo: containerRepo, volumeRepo: volumeRepo}
}

func (s *quotaService) getLimits(ctx context.Context, userID string) (map[string]int, error) {
	quota, err := s.quotaRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return map[string]int{
			"small":      config.DefaultQuotaSmall(),
			"medium":     config.DefaultQuotaMedium(),
			"large":      config.DefaultQuotaLarge(),
			"storage_mb": config.DefaultQuotaStorageMB(),
		}, nil
	}
	return map[string]int{
		"small":      quota.MaxSmall,
		"medium":     quota.MaxMedium,
		"large":      quota.MaxLarge,
		"storage_mb": quota.MaxStorageMB,
	}, nil
}

func (s *quotaService) GetQuota(ctx context.Context, userID string) (*QuotaInfo, error) {
	limits, err := s.getLimits(ctx, userID)
	if err != nil {
		return nil, err
	}
	usage, err := s.containerRepo.CountByUserIDPerResourceSize(ctx, userID)
	if err != nil {
		return nil, err
	}
	usedStorageMB, err := s.volumeRepo.SumSizeMBByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	usage["storage_mb"] = usedStorageMB
	return &QuotaInfo{Usage: usage, Limits: limits}, nil
}

func (s *quotaService) CheckQuota(ctx context.Context, userID, resourceSize string) error {
	limits, err := s.getLimits(ctx, userID)
	if err != nil {
		return err
	}
	usage, err := s.containerRepo.CountByUserIDPerResourceSize(ctx, userID)
	if err != nil {
		return err
	}
	max, ok := limits[resourceSize]
	if !ok {
		return nil
	}
	current := usage[resourceSize]
	if current >= max {
		return &apperrors.QuotaExceededError{ResourceSize: resourceSize, Current: current, Max: max}
	}
	return nil
}

func (s *quotaService) CheckQuotaForUpdate(ctx context.Context, userID, oldSize, newSize string) error {
	if oldSize == newSize {
		return nil
	}
	limits, err := s.getLimits(ctx, userID)
	if err != nil {
		return err
	}
	usage, err := s.containerRepo.CountByUserIDPerResourceSize(ctx, userID)
	if err != nil {
		return err
	}
	max, ok := limits[newSize]
	if !ok {
		return nil
	}
	// 旧サイズから新サイズへの変更なので、旧サイズの1コンテナ分はまだカウントされている
	// 新サイズの現在カウントと上限を比較（上限に達していれば超過）
	current := usage[newSize]
	if current >= max {
		return &apperrors.QuotaExceededError{ResourceSize: newSize, Current: current, Max: max}
	}
	return nil
}

func (s *quotaService) CheckStorageQuota(ctx context.Context, userID string, requestedMB int) error {
	limits, err := s.getLimits(ctx, userID)
	if err != nil {
		return err
	}
	usedMB, err := s.volumeRepo.SumSizeMBByUserID(ctx, userID)
	if err != nil {
		return err
	}
	maxMB := limits["storage_mb"]
	if usedMB+requestedMB > maxMB {
		return &apperrors.StorageQuotaExceededError{UsedMB: usedMB, RequestedMB: requestedMB, MaxMB: maxMB}
	}
	return nil
}

func (s *quotaService) SetQuota(ctx context.Context, userID string, maxSmall, maxMedium, maxLarge, maxStorageMB int) error {
	existing, err := s.quotaRepo.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if existing == nil {
		existing = &model.UserQuota{
			ID:     uuid.New(),
			UserID: userID,
		}
	}
	existing.MaxSmall = maxSmall
	existing.MaxMedium = maxMedium
	existing.MaxLarge = maxLarge
	existing.MaxStorageMB = maxStorageMB
	return s.quotaRepo.Upsert(ctx, existing)
}
