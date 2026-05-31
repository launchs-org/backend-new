package workflow

import (
	"time"

	"controller/activity"
	"launchs/shared/model"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CreateServiceWorkflow は Kubernetes Service を作成し、ClusterIP を DB に保存します。
func CreateServiceWorkflow(ctx workflow.Context, input CreateServiceInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	dbAct := &activity.DBActivity{}
	svcAct := &activity.ServiceActivity{}

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeCreateService)
	containerID := input.ContainerID
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	var clusterIP string
	if err := workflow.ExecuteActivity(ctx, svcAct.ServiceApply, input.ServiceSpec).Get(ctx, &clusterIP); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateRouteEndpoint, input.RouteID, clusterIP).Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)
	return nil
}

// DeleteServiceWorkflow は Kubernetes Service を削除します。
func DeleteServiceWorkflow(ctx workflow.Context, input DeleteServiceInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	svcAct := &activity.ServiceActivity{}
	dbAct := &activity.DBActivity{}

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeDeleteService)
	containerID := input.ContainerID
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	if err := workflow.ExecuteActivity(ctx, svcAct.ServiceDelete, input.Namespace, input.ServiceName).Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)
	return nil
}

// CreateIngressWorkflow は Traefik IngressRoute を作成します。
func CreateIngressWorkflow(ctx workflow.Context, input CreateIngressInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	ingressAct := &activity.IngressActivity{}
	dbAct := &activity.DBActivity{}

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeCreateIngress)
	containerID := input.ContainerID
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	if err := workflow.ExecuteActivity(ctx, ingressAct.IngressApply, input.IngressSpec).Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)
	return nil
}

// DeleteIngressWorkflow は Traefik IngressRoute を削除します。
func DeleteIngressWorkflow(ctx workflow.Context, input DeleteIngressInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	ingressAct := &activity.IngressActivity{}
	dbAct := &activity.DBActivity{}

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeDeleteIngress)
	containerID := input.ContainerID
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusRunning), &containerID, input.Label, nil,
	).Get(ctx, nil)

	if err := workflow.ExecuteActivity(ctx, ingressAct.IngressDelete, input.Namespace, input.IngressName).Get(ctx, nil); err != nil {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusFailed), &containerID, input.Label, &msg,
		).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)
	return nil
}
