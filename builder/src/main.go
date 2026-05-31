package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"builder/activity"
	"builder/workflow"

	"launchs/shared/config"
	"launchs/shared/database"
	launchs_temporal "launchs/shared/temporal"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	fmt.Println("[builder] starting...")

	// DB・K8s クライアント初期化
	database.Init()
	database.InitK8s()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 未使用変数の回避（環境変数は activity 内で参照）
	_ = os.Getenv("BUILDER_NAMESPACE")

	// Temporal クライアント初期化
	temporalClient, err := client.Dial(client.Options{
		HostPort: config.TemporalAddress(),
	})
	if err != nil {
		panic("Temporal クライアント接続失敗: " + err.Error())
	}
	defer temporalClient.Close()

	// builder-queue に Temporal ワーカーを起動
	w := worker.New(temporalClient, launchs_temporal.BuilderQueue, worker.Options{})

	// アクティビティ登録
	buildAct := &activity.BuildActivity{TemporalClient: temporalClient}
	w.RegisterActivity(buildAct)

	// ワークフロー登録
	w.RegisterWorkflow(workflow.BuildDeployWorkflow)
	w.RegisterWorkflow(workflow.CancelBuildWorkflow)

	// ワーカー起動（非ブロッキング）
	if err := w.Start(); err != nil {
		panic("Temporal ワーカー起動失敗: " + err.Error())
	}
	defer w.Stop()

	fmt.Println("[builder] worker started, waiting for jobs...")
	<-ctx.Done()
	fmt.Println("[builder] shutting down")
}
