package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"controller/activity"
	"controller/workflow"

	"launchs/shared/config"
	"launchs/shared/database"
	launchs_temporal "launchs/shared/temporal"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// DB・K8s クライアント初期化
	database.Init()
	database.InitK8s()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Temporal クライアント初期化
	temporalClient, err := client.Dial(client.Options{
		HostPort: config.TemporalAddress(),
	})
	if err != nil {
		panic("Temporal クライアント接続失敗: " + err.Error())
	}
	defer temporalClient.Close()

	// controller-queue に Temporal ワーカーを起動
	w := worker.New(temporalClient, launchs_temporal.ControllerQueue, worker.Options{})

	// アクティビティ登録
	nsAct := &activity.NamespaceActivity{}
	deployAct := &activity.DeploymentActivity{}
	svcAct := &activity.ServiceActivity{}
	ingressAct := &activity.IngressActivity{}
	pvcAct := &activity.PVCActivity{}
	harborAct := &activity.HarborActivity{}
	dbAct := &activity.DBActivity{}

	w.RegisterActivity(nsAct)
	w.RegisterActivity(deployAct)
	w.RegisterActivity(svcAct)
	w.RegisterActivity(ingressAct)
	w.RegisterActivity(pvcAct)
	w.RegisterActivity(harborAct)
	w.RegisterActivity(dbAct)

	// ワークフロー登録
	w.RegisterWorkflow(workflow.CreateProjectWorkflow)
	w.RegisterWorkflow(workflow.DeleteProjectWorkflow)
	w.RegisterWorkflow(workflow.DeployWorkflow)
	w.RegisterWorkflow(workflow.RedeployWorkflow)
	w.RegisterWorkflow(workflow.DeleteContainerWorkflow)
	w.RegisterWorkflow(workflow.ScaleWorkflow)
	w.RegisterWorkflow(workflow.CreateVolumeWorkflow)
	w.RegisterWorkflow(workflow.DeleteVolumeWorkflow)
	w.RegisterWorkflow(workflow.MountVolumeWorkflow)
	w.RegisterWorkflow(workflow.UnmountVolumeWorkflow)
	w.RegisterWorkflow(workflow.CreateServiceWorkflow)
	w.RegisterWorkflow(workflow.DeleteServiceWorkflow)
	w.RegisterWorkflow(workflow.CreateIngressWorkflow)
	w.RegisterWorkflow(workflow.DeleteIngressWorkflow)
	w.RegisterWorkflow(workflow.DeployProjectWorkflow)
	w.RegisterWorkflow(workflow.RestoreSnapshotWorkflow)

	// ワーカー起動（非ブロッキング）
	if err := w.Start(); err != nil {
		panic("Temporal ワーカー起動失敗: " + err.Error())
	}
	defer w.Stop()

	// シグナル待機
	<-ctx.Done()
}
