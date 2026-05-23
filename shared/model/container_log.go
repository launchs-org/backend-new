package model

import (
	"time"

	"github.com/google/uuid"
)

// ContainerLog はランタイムログおよびビルドログを統一的に保持します。
// ビルドログの場合は PodName に "build:{build_job_id}" をセットします。
// Watcher が 3 秒バッファリングして一括 INSERT します。
type ContainerLog struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"     json:"id"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;index" json:"container_id"`
	Timestamp   time.Time `gorm:"not null;index"           json:"timestamp"`
	Level       string    `gorm:"not null;default:'INFO'"  json:"level"`
	Message     string    `gorm:"not null;type:text"       json:"message"`
	PodName     *string   `                                json:"pod_name"`
}
