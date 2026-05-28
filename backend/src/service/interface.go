// Package service はビジネスロジックのインターフェースを定義します。
// handler 層はこのインターフェースを介して service を利用します。
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"launchs/shared/model"
)

// ---- Request / Response 型 ----

// BuildDeployRequest は GitHub からのコンテナ作成・デプロイのリクエスト情報です。
type BuildDeployRequest struct {
	Name         string
	GitRepo      string
	GitBranch    string
	GitCommit    string
	Subdir       string
	ResourceSize string
	Replicas     int
	EnvVars      []EnvVarInput
	Ports        []PortInput
}

// TemplateDeployRequest はテンプレートからのコンテナ作成リクエスト情報です。
type TemplateDeployRequest struct {
	Name         string
	TemplateName string
	ResourceSize string // 空の場合はテンプレート YAML の spec.resource_size を使用
	Replicas     int    // 0 の場合はテンプレート YAML の spec.replicas を使用
	Params       map[string]string
	VolumeID     *uuid.UUID // ユーザーが既存ボリュームを指定する場合
	MountPath    *string    // ユーザーが既存ボリュームを指定する場合のマウントパス
	CreateVolume bool       // テンプレート定義に基づいてボリュームを自動作成するか
	VolumeSize   int        // ボリュームサイズ(MB), 0 の場合はテンプレートデフォルト
}

// EnvVarInput は環境変数の入力値です。
type EnvVarInput struct {
	Key   string
	Value string
}

// PortInput はポートの入力値です。
type PortInput struct {
	Port     int
	Protocol string
}

// ---- Service インターフェース ----

// ProjectService はプロジェクトのビジネスロジックを抽象化します。
type ProjectService interface {
	Create(ctx context.Context, userID, name string) (*model.Project, string, error)
	List(ctx context.Context, userID string) ([]model.Project, error)
	Get(ctx context.Context, userID string, id uuid.UUID) (*model.Project, error)
	Delete(ctx context.Context, userID string, id uuid.UUID) (string, error)
	// DeployAll は全コンテナを一括再デプロイし、スナップショットを作成します。
	DeployAll(ctx context.Context, userID string, projectID uuid.UUID) (snapshotID string, workflowIDs []string, err error)
}

// ContainerService はコンテナのビジネスロジックを抽象化します。
type ContainerService interface {
	List(ctx context.Context, projectID uuid.UUID) ([]model.Container, error)
	Get(ctx context.Context, projectID, containerID uuid.UUID) (*model.Container, error)
	BuildDeploy(ctx context.Context, projectID uuid.UUID, req BuildDeployRequest) (container *model.Container, workflowID string, err error)
	DeployFromTemplate(ctx context.Context, projectID uuid.UUID, req TemplateDeployRequest) (container *model.Container, workflowID string, err error)
	Scale(ctx context.Context, projectID, containerID uuid.UUID, replicas int) (workflowID string, err error)
	Redeploy(ctx context.Context, projectID, containerID uuid.UUID) (workflowID string, err error)
	Delete(ctx context.Context, projectID, containerID uuid.UUID) (workflowID string, err error)
	Update(ctx context.Context, projectID, containerID uuid.UUID, resourceSize string) error
	Rebuild(ctx context.Context, projectID, containerID uuid.UUID) (workflowID string, err error)
	CreateWebhook(ctx context.Context, projectID, containerID uuid.UUID) (webhookURL, token string, err error)
	HandleWebhook(ctx context.Context, token string) error
}

// EnvVarService は環境変数のビジネスロジックを抽象化します。
type EnvVarService interface {
	ListProject(ctx context.Context, userID string, projectID uuid.UUID) ([]model.ProjectEnvVar, error)
	UpsertProject(ctx context.Context, userID string, projectID uuid.UUID, vars []EnvVarInput) error
	DeleteProject(ctx context.Context, userID string, projectID uuid.UUID, key string) error
	ListContainer(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]model.ContainerEnvVar, error)
	UpsertContainer(ctx context.Context, userID string, projectID, containerID uuid.UUID, vars []EnvVarInput) error
	DeleteContainer(ctx context.Context, userID string, projectID, containerID uuid.UUID, key string) error
	GetSelectedProjectEnvVarKeys(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]string, error)
	SetSelectedProjectEnvVarKeys(ctx context.Context, userID string, projectID, containerID uuid.UUID, keys []string) error
}

// PortService はポートのビジネスロジックを抽象化します。
type PortService interface {
	List(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]model.Port, error)
	Create(ctx context.Context, userID string, projectID, containerID uuid.UUID, port int, protocol string) (*model.Port, error)
	Delete(ctx context.Context, userID string, projectID, containerID, portID uuid.UUID) error
}

// RouteWithEndpoint は NetworkRoute の alias です（DB に ClusterIP が含まれているため）。
type RouteWithEndpoint = model.NetworkRoute

// RouteService はネットワークルートのビジネスロジックを抽象化します。
type RouteService interface {
	List(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]RouteWithEndpoint, error)
	CreateService(ctx context.Context, userID string, projectID, containerID uuid.UUID, port int, protocol string) (workflowID string, err error)
	CreateIngress(ctx context.Context, userID string, projectID, containerID uuid.UUID, port int) (routeID, subdomain, workflowID string, err error)
	Delete(ctx context.Context, userID string, projectID, containerID, routeID uuid.UUID) (workflowID string, err error)
}

// VolumeService はボリュームのビジネスロジックを抽象化します。
type VolumeService interface {
	List(ctx context.Context, userID string, projectID uuid.UUID) ([]model.Volume, error)
	Create(ctx context.Context, userID string, projectID uuid.UUID, name string, sizeMB int, storageClass string) (volumeID, workflowID string, err error)
	Delete(ctx context.Context, userID string, projectID, volumeID uuid.UUID) (workflowID string, err error)
	Mount(ctx context.Context, userID string, projectID, containerID, volumeID uuid.UUID, mountPath string) (workflowID string, err error)
	Unmount(ctx context.Context, userID string, projectID, containerID, volumeID uuid.UUID) (workflowID string, err error)
}

// LogService はログ取得のビジネスロジックを抽象化します。
type LogService interface {
	GetContainerLogs(ctx context.Context, userID string, projectID, containerID uuid.UUID, cursor time.Time, limit int, podName string) (logs []model.ContainerLog, nextCursor *time.Time, err error)
	GetBuildJobLogs(ctx context.Context, userID string, projectID, buildJobID uuid.UUID, cursor time.Time, limit int) (logs []model.ContainerLog, nextCursor *time.Time, err error)
}

// MetricService はメトリクス取得のビジネスロジックを抽象化します。
type MetricService interface {
	Get(ctx context.Context, userID string, projectID, containerID uuid.UUID, from, to time.Time) (cpu, memory []model.ContainerMetric, err error)
}

// BuildJobService はビルドジョブ管理のビジネスロジックを抽象化します。
type BuildJobService interface {
	List(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]model.BuildJob, error)
	Cancel(ctx context.Context, userID string, projectID, buildJobID uuid.UUID) error
}

// TemplateService はテンプレート一覧・詳細取得を抽象化します。
type TemplateService interface {
	List(ctx context.Context) ([]TemplateSummary, error)
	Get(ctx context.Context, name string) (*TemplateDetail, error)
}

// SnapshotService はスナップショットのビジネスロジックを抽象化します。
type SnapshotService interface {
	List(ctx context.Context, userID string, projectID uuid.UUID) ([]model.Snapshot, error)
	Get(ctx context.Context, userID string, projectID, snapshotID uuid.UUID) (*model.Snapshot, error)
	Restore(ctx context.Context, userID string, projectID, snapshotID uuid.UUID) (workflowID string, err error)
}

// ConnectionService はフロー可視化用コネクション取得を抽象化します。
type ConnectionService interface {
	List(ctx context.Context, userID string, projectID uuid.UUID) ([]model.ServiceConnection, error)
}
