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
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID  `gorm:"type:uuid;not null;index"`
	GitRepo     string     `gorm:"not null"`
	GitBranch   string     `gorm:"not null"`
	GitCommit   string     `gorm:"not null"`
	Subdir      string     `gorm:"not null;default:'.'"`
	Status      string     `gorm:"not null;default:'pending'"`
	// Temporal ワークフロー ID
	TemporalWorkflowID *string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	ImageID            *uuid.UUID `gorm:"type:uuid"`
	CreatedAt          time.Time
}
