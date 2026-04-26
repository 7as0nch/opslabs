#!/bin/bash
# 一键构建所有场景镜像
# 依赖:Docker 可用
# 用法:
#   ./scripts/build-all-scenarios.sh                # 扫 scenarios/ 下所有带 Dockerfile 的目录
#   ./scripts/build-all-scenarios.sh hello-world    # 只构建指定场景
#
# 构建顺序:
#   1) scenarios-build/<name>/Dockerfile  → opslabs/<name>:v1(全部基础镜像)
#   2) scenarios/<slug>/Dockerfile        → opslabs/<slug>:v1(场景镜像,自动扫描)
#
# 2026-04-24:原先 SCENARIOS 是硬编码数组,加场景要动脚本;现改为扫 scenarios/
#             自动发现 Dockerfile,新增场景只要建目录就行,脚本无需改动。
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

# -------- 2. 构建场景镜像(自动扫描 scenarios/*/Dockerfile) --------
if [ -n "$1" ]; then
    # 单独构建一个场景
    SCENARIOS=("$1")
else
    # 自动发现所有带 Dockerfile 的场景目录
    SCENARIOS=()
    if [ -d "$BASE_DIR/scenarios" ]; then
        for d in "$BASE_DIR/scenarios"/*/; do
            [ -f "$d/Dockerfile" ] || continue
            SCENARIOS+=("$(basename "$d")")
        done
    fi
fi

if [ ${#SCENARIOS[@]} -eq 0 ]; then
    echo "==> no scenarios with Dockerfile found under scenarios/"
    echo "    (non-sandbox 模式场景如 static / web-container / wasm-linux 不需要构建镜像)"
    exit 0
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
