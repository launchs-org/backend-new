package workflow

import (
	"fmt"
	"time"

	"builder/activity"

	"launchs/shared/config"
	launchs_temporal "launchs/shared/temporal"

	"github.com/google/uuid"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BuildWorkflowInput は BuildDeployWorkflow への入力です。
type BuildWorkflowInput struct {
	ContainerID         uuid.UUID
	BuildJobID          uuid.UUID
	ProjectID           uuid.UUID
	Namespace           string
	GitRepo             string
	GitBranch           string
	GitSubdir           string
	HarborProjectName   string
	HarborRobotUsername string
	HarborRobotPassword string
	// デプロイ用の追加情報
	Replicas     int
	ResourceSize string
	EnvVars      []EnvVar
	Ports        []Port
	VolumeMounts []VolumeMount
}

// EnvVar はワークフロー入力用の環境変数です。
type EnvVar struct {
	Key   string
	Value string
}

// Port はワークフロー入力用のポートです。
type Port struct {
	Port     int
	Protocol string
}

// VolumeMount はワークフロー入力用のボリュームマウントです。
type VolumeMount struct {
	PVCName   string
	MountPath string
}

// BuildDeployWorkflow はビルドを実行し、成功したら Controller に DeployWorkflow を起動します。
// 1. ステータスを building に更新
// 2. BuildActivity でイメージをビルドして Harbor にプッシュ
// 3. Image レコードを DB に作成
// 4. コンテナの current_image_id を更新
// 5. Controller キューに DeployWorkflow を起動
func BuildDeployWorkflow(ctx workflow.Context, input BuildWorkflowInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 35 * time.Minute, // ビルドタイムアウト + マージン
		RetryPolicy: &sdktemporal.RetryPolicy{
			MaximumAttempts: 1, // ビルドは冪等でないためリトライしない
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	buildAct := &activity.BuildActivity{}

	// 1. ステータスを building に更新
	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateContainerStatus, input.ContainerID, "building").Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateBuildJobStatus, input.BuildJobID, "running", "").Get(ctx, nil); err != nil {
		return err
	}

	// イメージタグ: build_job_id の先頭8文字
	imageTag := input.BuildJobID.String()
	if len(imageTag) > 8 {
		imageTag = imageTag[:8]
	}
	containerName := input.ContainerID.String()
	if len(containerName) > 8 {
		containerName = containerName[:8]
	}

	buildInput := activity.BuildInput{
		ContainerID:         input.ContainerID,
		BuildJobID:          input.BuildJobID,
		GitRepo:             input.GitRepo,
		GitBranch:           input.GitBranch,
		GitSubdir:           input.GitSubdir,
		HarborProjectName:   input.HarborProjectName,
		HarborRobotUsername: input.HarborRobotUsername,
		HarborRobotPassword: input.HarborRobotPassword,
		ImageName:           containerName,
		ImageTag:            imageTag,
	}

	// 2. イメージビルド（最大30分）
	var buildResult activity.BuildResult
	if err := workflow.ExecuteActivity(ctx, buildAct.Build, buildInput).Get(ctx, &buildResult); err != nil {
		// ビルド失敗時はステータスを failed に更新
		_ = workflow.ExecuteActivity(ctx, buildAct.UpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, buildAct.UpdateBuildJobStatus, input.BuildJobID, "failed", "").Get(ctx, nil)
		return err
	}

	// 3. Image レコードを DB に作成
	var imageID uuid.UUID
	if err := workflow.ExecuteActivity(ctx, buildAct.CreateImageRecord, input.ContainerID, input.BuildJobID, buildResult.ImageRef).Get(ctx, &imageID); err != nil {
		return err
	}

	// 4. コンテナの current_image_id を更新
	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateContainerImage, input.ContainerID, imageID).Get(ctx, nil); err != nil {
		return err
	}

	// 5. BuildJob を complete に更新
	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateBuildJobStatus, input.BuildJobID, "complete", buildResult.ImageRef).Get(ctx, nil); err != nil {
		return err
	}

	// 6. Controller の DeployWorkflow を子ワークフローとして起動
	sizes := config.ResourceSizes()
	size, ok := sizes[input.ResourceSize]
	if !ok {
		size = sizes["small"]
	}

	// EnvVars / Ports / VolumeMounts を controller の型に変換
	// （controller パッケージに依存しないためここでは workflow.EnvVar を使う）
	deployInput := buildDeployControllerInput{
		ContainerID:    input.ContainerID,
		Namespace:      input.Namespace,
		DeploymentName: fmt.Sprintf("%s-%s", containerName, containerName),
		ImageRef:       buildResult.ImageRef,
		Replicas:       input.Replicas,
		ResourceSize:   input.ResourceSize,
		CPURequest:     size.CPURequest,
		CPULimit:       size.CPULimit,
		MemoryRequest:  size.MemoryRequest,
		MemoryLimit:    size.MemoryLimit,
		EnvVars:        input.EnvVars,
		Ports:          input.Ports,
		VolumeMounts:   input.VolumeMounts,
	}

	cwo := workflow.ChildWorkflowOptions{
		WorkflowID:         fmt.Sprintf("%s-%s", launchs_temporal.WorkflowDeploy, input.ContainerID),
		TaskQueue:          launchs_temporal.ControllerQueue,
		WorkflowRunTimeout: 10 * time.Minute,
	}
	childCtx := workflow.WithChildOptions(ctx, cwo)

	return workflow.ExecuteChildWorkflow(childCtx, launchs_temporal.WorkflowDeploy, deployInput).Get(ctx, nil)
}

// buildDeployControllerInput は controller の DeployWorkflow への入力を表す中間型です。
// controller パッケージへの直接依存を避けるため、同じフィールドを持つ独自型を定義します。
type buildDeployControllerInput struct {
	ContainerID    uuid.UUID
	Namespace      string
	DeploymentName string
	ImageRef       string
	Replicas       int
	ResourceSize   string
	CPURequest     string
	CPULimit       string
	MemoryRequest  string
	MemoryLimit    string
	EnvVars        []EnvVar
	Ports          []Port
	VolumeMounts   []VolumeMount
}
