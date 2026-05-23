package model

import (
	"time"

	"github.com/google/uuid"
)

// PodStatus は Kubernetes Pod 単位のリアルタイムステータスを保持します。
// Watcher が Pod Watch イベントを受けて UPSERT します。
// Pod 削除イベント受信時はレコードを DELETE します。
type PodStatus struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"      json:"id"`
	ContainerID  uuid.UUID  `gorm:"type:uuid;not null;index"  json:"container_id"`
	PodName      string     `gorm:"not null;uniqueIndex"      json:"pod_name"`
	Status       string     `gorm:"not null"                  json:"status"`
	Ready        bool       `gorm:"not null;default:false"    json:"ready"`
	RestartCount int        `gorm:"not null;default:0"        json:"restart_count"`
	NodeName     string     `                                 json:"node_name"`
	StartedAt    *time.Time `                                 json:"started_at"`
	UpdatedAt    time.Time  `                                 json:"updated_at"`
}
