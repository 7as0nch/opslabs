#!/bin/bash
# frontend-devserver-down check:确认 dev server 真的起来了,而不是那个占坑进程
#
# 通关条件(必须同时满足):
#   1. curl http://localhost:3000/ 返回 2xx
#   2. 响应体含 "<html"(Vite dev server 返 HTML;占坑进程返的是 directory listing,
#      也能匹配到 "<html"!—— 再加一层条件:body 里必须能见到 /src/main.jsx 引用,
#      这是 Vite 注入的 module 入口标识,占坑进程绝对不会有)
#
# stdout 首行 == "OK" 视为通关(与 hello-world 同协议)
set -o pipefail

BODY=$(curl -s -m 3 -o - -w '__HTTP_CODE__:%{http_code}' http://localhost:3000/ 2>/dev/null || true)
CODE="${BODY##*__HTTP_CODE__:}"
BODY="${BODY%__HTTP_CODE__:*}"

if [ -z "$CODE" ] || [ "$CODE" -lt 200 ] || [ "$CODE" -ge 300 ]; then
    echo "NO"
    echo "http://localhost:3000/ 未返回 2xx(拿到 code=${CODE:-none})" >&2
    echo "提示:确认 vite dev 已经起来,比如 'npm run dev &' 让它后台跑" >&2
    exit 0
fi

# Vite dev 会注入 type="module" + /src/main.jsx 引用,占坑进程没这个
if ! echo "$BODY" | grep -q '/src/main.jsx'; then
    echo "NO"
    echo "3000 端口能响应,但看起来不是 Vite dev server(可能还被占坑进程占着)" >&2
    echo "提示:lsof -i:3000 或 ss -ltnp | grep 3000 找出占坑进程并 kill" >&2
    exit 0
fi

echo "OK"
exit 0
