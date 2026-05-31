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
	ProjectID      uuid.UUID
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
	// Label は WorkflowRun の表示用補足情報（コンテナ名など）
	Label *string
}

// RedeployInput は RedeployWorkflow への入力です。
type RedeployInput struct {
	ContainerID    uuid.UUID
	ProjectID      uuid.UUID
	Namespace      string
	DeploymentName string
	Label          *string
}

// DeleteContainerInput は DeleteContainerWorkflow への入力です。
type DeleteContainerInput struct {
	ContainerID    uuid.UUID
	ProjectID      uuid.UUID
	Namespace      string
	DeploymentName string
	Label          *string
}

// ScaleInput は ScaleWorkflow への入力です。
type ScaleInput struct {
	ContainerID    uuid.UUID
	ProjectID      uuid.UUID
	Namespace      string
	DeploymentName string
	Replicas       int
	Label          *string
}

// CreateVolumeInput は CreateVolumeWorkflow への入力です。
type CreateVolumeInput struct {
	VolumeID    uuid.UUID
	ProjectID   uuid.UUID
	Namespace   string
	PVCName     string
	StorageSize string
	Label       *string
}

// DeleteVolumeInput は DeleteVolumeWorkflow への入力です。
type DeleteVolumeInput struct {
	VolumeID  uuid.UUID
	ProjectID uuid.UUID
	Namespace string
	PVCName   string
	Label     *string
}

// MountVolumeInput は MountVolumeWorkflow への入力です。
// ワークフロー内で DB からコンテナ情報を取得して Deployment を再 Apply します。
type MountVolumeInput struct {
	ContainerID uuid.UUID
	ProjectID   uuid.UUID
	Namespace   string
	VolumeID    uuid.UUID
	PVCName     string
	MountPath   string
	Label       *string
}

// UnmountVolumeInput は UnmountVolumeWorkflow への入力です。
type UnmountVolumeInput struct {
	ContainerID uuid.UUID
	ProjectID   uuid.UUID
	Namespace   string
	VolumeID    uuid.UUID
	PVCName     string
	Label       *string
}

// CreateServiceInput は CreateServiceWorkflow への入力です。
type CreateServiceInput struct {
	ContainerID uuid.UUID
	ProjectID   uuid.UUID
	RouteID     uuid.UUID
	ServiceSpec activity.ServiceSpec
	Label       *string
}

// DeleteServiceInput は DeleteServiceWorkflow への入力です。
type DeleteServiceInput struct {
	ContainerID uuid.UUID
	ProjectID   uuid.UUID
	Namespace   string
	ServiceName string
	Label       *string
}

// CreateIngressInput は CreateIngressWorkflow への入力です。
type CreateIngressInput struct {
	ContainerID uuid.UUID
	ProjectID   uuid.UUID
	IngressSpec activity.IngressSpec
	Label       *string
}

// DeleteIngressInput は DeleteIngressWorkflow への入力です。
type DeleteIngressInput struct {
	ContainerID uuid.UUID
	ProjectID   uuid.UUID
	Namespace   string
	IngressName string
	Label       *string
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

// VolumeRecordInput はワークフローに渡すボリューム DB レコード情報です。
type VolumeRecordInput struct {
	ID     string
	Name   string
	SizeMB int
}

// RouteRecordInput はワークフローに渡す Route DB レコード情報です。
type RouteRecordInput struct {
	ID       string
	Port     int
	Protocol string
}

// DeployTemplateInput は DeployTemplateWorkflow への入力です。
type DeployTemplateInput struct {
	ContainerID    uuid.UUID
	ProjectID      uuid.UUID
	Namespace      string
	DeploymentName string
	ImageRef       string
	ResourceSize   string
	Replicas       int
	EnvVars        []activity.EnvVar
	Label          *string
	// VolumeRecord は新規作成するボリュームの情報（nil の場合は作成しない）
	VolumeRecord    *VolumeRecordInput
	VolumeMountPath string
	// ExistingVolumeMounts は既存ボリュームのマウント情報（PVC 作成不要）
	ExistingVolumeMounts []activity.VolumeMount
	RouteRecords         []RouteRecordInput
}
