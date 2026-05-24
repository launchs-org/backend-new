package model

import (
	"time"

	"github.com/google/uuid"
)

// ContainerStatus はコンテナのライフサイクル状態を表します。
type ContainerStatus string

const (
	ContainerStatusPending   ContainerStatus = "pending"
	ContainerStatusBuilding  ContainerStatus = "building"
	ContainerStatusDeploying ContainerStatus = "deploying"
	ContainerStatusRunning   ContainerStatus = "running"
	ContainerStatusScaling   ContainerStatus = "scaling"
	ContainerStatusFailed    ContainerStatus = "failed"
	ContainerStatusStopped   ContainerStatus = "stopped"
)

// Container は Kubernetes Deployment に対応するリソースです。
// ActiveDeployWorkflowID / ActiveScaleWorkflowID は Temporal ワークフロー ID を保持し、
// 同時実行制御のために使います。
type Container struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Name         string    `gorm:"not null"`
	Status       ContainerStatus    `gorm:"not null;default:'pending'"`
	Replicas     int       `gorm:"not null;default:1"`
	ReadyReplicas  int     `gorm:"not null;default:0"`
	FailedReplicas int     `gorm:"not null;default:0"`
	// small / medium / large
	ResourceSize string `gorm:"not null;default:'small'"`

	CurrentImageID *uuid.UUID `gorm:"type:uuid"`

	// 実行中の Temporal ワークフロー ID（nil = アイドル）
	ActiveDeployWorkflowID *string
	ActiveScaleWorkflowID  *string

	// Webhook 自動デプロイ用トークン
	WebhookToken *string `gorm:"uniqueIndex"`

	// GitHub ソース情報
	GitRepo   *string
	GitBranch *string
	GitSubdir *string

	CreatedAt time.Time
	UpdatedAt time.Time

	EnvVars         []ContainerEnvVar        `gorm:"foreignKey:ContainerID"`
	Ports           []Port                   `gorm:"foreignKey:ContainerID"`
	Routes          []NetworkRoute           `gorm:"foreignKey:ContainerID"`
	BuildJobs       []BuildJob               `gorm:"foreignKey:ContainerID"`
	StatusHistories []ContainerStatusHistory `gorm:"foreignKey:ContainerID"`
	PodStatuses     []PodStatus              `gorm:"foreignKey:ContainerID"`
}
