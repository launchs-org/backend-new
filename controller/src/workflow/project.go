package workflow

import (
	"time"

	"controller/activity"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// HarborCredentials は CreateProjectWorkflow の返り値です。
type HarborCredentials struct {
	ProjectName string
	Username    string
	Password    string
}

// CreateProjectWorkflow は Namespace 作成と Harbor プロジェクト初期化を行います。
func CreateProjectWorkflow(ctx workflow.Context, input CreateProjectInput) (*HarborCredentials, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	projectID, _ := uuid.Parse(input.ProjectID)

	nsAct := &activity.NamespaceActivity{}
	harborAct := &activity.HarborActivity{}
	dbAct := &activity.DBActivity{}

	// 1. Kubernetes Namespace を作成
	if err := workflow.ExecuteActivity(ctx, nsAct.NamespaceCreate, input.Namespace, input.ProjectID).Get(ctx, nil); err != nil {
		return nil, err
	}

	// 2. Harbor プロジェクトを作成
	if err := workflow.ExecuteActivity(ctx, harborAct.HarborCreateProject, input.Namespace).Get(ctx, nil); err != nil {
		return nil, err
	}

	// 3. Harbor ロボットアカウントを作成
	var robotResult activity.RobotAccountResult
	if err := workflow.ExecuteActivity(ctx, harborAct.HarborCreateRobotAccount, input.Namespace).Get(ctx, &robotResult); err != nil {
		return nil, err
	}

	// 4. DB に Harbor 認証情報を保存
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateProjectHarbor, projectID, input.Namespace, robotResult.Username, robotResult.Password).Get(ctx, nil); err != nil {
		return nil, err
	}

	return &HarborCredentials{
		ProjectName: input.Namespace,
		Username:    robotResult.Username,
		Password:    robotResult.Password,
	}, nil
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
	if err := workflow.ExecuteActivity(ctx, harborAct.HarborDeleteProject, input.Namespace).Get(ctx, nil); err != nil {
		return err
	}

	// 2. Namespace を削除（配下の全リソースが削除される）
	if err := workflow.ExecuteActivity(ctx, nsAct.NamespaceDelete, input.Namespace).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}
