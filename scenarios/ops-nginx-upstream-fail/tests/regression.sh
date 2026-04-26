#!/bin/bash
# ops-nginx-upstream-fail regression
set -e

IMAGE=${IMAGE:-opslabs/ops-nginx-upstream-fail:v1}
NAME="opslabs-regression-nginx-$$"

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "==> starting container"
docker run -d --name "$NAME" \
    --cap-drop ALL --security-opt no-new-privileges:true --pids-limit 256 \
    "$IMAGE" >/dev/null
sleep 4

echo "==> step 1: initial check.sh must NOT pass (502)"
OUT=$(docker exec -u root "$NAME" /opt/opslabs/check.sh || true)
FIRST="$(echo "$OUT" | head -n 1)"
[ "$FIRST" = "OK" ] && { echo "!! FAIL: 初始就通过了"; exit 1; }
echo "   ok, initial=\"$FIRST\""

echo "==> step 2: run solution"
SRC=$(cd "$(dirname "$0")" && pwd)/solution.sh
docker cp "$SRC" "$NAME:/tmp/solution.sh"
docker exec -u player "$NAME" bash /tmp/solution.sh

echo "==> step 3: check.sh must pass"
OUT=$(docker exec -u root "$NAME" /opt/opslabs/check.sh)
FIRST="$(echo "$OUT" | head -n 1)"
[ "$FIRST" != "OK" ] && { echo "!! FAIL: $OUT"; exit 1; }
echo "   ok, post-solution=\"$FIRST\""
echo 'regression passed'
