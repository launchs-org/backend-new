package workflow

import (
	"time"

	"controller/activity"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CreateServiceWorkflow は Kubernetes Service を作成します。
func CreateServiceWorkflow(ctx workflow.Context, input CreateServiceInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	svcAct := &activity.ServiceActivity{}
	return workflow.ExecuteActivity(ctx, svcAct.ServiceApply, input.ServiceSpec).Get(ctx, nil)
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
	return workflow.ExecuteActivity(ctx, svcAct.ServiceDelete, input.Namespace, input.ServiceName).Get(ctx, nil)
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
	return workflow.ExecuteActivity(ctx, ingressAct.IngressApply, input.IngressSpec).Get(ctx, nil)
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
	return workflow.ExecuteActivity(ctx, ingressAct.IngressDelete, input.Namespace, input.IngressName).Get(ctx, nil)
}
