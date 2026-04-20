#!/bin/bash

set -u

# --- 配置区域 ---
DOCKERHUB_USER="7as0nch"
PLATFORM="linux/amd64" # 统一指定构建平台
BUILDER_NAME="${BUILDER_NAME:-}"
USE_CONTAINER_BUILDER="${USE_CONTAINER_BUILDER:-0}"

# 各个服务的版本号独立管理
VERSION_BACKEND="v1.0.4-beta.12"
VERSION_BACKWEB="v1.0.4-beta.2"
VERSION_LITECHAT="v1.0.4-beta.10"

# --- 脚本逻辑 ---
SERVICE=${1:-}  # 接收第一个参数作为指定服务名

prepare_builder() {
    if [ "$USE_CONTAINER_BUILDER" = "1" ]; then
        BUILDER_NAME="${BUILDER_NAME:-mybuilder}"
        echo "使用独立 buildx builder: $BUILDER_NAME (docker-container)"
        docker buildx create --use --name "$BUILDER_NAME" 2>/dev/null || docker buildx use "$BUILDER_NAME"
        return
    fi

    if [ -z "$BUILDER_NAME" ]; then
        BUILDER_NAME=$(docker context show)
    fi

    echo "使用当前 Docker context 对应的 builder: $BUILDER_NAME"
    docker buildx use "$BUILDER_NAME"
}

prepare_frontend_assets() {
    local name=$1

    case "$name" in
        backweb)
            echo "本地构建前端产物: backweb"
            (
                cd backweb
                pnpm run build:antd
            ) || exit 1

            if [ ! -d "backweb/apps/web-antd/dist" ]; then
                echo "backweb 构建完成后未找到产物目录: backweb/apps/web-antd/dist"
                exit 1
            fi
            ;;
        litechat)
            echo "本地构建前端产物: litechat"
            (
                cd litechat
                pnpm run build
            ) || exit 1

            if [ ! -d "litechat/dist" ]; then
                echo "litechat 构建完成后未找到产物目录: litechat/dist"
                exit 1
            fi
            ;;
    esac
}

build_and_push() {
    local name=$1
    local dir=$2
    local dockerfile=$3
    local context=$4
    local version=$5

    echo "=== 正在构建服务: $name (版本: $version, 平台: $PLATFORM) ==="

    if [ "$name" = "backweb" ] || [ "$name" = "litechat" ]; then
        prepare_frontend_assets "$name"
    fi

    if [ -n "$BUILDER_NAME" ]; then
        docker buildx build --builder "$BUILDER_NAME" --platform $PLATFORM \
            -t $DOCKERHUB_USER/aichat-$name:$version \
            -t $DOCKERHUB_USER/aichat-$name:latest \
            -f "$dir/$dockerfile" "$dir/$context" --push
    else
        # 使用当前 Docker context 的默认 builder
        docker buildx build --platform $PLATFORM \
            -t $DOCKERHUB_USER/aichat-$name:$version \
            -t $DOCKERHUB_USER/aichat-$name:latest \
            -f "$dir/$dockerfile" "$dir/$context" --push
    fi

    if [ $? -eq 0 ]; then
        echo "Successfully pushed $name"
        # 自动同步更新 k8s-deployment.yaml 中的版本号
        # 使用更宽松的正则来匹配版本号（包括 beta, dots, dashes）
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s|7as0nch/aichat-$name:[^[:space:]]*|7as0nch/aichat-$name:$version|g" k8s-deployment.yaml
        else
            sed -i "s|7as0nch/aichat-$name:[^[:space:]]*|7as0nch/aichat-$name:$version|g" k8s-deployment.yaml
        fi
        echo "K8s deployment sync done for $name"
    else
        echo "Failed to build/push $name"
        exit 1
    fi
}

prepare_builder

if [ -z "$SERVICE" ] || [ "$SERVICE" == "backend" ]; then
    build_and_push "backend" "backend" "Dockerfile" "." "$VERSION_BACKEND"
fi

if [ -z "$SERVICE" ] || [ "$SERVICE" == "backweb" ]; then
    build_and_push "backweb" "backweb" "scripts/deploy/Dockerfile" "." "$VERSION_BACKWEB"
fi

if [ -z "$SERVICE" ] || [ "$SERVICE" == "litechat" ]; then
    build_and_push "litechat" "litechat" "dockerfile" "." "$VERSION_LITECHAT"
fi

echo "Done! 所有选定服务已发布，K8s 配置已更新。"

echo "将k8s-deployment.yaml增量同步到远程服务器"
rsync -avz --delete k8s-deployment.yaml root@sshjd.aihelper.chat:/root/k3s/aichat/aichat.yaml

echo "远程执行命令：kubectl apply -f pipeline.yaml"
ssh root@sshjd.aihelper.chat "kubectl apply -f /root/k3s/aichat/aichat.yaml"
