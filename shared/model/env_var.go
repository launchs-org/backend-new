package model

import (
	"github.com/google/uuid"
)

// ProjectEnvVar はプロジェクト共通の環境変数です。
// コンテナデプロイ時にコンテナ固有の環境変数とマージされます（コンテナ優先）。
type ProjectEnvVar struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_project_env_var_project_key"`
	Key       string    `gorm:"not null;uniqueIndex:idx_project_env_var_project_key"`
	Value     string    `gorm:"not null"`
}

// ContainerEnvVar はコンテナ固有の環境変数です。
// 同名のキーがある場合、ProjectEnvVar より優先されます。
type ContainerEnvVar struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_container_env_var_container_key"`
	Key         string    `gorm:"not null;uniqueIndex:idx_container_env_var_container_key"`
	Value       string    `gorm:"not null"`
}

// ContainerSelectedProjectEnvVar はコンテナが選択したプロジェクト環境変数のキーを管理します。
// デプロイ時に選択済みのプロジェクト変数のみコンテナに注入されます。
type ContainerSelectedProjectEnvVar struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_container_selected_env_var"`
	Key         string    `gorm:"not null;uniqueIndex:idx_container_selected_env_var"`
}
