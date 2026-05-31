package workflow

import (
	"time"

	"controller/activity"

	"launchs/shared/config"
	"launchs/shared/model"

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

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeDeploy)
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

	// 1. ステータスを deploying に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, model.ContainerStatusDeploying).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// リソースサイズに対応した CPU/Memory 設定を取得
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

	// 2. Kubernetes Deployment を Apply（Pod が Ready になるまで内部で待機）
	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, spec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		failRun(err)
		return err
	}

	// 3. ステータスを running に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, model.ContainerStatusRunning).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 4. Deployment レコードを DB に記録
	if err := workflow.ExecuteActivity(ctx, dbAct.DBCreateDeploymentRecord, input.ContainerID, input.ImageRef, input.Replicas).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 5. ワークフローID をクリア
	_ = workflow.ExecuteActivity(ctx, dbAct.DBClearContainerWorkflowID, input.ContainerID).Get(ctx, nil)

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)

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

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeRedeploy)
	containerID := input.ContainerID

	// 1. ワークフローの状態を更新
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	failRun := func(err error) {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
	}

	// 1. ステータスを deploying に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "deploying").Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 2. rollout restart（Pod が Ready になるまで内部で待機）
	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentRolloutRestart, input.Namespace, input.DeploymentName).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, "failed").Get(ctx, nil)
		failRun(err)
		return err
	}

	// 3. ステータスを running に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus, input.ContainerID, model.ContainerStatusRunning).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 4. ワークフローID をクリア
	_ = workflow.ExecuteActivity(ctx, dbAct.DBClearContainerWorkflowID, input.ContainerID).Get(ctx, nil)

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)

	return nil
}

// DeleteContainerWorkflow はコンテナの Deployment を削除します。
// K8s リソースが完全に削除された後に DB レコードを削除します。
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

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeDeleteContainer)
	containerID := input.ContainerID

	// 1. ワークフローの状態を更新
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	// 失敗した時のエラー処理
	failRun := func(err error) {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
	}

	// 1. Kubernetes Deployment を削除
	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentDelete, input.Namespace, input.DeploymentName).Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)

		// 3. ワークフローの状態を更新
		failRun(err)

		return err
	}

	// 2. 全リソース削除完了後に DB レコードを削除（WorkflowRun も cascade 削除される）
	if err := workflow.ExecuteActivity(ctx, dbAct.DBDeleteContainer, input.ContainerID).Get(ctx, nil); err != nil {
		// 3. ワークフローの状態を更新
		failRun(err)

		return err
	}

	// 3. ワークフローの状態を更新
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}
