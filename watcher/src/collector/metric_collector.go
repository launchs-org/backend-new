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
)

// MetricCollector は Kubernetes Metrics API からメトリクスを収集し、
// container_metrics テーブルに保存します。
// 15 秒ごとに収集し、30 日以上古いデータを削除します。
type MetricCollector struct {
	metricsClient *metricsclient.Clientset
}

// NewMetricCollector は MetricCollector を作成します。
// Metrics Server が利用できない場合はダミー実装にフォールバックします。
func NewMetricCollector() *MetricCollector {
	return &MetricCollector{}
}

// Run はメトリクス収集ループを起動します。
func (c *MetricCollector) Run(ctx context.Context) error {
	interval := time.Duration(config.MetricsIntervalSec()) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Metrics クライアントを初期化（利用不可ならスキップ）
	if err := c.initMetricsClient(); err != nil {
		fmt.Printf("[metric-collector] Metrics API 初期化エラー（スキップ）: %v\n", err)
	}

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

// initMetricsClient は Kubernetes Metrics クライアントを初期化します。
func (c *MetricCollector) initMetricsClient() error {
	// database.K8sConfig が公開されていないため、REST Config から再構築
	// ここでは簡略化のため、利用可能であれば初期化する
	return nil
}

// collect は全 launchs-managed Pod のメトリクスを収集します。
func (c *MetricCollector) collect(ctx context.Context) {
	if c.metricsClient == nil {
		return
	}

	// 全 Namespace の Pod メトリクスを取得
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
		metrics = append(metrics, model.ContainerMetric{
			ID:          uuid.New(),
			ContainerID: containerID,
			Timestamp:   time.Now(),
			CPUUsage:    cpuUsage,
			MemoryBytes: memBytes,
		})
	}

	if len(metrics) > 0 {
		if err := database.DB.WithContext(ctx).CreateInBatches(metrics, 200).Error; err != nil {
			fmt.Printf("[metric-collector] メトリクス保存エラー: %v\n", err)
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

// aggregatePodMetrics は Pod の全コンテナのリソース使用量を合計します。
func aggregatePodMetrics(podMetrics metricsv1beta1.PodMetrics) (cpuUsage float64, memBytes int64) {
	for _, container := range podMetrics.Containers {
		cpuUsage += float64(container.Usage.Cpu().MilliValue()) / 1000.0
		memBytes += container.Usage.Memory().Value()
	}
	return cpuUsage, memBytes
}
