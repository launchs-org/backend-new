package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"launchs/shared/model"
)

type envVarRepository struct {
	db *gorm.DB
}

func NewEnvVarRepository(db *gorm.DB) EnvVarRepository {
	return &envVarRepository{db: db}
}

func (r *envVarRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.ProjectEnvVar, error) {
	var vars []model.ProjectEnvVar
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&vars).Error
	return vars, err
}

func (r *envVarRepository) UpsertProjectEnvVars(ctx context.Context, projectID uuid.UUID, envVars []model.ProjectEnvVar) error {
	if len(envVars) == 0 {
		return nil
	}
	// key が重複した場合は value を更新します
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).
		Create(&envVars).Error
}

func (r *envVarRepository) DeleteProjectEnvVar(ctx context.Context, projectID uuid.UUID, key string) error {
	return r.db.WithContext(ctx).
		Where("project_id = ? AND key = ?", projectID, key).
		Delete(&model.ProjectEnvVar{}).Error
}

func (r *envVarRepository) FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.ContainerEnvVar, error) {
	var vars []model.ContainerEnvVar
	err := r.db.WithContext(ctx).Where("container_id = ?", containerID).Find(&vars).Error
	return vars, err
}

func (r *envVarRepository) UpsertContainerEnvVars(ctx context.Context, containerID uuid.UUID, envVars []model.ContainerEnvVar) error {
	if len(envVars) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "container_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).
		Create(&envVars).Error
}

func (r *envVarRepository) DeleteContainerEnvVar(ctx context.Context, containerID uuid.UUID, key string) error {
	return r.db.WithContext(ctx).
		Where("container_id = ? AND key = ?", containerID, key).
		Delete(&model.ContainerEnvVar{}).Error
}
