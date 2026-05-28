package collector

import (
	"bufio"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"launchs/shared/config"
	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// LogCollector は launchs-managed=true の Pod からログを収集し、
// バッファリングして ContainerLog テーブルに一括 INSERT します。
type LogCollector struct {
	mu     sync.Mutex
	buffer []model.ContainerLog
	// 現在ストリーム中の Pod をトラッキング（重複起動防止）
	activePods map[string]bool
	podMu      sync.Mutex

	lastActivity atomic.Value // stores time.Time
	watcherErr   chan error    // buffered cap=1: scanExistingPods からのエラー通知
}

// NewLogCollector は LogCollector を作成します。
func NewLogCollector() *LogCollector {
	c := &LogCollector{
		activePods: make(map[string]bool),
		watcherErr: make(chan error, 1),
	}
	c.lastActivity.Store(time.Now())
	return c
}

// Run はフラッシュループと Pod 監視ループを起動します。
func (c *LogCollector) Run(ctx context.Context) error {
	flushInterval := time.Duration(config.LogFlushIntervalSec()) * time.Second
	healthInterval := time.Duration(config.LogHealthCheckIntervalSec()) * time.Second
	staleThreshold := time.Duration(config.LogStaleThresholdSec()) * time.Second

	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()
	healthTicker := time.NewTicker(healthInterval)
	defer healthTicker.Stop()

	// 既存の Running Pod を初期スキャン
	go c.scanExistingPods(ctx)

	// Pod Watch でログストリームを動的に追加
	go c.watchPods(ctx)

	for {
		select {
		case <-ctx.Done():
			c.flush(context.Background()) // 終了前に残りをフラッシュ
			return nil
		case <-flushTicker.C:
			c.flush(ctx)
		case err := <-c.watcherErr:
			fmt.Printf("[log-collector] watcherErr 受信: %v\n", err)
		case <-healthTicker.C:
			if err := c.checkHealth(ctx, staleThreshold); err != nil {
				return err
			}
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
		select {
		case c.watcherErr <- fmt.Errorf("初期 Pod スキャン失敗: %w", err):
		default:
		}
		return
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			go c.streamPodLogs(ctx, pod.Name, pod.Namespace, pod.Labels["container-id"])
		}
	}
}

// watchPods は新規に Running になった Pod のログを自動収集します。
// Watch チャンネルが閉じた場合は指数バックオフで再接続します。
func (c *LogCollector) watchPods(ctx context.Context) {
	backoff := 2 * time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		k8s := database.K8sClientset
		watcher, err := k8s.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
			LabelSelector: "launchs-managed=true",
		})
		if err != nil {
			fmt.Printf("[log-collector] Pod Watch 開始エラー（%s 後に再試行）: %v\n", backoff, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}
		backoff = 2 * time.Second // 成功したらリセット

		channelClosed := c.drainWatcher(ctx, watcher)
		watcher.Stop()

		if !channelClosed {
			// ctx がキャンセルされた
			return
		}
		fmt.Println("[log-collector] Pod Watch チャンネルが閉じられました。再接続します")
	}
}

// drainWatcher はイベントを読み続けます。
// チャンネルが閉じられた場合は true、ctx がキャンセルされた場合は false を返します。
func (c *LogCollector) drainWatcher(ctx context.Context, watcher watch.Interface) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return true
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

	for {
		if err := c.openLogStream(ctx, podName, namespace, containerID); err != nil {
			fmt.Printf("[log-collector] ストリームエラー（%s）: %v\n", podName, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second): // 5秒待って再接続
		}

		// Pod がまだ Running か確認してから再接続
		k8s := database.K8sClientset
		pod, err := k8s.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil || pod.Status.Phase != corev1.PodRunning {
			return // Pod が消えていたら終了
		}
	}
}

// openLogStream はストリームを開いて読み切るまでブロックします。
// ログ行が lineTimeout 秒間届かない場合はウォッチドッグがストリームを強制クローズします。
func (c *LogCollector) openLogStream(ctx context.Context, podName, namespace string, containerID uuid.UUID) error {
	k8s := database.K8sClientset
	logOptions := &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
	}
	req := k8s.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("ストリーム開始失敗: %w", err)
	}
	defer stream.Close()

	lineTimeout := time.Duration(config.LogStaleThresholdSec()) * time.Second
	lastLine := make(chan struct{}, 1)
	wdCtx, cancelWD := context.WithCancel(ctx)
	defer cancelWD()

	// ウォッチドッグ: lineTimeout 内に新しいログ行が届かなければストリームを閉じる
	go func() {
		t := time.NewTimer(lineTimeout)
		defer t.Stop()
		for {
			select {
			case <-wdCtx.Done():
				return
			case <-lastLine:
				if !t.Stop() {
					<-t.C
				}
				t.Reset(lineTimeout)
			case <-t.C:
				fmt.Printf("[log-collector] ストリームタイムアウト（%s）: %s — ストリームを閉じます\n", lineTimeout, podName)
				stream.Close()
				return
			}
		}
	}()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()

		// ウォッチドッグにハートビートを送信
		select {
		case lastLine <- struct{}{}:
		default:
		}

		// ヘルスチェック用に最終活動時刻を更新
		c.lastActivity.Store(time.Now())

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

	cancelWD()
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("スキャンエラー: %w", err)
	}
	return nil
}

// checkHealth はログコレクターの健全性を確認します。
// Running Pod が存在するにもかかわらず staleThreshold 間ログが届いていない場合にエラーを返します。
func (c *LogCollector) checkHealth(ctx context.Context, staleThreshold time.Duration) error {
	c.podMu.Lock()
	activeCount := len(c.activePods)
	c.podMu.Unlock()

	if activeCount == 0 {
		// アクティブな Pod がない場合、k8s に Running Pod が実在するか確認する
		k8s := database.K8sClientset
		pods, err := k8s.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: "launchs-managed=true",
		})
		if err != nil {
			fmt.Printf("[log-collector] ヘルスチェック: Pod 一覧取得エラー（スキップ）: %v\n", err)
			return nil
		}

		runningCount := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				runningCount++
			}
		}

		if runningCount == 0 {
			// クラスターに Running Pod が存在しない — 正常なアイドル状態
			// Pod が新たに起動したときに即座に失敗しないよう lastActivity をリセット
			c.lastActivity.Store(time.Now())
			return nil
		}

		// Running Pod が存在するのにストリームがない
		last := c.lastActivity.Load().(time.Time)
		if time.Since(last) > staleThreshold {
			return fmt.Errorf(
				"ヘルスチェック失敗: %d 個の Running Pod が存在するが %s 間ログストリームが無い（最終活動: %s）",
				runningCount, staleThreshold, last.Format(time.RFC3339),
			)
		}
		return nil
	}

	// アクティブな Pod がある場合、最終活動時刻をチェック
	last := c.lastActivity.Load().(time.Time)
	if time.Since(last) > staleThreshold {
		return fmt.Errorf(
			"ヘルスチェック失敗: %d 個の Pod がアクティブだが %s 間ログ行を受信していない（最終活動: %s）",
			activeCount, staleThreshold, last.Format(time.RFC3339),
		)
	}

	fmt.Printf("[log-collector] ヘルスチェック OK: activePods=%d, 最終活動=%s 前\n",
		activeCount, time.Since(last).Round(time.Second))
	return nil
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

	// ログを標準出力に出力
	for _, log := range logs {
		fmt.Printf("[%s] %s\n", log.Level, log.Message)
	}

	if err := database.DB.WithContext(ctx).CreateInBatches(logs, 300).Error; err != nil {
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

