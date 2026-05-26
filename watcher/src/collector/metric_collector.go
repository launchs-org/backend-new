package collector

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/config"
	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// MetricCollector は Kubernetes Metrics API からメトリクスを収集し、
// container_metrics テーブルに保存します。
type MetricCollector struct {
	metricsClient *metricsclient.Clientset
	k8sClient     kubernetes.Interface
}

// NewMetricCollector は MetricCollector を作成します。
func NewMetricCollector() *MetricCollector {
	return &MetricCollector{}
}

// Run はメトリクス収集ループを起動します。
// 収集間隔は METRICS_INTERVAL_SEC 環境変数で設定できます（デフォルト: 15秒）。
func (c *MetricCollector) Run(ctx context.Context) error {
	if err := c.initMetricsClient(); err != nil {
		fmt.Printf("[metric-collector] Metrics API 初期化エラー（スキップ）: %v\n", err)
	}

	interval := time.Duration(config.MetricsIntervalSec()) * time.Second
	fmt.Printf("[metric-collector] 収集間隔: %v\n", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.collect(ctx)
			c.cleanup(ctx)
		}
	}
}

// initMetricsClient は shared の REST Config を使って Metrics クライアントを初期化します。
func (c *MetricCollector) initMetricsClient() error {
	restConfig := database.K8sRestConfig
	if restConfig == nil {
		return fmt.Errorf("K8s REST config が未初期化です")
	}

	mc, err := metricsclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("Metrics クライアント作成エラー: %w", err)
	}
	c.metricsClient = mc
	c.k8sClient = database.K8sClientset
	fmt.Println("[metric-collector] Metrics クライアント初期化完了")
	return nil
}

// collect は全 launchs-managed Pod のメトリクスを収集します。
func (c *MetricCollector) collect(ctx context.Context) {
	if c.metricsClient == nil {
		return
	}

	podMetricsList, err := c.metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{
		LabelSelector: "launchs-managed=true",
	})
	if err != nil {
		fmt.Printf("[metric-collector] PodMetrics 取得エラー: %v\n", err)
		return
	}

	metrics := make([]model.ContainerMetric, 0, len(podMetricsList.Items))
	for _, podMetrics := range podMetricsList.Items {
		containerIDStr := podMetrics.Labels["container-id"]
		if containerIDStr == "" {
			continue
		}
		containerID, err := uuid.Parse(containerIDStr)
		if err != nil {
			continue
		}

		cpuUsage, memBytes := aggregatePodMetrics(podMetrics)
		cpuRequestCores := c.getPodCPURequestCores(ctx, podMetrics.Namespace, podMetrics.Name)
		fmt.Printf("[metric-collector] pod=%s cpu=%.3f/%gcores mem=%dMi\n", podMetrics.Name, cpuUsage, cpuRequestCores, memBytes/1024/1024)
		metrics = append(metrics, model.ContainerMetric{
			ID:              uuid.New(),
			ContainerID:     containerID,
			PodName:         podMetrics.Name,
			Timestamp:       time.Now(),
			CPUUsage:        cpuUsage,
			CPURequestCores: cpuRequestCores,
			MemoryBytes:     memBytes,
		})
	}

	if len(metrics) > 0 {
		if err := database.DB.WithContext(ctx).CreateInBatches(metrics, 200).Error; err != nil {
			fmt.Printf("[metric-collector] メトリクス保存エラー: %v\n", err)
		} else {
			fmt.Printf("[metric-collector] %d 件のメトリクスを保存しました\n", len(metrics))
		}
	}
}

// cleanup は保持期限を超えた古いメトリクスを削除します。
func (c *MetricCollector) cleanup(ctx context.Context) {
	retentionDays := config.MetricsRetentionDays()
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := database.DB.WithContext(ctx).
		Where("timestamp < ?", cutoff).
		Delete(&model.ContainerMetric{})
	if result.Error != nil {
		fmt.Printf("[metric-collector] 古いメトリクス削除エラー: %v\n", result.Error)
	}
}

// aggregatePodMetrics は Pod の全コンテナの CPU・メモリ使用量を合計します。
// CPU はコア数（例: 0.25 = 250m）、メモリはバイト単位で返します。
func aggregatePodMetrics(podMetrics metricsv1beta1.PodMetrics) (cpuUsage float64, memBytes int64) {
	for _, container := range podMetrics.Containers {
		cpuUsage += float64(container.Usage.Cpu().MilliValue()) / 1000.0
		memBytes += container.Usage.Memory().Value()
	}
	return cpuUsage, memBytes
}

// getPodCPURequestCores は Pod spec から CPU requests の合計をコア数で返します。
// 取得できない場合は 0 を返します。
func (c *MetricCollector) getPodCPURequestCores(ctx context.Context, namespace, podName string) float64 {
	if c.k8sClient == nil {
		return 0
	}
	pod, err := c.k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return 0
	}
	var total float64
	for _, container := range pod.Spec.Containers {
		if req, ok := container.Resources.Requests["cpu"]; ok {
			total += float64(req.MilliValue()) / 1000.0
		}
	}
	return total
}
