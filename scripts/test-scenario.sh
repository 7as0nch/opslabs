#!/bin/bash
# 场景自测:regression(初态有故障) -> solution(跑参考解法) -> check(应 OK)
# 用法:
#   ./scripts/test-scenario.sh hello-world
set -e

SLUG=$1
if [ -z "$SLUG" ]; then
    echo "usage: $0 <scenario-slug>"
    exit 1
fi

BASE_DIR=$(cd "$(dirname "$0")/.." && pwd)
SCENARIO_DIR="$BASE_DIR/scenarios/$SLUG"
IMAGE="opslabs/$SLUG:v1"
NAME="opslabs-test-$SLUG-$$"

if [ ! -d "$SCENARIO_DIR" ]; then
    echo "scenario dir not found: $SCENARIO_DIR"
    exit 1
fi

cleanup() {
    docker rm -f "$NAME" > /dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> starting container $NAME"
docker run -d --name "$NAME" --privileged "$IMAGE" > /dev/null
# setup.sh 里会装服务、初始化数据,等一会
sleep 3

echo "[1/3] regression check (故障应已预埋)..."
docker cp "$SCENARIO_DIR/tests/regression.sh" "$NAME:/tmp/regression.sh"
docker exec "$NAME" bash /tmp/regression.sh

echo "[2/3] running solution..."
docker cp "$SCENARIO_DIR/tests/solution.sh" "$NAME:/tmp/solution.sh"
docker exec "$NAME" bash /tmp/solution.sh

echo "[3/3] check after solution (应返回 OK)..."
result=$(docker exec "$NAME" bash /opt/opslabs/check.sh | head -1)
if [ "$result" != "OK" ]; then
    echo "FAIL: check.sh after solution = '$result'"
    exit 1
fi

echo "all pass: $SLUG"
