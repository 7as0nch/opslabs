#!/bin/bash
# frontend-devserver-down regression:
#   在宿主机跑,独立启一个容器走完"初始应失败 → 跑 solution → 应通关"两步。
#
# 前置:
#   - opslabs/base:v1 已构建
#   - opslabs/frontend-devserver-down:v1 已构建
# 运行:
#   ./scripts/build-all-scenarios.sh
#   bash scenarios/frontend-devserver-down/tests/regression.sh
set -e

IMAGE=${IMAGE:-opslabs/frontend-devserver-down:v1}
NAME="opslabs-regression-fedown-$$"

cleanup() {
    docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> starting container from $IMAGE"
# 只做回归,不占用宿主端口映射;internet-allowed 让容器能走 npm
docker run -d --name "$NAME" \
    --cap-drop ALL --security-opt no-new-privileges:true --pids-limit 256 \
    "$IMAGE" >/dev/null

# 等 setup 跑完(故障就位)
sleep 3

echo "==> step 1: initial check.sh must NOT pass"
OUT=$(docker exec -u root "$NAME" /opt/opslabs/check.sh || true)
FIRST="$(echo "$OUT" | head -n 1)"
if [ "$FIRST" = "OK" ]; then
    echo "!! FAIL: check.sh 初始就通过了,故障没埋进去"
    exit 1
fi
echo "   ok, initial=\"$FIRST\""

echo "==> step 2: run solution.sh as player"
docker exec -u player "$NAME" bash /opt/opslabs/solution.sh 2>/dev/null || \
    docker exec -u player "$NAME" bash - <<'EOF'
# solution.sh 不在镜像里(测试期才用),用 docker cp 的备用方案见最下
exit 2
EOF

# 如果镜像里没 solution.sh,就从宿主机拷一份进去再跑
if [ $? -ne 0 ]; then
    SRC=$(cd "$(dirname "$0")" && pwd)/solution.sh
    docker cp "$SRC" "$NAME:/tmp/solution.sh"
    docker exec -u player "$NAME" bash /tmp/solution.sh
fi

echo "==> step 3: check.sh must pass"
OUT=$(docker exec -u root "$NAME" /opt/opslabs/check.sh)
FIRST="$(echo "$OUT" | head -n 1)"
if [ "$FIRST" != "OK" ]; then
    echo "!! FAIL: check.sh 解法后还是没通过,输出:"
    echo "$OUT"
    exit 1
fi
echo "   ok, post-solution=\"$FIRST\""

echo 'regression passed'
