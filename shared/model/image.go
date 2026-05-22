package model

import (
	"time"

	"github.com/google/uuid"
)

// ImageStatus はイメージのビルド状態です。
type ImageStatus string

const (
	ImageStatusBuilding  ImageStatus = "building"
	ImageStatusReady     ImageStatus = "ready"
	ImageStatusFailed    ImageStatus = "failed"
)

// Image は Harbor レジストリに格納されたコンテナイメージを表します。
// ImageRef 例: harbor.main-harbor/buildkit/my-app:abc123
type Image struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID  `gorm:"type:uuid;not null;index"`
	BuildJobID  *uuid.UUID `gorm:"type:uuid"`
	// Harbor のフル参照パス
	ImageRef  string    `gorm:"not null"`
	Status    string    `gorm:"not null;default:'building'"`
	CreatedAt time.Time
}
