// Package config は環境変数の読み取りとリソースサイズ設定を提供します。
package config

import (
	"encoding/base64"
	"os"
	"strconv"
)

// ResourceSizeSpec は Small / Medium / Large のリソース要求・制限値です。
type ResourceSizeSpec struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// ResourceSizes は各サイズのリソース仕様を返します。
// 環境変数が未設定の場合は設計書記載のデフォルト値を使います。
func ResourceSizes() map[string]ResourceSizeSpec {
	return map[string]ResourceSizeSpec{
		"small": {
			CPURequest:    getEnv("RESOURCE_SMALL_CPU_REQUEST", "100m"),
			CPULimit:      getEnv("RESOURCE_SMALL_CPU_LIMIT", "500m"),
			MemoryRequest: getEnv("RESOURCE_SMALL_MEM_REQUEST", "128Mi"),
			MemoryLimit:   getEnv("RESOURCE_SMALL_MEM_LIMIT", "256Mi"),
		},
		"medium": {
			CPURequest:    getEnv("RESOURCE_MEDIUM_CPU_REQUEST", "500m"),
			CPULimit:      getEnv("RESOURCE_MEDIUM_CPU_LIMIT", "1000m"),
			MemoryRequest: getEnv("RESOURCE_MEDIUM_MEM_REQUEST", "512Mi"),
			MemoryLimit:   getEnv("RESOURCE_MEDIUM_MEM_LIMIT", "1Gi"),
		},
		"large": {
			CPURequest:    getEnv("RESOURCE_LARGE_CPU_REQUEST", "1000m"),
			CPULimit:      getEnv("RESOURCE_LARGE_CPU_LIMIT", "2000m"),
			MemoryRequest: getEnv("RESOURCE_LARGE_MEM_REQUEST", "1Gi"),
			MemoryLimit:   getEnv("RESOURCE_LARGE_MEM_LIMIT", "2Gi"),
		},
	}
}

// DatabaseURL は PostgreSQL 接続 URL を返します。
func DatabaseURL() string {
	return getEnv("DATABASE_URL", "")
}

// HarborEndpoint は Harbor のエンドポイント URL を返します。
func HarborEndpoint() string {
	return getEnv("HARBOR_ENDPOINT", "https://harbor.main-harbor")
}

// HarborAdminUser は Harbor 管理者ユーザー名を返します。
func HarborAdminUser() string {
	// base64でデコード
	decoded,err := base64.StdEncoding.DecodeString(getEnv("HARBOR_ADMIN_USER", "admin"))
	if err == nil {
		return string(decoded)
	}
	
	return getEnv("HARBOR_ADMIN_USER", "admin")
}

// HarborAdminPassword は Harbor 管理者パスワードを返します。
func HarborAdminPassword() string {
	return getEnv("HARBOR_ADMIN_PASSWORD", "")
}

// HarborRegistry は Harbor レジストリのホスト名を返します（イメージ参照に使用）。
func HarborRegistry() string {
	return getEnv("HARBOR_REGISTRY", "harbor.launchs.org")
}

// TemporalAddress は Temporal サーバーアドレスを返します。
func TemporalAddress() string {
	return getEnv("TEMPORAL_ADDRESS", "temporal:7233")
}

// SnapshotMaxCount はプロジェクトごとのスナップショット保持上限数を返します。
func SnapshotMaxCount() int {
	return getEnvInt("SNAPSHOT_MAX_COUNT", 10)
}

// WorkflowQueueMax はコンテナごとのワークフローキュー上限数を返します。
func WorkflowQueueMax() int {
	return getEnvInt("WORKFLOW_QUEUE_MAX", 5)
}

// LogFlushIntervalSec はログバッファフラッシュ間隔（秒）を返します。
func LogFlushIntervalSec() int {
	return getEnvInt("LOG_FLUSH_INTERVAL_SEC", 1)
}

// MetricsIntervalSec はメトリクス収集間隔（秒）を返します。
func MetricsIntervalSec() int {
	return getEnvInt("METRICS_INTERVAL_SEC", 15)
}

// MetricsRetentionDays はメトリクス保持日数を返します。
func MetricsRetentionDays() int {
	return getEnvInt("METRICS_RETENTION_DAYS", 30)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}
