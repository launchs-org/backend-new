package model

import (
	"time"

	"github.com/google/uuid"
)

// ContainerMetric は Metrics Server から収集した Pod 単位のリソース使用量です。
// Watcher が 15 秒ごとに収集し、30 日経過したデータは削除します。
type ContainerMetric struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;index"`
	// 収集対象の Pod 名（複数 Pod の場合に集計に使う）
	PodName         string    `gorm:"not null"`
	Timestamp       time.Time `gorm:"not null;index"`
	// CPU 使用量（コア数。例: 0.25 = 250m）
	CPUUsage        float64
	// Pod に割り当てられた CPU requests（コア数）。使用率(%)の算出に使う
	CPURequestCores float64
	// メモリ使用量（バイト）
	MemoryBytes     int64
}
