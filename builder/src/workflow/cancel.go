package workflow

import (
	"time"

	"builder/activity"

	launchs_temporal "launchs/shared/temporal"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CancelBuildWorkflow はビルドをキャンセルします。
// 1. 実行中の BuildDeployWorkflow に Temporal キャンセルシグナルを送信
// 2. K8s ビルド Job（railpack-{buildJobID}）を削除
// 3. BuildJob ステータスを failed に更新・FinishedAt を記録
func CancelBuildWorkflow(ctx workflow.Context, input launchs_temporal.CancelBuildInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	buildAct := &activity.BuildActivity{}

	// 1. 実行中ワークフローに Temporal キャンセルシグナルを送信
	if input.TemporalWorkflowID != "" {
		_ = workflow.ExecuteActivity(ctx, buildAct.CancelTemporalWorkflow, input.TemporalWorkflowID).Get(ctx, nil)
	}

	// 2. K8s ビルド Job を即座に削除
	_ = workflow.ExecuteActivity(ctx, buildAct.DeleteBuildK8sJob, input.BuildJobID).Get(ctx, nil)

	// 3. FinishedAt を記録し、ステータスを failed に確定
	_ = workflow.ExecuteActivity(ctx, buildAct.FinishBuildJob, input.BuildJobID).Get(ctx, nil)

	return nil
}
