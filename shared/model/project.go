package model

import (
	"time"

	"github.com/google/uuid"
)

// ContainerStatus はコンテナのライフサイクル状態を表します。
type ProjectStatus string

const (
	ProjectStatusPending     ProjectStatus = "pending"
	ProjectStatusActive      ProjectStatus = "active"
	ProjectStatusTerminating ProjectStatus = "terminating"
	ProjectStatusFailed      ProjectStatus = "failed"
)

// Project は Launchs-org の最上位リソースです。
// Kubernetes 上では Namespace: project-{uuid} に対応します。
type Project struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID string    `gorm:"not null;index"`
	Name   string    `gorm:"not null"`
	// URL フレンドリーな識別子。一意制約あり。
	Slug string `gorm:"not null;uniqueIndex"`
	// Kubernetes Namespace 名: project-{uuid}
	Namespace           string `gorm:"not null;uniqueIndex"`
	HarborProjectName   string
	HarborRobotUsername string
	HarborRobotPassword string

	CreatedAt time.Time
	UpdatedAt time.Time

	// ステータスを保持する
	Status ProjectStatus `gorm:"not null;default:'pending'"`

	Containers []Container     `gorm:"foreignKey:ProjectID"`
	Volumes    []Volume        `gorm:"foreignKey:ProjectID"`
	EnvVars    []ProjectEnvVar `gorm:"foreignKey:ProjectID"`
	Snapshots  []Snapshot      `gorm:"foreignKey:ProjectID"`
}
