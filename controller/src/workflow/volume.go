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

	spec := activity.PVCSpec{
		Namespace:   input.Namespace,
		Name:        input.PVCName,
		StorageSize: input.StorageSize,
	}

	return workflow.ExecuteActivity(ctx, pvcAct.PVCCreate, spec).Get(ctx, nil)
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

	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, input.DeploySpec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	return workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "running").Get(ctx, nil)
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

	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, input.DeploySpec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		return err
	}

	return workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "running").Get(ctx, nil)
}
