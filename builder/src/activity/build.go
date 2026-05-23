package activity

import (
	"context"
	"fmt"
	"os"
	"time"

	"builder/railpack"

	"launchs/shared/config"
	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
	"k8s.io/client-go/kubernetes"
)

// BuildInput はビルドアクティビティへの入力です。
type BuildInput struct {
	ContainerID         uuid.UUID
	BuildJobID          uuid.UUID
	GitRepo             string
	GitBranch           string
	GitSubdir           string
	HarborProjectName   string
	HarborRobotUsername string
	HarborRobotPassword string
	ImageName           string
	ImageTag            string
}

// BuildResult はビルドアクティビティの結果です。
type BuildResult struct {
	// Harbor フル参照 (例: harbor.launchs.org/project-xxx/my-app:v1)
	ImageRef string
}

// BuildActivity は railpack を使ってコンテナイメージをビルドします。
type BuildActivity struct{}

// Build はイメージをビルドして Harbor にプッシュします。
// ビルドログは DB（ContainerLog）にバッファリングして保存します。
func (a *BuildActivity) Build(ctx context.Context, input BuildInput) (*BuildResult, error) {
	clientset := database.K8sClientset.(*kubernetes.Clientset)

	registry := config.HarborRegistry()
	imageRef := fmt.Sprintf("%s/%s/%s:%s", registry, input.HarborProjectName, input.ImageName, input.ImageTag)

	// ビルド用 Namespace（デフォルト: buildkit）
	buildNamespace := os.Getenv("BUILDER_NAMESPACE")
	if buildNamespace == "" {
		buildNamespace = "buildkit"
	}

	cfg := railpack.BuildConfig{
		GitRepo:          input.GitRepo,
		GitBranch:        input.GitBranch,
		Subdir:           input.GitSubdir,
		ImageName:        input.ImageName,
		ImageTag:         input.ImageTag,
		RegistryHost:     registry,
		RegistryProject:  input.HarborProjectName,
		RegistryUsername: input.HarborRobotUsername,
		RegistryPassword: input.HarborRobotPassword,
		Namespace:        buildNamespace,
		JobID:            input.BuildJobID.String(),
		Timeout:          30 * time.Minute,
	}

	client, err := railpack.New(clientset, cfg)
	if err != nil {
		return nil, fmt.Errorf("railpack クライアント作成エラー: %w", err)
	}

	// ビルドジョブ起動
	jobID, err := client.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("ビルドジョブ起動エラー: %w", err)
	}

	// ログをストリームして DB に非同期保存
	logCh, _ := client.StreamLogs(ctx, jobID)
	go func() {
		logs := make([]model.ContainerLog, 0, 100)
		buildSource := fmt.Sprintf("build:%s", input.BuildJobID)
		for line := range logCh {
			logs = append(logs, model.ContainerLog{
				ID:          uuid.New(),
				ContainerID: input.ContainerID,
				// ビルドログは "build:{build_job_id}" という PodName で識別
				PodName:   &buildSource,
				Timestamp: time.Now(),
				Level:     "INFO",
				Message:   line,
			})
			if len(logs) >= 100 {
				database.DB.Create(&logs)
				logs = logs[:0]
			}
		}
		if len(logs) > 0 {
			database.DB.Create(&logs)
		}
	}()

	// ビルド完了を待機
	status, err := client.Wait(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("ビルド待機エラー: %w", err)
	}

	if status != railpack.StatusComplete {
		return nil, fmt.Errorf("ビルド失敗 status=%v", status)
	}

	return &BuildResult{ImageRef: imageRef}, nil
}

// UpdateBuildJobStatus は BuildJob のステータスと ImageRef を DB に反映します。
func (a *BuildActivity) UpdateBuildJobStatus(ctx context.Context, buildJobID uuid.UUID, status string, imageRef string) error {
	updates := map[string]interface{}{"status": status}
	if imageRef != "" {
		updates["image_ref"] = imageRef
	}
	result := database.DB.WithContext(ctx).Model(&model.BuildJob{}).
		Where("id = ?", buildJobID).
		Updates(updates)
	return result.Error
}

// CreateImageRecord は Image レコードを DB に作成し、ImageID を返します。
func (a *BuildActivity) CreateImageRecord(ctx context.Context, containerID, buildJobID uuid.UUID, imageRef string) (uuid.UUID, error) {
	imageID := uuid.New()
	img := &model.Image{
		ID:          imageID,
		ContainerID: containerID,
		BuildJobID:  &buildJobID,
		ImageRef:    imageRef,
		Status:      "ready",
	}
	result := database.DB.WithContext(ctx).Create(img)
	if result.Error != nil {
		return uuid.Nil, fmt.Errorf("Image レコード作成エラー: %w", result.Error)
	}
	return imageID, nil
}

// UpdateContainerImage はコンテナの current_image_id を更新します。
func (a *BuildActivity) UpdateContainerImage(ctx context.Context, containerID, imageID uuid.UUID) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("current_image_id", imageID)
	return result.Error
}

// UpdateContainerStatus はコンテナのステータスを更新します。
func (a *BuildActivity) UpdateContainerStatus(ctx context.Context, containerID uuid.UUID, status string) error {
	result := database.DB.WithContext(ctx).Model(&model.Container{}).
		Where("id = ?", containerID).
		Update("status", status)
	return result.Error
}
