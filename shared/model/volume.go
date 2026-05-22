package model

import (
	"time"

	"github.com/google/uuid"
)

// VolumeStatus は PVC のバインド状態です。
type VolumeStatus string

const (
	VolumeStatusPending VolumeStatus = "pending"
	VolumeStatusBound   VolumeStatus = "bound"
	VolumeStatusLost    VolumeStatus = "lost"
)

// Volume は Kubernetes PersistentVolumeClaim に対応します。
// PVC 名: {volume-name}-{volume-id-short}
type Volume struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Name         string    `gorm:"not null"`
	SizeMB       int       `gorm:"not null"`
	StorageClass string    `gorm:"not null;default:'standard'"`
	Status       string    `gorm:"not null;default:'pending'"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Mounts []VolumeMount `gorm:"foreignKey:VolumeID"`
}

// VolumeMount はコンテナへのボリュームマウント情報を保持します。
type VolumeMount struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	VolumeID    uuid.UUID `gorm:"type:uuid;not null;index"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;index"`
	MountPath   string    `gorm:"not null"`
	CreatedAt   time.Time
}
