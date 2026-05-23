package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"watcher/collector"

	"launchs/shared/database"
)

func main() {
	fmt.Println("[watcher] starting...")

	// DB・K8s クライアント初期化
	database.Init()
	database.InitK8s()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("[watcher] initialized database and k8s client")

	// ステータス監視を起動（Pod Watch ベース、エラー時は自動再起動）
	statusCollector := &collector.StatusCollector{}
	go func() {
		for {
			if err := statusCollector.Run(ctx); err != nil {
				fmt.Printf("[watcher] status collector エラー（再起動）: %v\n", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			} else {
				return
			}
		}
	}()

	// ログ収集を起動（3 秒間隔でバッファフラッシュ）
	logCollector := collector.NewLogCollector()
	go func() {
		if err := logCollector.Run(ctx); err != nil {
			fmt.Printf("[watcher] log collector エラー: %v\n", err)
		}
	}()

	// メトリクス収集を起動（15 秒間隔）
	metricCollector := collector.NewMetricCollector()
	go func() {
		if err := metricCollector.Run(ctx); err != nil {
			fmt.Printf("[watcher] metric collector エラー: %v\n", err)
		}
	}()

	fmt.Println("[watcher] all collectors started")
	<-ctx.Done()
	fmt.Println("[watcher] shutting down")

	// 不要な変数回避
	_ = os.Getenv("WATCHER_NAMESPACE")
}
