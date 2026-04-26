#!/bin/bash
# backend-api-500 regression
set -e

IMAGE=${IMAGE:-opslabs/backend-api-500:v1}
NAME="opslabs-regression-bapi500-$$"

cleanup() {
    docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> starting container from $IMAGE"
docker run -d --name "$NAME" \
    --cap-drop ALL --security-opt no-new-privileges:true --pids-limit 256 \
    "$IMAGE" >/dev/null

# 等 setup + app boot
sleep 4

echo "==> step 1: initial check.sh must NOT pass (config has wrong password)"
OUT=$(docker exec -u root "$NAME" /opt/opslabs/check.sh || true)
FIRST="$(echo "$OUT" | head -n 1)"
if [ "$FIRST" = "OK" ]; then
    echo "!! FAIL: check.sh 初始就通过了,故障没埋进去"
    exit 1
fi
echo "   ok, initial=\"$FIRST\""

echo "==> step 2: run solution"
SRC=$(cd "$(dirname "$0")" && pwd)/solution.sh
docker cp "$SRC" "$NAME:/tmp/solution.sh"
docker exec -u player "$NAME" bash /tmp/solution.sh

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
