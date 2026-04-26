#!/bin/bash
# frontend-devserver-down solution:标准解法,用于回归脚本自动跑一遍
#
# 这个脚本在**容器内以 player 身份**执行,等价于一个"理想用户"的操作轨迹。
# 步骤:
#   1. 找到并 kill 占坑进程(python3 -m http.server 3000)
#   2. npm install 把 setup.sh 删掉的 @vitejs 依赖装回来
#   3. 后台起 vite dev
#   4. 等 server 起来(最多 15s)
#
# 返回 0 表示解法执行完成,不代表通关 —— 判题仍由 check.sh 决定
set -e

cd /home/player/webapp

# 1. 干掉占坑的 python http.server
if command -v pkill >/dev/null 2>&1; then
    pkill -f 'python3 -m http.server 3000' 2>/dev/null || true
fi
# 保险:直接按端口清
if command -v fuser >/dev/null 2>&1; then
    fuser -k -n tcp 3000 2>/dev/null || true
fi

# 2. 装回依赖
npm install --prefer-offline --no-audit --no-fund

# 3. 后台起 vite(脱离当前 shell,避免 solution.sh 结束时 npm 被 SIGHUP)
setsid bash -c 'cd /home/player/webapp && npm run dev >/tmp/vite.log 2>&1 </dev/null' &
disown $! 2>/dev/null || true

# 4. 轮询等 vite 起来 —— 最多 15 秒,每 1 秒探一次
for i in $(seq 1 15); do
    if curl -s -m 2 -o /dev/null -w '%{http_code}' http://localhost:3000/ 2>/dev/null | grep -q '^2'; then
        echo "vite up after ${i}s"
        exit 0
    fi
    sleep 1
done

echo "vite did not come up within 15s, tail of /tmp/vite.log:" >&2
tail -n 30 /tmp/vite.log >&2 2>/dev/null || true
exit 1
