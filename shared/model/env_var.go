package model

import (
	"github.com/google/uuid"
)

// ProjectEnvVar はプロジェクト共通の環境変数です。
// コンテナデプロイ時にコンテナ固有の環境変数とマージされます（コンテナ優先）。
type ProjectEnvVar struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;index"`
	Key       string    `gorm:"not null"`
	Value     string    `gorm:"not null"`
}

// ContainerEnvVar はコンテナ固有の環境変数です。
// 同名のキーがある場合、ProjectEnvVar より優先されます。
type ContainerEnvVar struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;index"`
	Key         string    `gorm:"not null"`
	Value       string    `gorm:"not null"`
}
