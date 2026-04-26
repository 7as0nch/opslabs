#!/bin/bash
# ops-nginx-upstream-fail setup:
#   1. 启动 app(监听 8080)
#   2. 启动 nginx(配置里 proxy_pass http://127.0.0.1:9090)
#   3. 用户进容器看到:nginx 跑、app 跑、curl localhost/ 仍 502
#
# 解法允许两条:
#   A. 改 nginx-app.conf 的 proxy_pass 端口为 8080,nginx -s reload
#   B. 给 app run.sh 加 PORT=9090 后 systemctl restart app
# check.sh 都接(只看 curl localhost/ 是否返 200 + Hello from app)
set -e

# 1) app 跑在 8080(默认)
systemctl start app || true

# 2) nginx 跑(读到的 proxy_pass 是 9090,故障状态)
systemctl start nginx || true

exit 0
