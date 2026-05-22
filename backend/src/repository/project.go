package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"launchs/shared/model"
)

type projectRepository struct {
	db *gorm.DB
}

// NewProjectRepository は ProjectRepository の実装を返します。
func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) Create(ctx context.Context, project *model.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *projectRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	var project model.Project
	if err := r.db.WithContext(ctx).First(&project, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *projectRepository) FindByUserID(ctx context.Context, userID string) ([]model.Project, error) {
	var projects []model.Project
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func (r *projectRepository) FindByIDWithDetails(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	var project model.Project
	err := r.db.WithContext(ctx).
		Preload("Containers").
		Preload("Containers.PodStatuses").
		Preload("Volumes").
		Preload("Volumes.Mounts").
		Preload("EnvVars").
		First(&project, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *projectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Project{}, "id = ?", id).Error
}

func (r *projectRepository) CountContainers(ctx context.Context, projectID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Container{}).Where("project_id = ?", projectID).Count(&count).Error
	return count, err
}
