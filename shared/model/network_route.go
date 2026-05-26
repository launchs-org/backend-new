package model

import (
	"time"

	"github.com/google/uuid"
)

// NetworkRouteType は公開方法を表します。
type NetworkRouteType string

const (
	// NetworkRouteTypeService は Kubernetes Service（プロジェクト内部向け）です。
	NetworkRouteTypeService NetworkRouteType = "service"
	// NetworkRouteTypeIngress は Traefik IngressRoute（HTTP 公開）です。
	NetworkRouteTypeIngress NetworkRouteType = "ingress"
)

// NetworkRoute は Service または IngressRoute の設定を保持します。
// IngressRoute のサブドメイン形式: {container-name}-{project-uuid}.launchs.org
type NetworkRoute struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContainerID uuid.UUID `gorm:"type:uuid;not null;index"`
	// service / ingress
	Type     string `gorm:"not null"`
	Port     int    `gorm:"not null"`
	Protocol string `gorm:"default:'TCP'"`
	// IngressRoute の場合のみセット
	Subdomain *string
	// Service type の場合: controller が Service 作成後に保存する ClusterIP
	ClusterIP string
	CreatedAt time.Time
}
