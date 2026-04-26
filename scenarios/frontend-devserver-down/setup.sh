#!/bin/bash
# frontend-devserver-down setup:制造两层故障
#
# 1) 删掉 node_modules 里 @vitejs 插件相关目录
#    → 用户跑 npm run dev 时会报 "Cannot find module '@vitejs/plugin-react'"
#    → 解法:npm install 重装即可(package.json 里已经声明了依赖)
#
# 2) 在 3000 端口起一个假的 Python http 服务(前台随便响应点东西),
#    用户想 npm run dev 时会发现 "Port 3000 is in use, trying another one..."
#    或 bind 冲突;需要用户 lsof / ss 找出来 kill 掉
#
# 预期耗时:5-10 分钟
# 允许多解:
#   - kill 占坑进程可以用 kill <pid> / pkill / fuser
#   - 也允许用户改 vite 起别的端口 —— check 只看 3000,所以那条路走不通,
#     这是有意的约束(要求用户真的释放 3000,而不是绕路)
set -e

# --- 故障 1: 删掉 @vitejs 插件 ---
if [ -d /home/player/webapp/node_modules/@vitejs ]; then
    rm -rf /home/player/webapp/node_modules/@vitejs
fi

# --- 故障 2: 3000 端口占坑进程 ---
# 用 python3 -m http.server 起一个快速的占坑服务,前台跑日志重定向到 /dev/null
# setsid 断开控制终端,避免跟着 entrypoint 一起被 SIGHUP
if command -v python3 >/dev/null 2>&1; then
    mkdir -p /tmp/squatter && echo 'squatter' > /tmp/squatter/index.html
    setsid bash -c 'cd /tmp/squatter && exec python3 -m http.server 3000 >/tmp/squatter.log 2>&1 </dev/null' &
    disown $! 2>/dev/null || true
fi

exit 0
