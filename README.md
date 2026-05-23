# Launchs-org Backend

Kubernetes 上でコンテナアプリケーションを自動デプロイ・管理するための PaaS バックエンド。

## アーキテクチャ概要

```
┌─────────────┐   JWT認証   ┌───────────┐
│   Frontend  │ ──────────► │  Backend  │  REST API (Echo v5)
└─────────────┘             └─────┬─────┘
                                  │ Temporal ワークフロー起動
                    ┌─────────────┼──────────────┐
                    ▼             ▼              ▼
              ┌──────────┐ ┌──────────┐ ┌──────────┐
              │Controller│ │ Builder  │ │ Watcher  │
              │          │ │          │ │          │
              │K8s操作   │ │railpack  │ │Pod監視   │
              │Harbor操作│ │ビルド    │ │ログ収集  │
              └──────────┘ └──────────┘ │メトリクス│
                                        └──────────┘
                    ↑ 全ワーカーは Temporal で協調動作 ↑

共通: PostgreSQL (GORM) / Redis / Temporal
```

| コンポーネント | 役割 |
|---|---|
| `backend` | REST API サーバー（Echo v5）。認証・ルーティング・ワークフロー起動 |
| `controller` | Temporal ワーカー。Kubernetes リソース操作・Harbor API 操作 |
| `builder` | Temporal ワーカー。railpack で Git からイメージをビルドし Harbor にプッシュ |
| `watcher` | Pod Watch でステータス監視・ログ収集・Metrics API でメトリクス収集 |
| `shared` | 全コンポーネント共通のモデル・設定・エラー型 |

---

## 必要な外部サービス

| サービス | 用途 | デフォルト接続先 |
|---|---|---|
| PostgreSQL 16 | メインDB（GORM AutoMigrate） | `db:5432 / maindb` |
| Temporal | ワークフローエンジン | `temporal:7233` |
| Harbor | コンテナレジストリ | `harbor.launchs.org` |
| Kubernetes | コンテナ実行環境 | `~/.kube/config` または InCluster |
| Traefik | Ingress（IngressRoute CRD） | クラスター内 |
| authbase | JWT 発行（Ed25519） | `auth:9000` |

---

## ローカル開発環境のセットアップ

### 前提条件

- Docker / Docker Compose
- [Taskfile](https://taskfile.dev) （`brew install go-task` 等でインストール）
- kubectl + kubeconfig（`~/.kube/config` に配置済み）
- Temporal Server（別途起動、または Kubernetes クラスター内）

### 1. 初回セットアップ

```bash
# JWT キー生成 + 全コンテナ起動
task setup
```

`task setup` は以下を実行します:
1. SSL 証明書（Nginx 用）と Ed25519 キーペア（JWT 用）を生成
2. 全サービスを `docker compose up -d --build` で起動

起動後、以下のサービスが使用可能になります:

| サービス | URL |
|---|---|
| Backend API | `http://localhost:8090` |
| Builder | `http://localhost:8091` |
| Nginx (HTTPS) | `https://localhost:8950` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |

### 2. Taskfile コマンド一覧

```bash
task setup      # 初回セットアップ（キー生成 + 起動）
task up         # docker compose up -d --build
task down       # docker compose down
task clean      # コンテナ・イメージ・ボリューム全削除
task genkey     # JWT / SSL キー再生成
task dbshell    # maindb に psql で接続
task dbreset    # PostgreSQL ボリュームをリセット
```

### 3. ログ確認

```bash
docker compose logs -f              # 全サービス
docker compose logs -f app          # backend のみ
docker compose logs -f controller
docker compose logs -f builder
docker compose logs -f watcher
```

---

## 環境変数

設定ファイルは `config/` ディレクトリに置く（docker-compose が自動ロード）。

### backend / controller / builder / watcher 共通

| 変数 | 説明 | デフォルト |
|---|---|---|
| `DATABASE_DSN` | PostgreSQL 接続 DSN | `host=db port=5432 ...` |
| `TEMPORAL_ADDRESS` | Temporal サーバーアドレス | `temporal:7233` |

### backend (`config/app.env`)

| 変数 | 説明 | デフォルト |
|---|---|---|
| `PORT` | HTTP ポート | `8080` |
| `JWT_PUBLIC_KEY_PATH` | Ed25519 公開鍵パス | — |
| `TEMPLATE_DIR` | テンプレート YAML ディレクトリ | `/templates` |
| `SNAPSHOT_MAX_COUNT` | スナップショット保持上限 | `10` |
| `HARBOR_ENDPOINT` | Harbor API エンドポイント | `https://harbor.launchs.org` |
| `HARBOR_REGISTRY` | イメージ参照用レジストリホスト | `harbor.launchs.org` |

### controller (`config/controller.env`)

| 変数 | 説明 | デフォルト |
|---|---|---|
| `HARBOR_ENDPOINT` | Harbor API エンドポイント | `https://harbor.launchs.org` |
| `HARBOR_ADMIN_USER` | Harbor 管理者ユーザー | `admin` |
| `HARBOR_ADMIN_PASSWORD` | Harbor 管理者パスワード | — |

### builder (`config/builder.env`)

| 変数 | 説明 | デフォルト |
|---|---|---|
| `BUILDER_NAMESPACE` | BuildKit Job を起動する Namespace | `buildkit` |
| `HARBOR_REGISTRY` | イメージプッシュ先レジストリ | `harbor.launchs.org` |

### watcher (`config/watcher.env`)

| 変数 | 説明 | デフォルト |
|---|---|---|
| `LOG_FLUSH_INTERVAL_SEC` | ログバッファフラッシュ間隔（秒） | `3` |
| `METRICS_INTERVAL_SEC` | メトリクス収集間隔（秒） | `15` |
| `METRICS_RETENTION_DAYS` | メトリクス保持日数 | `30` |

### リソースサイズ設定（backend / controller 共通）

| 変数 | デフォルト |
|---|---|
| `RESOURCE_SMALL_CPU_REQUEST` / `_LIMIT` | `100m` / `500m` |
| `RESOURCE_SMALL_MEM_REQUEST` / `_LIMIT` | `128Mi` / `256Mi` |
| `RESOURCE_MEDIUM_CPU_REQUEST` / `_LIMIT` | `500m` / `1000m` |
| `RESOURCE_MEDIUM_MEM_REQUEST` / `_LIMIT` | `512Mi` / `1Gi` |
| `RESOURCE_LARGE_CPU_REQUEST` / `_LIMIT` | `1000m` / `2000m` |
| `RESOURCE_LARGE_MEM_REQUEST` / `_LIMIT` | `1Gi` / `2Gi` |

---

## API エンドポイント一覧

ベースパス: `/api/v1`
認証: `Authorization: Bearer <JWT>` ヘッダー（一部を除く）

レスポンス形式:
```json
// 成功
{ "data": { ... }, "error": null }

// エラー
{ "data": null, "error": { "code": "NOT_FOUND", "message": "container not found" } }
```

### 認証不要

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/health` | ヘルスチェック |
| `GET` | `/api/v1/templates` | テンプレート一覧 |
| `GET` | `/api/v1/templates/:template_name` | テンプレート詳細 |
| `POST` | `/api/v1/webhooks/:token` | Webhook 受信（自動デプロイ） |

### Projects

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/projects` | プロジェクト一覧 |
| `POST` | `/projects` | プロジェクト作成 |
| `GET` | `/projects/:project_id` | プロジェクト詳細 |
| `DELETE` | `/projects/:project_id` | プロジェクト削除 |
| `POST` | `/projects/:project_id/deploy` | プロジェクト全体デプロイ（スナップショット作成） |
| `GET` | `/projects/:project_id/jobs` | ワークフロージョブ一覧 |

### Containers

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/projects/:project_id/containers` | コンテナ一覧 |
| `GET` | `/projects/:project_id/containers/:container_id` | コンテナ詳細 |
| `POST` | `/projects/:project_id/containers/deploy` | Git からビルドデプロイ |
| `POST` | `/projects/:project_id/containers/from-template` | テンプレートからデプロイ |
| `PUT` | `/projects/:project_id/containers/:container_id` | コンテナ設定更新 |
| `DELETE` | `/projects/:project_id/containers/:container_id` | コンテナ削除 |
| `POST` | `.../containers/:container_id/redeploy` | 再デプロイ（rollout restart） |
| `PUT` | `.../containers/:container_id/scale` | レプリカ数変更 |
| `POST` | `.../containers/:container_id/webhook` | Webhook URL 発行 |
| `GET` | `.../containers/:container_id/status-histories` | ステータス履歴（最新100件） |

### その他リソース

| メソッド | パス | 説明 |
|---|---|---|
| `GET/PUT/DELETE` | `/projects/:project_id/env-vars` | プロジェクト環境変数 |
| `GET/PUT/DELETE` | `.../containers/:container_id/env-vars` | コンテナ環境変数 |
| `GET/POST/DELETE` | `.../containers/:container_id/ports` | ポート管理 |
| `GET/POST/DELETE` | `.../containers/:container_id/routes` | ルート管理（Service / Ingress） |
| `POST/DELETE` | `.../containers/:container_id/mounts` | ボリュームマウント |
| `GET` | `.../containers/:container_id/logs` | ランタイムログ（カーソルページネーション） |
| `GET` | `/projects/:project_id/build-jobs/:build_job_id/logs` | ビルドログ |
| `GET` | `.../containers/:container_id/metrics` | メトリクス（from/to 範囲指定） |
| `GET/DELETE` | `.../containers/:container_id/build-jobs` | ビルドジョブ管理 |
| `GET/POST/DELETE` | `/projects/:project_id/volumes` | ボリューム管理 |
| `GET` | `/projects/:project_id/snapshots` | スナップショット一覧 |
| `GET` | `/projects/:project_id/snapshots/:snapshot_id` | スナップショット詳細 |
| `POST` | `/projects/:project_id/snapshots/:snapshot_id/restore` | スナップショット復元 |
| `GET` | `/projects/:project_id/connections` | サービス接続一覧（フロー可視化用） |

---

## ビルド方法

各コンポーネントは独立した Go モジュール。

```bash
# 全コンポーネントをビルド
cd shared        && go build ./...
cd backend/src   && go build ./...
cd controller/src && go build ./...
cd builder/src   && go build ./...
cd watcher/src   && go build ./...
```

モジュール間の依存は `go.mod` の `replace` ディレクティブで解決します:
```go
replace launchs/shared => ../../shared
```

---

## Temporal ワークフロー

| キュー | ワークフロー | 説明 |
|---|---|---|
| `controller-queue` | `CreateProjectWorkflow` | Namespace + Harbor プロジェクト作成 |
| `controller-queue` | `DeleteProjectWorkflow` | Harbor + Namespace 削除 |
| `controller-queue` | `DeployWorkflow` | Deployment Apply（ステータス: deploying → running） |
| `controller-queue` | `RedeployWorkflow` | rollout restart |
| `controller-queue` | `DeleteContainerWorkflow` | Deployment 削除 |
| `controller-queue` | `ScaleWorkflow` | レプリカ数変更 |
| `controller-queue` | `CreateVolumeWorkflow` | PVC 作成 |
| `controller-queue` | `DeleteVolumeWorkflow` | PVC 削除 |
| `controller-queue` | `MountVolumeWorkflow` | ボリュームマウント（Deployment 再Apply） |
| `controller-queue` | `UnmountVolumeWorkflow` | ボリュームアンマウント |
| `controller-queue` | `CreateServiceWorkflow` | K8s Service 作成 |
| `controller-queue` | `DeleteServiceWorkflow` | K8s Service 削除 |
| `controller-queue` | `CreateIngressWorkflow` | Traefik IngressRoute 作成 |
| `controller-queue` | `DeleteIngressWorkflow` | Traefik IngressRoute 削除 |
| `controller-queue` | `DeployProjectWorkflow` | 全コンテナ並列デプロイ（子ワークフロー） |
| `controller-queue` | `RestoreSnapshotWorkflow` | スナップショットから全コンテナ復元 |
| `builder-queue` | `BuildDeployWorkflow` | ビルド → Harbor プッシュ → DeployWorkflow 起動 |

---

## Kubernetes リソース命名規則

冪等性担保のため、全 K8s リソース名を決定論的に生成します。

| リソース | 命名規則 |
|---|---|
| Deployment / Service / IngressRoute | `{container-name}-{container-id[0:8]}` |
| PVC | `{volume-name}-{volume-id[0:8]}` |
| Namespace | `project-{project-uuid}` |
| サブドメイン | `{container-name}-{project-uuid}.launchs.org` |

---

## データベース初期化

`docker compose up` 時に `database/script/init.sql` が自動実行されます。

| DB名 | ユーザー | パスワード | 用途 |
|---|---|---|---|
| `maindb` | `main` | `main` | メインアプリ（GORM AutoMigrate） |
| `authdb` | `main` | `main` | authbase 認証サービス |

テーブルマイグレーションは backend 初回起動時に GORM AutoMigrate で自動実行されます。

---

## ディレクトリ構成

```
backend-new/
├── backend/
│   ├── dockerfile
│   └── src/
│       ├── main.go
│       ├── handler/       # Echo ハンドラ
│       ├── service/       # ビジネスロジック・Temporal ワークフロー起動
│       ├── repository/    # GORM リポジトリ（インターフェース + 実装）
│       ├── middlewares/   # JWT Ed25519 認証ミドルウェア
│       └── response/      # 統一レスポンス型 { data, error }
├── controller/
│   ├── dockerfile
│   └── src/
│       ├── main.go
│       ├── workflow/      # Temporal ワークフロー（16種）
│       └── activity/      # K8s / Harbor / DB 操作アクティビティ
├── builder/
│   ├── dockerfile
│   └── src/
│       ├── main.go
│       ├── workflow/      # BuildDeployWorkflow
│       ├── activity/      # railpack ビルド・DB 更新
│       ├── railpack/      # K8s Job ベースのビルドライブラリ
│       └── harbor/        # Harbor REST API クライアント
├── watcher/
│   ├── dockerfile
│   └── src/
│       ├── main.go
│       └── collector/
│           ├── status_collector.go   # Pod Watch → pod_statuses UPSERT
│           ├── log_collector.go      # ログストリーム → バッファ → DB
│           └── metric_collector.go  # Metrics API → container_metrics
├── shared/                # launchs/shared モジュール
│   ├── model/             # GORM モデル（17テーブル）
│   ├── temporal/          # キュー名・ワークフロー名定数
│   ├── config/            # 環境変数ラッパー
│   ├── errors/            # アプリエラー型
│   └── database/          # DB / K8s / Redis クライアント初期化
├── config/                # 環境変数ファイル（*.env）
├── database/              # DB 初期化 SQL
├── nginx/                 # Nginx 設定・証明書
├── openssl/               # JWT・SSL キー生成用 Dockerfile
├── docs/                  # 設計ドキュメント
│   ├── 01_企画書.md
│   ├── 02_API設計書.md
│   ├── 03_実装計画書.md
│   └── 04_openapi.yaml
└── docker-compose.yaml
```
