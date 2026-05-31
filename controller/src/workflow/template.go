package workflow

import (
	"fmt"
	"time"

	"controller/activity"

	"launchs/shared/config"
	"launchs/shared/model"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DeployTemplateWorkflow はテンプレートからコンテナを作成する統合ワークフローです。
func DeployTemplateWorkflow(ctx workflow.Context, input DeployTemplateInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	dbAct := &activity.DBActivity{}
	pvcAct := &activity.PVCActivity{}
	deployAct := &activity.DeploymentActivity{}
	svcAct := &activity.ServiceActivity{}

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	wfType := string(model.WorkflowRunTypeDeployTemplate)
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
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus,
		input.ContainerID, model.ContainerStatusDeploying).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 2. ボリューム PVC 作成 + 既存ボリュームのマウント設定
	volumeMounts := make([]activity.VolumeMount, 0)
	volumeMounts = append(volumeMounts, input.ExistingVolumeMounts...)

	if input.VolumeRecord != nil {
		volumeID, err := uuid.Parse(input.VolumeRecord.ID)
		if err != nil {
			failRun(fmt.Errorf("invalid volume ID: %w", err))
			return fmt.Errorf("invalid volume ID: %w", err)
		}

		pvcName := fmt.Sprintf("%s-%s", input.VolumeRecord.Name, input.VolumeRecord.ID)
		pvcSpec := activity.PVCSpec{
			Namespace:   input.Namespace,
			Name:        pvcName,
			StorageSize: fmt.Sprintf("%dMi", input.VolumeRecord.SizeMB),
		}

		if err := workflow.ExecuteActivity(ctx, pvcAct.PVCCreate, pvcSpec).Get(ctx, nil); err != nil {
			_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus,
				input.ContainerID, model.ContainerStatusFailed).Get(ctx, nil)
			failRun(fmt.Errorf("PVC 作成失敗: %w", err))
			return fmt.Errorf("PVC 作成失敗: %w", err)
		}

		if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateVolumeStatus,
			volumeID, "created").Get(ctx, nil); err != nil {
			failRun(err)
			return err
		}

		if err := workflow.ExecuteActivity(ctx, dbAct.DBCreateVolumeMountRecord,
			input.ContainerID, volumeID, input.VolumeMountPath).Get(ctx, nil); err != nil {
			failRun(err)
			return err
		}

		volumeMounts = append(volumeMounts, activity.VolumeMount{
			PVCName:   pvcName,
			MountPath: input.VolumeMountPath,
		})
	}

	// 3. リソースサイズに対応した CPU/Memory 設定を取得
	sizes := config.ResourceSizes()
	size, ok := sizes[input.ResourceSize]
	if !ok {
		size = sizes["small"]
	}

	ports := make([]activity.Port, 0, len(input.RouteRecords))
	for _, r := range input.RouteRecords {
		ports = append(ports, activity.Port{Port: r.Port, Protocol: r.Protocol})
	}

	replicas := input.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	spec := activity.DeploymentSpec{
		Namespace:     input.Namespace,
		Name:          input.DeploymentName,
		Image:         input.ImageRef,
		Replicas:      replicas,
		CPURequest:    size.CPURequest,
		CPULimit:      size.CPULimit,
		MemoryRequest: size.MemoryRequest,
		MemoryLimit:   size.MemoryLimit,
		EnvVars:       input.EnvVars,
		Ports:         ports,
		VolumeMounts:  volumeMounts,
		Labels: map[string]string{
			"launchs-managed": "true",
			"container-id":    input.ContainerID.String(),
		},
	}

	if err := workflow.ExecuteActivity(ctx, deployAct.DeploymentApply, spec).Get(ctx, nil); err != nil {
		_ = workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus,
			input.ContainerID, model.ContainerStatusFailed).Get(ctx, nil)
		failRun(fmt.Errorf("Deployment Apply 失敗: %w", err))
		return fmt.Errorf("Deployment Apply 失敗: %w", err)
	}

	// 4. ステータスを running に変更
	if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateContainerStatus,
		input.ContainerID, model.ContainerStatusRunning).Get(ctx, nil); err != nil {
		failRun(err)
		return err
	}

	// 5. K8s Service 作成
	for _, route := range input.RouteRecords {
		routeID, err := uuid.Parse(route.ID)
		if err != nil {
			continue
		}

		svcSpec := activity.ServiceSpec{
			Namespace: input.Namespace,
			Name:      input.DeploymentName,
			Ports: []activity.Port{
				{Port: route.Port, Protocol: route.Protocol},
			},
			SelectorLabels: map[string]string{
				"container-id": input.ContainerID.String(),
			},
		}

		var clusterIP string
		if err := workflow.ExecuteActivity(ctx, svcAct.ServiceApply, svcSpec).Get(ctx, &clusterIP); err != nil {
			workflow.GetLogger(ctx).Warn("Service 作成失敗（続行）",
				"port", route.Port, "error", err)
			continue
		}

		if err := workflow.ExecuteActivity(ctx, dbAct.DBUpdateRouteEndpoint,
			routeID, clusterIP).Get(ctx, nil); err != nil {
			workflow.GetLogger(ctx).Warn("Route ClusterIP 更新失敗（続行）",
				"routeID", route.ID, "error", err)
		}
	}

	// 6. ワークフロー ID クリア
	_ = workflow.ExecuteActivity(ctx, dbAct.DBClearContainerWorkflowID,
		input.ContainerID).Get(ctx, nil)

	_ = workflow.ExecuteActivity(ctx, dbAct.DBUpsertWorkflowRun,
		input.ProjectID, wfID, wfType, string(model.WorkflowRunStatusSucceeded), &containerID, input.Label, nil,
	).Get(ctx, nil)

	return nil
}
