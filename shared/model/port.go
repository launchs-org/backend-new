package model

import (
	"github.com/google/uuid"
)

// Port はコンテナが公開するポートの定義です。
// Deployment の containerPort および Service / IngressRoute 作成時に参照します。
type Port struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;index"`
	Port        int       `gorm:"not null"`
	// TCP / UDP
	Protocol string `gorm:"not null;default:'TCP'"`
}
