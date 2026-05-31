package workflow

import (
	"launchs/shared/model"
	"time"

	"controller/activity"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func projectWorkflowID(ctx workflow.Context) string {
	return workflow.GetInfo(ctx).WorkflowExecution.ID
}

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
	wfID := projectWorkflowID(ctx)

	nsAct := &activity.NamespaceActivity{}
	harborAct := &activity.HarborActivity{}
	dbAct := &activity.DBActivity{}

	wfType := string(model.WorkflowRunTypeCreateProject)
	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		projectID, wfID, wfType, string(model.WorkflowRunStatusRunning), nil, nil, nil,
	).Get(ctx, nil)

	logOnFail := func(err error) {
		if err != nil {
			msg := err.Error()
			_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
				projectID, wfID, wfType, string(model.WorkflowRunStatusFailed), nil, nil, &msg,
			).Get(ctx, nil)
		}
	}

	// 1. Kubernetes Namespace を作成
	if err := workflow.ExecuteActivity(ctx, nsAct.NamespaceCreate, input.Namespace, input.ProjectID).Get(ctx, nil); err != nil {
		logOnFail(err)
		return nil, err
	}

	// 2. Harbor プロジェクトを作成
	if err := workflow.ExecuteActivity(ctx, harborAct.HarborCreateProject, input.Namespace).Get(ctx, nil); err != nil {
		logOnFail(err)
		return nil, err
	}

	// 3. Harbor ロボットアカウントを作成
	var robotResult activity.RobotAccountResult
	if err := workflow.ExecuteActivity(ctx, harborAct.HarborCreateRobotAccount, input.Namespace).Get(ctx, &robotResult); err != nil {
		logOnFail(err)
		return nil, err
	}

	// 4. DB に Harbor 認証情報を保存
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateProjectHarbor, projectID, input.Namespace, robotResult.Username, robotResult.Password).Get(ctx, nil); err != nil {
		logOnFail(err)
		return nil, err
	}

	// 5. DB を更新して作成済みにする
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateProjectStatus, projectID, model.ProjectStatusActive).Get(ctx, nil); err != nil {
		logOnFail(err)
		return nil, err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		projectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), nil, nil, nil,
	).Get(ctx, nil)

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
		StartToCloseTimeout: 15 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	harborAct := &activity.HarborActivity{}
	nsAct := &activity.NamespaceActivity{}
	dbAct := &activity.DBActivity{}

	projectID, _ := uuid.Parse(input.ProjectID)
	wfID := projectWorkflowID(ctx)
	wfType := string(model.WorkflowRunTypeDeleteProject)

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		projectID, wfID, wfType, string(model.WorkflowRunStatusRunning), nil, nil, nil,
	).Get(ctx, nil)

	// fail run を作成
	failRun := func(err error) {
		msg := err.Error()
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
			projectID, wfID, wfType, string(model.WorkflowRunStatusFailed), nil, nil, &msg,
		).Get(ctx, nil)
	}

	// 1. Harbor プロジェクトを削除（イメージもまとめて削除）
	if err := workflow.ExecuteActivity(ctx, harborAct.HarborDeleteProject, input.Namespace).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 2. Namespace を削除（配下の全リソースが削除される）
	if err := workflow.ExecuteActivity(ctx, nsAct.NamespaceDelete, input.Namespace).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 全てを削除し終えた場合にDBからも削除（WorkflowRun も cascade 削除される）
	if err := workflow.ExecuteActivity(ctx, dbAct.DBDeleteProject, input.ProjectID).Get(ctx, nil); err != nil {
		return err
	}

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		projectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), nil, nil, nil,
	).Get(ctx, nil)

	return nil
}
