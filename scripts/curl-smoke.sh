#!/bin/bash
# Phase A 冒烟脚本:走一遍 list -> get -> start -> check -> terminate
# 依赖:后端已启动(mock 或 docker 均可),jq 在 PATH 里
set -e

BASE=${BASE:-http://localhost:6039}
SLUG=${SLUG:-hello-world}

echo "==> list scenarios"
curl -s "$BASE/v1/scenarios" | jq '.data.scenarios[].slug // .scenarios[].slug'

echo "==> get scenario $SLUG"
curl -s "$BASE/v1/scenarios/$SLUG" | jq '.data.scenario.title // .scenario.title'

echo "==> start attempt"
START_RESP=$(curl -s -X POST "$BASE/v1/scenarios/$SLUG/start" \
    -H 'content-type: application/json' -d '{}')
echo "$START_RESP" | jq .
ATTEMPT_ID=$(echo "$START_RESP" | jq -r '.data.attemptId // .attemptId')
if [ -z "$ATTEMPT_ID" ] || [ "$ATTEMPT_ID" = "null" ]; then
    echo "failed to get attemptId"
    exit 1
fi
echo "==> attemptId: $ATTEMPT_ID"

echo "==> get attempt"
curl -s "$BASE/v1/attempts/$ATTEMPT_ID" | jq .

echo "==> check attempt"
curl -s -X POST "$BASE/v1/attempts/$ATTEMPT_ID/check" \
    -H 'content-type: application/json' -d '{}' | jq .

echo "==> terminate attempt"
curl -s -X POST "$BASE/v1/attempts/$ATTEMPT_ID/terminate" \
    -H 'content-type: application/json' -d '{}' | jq .

echo 'smoke ok'
