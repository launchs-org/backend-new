package workflow

import (
	"time"

	"builder/activity"

	"launchs/shared/model"
	launchs_temporal "launchs/shared/temporal"

	"github.com/google/uuid"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CancelBuildWorkflow はビルドをキャンセルします。
func CancelBuildWorkflow(ctx workflow.Context, input launchs_temporal.CancelBuildInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	buildAct := &activity.BuildActivity{}

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeCancelBuild)

	if input.ProjectID != "" {
		projectID, _ := uuid.Parse(input.ProjectID)
		var containerIDPtr *uuid.UUID
		if input.ContainerID != "" {
			cid, _ := uuid.Parse(input.ContainerID)
			containerIDPtr = &cid
		}
		_ = workflow.ExecuteActivity(ctx, buildAct.DBUpsertWorkflowRun,
			projectID, wfID, wfType, string(model.WorkflowRunStatusRunning), containerIDPtr, input.Label, nil,
		).Get(ctx, nil)

		defer func() {
			_ = workflow.ExecuteActivity(ctx, buildAct.DBUpsertWorkflowRun,
				projectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), containerIDPtr, input.Label, nil,
			).Get(ctx, nil)
		}()
	}

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
