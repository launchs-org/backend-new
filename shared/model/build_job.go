package model

import (
	"time"

	"github.com/google/uuid"
)

// BuildJobStatus はビルドジョブの状態です。
type BuildJobStatus string

const (
	BuildJobStatusPending  BuildJobStatus = "pending"
	BuildJobStatusRunning  BuildJobStatus = "running"
	BuildJobStatusComplete BuildJobStatus = "complete"
	BuildJobStatusFailed   BuildJobStatus = "failed"
)

// BuildJob は railpack / BuildKit によるイメージビルドの実行記録です。
// ビルドログは ContainerLog テーブルに pod_name = "build:{build_job_id}" で格納します。
type BuildJob struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"           json:"id"`
	ContainerID uuid.UUID  `gorm:"type:uuid;not null;index"       json:"container_id"`
	GitRepo     string     `gorm:"not null"                       json:"git_repo"`
	GitBranch   string     `gorm:"not null"                       json:"git_branch"`
	GitCommit   string     `gorm:"not null"                       json:"git_commit"`
	Subdir      string     `gorm:"not null;default:'.'"`
	Status      string     `gorm:"not null;default:'pending'"     json:"status"`
	ImageRef           *string    `json:"image_ref"`
	TemporalWorkflowID *string    `json:"temporal_workflow_id"`
	StartedAt          *time.Time `json:"started_at"`
	FinishedAt         *time.Time `json:"finished_at"`
	ImageID            *uuid.UUID `gorm:"type:uuid"                  json:"image_id"`
	CreatedAt          time.Time  `json:"created_at"`
}
