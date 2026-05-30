// Package repository はデータ永続化の抽象インターフェースを定義します。
// 各インターフェースを実装した struct を service 層に DI します。
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"launchs/shared/model"
)

// ProjectRepository はプロジェクトの CRUD 操作を抽象化します。
type ProjectRepository interface {
	Create(ctx context.Context, project *model.Project) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	FindByUserID(ctx context.Context, userID string) ([]model.Project, error)
	// WithContainers は containers, volumes, env_vars を Preload して返します。
	FindByIDWithDetails(ctx context.Context, id uuid.UUID) (*model.Project, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// LastDeployedAt はプロジェクト内の最終デプロイ日時を返します（コンテナ数も含む）。
	CountContainers(ctx context.Context, projectID uuid.UUID) (int64, error)

	// ステータスを更新する
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// ContainerRepository はコンテナの CRUD・ステータス更新を抽象化します。
type ContainerRepository interface {
	Create(ctx context.Context, container *model.Container) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Container, error)
	FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.Container, error)
	FindByIDWithDetails(ctx context.Context, id uuid.UUID) (*model.Container, error)
	FindByWebhookToken(ctx context.Context, token string) (*model.Container, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateReplicas(ctx context.Context, id uuid.UUID, ready, failed int) error
	UpdateActiveDeployWorkflowID(ctx context.Context, id uuid.UUID, workflowID *string) error
	UpdateActiveScaleWorkflowID(ctx context.Context, id uuid.UUID, workflowID *string) error
	UpdateCurrentImageID(ctx context.Context, id uuid.UUID, imageID uuid.UUID) error
	UpdateWebhookToken(ctx context.Context, id uuid.UUID, token *string) error
	Update(ctx context.Context, container *model.Container) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteRelated(ctx context.Context, id uuid.UUID) error
	// CountByUserIDPerResourceSize はユーザーの全プロジェクトにわたるリソースサイズ別コンテナ数を返します。
	// 削除済み・停止済みコンテナは含みません。
	CountByUserIDPerResourceSize(ctx context.Context, userID string) (map[string]int, error)
}

// UserQuotaRepository はユーザーごとのリソースクォータを管理します。
type UserQuotaRepository interface {
	FindByUserID(ctx context.Context, userID string) (*model.UserQuota, error)
	Upsert(ctx context.Context, quota *model.UserQuota) error
}

// ContainerStatusHistoryRepository はステータス履歴の INSERT と古いレコードの削除を抽象化します。
type ContainerStatusHistoryRepository interface {
	Insert(ctx context.Context, history *model.ContainerStatusHistory) error
	FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.ContainerStatusHistory, error)
	// DeleteOldRecords は最新 keepCount 件を超える古いレコードを削除します。
	DeleteOldRecords(ctx context.Context, containerID uuid.UUID, keepCount int) error
}

// PodStatusRepository は Pod 単位のステータスの UPSERT・削除を抽象化します。
type PodStatusRepository interface {
	Upsert(ctx context.Context, pod *model.PodStatus) error
	DeleteByPodName(ctx context.Context, podName string) error
	DeleteByContainerID(ctx context.Context, containerID uuid.UUID) error
	FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.PodStatus, error)
}

// LogRepository はコンテナログ・ビルドログの一括 INSERT とカーソルベース取得を抽象化します。
type LogRepository interface {
	BulkInsert(ctx context.Context, logs []model.ContainerLog) error
	// FindAfterCursor は cursor（タイムスタンプ）以降のログを limit 件返します。
	// podName が空の場合は全 Pod のログを返します。
	FindAfterCursor(ctx context.Context, containerID uuid.UUID, cursor time.Time, limit int, podName string) ([]model.ContainerLog, error)
	// FindBuildJobLogs はビルドジョブのログを取得します（PodName = "build:{build_job_id}"）。
	FindBuildJobLogs(ctx context.Context, buildJobID uuid.UUID, cursor time.Time, limit int) ([]model.ContainerLog, error)
}

// MetricRepository はコンテナメトリクスの一括 INSERT・範囲取得・古いデータ削除を抽象化します。
type MetricRepository interface {
	BulkInsert(ctx context.Context, metrics []model.ContainerMetric) error
	FindByRange(ctx context.Context, containerID uuid.UUID, from, to time.Time) ([]model.ContainerMetric, error)
	// DeleteOlderThan は指定日時より古いメトリクスを削除します。
	DeleteOlderThan(ctx context.Context, t time.Time) error
}

// BuildJobRepository はビルドジョブの CRUD を抽象化します。
type BuildJobRepository interface {
	Create(ctx context.Context, job *model.BuildJob) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.BuildJob, error)
	FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.BuildJob, error)
	FindActiveByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.BuildJob, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateWorkflowID(ctx context.Context, id uuid.UUID, workflowID string) error
	UpdateFinishedAt(ctx context.Context, id uuid.UUID) error
	UpdateImageID(ctx context.Context, id uuid.UUID, imageID uuid.UUID) error
	DeleteByContainerID(ctx context.Context, containerID uuid.UUID) error
}

// VolumeRepository はボリューム・マウント情報の CRUD を抽象化します。
type VolumeRepository interface {
	Create(ctx context.Context, volume *model.Volume) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Volume, error)
	FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.Volume, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	Delete(ctx context.Context, id uuid.UUID) error
	CreateMount(ctx context.Context, mount *model.VolumeMount) error
	DeleteMount(ctx context.Context, volumeID, containerID uuid.UUID) error
	FindMountsByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.VolumeMount, error)
	FindMountsByVolumeID(ctx context.Context, volumeID uuid.UUID) ([]model.VolumeMount, error)
	// SumSizeMBByUserID はユーザーの全プロジェクトのボリューム合計サイズ（MB）を返します。
	SumSizeMBByUserID(ctx context.Context, userID string) (int, error)
}

// NetworkRouteRepository はネットワークルートの CRUD を抽象化します。
type NetworkRouteRepository interface {
	Create(ctx context.Context, route *model.NetworkRoute) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.NetworkRoute, error)
	FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.NetworkRoute, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// EnvVarRepository はプロジェクト・コンテナ環境変数の CRUD を抽象化します。
type EnvVarRepository interface {
	FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.ProjectEnvVar, error)
	UpsertProjectEnvVars(ctx context.Context, projectID uuid.UUID, envVars []model.ProjectEnvVar) error
	DeleteProjectEnvVar(ctx context.Context, projectID uuid.UUID, key string) error
	FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.ContainerEnvVar, error)
	UpsertContainerEnvVars(ctx context.Context, containerID uuid.UUID, envVars []model.ContainerEnvVar) error
	DeleteContainerEnvVar(ctx context.Context, containerID uuid.UUID, key string) error
	FindSelectedProjectEnvVarKeys(ctx context.Context, containerID uuid.UUID) ([]string, error)
	SetSelectedProjectEnvVarKeys(ctx context.Context, containerID uuid.UUID, keys []string) error
}

// PortRepository はポートの CRUD を抽象化します。
type PortRepository interface {
	Create(ctx context.Context, port *model.Port) error
	FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.Port, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// SnapshotRepository はスナップショットの CRUD を抽象化します。
type SnapshotRepository interface {
	Create(ctx context.Context, snapshot *model.Snapshot) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Snapshot, error)
	FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.Snapshot, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// CountByProjectID はプロジェクトのスナップショット数を返します。
	CountByProjectID(ctx context.Context, projectID uuid.UUID) (int64, error)
	// FindOldestByProjectID は最古のスナップショット ID を返します。
	FindOldestByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]model.Snapshot, error)
}

// ServiceConnectionRepository はコンテナ間接続情報の CRUD を抽象化します。
type ServiceConnectionRepository interface {
	FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]model.ServiceConnection, error)
}

// ImageRepository はイメージの CRUD を抽象化します。
type ImageRepository interface {
	Create(ctx context.Context, image *model.Image) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Image, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// DeploymentRepository はデプロイ履歴の CRUD を抽象化します。
type DeploymentRepository interface {
	Create(ctx context.Context, deployment *model.Deployment) error
	FindByContainerID(ctx context.Context, containerID uuid.UUID) ([]model.Deployment, error)
}
