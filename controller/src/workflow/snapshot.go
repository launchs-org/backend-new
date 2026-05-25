package workflow

import (
	"fmt"
	"time"

	"controller/activity"

	launchs_shared_temporal "launchs/shared/temporal"

	"github.com/google/uuid"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// RestoreSnapshotWorkflow はスナップショットから全コンテナを復元します。
// スナップショット内の各コンテナ設定を使って DeployWorkflow を並列起動します。
func RestoreSnapshotWorkflow(ctx workflow.Context, input RestoreSnapshotInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	futures := make([]workflow.Future, 0, len(input.SnapshotData.Containers))
	for _, c := range input.SnapshotData.Containers {
		// 環境変数を変換（プロジェクト変数を先に追加し、コンテナ変数で上書き）
		envVars := make([]activity.EnvVar, 0, len(c.EnvVars)+len(input.SnapshotData.ProjectEnvVars))
		for _, e := range input.SnapshotData.ProjectEnvVars {
			envVars = append(envVars, activity.EnvVar{Key: e.Key, Value: e.Value})
		}
		for _, e := range c.EnvVars {
			envVars = append(envVars, activity.EnvVar{Key: e.Key, Value: e.Value})
		}

		// ポートを変換
		ports := make([]activity.Port, 0, len(c.Ports))
		for _, p := range c.Ports {
			ports = append(ports, activity.Port{Port: p.Port, Protocol: p.Protocol})
		}

		// ボリュームマウントを変換
		mounts := make([]activity.VolumeMount, 0, len(c.Mounts))
		for _, m := range c.Mounts {
			mounts = append(mounts, activity.VolumeMount{
				PVCName:   fmt.Sprintf("%s-%s", c.ContainerName, m.VolumeID),
				MountPath: m.MountPath,
			})
		}

		// ContainerID を uuid.UUID にパース
		containerID, _ := uuid.Parse(c.ContainerID)

		deployInput := DeployInput{
			ContainerID:    containerID,
			Namespace:      input.Namespace,
			DeploymentName: fmt.Sprintf("%s-%s", "container", c.ContainerID),
			ImageRef:       c.ImageTag,
			Replicas:       c.Replicas,
			ResourceSize:   c.ResourceSize,
			EnvVars:        envVars,
			Ports:          ports,
			VolumeMounts:   mounts,
		}

		cwo := workflow.ChildWorkflowOptions{
			WorkflowID:         fmt.Sprintf("%s-%s", launchs_shared_temporal.WorkflowDeploy, c.ContainerID),
			WorkflowRunTimeout: 10 * time.Minute,
		}
		childCtx := workflow.WithChildOptions(ctx, cwo)
		f := workflow.ExecuteChildWorkflow(childCtx, DeployWorkflow, deployInput)
		futures = append(futures, f)
	}

	var lastErr error
	for _, f := range futures {
		if err := f.Get(ctx, nil); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
