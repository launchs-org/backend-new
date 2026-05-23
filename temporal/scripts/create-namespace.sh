#!/bin/sh
# Temporal の default namespace を作成するスクリプト。

set -e

echo "[temporal-setup] namespace を作成中: ${DEFAULT_NAMESPACE:-default}"

temporal operator namespace create \
  --address "${TEMPORAL_ADDRESS}" \
  "${DEFAULT_NAMESPACE:-default}" || true

echo "[temporal-setup] namespace 作成完了"
