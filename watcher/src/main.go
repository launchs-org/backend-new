package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"watcher/collector"

	"launchs/shared/config"
	"launchs/shared/database"
	"launchs/shared/leader"
)

func main() {
	fmt.Println("[watcher] starting...")

	database.Init()
	database.InitK8s()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("[watcher] database and k8s client initialized")

	if err := leader.EnsureTable(database.DB); err != nil {
		fmt.Printf("[watcher] リーダーエレクションテーブル作成エラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[watcher] leader election table ready")

	podID := resolvePodID()
	leaseTTL := time.Duration(config.LeaderLeaseTTLSec()) * time.Second
	renewInterval := time.Duration(config.LeaderRenewIntervalSec()) * time.Second
	restartDelay := time.Duration(config.CollectorRestartDelaySec()) * time.Second

	fmt.Printf("[watcher] pod_id=%s  lease_ttl=%s  renew_interval=%s  restart_delay=%s\n",
		podID, leaseTTL, renewInterval, restartDelay)

	elector := leader.New(database.DB, podID, leaseTTL, renewInterval)

	elector.Run(ctx, func(leaderCtx context.Context) error {
		return runCollectors(leaderCtx, restartDelay)
	})

	fmt.Println("[watcher] shutdown complete")
}

// runCollectors は 3 つのコレクターを起動し、全コレクターが終了したら return します。
func runCollectors(ctx context.Context, restartDelay time.Duration) error {
	fmt.Println("[watcher] コレクターを起動します")

	statusCollector := &collector.StatusCollector{}
	logCollector := collector.NewLogCollector()
	metricCollector := collector.NewMetricCollector()

	done := make(chan struct{}, 3)

	go func() {
		runWithRestart(ctx, "status", restartDelay, func() error {
			return statusCollector.Run(ctx)
		})
		done <- struct{}{}
	}()

	go func() {
		runWithRestart(ctx, "log", restartDelay, func() error {
			return logCollector.Run(ctx)
		})
		done <- struct{}{}
	}()

	go func() {
		runWithRestart(ctx, "metric", restartDelay, func() error {
			return metricCollector.Run(ctx)
		})
		done <- struct{}{}
	}()

	fmt.Println("[watcher] 全コレクター起動完了（status / log / metric）")

	for i := 0; i < 3; i++ {
		<-done
	}

	fmt.Println("[watcher] 全コレクター終了")
	return nil
}

// resolvePodID はインスタンス識別子を決定します。
// 優先順位: POD_NAME 環境変数 → ホスト名-UUID先頭8文字 → watcher-UUID先頭8文字
func resolvePodID() string {
	if id := config.WatcherPodID(); id != "" {
		return id
	}
	suffix := uuid.New().String()[:8]
	if host, err := os.Hostname(); err == nil && host != "" {
		return host + "-" + suffix
	}
	return "watcher-" + suffix
}

// runWithRestart はコレクター関数をエラー終了時に restartDelay 後に再起動します。
// context がキャンセルされた場合はループを終了します。
func runWithRestart(ctx context.Context, name string, restartDelay time.Duration, fn func() error) {
	attempt := 0
	for {
		attempt++
		fmt.Printf("[watcher] %s collector 起動（attempt=%d）\n", name, attempt)

		if err := fn(); err != nil {
			fmt.Printf("[watcher] %s collector がエラーで終了しました（attempt=%d）: %v\n", name, attempt, err)
			fmt.Printf("[watcher] %s collector を %s 後に再起動します\n", name, restartDelay)
		} else {
			fmt.Printf("[watcher] %s collector が正常終了しました（ctx キャンセル）\n", name)
			return
		}

		select {
		case <-ctx.Done():
			fmt.Printf("[watcher] %s collector: コンテキストキャンセルにより再起動をスキップします\n", name)
			return
		case <-time.After(restartDelay):
		}
	}
}
