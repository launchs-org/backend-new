#!/bin/sh
# Temporal の PostgreSQL スキーマをセットアップするスクリプト。
# temporal-admin-tools コンテナ内で実行される。

set -e

echo "[temporal-setup] PostgreSQL スキーマをセットアップ中..."

# デフォルトスキーマの作成
temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT:-5432}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal \
  setup-schema -v 0.0

temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT:-5432}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal \
  update-schema -d /etc/temporal/schema/postgresql/v12/temporal/versioned

# Visibility スキーマの作成
temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT:-5432}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal_visibility \
  setup-schema -v 0.0

temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT:-5432}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal_visibility \
  update-schema -d /etc/temporal/schema/postgresql/v12/visibility/versioned

echo "[temporal-setup] スキーマセットアップ完了"
