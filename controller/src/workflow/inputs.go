package workflow

import (
	"launchs/shared/model"

	"github.com/google/uuid"

	"controller/activity"
)

// CreateProjectInput は CreateProjectWorkflow への入力です。
// ProjectID は文字列で受け取り、内部で uuid.Parse します。
type CreateProjectInput struct {
	ProjectID string
	Namespace string
}

// DeleteProjectInput は DeleteProjectWorkflow への入力です。
type DeleteProjectInput struct {
	ProjectID string
	Namespace string
}

// DeployInput は DeployWorkflow への入力です。
type DeployInput struct {
	ContainerID    uuid.UUID
	Namespace      string
	DeploymentName string
	ImageRef       string
	Replicas       int
	ResourceSize   string
	// リソースサイズの具体値（未指定時は ResourceSize から自動解決）
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
	EnvVars       []activity.EnvVar
	Ports         []activity.Port
	VolumeMounts  []activity.VolumeMount
}

// RedeployInput は RedeployWorkflow への入力です。
type RedeployInput struct {
	ContainerID    uuid.UUID
	Namespace      string
	DeploymentName string
}

// DeleteContainerInput は DeleteContainerWorkflow への入力です。
type DeleteContainerInput struct {
	ContainerID    uuid.UUID
	Namespace      string
	DeploymentName string
}

// ScaleInput は ScaleWorkflow への入力です。
type ScaleInput struct {
	ContainerID    uuid.UUID
	Namespace      string
	DeploymentName string
	Replicas       int
}

// CreateVolumeInput は CreateVolumeWorkflow への入力です。
type CreateVolumeInput struct {
	VolumeID    uuid.UUID
	Namespace   string
	PVCName     string
	StorageSize string
}

// DeleteVolumeInput は DeleteVolumeWorkflow への入力です。
type DeleteVolumeInput struct {
	VolumeID  uuid.UUID
	Namespace string
	PVCName   string
}

// MountVolumeInput は MountVolumeWorkflow への入力です。
// ワークフロー内で DB からコンテナ情報を取得して Deployment を再 Apply します。
type MountVolumeInput struct {
	ContainerID uuid.UUID
	Namespace   string
	VolumeID    uuid.UUID
	PVCName     string
	MountPath   string
}

// UnmountVolumeInput は UnmountVolumeWorkflow への入力です。
type UnmountVolumeInput struct {
	ContainerID uuid.UUID
	Namespace   string
	VolumeID    uuid.UUID
	PVCName     string
}

// CreateServiceInput は CreateServiceWorkflow への入力です。
type CreateServiceInput struct {
	ContainerID uuid.UUID
	ServiceSpec activity.ServiceSpec
}

// DeleteServiceInput は DeleteServiceWorkflow への入力です。
type DeleteServiceInput struct {
	ContainerID uuid.UUID
	Namespace   string
	ServiceName string
}

// CreateIngressInput は CreateIngressWorkflow への入力です。
type CreateIngressInput struct {
	ContainerID uuid.UUID
	IngressSpec activity.IngressSpec
}

// DeleteIngressInput は DeleteIngressWorkflow への入力です。
type DeleteIngressInput struct {
	ContainerID uuid.UUID
	Namespace   string
	IngressName string
}

// DeployProjectInput は DeployProjectWorkflow への入力です。
type DeployProjectInput struct {
	ProjectID  uuid.UUID
	Namespace  string
	Containers []DeployInput
}

// RestoreSnapshotInput は RestoreSnapshotWorkflow への入力です。
type RestoreSnapshotInput struct {
	ProjectID    uuid.UUID
	Namespace    string
	SnapshotData model.SnapshotData
}

// BuildDeployInput は BuildDeployWorkflow への入力です（Builder キューで実行）。
type BuildDeployInput struct {
	ContainerID uuid.UUID
	ProjectID   uuid.UUID
}

// DeployTemplateInput は DeployTemplateWorkflow への入力です。
type DeployTemplateInput struct {
	ContainerID  uuid.UUID
	ProjectID    string
	TemplateName string
	ResourceSize string
	Params       map[string]string
	VolumeID     *string
	MountPath    *string
}
