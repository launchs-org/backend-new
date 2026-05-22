package model

import (
	"time"

	"github.com/google/uuid"
)

// ContainerLog はランタイムログおよびビルドログを統一的に保持します。
// ビルドログの場合は PodName に "build:{build_job_id}" をセットします。
// Watcher が 3 秒バッファリングして一括 INSERT します。
type ContainerLog struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;index"`
	// カーソルベースページネーションのキー
	Timestamp time.Time `gorm:"not null;index"`
	// DEBUG / INFO / WARN / ERROR
	Level   string `gorm:"not null;default:'INFO'"`
	Message string `gorm:"not null;type:text"`
	// ランタイムログの場合は Kubernetes Pod 名、ビルドログは "build:{id}"
	PodName *string
}
