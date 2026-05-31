package model

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowRunStatus はワークフロー実行の状態です。
type WorkflowRunStatus string

const (
	WorkflowRunStatusRunning   WorkflowRunStatus = "running"
	WorkflowRunStatusSucceeded WorkflowRunStatus = "succeeded"
	WorkflowRunStatusFailed    WorkflowRunStatus = "failed"
	WorkflowRunStatusCanceled  WorkflowRunStatus = "canceled"
)

// WorkflowRunType はワークフローの種別です。
type WorkflowRunType string

const (
	WorkflowRunTypeCreateProject    WorkflowRunType = "CreateProject"
	WorkflowRunTypeDeleteProject    WorkflowRunType = "DeleteProject"
	WorkflowRunTypeDeployProject    WorkflowRunType = "DeployProject"
	WorkflowRunTypeDeploy           WorkflowRunType = "Deploy"
	WorkflowRunTypeRedeploy         WorkflowRunType = "Redeploy"
	WorkflowRunTypeDeleteContainer  WorkflowRunType = "DeleteContainer"
	WorkflowRunTypeScale            WorkflowRunType = "Scale"
	WorkflowRunTypeBuildDeploy      WorkflowRunType = "BuildDeploy"
	WorkflowRunTypeCancelBuild      WorkflowRunType = "CancelBuild"
	WorkflowRunTypeCreateVolume     WorkflowRunType = "CreateVolume"
	WorkflowRunTypeDeleteVolume     WorkflowRunType = "DeleteVolume"
	WorkflowRunTypeMountVolume      WorkflowRunType = "MountVolume"
	WorkflowRunTypeUnmountVolume    WorkflowRunType = "UnmountVolume"
	WorkflowRunTypeCreateService    WorkflowRunType = "CreateService"
	WorkflowRunTypeDeleteService    WorkflowRunType = "DeleteService"
	WorkflowRunTypeCreateIngress    WorkflowRunType = "CreateIngress"
	WorkflowRunTypeDeleteIngress    WorkflowRunType = "DeleteIngress"
	WorkflowRunTypeRestoreSnapshot  WorkflowRunType = "RestoreSnapshot"
	WorkflowRunTypeDeployTemplate   WorkflowRunType = "DeployTemplate"
)

// WorkflowRun はプロジェクト単位のワークフロー実行記録です。
type WorkflowRun struct {
	ID           uuid.UUID         `gorm:"type:uuid;primaryKey"     json:"id"`
	ProjectID    uuid.UUID         `gorm:"type:uuid;not null;index" json:"project_id"`
	WorkflowID   string            `gorm:"not null"                 json:"workflow_id"`
	WorkflowType WorkflowRunType   `gorm:"not null"                 json:"workflow_type"`
	Status       WorkflowRunStatus `gorm:"not null;default:'running'" json:"status"`
	// ContainerID は任意（コンテナ操作ワークフローのみ設定）
	ContainerID *uuid.UUID `gorm:"type:uuid" json:"container_id,omitempty"`
	// Label は表示用の補足情報（コンテナ名など）
	Label *string   `json:"label,omitempty"`
	Log   *string   `gorm:"type:text" json:"log,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkflowRunEvent はワークフロー実行の状態遷移の1イベントを記録します。
type WorkflowRunEvent struct {
	ID            uuid.UUID         `gorm:"type:uuid;primaryKey"         json:"id"`
	WorkflowRunID uuid.UUID         `gorm:"type:uuid;not null;index"     json:"workflow_run_id"`
	Status        WorkflowRunStatus `gorm:"not null"                     json:"status"`
	Message       *string           `gorm:"type:text"                    json:"message,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
}
