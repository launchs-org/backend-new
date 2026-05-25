package workflow

import (
	"time"

	"controller/activity"

	"launchs/shared/model"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ScaleWorkflow はコンテナのレプリカ数を変更します。
func ScaleWorkflow(ctx workflow.Context, input ScaleInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	deployAct := &activity.DeploymentActivity{}
	dbAct := &activity.DBActivity{}

	// 1. ステータスを applying に変更（Pod 増減が完了するまで Watcher が監視）
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, model.ContainerStatusApplying).Get(ctx, nil); err != nil {
		return err
	}

	// 2. Kubernetes Deployment のレプリカ数を更新
	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentUpdateReplicas, input.Namespace, input.DeploymentName, input.Replicas).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, model.ContainerStatusFailed).Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, dbAct.DBClearContainerScaleWorkflowID, input.ContainerID).Get(ctx, nil)
		return err
	}

	// 3. DB のレプリカ数を更新
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerReplicas, input.ContainerID, input.Replicas).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBClearContainerScaleWorkflowID, input.ContainerID).Get(ctx, nil)
		return err
	}

	// 4. スケールワークフローID をクリア（以降のステータス管理を Watcher に委譲）
	_ = workflow.ExecuteActivity(ctx, dbAct.DBClearContainerScaleWorkflowID, input.ContainerID).Get(ctx, nil)

	return nil
}
