package workflow

import (
	"time"

	"controller/activity"
	"launchs/shared/model"

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

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeCreateVolume)
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), nil, input.Label, nil,
	).Get(ctx, nil)

	spec := activity.PVCSpec{
		Namespace:   input.Namespace,
		Name:        input.PVCName,
		StorageSize: input.StorageSize,
	}

	if err := workflow.ExecuteActivity(ctx, pvcAct.PVCCreate, spec).Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), nil, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateVolumeStatus, input.VolumeID, "created").Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), nil, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), nil, input.Label, nil,
	).Get(ctx, nil)
	return nil
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
	dbAct := &activity.DBActivity{}

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeDeleteVolume)
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), nil, input.Label, nil,
	).Get(ctx, nil)

	if err := workflow.ExecuteActivity(ctx, pvcAct.PVCDelete, input.Namespace, input.PVCName).Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), nil, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), nil, input.Label, nil,
	).Get(ctx, nil)
	return nil
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

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeMountVolume)
	containerID := input.ContainerID
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	failRun := func(err error) {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
	}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "deploying").Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	var spec activity.DeploymentSpec
	if err := workflow.ExecuteActivity(ctx, dbAct.DBBuildDeploySpec, input.ContainerID, input.Namespace).Get(ctx, &spec); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		failRun(err)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, spec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		failRun(err)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "running").Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)
	return nil
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

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeUnmountVolume)
	containerID := input.ContainerID
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	failRun := func(err error) {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
	}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "deploying").Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	var spec activity.DeploymentSpec
	if err := workflow.ExecuteActivity(ctx, dbAct.DBBuildDeploySpec, input.ContainerID, input.Namespace).Get(ctx, &spec); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		failRun(err)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, spec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		failRun(err)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "running").Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)
	return nil
}
