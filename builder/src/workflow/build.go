package workflow

import (
	"fmt"
	"time"

	"builder/activity"

	"launchs/shared/config"
	"launchs/shared/model"
	launchs_temporal "launchs/shared/temporal"

	"github.com/google/uuid"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BuildWorkflowInput は BuildDeployWorkflow への入力です。
// UUID フィールドはすべて string で受け取り、内部で uuid.Parse します。
type BuildWorkflowInput struct {
	ContainerID         string
	BuildJobID          string
	ProjectID           string
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
	// Label は WorkflowRun の表示用補足情報（コンテナ名など）
	Label *string
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
func BuildDeployWorkflow(ctx workflow.Context, input BuildWorkflowInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 35 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	buildAct := &activity.BuildActivity{}

	containerID, _ := uuid.Parse(input.ContainerID)
	buildJobID, _ := uuid.Parse(input.BuildJobID)
	projectID, _ := uuid.Parse(input.ProjectID)
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeBuildDeploy)

	_ = workflow.ExecuteActivity(ctx, buildAct.DBUpsertWorkflowRun,
		projectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	// キャンセル・失敗時に cleanup して WorkflowRun を failed に更新する
	defer func() {
		if ctx.Err() == nil {
			return
		}
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		cleanupAo := workflow.ActivityOptions{
			StartToCloseTimeout: 2 * time.Minute,
			RetryPolicy: &sdktemporal.RetryPolicy{
				MaximumAttempts: 3,
			},
		}
		cleanupCtx = workflow.WithActivityOptions(cleanupCtx, cleanupAo)
		_ = workflow.ExecuteActivity(cleanupCtx, buildAct.DeleteBuildK8sJob, input.BuildJobID).Get(cleanupCtx, nil)
		_ = workflow.ExecuteActivity(cleanupCtx, buildAct.UpdateContainerStatus, containerID, "failed").Get(cleanupCtx, nil)
		_ = workflow.ExecuteActivity(cleanupCtx, buildAct.UpdateBuildJobStatus, buildJobID, "failed", "").Get(cleanupCtx, nil)
		canceled := "canceled"
		_ = workflow.ExecuteActivity(cleanupCtx, buildAct.DBUpsertWorkflowRun,
			projectID, wfID, wfType, string(model.WorkflowRunStatusCanceled), &containerID, input.Label, &canceled,
		).Get(cleanupCtx, nil)
	}()

	failRun := func(err error) {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, buildAct.DBUpsertWorkflowRun,
			projectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
	}

	// 1. ステータスを building に更新
	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateContainerStatus, containerID, "building").Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateBuildJobStatus, buildJobID, "running", "").Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	imageTag := input.BuildJobID
	if len(imageTag) > 8 {
		imageTag = imageTag[:8]
	}
	containerName := input.ContainerID
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
		_ = workflow.ExecuteActivity(ctx, buildAct.UpdateContainerStatus, containerID, "failed").Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, buildAct.UpdateBuildJobStatus, buildJobID, "failed", "").Get(ctx, nil)
		failRun(err)
		return err
	}

	// 3. Image レコードを DB に作成
	var imageID uuid.UUID
	if err := workflow.ExecuteActivity(ctx, buildAct.CreateImageRecord, containerID, buildJobID, buildResult.ImageRef).Get(ctx, &imageID); err != nil {
		failRun(err)
		return err
	}

	// 4. コンテナの current_image_id を更新
	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateContainerImage, containerID, imageID).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 5. BuildJob を complete に更新
	if err := workflow.ExecuteActivity(ctx, buildAct.UpdateBuildJobStatus, buildJobID, "complete", buildResult.ImageRef).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 6. Controller の DeployWorkflow を子ワークフローとして起動
	sizes := config.ResourceSizes()
	size, ok := sizes[input.ResourceSize]
	if !ok {
		size = sizes["small"]
	}

	deployInput := buildDeployControllerInput{
		ContainerID:    containerID,
		ProjectID:      projectID,
		Namespace:      input.Namespace,
		DeploymentName: fmt.Sprintf("%s-%s", "container", containerID.String()),
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
		Label:          input.Label,
	}

	cwo := workflow.ChildWorkflowOptions{
		WorkflowID:         fmt.Sprintf("%s-%s", launchs_temporal.WorkflowDeploy, input.ContainerID),
		TaskQueue:          launchs_temporal.ControllerQueue,
		WorkflowRunTimeout: 10 * time.Minute,
	}
	childCtx := workflow.WithChildOptions(ctx, cwo)

	if err := workflow.ExecuteChildWorkflow(childCtx, launchs_temporal.WorkflowDeploy, deployInput).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, buildAct.DBUpsertWorkflowRun,
		projectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)

	return nil
}

// buildDeployControllerInput は controller の DeployWorkflow への入力を表す中間型です。
// controller パッケージへの直接依存を避けるため、同じフィールドを持つ独自型を定義します。
type buildDeployControllerInput struct {
	ContainerID    uuid.UUID
	ProjectID      uuid.UUID
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
	Label          *string
}
