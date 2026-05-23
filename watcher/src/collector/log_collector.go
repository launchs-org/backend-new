package collector

import (
	"bufio"
	"context"
	"fmt"
	"sync"
	"time"

	"launchs/shared/config"
	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogCollector は launchs-managed=true の Pod からログを収集し、
// バッファリングして ContainerLog テーブルに一括 INSERT します。
type LogCollector struct {
	mu     sync.Mutex
	buffer []model.ContainerLog
	// 現在ストリーム中の Pod をトラッキング（重複起動防止）
	activePods map[string]bool
	podMu      sync.Mutex
}

// NewLogCollector は LogCollector を作成します。
func NewLogCollector() *LogCollector {
	return &LogCollector{
		activePods: make(map[string]bool),
	}
}

// Run はフラッシュループと Pod 監視ループを起動します。
func (c *LogCollector) Run(ctx context.Context) error {
	flushInterval := time.Duration(config.LogFlushIntervalSec()) * time.Second
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	// 既存の Running Pod を初期スキャン
	go c.scanExistingPods(ctx)

	// Pod Watch でログストリームを動的に追加
	go c.watchPods(ctx)

	for {
		select {
		case <-ctx.Done():
			c.flush(context.Background()) // 終了前に残りをフラッシュ
			return nil
		case <-ticker.C:
			c.flush(ctx)
		}
	}
}

// scanExistingPods は起動時に既存の Running Pod を全て収集対象にします。
func (c *LogCollector) scanExistingPods(ctx context.Context) {
	k8s := database.K8sClientset
	pods, err := k8s.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: "launchs-managed=true",
	})
	if err != nil {
		fmt.Printf("[log-collector] 初期 Pod スキャンエラー: %v\n", err)
		return
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			go c.streamPodLogs(ctx, pod.Name, pod.Namespace, pod.Labels["container-id"])
		}
	}
}

// watchPods は新規に Running になった Pod のログを自動収集します。
func (c *LogCollector) watchPods(ctx context.Context) {
	k8s := database.K8sClientset
	watcher, err := k8s.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
		LabelSelector: "launchs-managed=true",
	})
	if err != nil {
		fmt.Printf("[log-collector] Pod Watch 開始エラー: %v\n", err)
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			if pod.Status.Phase == corev1.PodRunning {
				containerIDStr := pod.Labels["container-id"]
				go c.streamPodLogs(ctx, pod.Name, pod.Namespace, containerIDStr)
			}
		}
	}
}

// streamPodLogs は指定した Pod のログをストリームして buffer に追加します。
// 同一 Pod の重複起動を防ぐためロックを使います。
func (c *LogCollector) streamPodLogs(ctx context.Context, podName, namespace, containerIDStr string) {
	c.podMu.Lock()
	if c.activePods[podName] {
		c.podMu.Unlock()
		return
	}
	c.activePods[podName] = true
	c.podMu.Unlock()

	defer func() {
		c.podMu.Lock()
		delete(c.activePods, podName)
		c.podMu.Unlock()
	}()

	containerID, err := uuid.Parse(containerIDStr)
	if err != nil {
		return
	}

	k8s := database.K8sClientset
	// sinceSeconds=0 で Pod 起動からの全ログを取得し、Follow で追い続ける
	logOptions := &corev1.PodLogOptions{
		Follow:    true,
		Timestamps: true,
	}
	req := k8s.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		return
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		level := detectLogLevel(line)

		c.mu.Lock()
		c.buffer = append(c.buffer, model.ContainerLog{
			ID:          uuid.New(),
			ContainerID: containerID,
			PodName:     &podName,
			Timestamp:   time.Now(),
			Level:       level,
			Message:     line,
		})
		c.mu.Unlock()
	}
}

// flush はバッファを DB に一括 INSERT してクリアします。
func (c *LogCollector) flush(ctx context.Context) {
	c.mu.Lock()
	if len(c.buffer) == 0 {
		c.mu.Unlock()
		return
	}
	logs := make([]model.ContainerLog, len(c.buffer))
	copy(logs, c.buffer)
	c.buffer = c.buffer[:0]
	c.mu.Unlock()

	if err := database.DB.WithContext(ctx).CreateInBatches(logs, 500).Error; err != nil {
		fmt.Printf("[log-collector] ログ保存エラー: %v\n", err)
	}
}

// detectLogLevel はログ行からレベルを推定します。
func detectLogLevel(line string) string {
	for _, kw := range []string{"ERROR", "error", "FATAL", "fatal", "CRIT", "crit"} {
		for i := 0; i < len(line)-len(kw)+1; i++ {
			if line[i:i+len(kw)] == kw {
				return "ERROR"
			}
		}
	}
	for _, kw := range []string{"WARN", "warn", "WARNING", "warning"} {
		for i := 0; i < len(line)-len(kw)+1; i++ {
			if line[i:i+len(kw)] == kw {
				return "WARN"
			}
		}
	}
	for _, kw := range []string{"DEBUG", "debug", "TRACE", "trace"} {
		for i := 0; i < len(line)-len(kw)+1; i++ {
			if line[i:i+len(kw)] == kw {
				return "DEBUG"
			}
		}
	}
	return "INFO"
}
