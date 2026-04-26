#!/bin/bash
# API 冒烟脚本:走一遍 list → detail → start → get → check → terminate
#
# 用法:
#   ./scripts/curl-smoke.sh                 # 跑 4 种 execution mode 各一个代表性场景
#   ./scripts/curl-smoke.sh hello-world     # 只跑指定 slug
#
# 依赖:
#   - 后端已启动(mock 或 docker 均可,sandbox 的 check 在 mock 下会被跳过)
#   - jq 在 PATH 里(解析 JSON)
#
# 判题行为差异:
#   - sandbox  :后端 docker exec check.sh,mock runtime 下 check 可能直接失败,
#                这时脚本只要求能 start / get / terminate 即可(CHECK_OPTIONAL=1)
#   - 非 sandbox:client 必须上报 clientResult,脚本构造 passed=true 冒烟一次
set -e

BASE=${BASE:-http://localhost:6039}

# 4 种模式的代表性场景(必须跟 backend/internal/scenario/registry.go 对齐)
#   slug | execution_mode
# 增加新场景后在这里补一行即可。
DEFAULT_SLUGS=(
    "hello-world:sandbox"
    "css-flex-center:static"
    "webcontainer-node-hello:web-container"
    "wasm-linux-hello:wasm-linux"
)

# 传单 slug 时退化为只跑那个
if [ -n "$1" ]; then
    DEFAULT_SLUGS=("$1:auto")
fi

echo "==> list scenarios"
curl -s "$BASE/v1/scenarios" | jq '.data.scenarios[].slug // .scenarios[].slug'

FAIL=0
for entry in "${DEFAULT_SLUGS[@]}"; do
    slug="${entry%%:*}"
    mode="${entry##*:}"
    echo
    echo "================================================================"
    echo " smoke: $slug  (mode=$mode)"
    echo "================================================================"

    echo "==> detail"
    curl -s "$BASE/v1/scenarios/$slug" | jq '.data.scenario.title // .scenario.title'

    echo "==> start"
    START_RESP=$(curl -s -X POST "$BASE/v1/scenarios/$slug/start" \
        -H 'content-type: application/json' -d '{}')
    ATTEMPT_ID=$(echo "$START_RESP" | jq -r '.data.attemptId // .attemptId // empty')
    if [ -z "$ATTEMPT_ID" ]; then
        echo "!! start failed for $slug"
        echo "$START_RESP" | jq .
        FAIL=$((FAIL + 1))
        continue
    fi
    echo "   attemptId=$ATTEMPT_ID"

    echo "==> get"
    curl -s "$BASE/v1/attempts/$ATTEMPT_ID" | jq '.data.attempt.status // .attempt.status'

    echo "==> check"
    # 非 sandbox 必须带 clientResult;sandbox 发空 body 让后端 docker exec
    if [ "$mode" = "sandbox" ] || [ "$mode" = "auto" ]; then
        CHECK_BODY='{}'
    else
        CHECK_BODY='{"clientResult":{"passed":true,"exitCode":0,"stdout":"OK (smoke)","stderr":""}}'
    fi
    CHECK_RESP=$(curl -s -X POST "$BASE/v1/attempts/$ATTEMPT_ID/check" \
        -H 'content-type: application/json' -d "$CHECK_BODY")
    # mode=sandbox + mock runtime 时 check 可能 500,这里只打印不 fail
    echo "$CHECK_RESP" | jq '{passed: (.data.passed // .passed), message: (.data.message // .message), code: .code}'

    echo "==> terminate"
    curl -s -X POST "$BASE/v1/attempts/$ATTEMPT_ID/terminate" \
        -H 'content-type: application/json' -d '{}' | jq '{status: (.data.status // .status)}'
done

echo
if [ "$FAIL" -gt 0 ]; then
    echo "smoke FAIL: $FAIL scenario(s) failed to start"
    exit 1
fi
echo 'smoke ok'
