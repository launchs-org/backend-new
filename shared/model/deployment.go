package model

import (
	"time"

	"github.com/google/uuid"
)

// Deployment はコンテナのデプロイ履歴を記録します。
// BuildDeployWorkflow / DeployWorkflow / RedeployWorkflow 完了時に INSERT します。
type Deployment struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID  `gorm:"type:uuid;not null;index"`
	ImageID     *uuid.UUID `gorm:"type:uuid"`
	// Temporal ワークフロー ID
	WorkflowID  string    `gorm:"not null"`
	Status      string    `gorm:"not null;default:'deploying'"`
	CreatedAt   time.Time
	CompletedAt *time.Time
}
