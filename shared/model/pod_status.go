package model

import (
	"time"

	"github.com/google/uuid"
)

// PodStatus は Kubernetes Pod 単位のリアルタイムステータスを保持します。
// Watcher が Pod Watch イベントを受けて UPSERT します。
// Pod 削除イベント受信時はレコードを DELETE します。
type PodStatus struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID  `gorm:"type:uuid;not null;index"`
	// Kubernetes Pod 名。一意制約あり。
	PodName      string    `gorm:"not null;uniqueIndex"`
	// Running / Pending / Failed / Succeeded / Unknown
	Status       string    `gorm:"not null"`
	Ready        bool      `gorm:"not null;default:false"`
	RestartCount int       `gorm:"not null;default:0"`
	NodeName     string
	StartedAt    *time.Time
	UpdatedAt    time.Time
}
