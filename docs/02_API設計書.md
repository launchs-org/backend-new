# Launchs-org API 設計書

## 基本仕様

- ベースパス: `/api/v1`
- 認証: JWT（既存 Echo Middleware により `userId` を取得）
- レスポンス形式:

```json
// 成功
{ "data": { ... }, "error": null }

// エラー
{ "data": null, "error": { "code": "NOT_FOUND", "message": "container not found" } }
```

- ページネーション:
  - ログ: タイムスタンプカーソルベース（`next_cursor` を返す）
  - メトリクス: `from` / `to` の範囲指定
  - その他リスト: 全件返却

---

## エラーコード一覧

| コード | HTTP ステータス | 説明 |
|--------|----------------|------|
| `UNAUTHORIZED` | 401 | 認証エラー |
| `FORBIDDEN` | 403 | 権限なし |
| `NOT_FOUND` | 404 | リソースが存在しない |
| `CONFLICT` | 409 | リソースが既に存在する |
| `QUEUE_FULL` | 429 | ワークフローキューが上限に達している |
| `INTERNAL_ERROR` | 500 | サーバー内部エラー |

---

## Projects

### プロジェクト一覧取得

```
GET /v1/projects
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "my-project",
      "slug": "my-project",
      "namespace": "project-uuid",
      "container_count": 3,
      "last_deployed_at": "2026-01-01T00:00:00Z",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```

---

### プロジェクト作成

```
POST /v1/projects
```

**リクエスト**
```json
{
  "name": "my-project"
}
```

**レスポンス**
```json
{
  "data": {
    "id": "uuid",
    "name": "my-project",
    "slug": "my-project",
    "namespace": "project-uuid",
    "workflow_id": "temporal-workflow-id",
    "created_at": "2026-01-01T00:00:00Z"
  },
  "error": null
}
```

---

### プロジェクト取得

```
GET /v1/projects/:project_id
```

**レスポンス**
```json
{
  "data": {
    "id": "uuid",
    "name": "my-project",
    "slug": "my-project",
    "namespace": "project-uuid",
    "containers": [...],
    "volumes": [...],
    "env_vars": [...],
    "created_at": "2026-01-01T00:00:00Z"
  },
  "error": null
}
```

---

### プロジェクト削除

```
DELETE /v1/projects/:project_id
```

**レスポンス**
```json
{
  "data": { "workflow_id": "temporal-workflow-id" },
  "error": null
}
```

---

### プロジェクト一括デプロイ

```
POST /v1/projects/:project_id/deploy
```

**レスポンス**
```json
{
  "data": {
    "snapshot_id": "uuid",
    "workflow_ids": ["workflow-id-1", "workflow-id-2"]
  },
  "error": null
}
```

---

### プロジェクトジョブ進捗一覧

```
GET /v1/projects/:project_id/jobs
```

**レスポンス**
```json
{
  "data": [
    {
      "workflow_id": "temporal-workflow-id",
      "workflow_type": "BuildDeployWorkflow",
      "container_id": "uuid",
      "container_name": "my-app",
      "status": "running",
      "started_at": "2026-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```

---

## Project Env Vars

### プロジェクト環境変数一覧取得

```
GET /v1/projects/:project_id/env-vars
```

**レスポンス**
```json
{
  "data": [
    { "id": "uuid", "key": "DATABASE_URL", "value": "postgres://..." }
  ],
  "error": null
}
```

---

### プロジェクト環境変数作成・更新（一括）

```
PUT /v1/projects/:project_id/env-vars
```

**リクエスト**
```json
{
  "env_vars": [
    { "key": "DATABASE_URL", "value": "postgres://..." },
    { "key": "REDIS_URL", "value": "redis://..." }
  ]
}
```

---

### プロジェクト環境変数削除

```
DELETE /v1/projects/:project_id/env-vars
```

**リクエスト**
```json
{ "key": "DATABASE_URL" }
```

---

## Containers

### コンテナ一覧取得

```
GET /v1/projects/:project_id/containers
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "my-app",
      "status": "running",
      "replicas": 2,
      "ready_replicas": 2,
      "failed_replicas": 0,
      "resource_size": "small",
      "active_deploy_workflow_id": null,
      "active_scale_workflow_id": null,
      "pods": [
        {
          "pod_name": "my-app-7d9f8b-xk2p9",
          "status": "Running",
          "ready": true,
          "restart_count": 0,
          "node_name": "node-1",
          "started_at": "2026-01-01T00:00:00Z"
        },
        {
          "pod_name": "my-app-7d9f8b-mn3q1",
          "status": "Running",
          "ready": true,
          "restart_count": 1,
          "node_name": "node-2",
          "started_at": "2026-01-01T00:01:00Z"
        }
      ],
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```

---

### コンテナ取得

```
GET /v1/projects/:project_id/containers/:container_id
```

---

### GitHub からコンテナ作成・デプロイ

```
POST /v1/projects/:project_id/containers/deploy
```

**リクエスト**
```json
{
  "name": "my-app",
  "git_repo": "https://github.com/org/repo",
  "git_branch": "main",
  "git_commit": "abc123",
  "subdir": ".",
  "resource_size": "small",
  "replicas": 1,
  "env_vars": [
    { "key": "PORT", "value": "3000" }
  ],
  "ports": [
    { "port": 3000, "protocol": "TCP" }
  ]
}
```

**レスポンス**
```json
{
  "data": {
    "container_id": "uuid",
    "workflow_id": "temporal-workflow-id"
  },
  "error": null
}
```

---

### テンプレートからコンテナ作成

```
POST /v1/projects/:project_id/containers/from-template
```

**リクエスト**
```json
{
  "name": "my-mysql",
  "template_name": "mysql",
  "resource_size": "medium",
  "params": {
    "MYSQL_ROOT_PASSWORD": "secret",
    "MYSQL_DATABASE": "app"
  },
  "volume_id": "uuid",
  "mount_path": "/var/lib/mysql"
}
```

---

### コンテナ再デプロイ

```
POST /v1/projects/:project_id/containers/:container_id/redeploy
```

**レスポンス**
```json
{
  "data": { "workflow_id": "temporal-workflow-id" },
  "error": null
}
```

---

### コンテナスケール

```
PUT /v1/projects/:project_id/containers/:container_id/scale
```

**リクエスト**
```json
{ "replicas": 3 }
```

---

### コンテナ削除

```
DELETE /v1/projects/:project_id/containers/:container_id
```

---

### コンテナ設定更新

```
PUT /v1/projects/:project_id/containers/:container_id
```

**リクエスト**
```json
{
  "resource_size": "medium",
  "env_vars": [
    { "key": "PORT", "value": "8080" }
  ]
}
```

---

### コンテナ Webhook 作成

```
POST /v1/projects/:project_id/containers/:container_id/webhook
```

**レスポンス**
```json
{
  "data": {
    "webhook_url": "https://api.launchs.org/api/v1/webhooks/{token}",
    "token": "secret-token"
  },
  "error": null
}
```

---

### Webhook 受信エンドポイント

```
POST /v1/webhooks/:token
```

---

## Container Env Vars

### コンテナ環境変数一覧取得

```
GET /v1/projects/:project_id/containers/:container_id/env-vars
```

---

### コンテナ環境変数作成・更新（一括）

```
PUT /v1/projects/:project_id/containers/:container_id/env-vars
```

**リクエスト**
```json
{
  "env_vars": [
    { "key": "PORT", "value": "3000" }
  ]
}
```

---

### コンテナ環境変数削除

```
DELETE /v1/projects/:project_id/containers/:container_id/env-vars
```

**リクエスト**
```json
{ "key": "PORT" }
```

---

## Container Ports

### ポート一覧取得

```
GET /v1/projects/:project_id/containers/:container_id/ports
```

**レスポンス**
```json
{
  "data": [
    { "id": "uuid", "port": 3000, "protocol": "TCP" }
  ],
  "error": null
}
```

---

### ポート追加

```
POST /v1/projects/:project_id/containers/:container_id/ports
```

**リクエスト**
```json
{ "port": 3000, "protocol": "TCP" }
```

---

### ポート削除

```
DELETE /v1/projects/:project_id/containers/:container_id/ports/:port_id
```

---

## Network Routes (Services / IngressRoutes)

### ネットワークルート一覧取得

```
GET /v1/projects/:project_id/containers/:container_id/routes
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "type": "service",
      "port": 3000,
      "protocol": "TCP"
    },
    {
      "id": "uuid",
      "type": "ingress",
      "subdomain": "my-app-project-uuid.launchs.org",
      "port": 3000
    }
  ],
  "error": null
}
```

---

### Kubernetes Service 作成

```
POST /v1/projects/:project_id/containers/:container_id/routes/service
```

**リクエスト**
```json
{ "port": 3000, "protocol": "TCP" }
```

---

### IngressRoute 作成

```
POST /v1/projects/:project_id/containers/:container_id/routes/ingress
```

**リクエスト**
```json
{ "port": 3000 }
```

**レスポンス**
```json
{
  "data": {
    "id": "uuid",
    "subdomain": "my-app-project-uuid.launchs.org",
    "workflow_id": "temporal-workflow-id"
  },
  "error": null
}
```

---

### ルート削除

```
DELETE /v1/projects/:project_id/containers/:container_id/routes/:route_id
```

---

## Logs

### コンテナランタイムログ取得

```
GET /v1/projects/:project_id/containers/:container_id/logs?cursor={timestamp}&limit={n}&pod_name={pod_name}
```

**クエリパラメータ**

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `cursor` | timestamp | このタイムスタンプ以降のログを取得 |
| `limit` | int | 取得件数（デフォルト: 100） |
| `pod_name` | string | Pod 名でフィルタリング（省略時は全 Pod） |

**レスポンス**
```json
{
  "data": {
    "logs": [
      {
        "timestamp": "2026-01-01T00:00:00.000Z",
        "level": "INFO",
        "message": "Server started on port 3000",
        "pod_name": "my-app-7d9f8b-xk2p9"
      }
    ],
    "next_cursor": "2026-01-01T00:00:01.000Z"
  },
  "error": null
}
```

---

### ビルドジョブログ取得

```
GET /v1/projects/:project_id/build-jobs/:build_job_id/logs?cursor={timestamp}&limit={n}
```

**クエリパラメータ**

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `cursor` | timestamp | このタイムスタンプ以降のログを取得 |
| `limit` | int | 取得件数（デフォルト: 100） |

**レスポンス**
```json
{
  "data": {
    "logs": [
      {
        "timestamp": "2026-01-01T00:00:00.000Z",
        "level": "INFO",
        "message": "[git-clone] Cloning into '/workspace/repo'..."
      }
    ],
    "next_cursor": "2026-01-01T00:00:01.000Z"
  },
  "error": null
}
```

---

## Metrics

### コンテナメトリクス取得

```
GET /v1/projects/:project_id/containers/:container_id/metrics?from={timestamp}&to={timestamp}
```

**クエリパラメータ**

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `from` | timestamp | 取得開始日時 |
| `to` | timestamp | 取得終了日時 |

**レスポンス**
```json
{
  "data": {
    "cpu": [
      { "timestamp": "2026-01-01T00:00:00Z", "value": 0.25 }
    ],
    "memory": [
      { "timestamp": "2026-01-01T00:00:00Z", "value": 134217728 }
    ]
  },
  "error": null
}
```

---

## Build Jobs

### ビルドジョブ一覧取得

```
GET /v1/projects/:project_id/containers/:container_id/build-jobs
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "git_repo": "https://github.com/org/repo",
      "git_branch": "main",
      "git_commit": "abc123",
      "status": "complete",
      "temporal_workflow_id": "workflow-id",
      "started_at": "2026-01-01T00:00:00Z",
      "finished_at": "2026-01-01T00:05:00Z",
      "image_id": "uuid"
    }
  ],
  "error": null
}
```

---

### ビルドジョブ停止

```
DELETE /v1/projects/:project_id/build-jobs/:build_job_id
```

---

## Volumes

### ボリューム一覧取得

```
GET /v1/projects/:project_id/volumes
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "mysql-data",
      "size_mb": 10240,
      "storage_class": "standard",
      "status": "bound",
      "mounts": [
        {
          "container_id": "uuid",
          "container_name": "my-mysql",
          "mount_path": "/var/lib/mysql"
        }
      ],
      "created_at": "2026-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```

---

### ボリューム作成

```
POST /v1/projects/:project_id/volumes
```

**リクエスト**
```json
{
  "name": "mysql-data",
  "size_mb": 10240,
  "storage_class": "standard"
}
```

---

### ボリューム削除

```
DELETE /v1/projects/:project_id/volumes/:volume_id
```

---

### ボリュームマウント

```
POST /v1/projects/:project_id/containers/:container_id/mounts
```

**リクエスト**
```json
{
  "volume_id": "uuid",
  "mount_path": "/var/lib/mysql"
}
```

**レスポンス**
```json
{
  "data": { "workflow_id": "temporal-workflow-id" },
  "error": null
}
```

---

### ボリュームアンマウント

```
DELETE /v1/projects/:project_id/containers/:container_id/mounts/:volume_id
```

---

## Templates

### テンプレート一覧取得

```
GET /v1/templates
```

**レスポンス**
```json
{
  "data": [
    {
      "name": "mysql",
      "display_name": "MySQL",
      "category": "database",
      "description": "MySQL 8.0 リレーショナルデータベース",
      "version": "8.0",
      "icon": "database",
      "color": "orange"
    }
  ],
  "error": null
}
```

---

### テンプレート詳細取得

```
GET /v1/templates/:template_name
```

**レスポンス**
```json
{
  "data": {
    "name": "mysql",
    "display_name": "MySQL",
    "env_vars": [
      {
        "key": "MYSQL_ROOT_PASSWORD",
        "required": true,
        "description": "rootパスワード（必須）",
        "auto_generate": true,
        "generate_type": "password",
        "default": ""
      }
    ],
    "volume": {
      "required": true,
      "mount_path": "/var/lib/mysql",
      "default_size_mb": 10240
    }
  },
  "error": null
}
```

---

## Snapshots

### スナップショット一覧取得

```
GET /v1/projects/:project_id/snapshots
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "created_at": "2026-01-01T00:00:00Z",
      "container_count": 3,
      "description": "Deploy #5"
    }
  ],
  "error": null
}
```

---

### スナップショット詳細取得

```
GET /v1/projects/:project_id/snapshots/:snapshot_id
```

**レスポンス**
```json
{
  "data": {
    "id": "uuid",
    "created_at": "2026-01-01T00:00:00Z",
    "containers": [
      {
        "container_id": "uuid",
        "container_name": "my-app",
        "image_tag": "harbor.main-harbor/buildkit/my-app:abc123",
        "replicas": 2,
        "resource_size": "small",
        "env_vars": [...],
        "ports": [...],
        "mounts": [...]
      }
    ],
    "project_env_vars": [...]
  },
  "error": null
}
```

---

### スナップショットから復元

```
POST /v1/projects/:project_id/snapshots/:snapshot_id/restore
```

**レスポンス**
```json
{
  "data": {
    "workflow_id": "temporal-workflow-id"
  },
  "error": null
}
```

---

## Service Connections（フロー可視化）

### コネクション一覧取得

```
GET /v1/projects/:project_id/connections
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "source_container_id": "uuid",
      "source_container_name": "my-app",
      "target_container_id": "uuid",
      "target_container_name": "my-mysql"
    }
  ],
  "error": null
}
```

---

## Container Status Histories

### ステータス履歴取得

```
GET /v1/projects/:project_id/containers/:container_id/status-histories
```

**レスポンス**
```json
{
  "data": [
    {
      "id": "uuid",
      "status": "running",
      "replicas": 2,
      "ready_replicas": 2,
      "failed_replicas": 0,
      "created_at": "2026-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```
