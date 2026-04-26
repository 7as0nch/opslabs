#!/bin/bash
# ops-nginx-upstream-fail check:
#   curl http://localhost/ 必须返 200 且响应体含 "Hello from app"
#   —— 既验 nginx 反代通了,又验后端 app 真在响应(不是 nginx 自己 fallback)
set -o pipefail

BODY=$(curl -s -m 3 -o - -w '__HTTP_CODE__:%{http_code}' http://localhost/ 2>/dev/null || true)
CODE="${BODY##*__HTTP_CODE__:}"
BODY="${BODY%__HTTP_CODE__:*}"

if [ -z "$CODE" ] || [ "$CODE" -lt 200 ] || [ "$CODE" -ge 300 ]; then
    echo "NO"
    echo "http://localhost/ 返回 code=${CODE:-none},nginx 反代仍未通" >&2
    echo "提示:tail -n 30 /var/log/nginx/error.log 看 upstream 报错" >&2
    exit 0
fi

if ! echo "$BODY" | grep -q 'Hello from app'; then
    echo "NO"
    echo "200 但响应体不是 app 的输出(可能命中了 nginx default page)" >&2
    exit 0
fi

echo "OK"
exit 0
