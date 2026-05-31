package workflow

import (
	"fmt"
	"time"

	"controller/activity"
	"launchs/shared/model"
	launchs_shared_temporal "launchs/shared/temporal"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DeployProjectWorkflow は全コンテナを並列でデプロイします。
// スナップショット作成後に呼ばれ、プロジェクト全体を一括更新します。
func DeployProjectWorkflow(ctx workflow.Context, input DeployProjectInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	dbAct := &activity.DBActivity{}
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeDeployProject)

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), nil, nil, nil,
	).Get(ctx, nil)

	// 各コンテナの DeployWorkflow を子ワークフローとして並列起動
	futures := make([]workflow.Future, 0, len(input.Containers))
	for _, container := range input.Containers {
		cwo := workflow.ChildWorkflowOptions{
			WorkflowID:         fmt.Sprintf("%s-%s", launchs_shared_temporal.WorkflowDeploy, container.ContainerID),
			WorkflowRunTimeout: 10 * time.Minute,
			RetryPolicy: &sdktemporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		}
		childCtx := workflow.WithChildOptions(ctx, cwo)
		f := workflow.ExecuteChildWorkflow(childCtx, DeployWorkflow, container)
		futures = append(futures, f)
	}

	// 全子ワークフローの完了を待つ（エラーが出ても全部待つ）
	var lastErr error
	for _, f := range futures {
		if err := f.Get(ctx, nil); err != nil {
			lastErr = err
		}
	}

	finalStatus := model.WorkflowRunStatusSucceeded
	var logMsg *string
	if lastErr != nil {
		finalStatus = model.WorkflowRunStatusFailed
		msg := lastErr.Error()
		logMsg = &msg
	}
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(finalStatus), nil, nil, logMsg,
	).Get(ctx, nil)

	return lastErr
}
