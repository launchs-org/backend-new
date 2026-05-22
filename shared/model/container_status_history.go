package model

import (
	"time"

	"github.com/google/uuid"
)

// ContainerStatusHistory はコンテナのステータス変化履歴を記録します。
// 最新 100 件のみ保持し、古いレコードは Watcher が定期的に削除します。
type ContainerStatusHistory struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Status         string    `gorm:"not null"`
	Replicas       int
	ReadyReplicas  int
	FailedReplicas int
	CreatedAt      time.Time
}
