package model

import (
	"github.com/google/uuid"
)

// ServiceConnection はプロジェクト内コンテナ間の接続関係を表します。
// フロントエンドの React Flow による可視化に使います。
type ServiceConnection struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID         uuid.UUID `gorm:"type:uuid;not null;index"`
	SourceContainerID uuid.UUID `gorm:"type:uuid;not null"`
	TargetContainerID uuid.UUID `gorm:"type:uuid;not null"`
}
