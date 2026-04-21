#!/bin/bash
# 一键构建所有场景镜像
# 依赖:Docker 可用
# 用法:
#   ./scripts/build-all-scenarios.sh                # 全部构建
#   ./scripts/build-all-scenarios.sh hello-world    # 只构建指定场景
#
# 构建顺序:
#   1) scenarios-build/<name>/Dockerfile  → opslabs/<name>:v1(全部基础镜像)
#   2) scenarios/<slug>/Dockerfile        → opslabs/<slug>:v1
set -e

BASE_DIR=$(cd "$(dirname "$0")/.." && pwd)

# -------- 1. 扫描并构建所有 base-* 镜像 --------
if [ -d "$BASE_DIR/scenarios-build" ]; then
    for d in "$BASE_DIR/scenarios-build"/*/; do
        [ -f "$d/Dockerfile" ] || continue
        name=$(basename "$d")
        echo "==> building opslabs/$name:v1"
        docker build -t "opslabs/$name:v1" "$d"
    done
fi

# -------- 2. 构建场景镜像 --------
# 当前可用场景列表;加新场景在这里追加一行即可
SCENARIOS=(
    hello-world
    # frontend-devserver-down     # Week 2
    # backend-api-500             # Week 2
    # ops-nginx-upstream-fail     # Week 2
)

if [ -n "$1" ]; then
    SCENARIOS=("$1")
fi

for slug in "${SCENARIOS[@]}"; do
    if [ ! -f "$BASE_DIR/scenarios/$slug/Dockerfile" ]; then
        echo "==> skip $slug (no Dockerfile)"
        continue
    fi
    echo "==> building opslabs/$slug:v1"
    docker build -t "opslabs/$slug:v1" "$BASE_DIR/scenarios/$slug"
done

echo "all images built"
