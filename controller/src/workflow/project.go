package workflow

import (
	"time"

	"controller/activity"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CreateProjectWorkflow は Namespace 作成と Harbor プロジェクト初期化を行います。
func CreateProjectWorkflow(ctx workflow.Context, input CreateProjectInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	nsAct := &activity.NamespaceActivity{}
	harborAct := &activity.HarborActivity{}
	dbAct := &activity.DBActivity{}

	// 1. Kubernetes Namespace を作成
	if err := workflow.ExecuteActivity(ctx, nsAct.Create, input.Namespace, input.ProjectID.String()).Get(ctx, nil); err != nil {
		return err
	}

	// 2. Harbor プロジェクトを作成
	if err := workflow.ExecuteActivity(ctx, harborAct.CreateProject, input.Namespace).Get(ctx, nil); err != nil {
		return err
	}

	// 3. Harbor ロボットアカウントを作成
	var robotResult activity.RobotAccountResult
	if err := workflow.ExecuteActivity(ctx, harborAct.CreateRobotAccount, input.Namespace).Get(ctx, &robotResult); err != nil {
		return err
	}

	// 4. DB に Harbor 認証情報を保存
	if err := workflow.ExecuteActivity(ctx, dbAct.UpdateProjectHarbor, input.ProjectID, input.Namespace, robotResult.Username, robotResult.Password).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}

// DeleteProjectWorkflow は Harbor プロジェクトと Namespace を削除します。
// Namespace 削除によって配下のリソースが全て削除されます。
func DeleteProjectWorkflow(ctx workflow.Context, input DeleteProjectInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	harborAct := &activity.HarborActivity{}
	nsAct := &activity.NamespaceActivity{}

	// 1. Harbor プロジェクトを削除（イメージもまとめて削除）
	if err := workflow.ExecuteActivity(ctx, harborAct.DeleteProject, input.Namespace).Get(ctx, nil); err != nil {
		return err
	}

	// 2. Namespace を削除（配下の全リソースが削除される）
	if err := workflow.ExecuteActivity(ctx, nsAct.Delete, input.Namespace).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}
