package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Snapshot はプロジェクトの全コンテナ状態を JSON として保存します。
// DeployProjectWorkflow 完了時に自動作成されます。
// SNAPSHOT_MAX_COUNT を超えた古いスナップショットは自動削除されます。
type Snapshot struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID       `gorm:"type:uuid;not null;index"`
	// 全コンテナ・環境変数・ポート・マウント・ルートを含む完全状態
	Data        json.RawMessage `gorm:"type:jsonb;not null"`
	Description string
	CreatedAt   time.Time
}

// SnapshotContainerData はスナップショット内の各コンテナの状態です。
type SnapshotContainerData struct {
	ContainerID  string                  `json:"container_id"`
	ContainerName string                 `json:"container_name"`
	ImageTag     string                  `json:"image_tag"`
	Replicas     int                     `json:"replicas"`
	ResourceSize string                  `json:"resource_size"`
	EnvVars      []SnapshotEnvVar        `json:"env_vars"`
	Ports        []SnapshotPort          `json:"ports"`
	Mounts       []SnapshotVolumeMount   `json:"mounts"`
}

type SnapshotEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SnapshotPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type SnapshotVolumeMount struct {
	VolumeID  string `json:"volume_id"`
	MountPath string `json:"mount_path"`
}

// SnapshotData はスナップショット全体の構造です。
type SnapshotData struct {
	Containers     []SnapshotContainerData `json:"containers"`
	ProjectEnvVars []SnapshotEnvVar        `json:"project_env_vars"`
}
