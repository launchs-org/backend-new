package workflow

import (
	"time"

	"controller/activity"

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

	// 1. Kubernetes Deployment のレプリカ数を更新
	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentUpdateReplicas, input.Namespace, input.DeploymentName, input.Replicas).Get(ctx, nil); err != nil {
		return err
	}

	// 2. DB のレプリカ数を更新
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerReplicas, input.ContainerID, input.Replicas).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}
