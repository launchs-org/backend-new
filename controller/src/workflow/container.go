package workflow

import (
	"time"

	"controller/activity"

	"launchs/shared/config"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DeployWorkflow はコンテナを Kubernetes にデプロイします。
// ステータスを deploying → running と更新し、Deployment を Apply します。
func DeployWorkflow(ctx workflow.Context, input DeployInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	dbAct := &activity.DBActivity{}
	deployAct := &activity.DeploymentActivity{}

	// 1. ステータスを deploying に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.UpdateContainerStatus, input.ContainerID, "deploying").Get(ctx, nil); err != nil {
		return err
	}

	// リソースサイズに対応した CPU/Memory 設定を取得
	// DeployInput に直接指定されていれば優先、なければ ResourceSize から解決
	cpuReq := input.CPURequest
	cpuLim := input.CPULimit
	memReq := input.MemoryRequest
	memLim := input.MemoryLimit
	if cpuReq == "" || cpuLim == "" || memReq == "" || memLim == "" {
		sizes := config.ResourceSizes()
		size, ok := sizes[input.ResourceSize]
		if !ok {
			size = sizes["small"]
		}
		if cpuReq == "" {
			cpuReq = size.CPURequest
		}
		if cpuLim == "" {
			cpuLim = size.CPULimit
		}
		if memReq == "" {
			memReq = size.MemoryRequest
		}
		if memLim == "" {
			memLim = size.MemoryLimit
		}
	}

	spec := activity.DeploymentSpec{
		Namespace:     input.Namespace,
		Name:          input.DeploymentName,
		Image:         input.ImageRef,
		Replicas:      input.Replicas,
		CPURequest:    cpuReq,
		CPULimit:      cpuLim,
		MemoryRequest: memReq,
		MemoryLimit:   memLim,
		EnvVars:       input.EnvVars,
		Ports:         input.Ports,
		VolumeMounts:  input.VolumeMounts,
		Labels: map[string]string{
			"launchs-managed": "true",
			"container-id":    input.ContainerID.String(),
		},
	}

	// 2. Kubernetes Deployment を Apply
	if err := workflow.ExecuteActivity(ctx, deployAct.CreateOrUpdate, spec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.UpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	// 3. ステータスを running に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.UpdateContainerStatus, input.ContainerID, "running").Get(ctx, nil); err != nil {
		return err
	}

	// 4. Deployment レコードを DB に記録
	if err := workflow.ExecuteActivity(ctx, dbAct.CreateDeploymentRecord, input.ContainerID, input.ImageRef, input.Replicas).Get(ctx, nil); err != nil {
		return err
	}

	// 5. ワークフローID をクリア
	_ = workflow.ExecuteActivity(ctx, dbAct.ClearContainerWorkflowID, input.ContainerID).Get(ctx, nil)

	return nil
}

// RedeployWorkflow は既存コンテナを rollout restart します。
func RedeployWorkflow(ctx workflow.Context, input RedeployInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	dbAct := &activity.DBActivity{}
	deployAct := &activity.DeploymentActivity{}

	// 1. ステータスを deploying に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.UpdateContainerStatus, input.ContainerID, "deploying").Get(ctx, nil); err != nil {
		return err
	}

	// 2. rollout restart
	if err := workflow.ExecuteActivity(ctx, deployAct.RolloutRestart, input.Namespace, input.DeploymentName).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.UpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	// 3. ステータスを running に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.UpdateContainerStatus, input.ContainerID, "running").Get(ctx, nil); err != nil {
		return err
	}

	return nil
}

// DeleteContainerWorkflow はコンテナの Deployment を削除します。
func DeleteContainerWorkflow(ctx workflow.Context, input DeleteContainerInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	dbAct := &activity.DBActivity{}
	deployAct := &activity.DeploymentActivity{}

	// 1. ステータスを stopped に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.UpdateContainerStatus, input.ContainerID, "stopped").Get(ctx, nil); err != nil {
		return err
	}

	// 2. Kubernetes Deployment を削除
	if err := workflow.ExecuteActivity(ctx, deployAct.Delete, input.Namespace, input.DeploymentName).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}
