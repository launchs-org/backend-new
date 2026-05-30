package model

import (
	"time"

	"github.com/google/uuid"
)

// UserQuota はユーザーごとのリソースサイズ別コンテナ上限を管理します。
// レコードが存在しない場合は config のデフォルト値が適用されます。
type UserQuota struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       string    `gorm:"not null;uniqueIndex"`
	MaxSmall     int       `gorm:"not null;default:5"`
	MaxMedium    int       `gorm:"not null;default:3"`
	MaxLarge     int       `gorm:"not null;default:1"`
	MaxStorageMB int       `gorm:"not null;default:20480"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
