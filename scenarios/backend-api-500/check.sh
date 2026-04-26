#!/bin/bash
# backend-api-500 check:
#   连续 3 次 curl http://localhost:8080/users/1 都返回 200,且响应体含 "id" 字段
#   → 这一组合杜绝"碰巧第一次就过、之后又挂"的偶然态,也排除"app 被改成无脑返 200"
set -o pipefail

PASS=0
LAST_BODY=""
for i in 1 2 3; do
    BODY=$(curl -s -m 3 -o - -w '__HTTP_CODE__:%{http_code}' \
        http://localhost:8080/users/1 2>/dev/null || true)
    CODE="${BODY##*__HTTP_CODE__:}"
    BODY="${BODY%__HTTP_CODE__:*}"
    LAST_BODY="$BODY"

    if [ -z "$CODE" ] || [ "$CODE" -lt 200 ] || [ "$CODE" -ge 300 ]; then
        echo "NO"
        echo "第 ${i} 次 curl 返回 code=${CODE:-none},尚未排除完故障" >&2
        exit 0
    fi

    if ! echo "$BODY" | grep -q '"id"'; then
        echo "NO"
        echo "第 ${i} 次 curl 返回 200 但响应体缺 \"id\" 字段(可能 schema 错了)" >&2
        exit 0
    fi

    PASS=$((PASS + 1))
    sleep 0.2
done

if [ "$PASS" -eq 3 ]; then
    echo "OK"
    exit 0
fi

echo "NO"
echo "意外路径,最后一次响应:$LAST_BODY" >&2
exit 0
