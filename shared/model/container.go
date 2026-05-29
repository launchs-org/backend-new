package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ContainerStatus はコンテナのライフサイクル状態を表します。
type ContainerStatus string

const (
	ContainerStatusPending   ContainerStatus = "pending"
	ContainerStatusBuilding  ContainerStatus = "building"
	ContainerStatusDeploying ContainerStatus = "deploying"
	// ContainerStatusApplying は K8s Deployment の apply が完了し Pod が Ready になるのを待っている状態です。
	// DeployWorkflow / RedeployWorkflow / ScaleWorkflow が apply 後にセットし、
	// Watcher が desired replicas 分の Pod が全て Ready になった時点で running に遷移させます。
	ContainerStatusApplying  ContainerStatus = "applying"
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

	// テンプレートから作成されたコンテナかどうか（true の場合ビルドログは非表示）
	IsTemplate bool `gorm:"not null;default:false"`

	// Dockerイメージを直接指定してデプロイしたコンテナかどうか（true の場合ビルドログは非表示）
	IsImageDeploy bool    `gorm:"not null;default:false"`
	ImageRef      *string // 直接デプロイ時のイメージ参照（例: nginx:latest）

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

// GetDeploymentName は Kubernetes Deployment 名を返します。
// コンテナIDのみを使うことで、コンテナ名変更後も同じ Deployment を指し続けます。
func GetDeploymentName(containerID uuid.UUID) string {
	return fmt.Sprintf("container-%s", containerID.String())
}

// GetK8sNamespace は Kubernetes Namespace 名を返します。
func GetK8sNamespace(projectID uuid.UUID) string {
	return fmt.Sprintf("project-%s", projectID.String())
}
