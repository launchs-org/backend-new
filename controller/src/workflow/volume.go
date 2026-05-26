package workflow

import (
	"time"

	"controller/activity"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CreateVolumeWorkflow は PVC を作成します。
func CreateVolumeWorkflow(ctx workflow.Context, input CreateVolumeInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	pvcAct := &activity.PVCActivity{}
	dbAct := &activity.DBActivity{}

	spec := activity.PVCSpec{
		Namespace:   input.Namespace,
		Name:        input.PVCName,
		StorageSize: input.StorageSize,
	}

	// 実際にPVCを作成
	err := workflow.ExecuteActivity(ctx, pvcAct.PVCCreate, spec).Get(ctx, nil)

	// エラー処理
	if err != nil {
		return err
	}

	// ボリュームを作成済みに更新
	return workflow.ExecuteActivity(ctx, dbAct.DBUpdateVolumeStatus,input.VolumeID, "created").Get(ctx, nil)
}

// DeleteVolumeWorkflow は PVC を削除します。
func DeleteVolumeWorkflow(ctx workflow.Context, input DeleteVolumeInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	pvcAct := &activity.PVCActivity{}

	return workflow.ExecuteActivity(ctx, pvcAct.PVCDelete, input.Namespace, input.PVCName).Get(ctx, nil)
}

// MountVolumeWorkflow はボリュームをマウントした状態で Deployment を再 Apply します。
func MountVolumeWorkflow(ctx workflow.Context, input MountVolumeInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	deployAct := &activity.DeploymentActivity{}
	dbAct := &activity.DBActivity{}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "deploying").Get(ctx, nil); err != nil {
		return err
	}

	var spec activity.DeploymentSpec
	if err := workflow.ExecuteActivity(ctx, dbAct.DBBuildDeploySpec, input.ContainerID, input.Namespace).Get(ctx, &spec); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, spec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	// applying に変更（Pod が Ready になるまで Watcher が監視して running に遷移）
	return workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "applying").Get(ctx, nil)
}

// UnmountVolumeWorkflow はボリュームをアンマウントした状態で Deployment を再 Apply します。
func UnmountVolumeWorkflow(ctx workflow.Context, input UnmountVolumeInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	deployAct := &activity.DeploymentActivity{}
	dbAct := &activity.DBActivity{}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "deploying").Get(ctx, nil); err != nil {
		return err
	}

	var spec activity.DeploymentSpec
	if err := workflow.ExecuteActivity(ctx, dbAct.DBBuildDeploySpec, input.ContainerID, input.Namespace).Get(ctx, &spec); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, spec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	// applying に変更（Pod が Ready になるまで Watcher が監視して running に遷移）
	return workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "applying").Get(ctx, nil)
}
