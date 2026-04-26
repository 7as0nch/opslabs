#!/bin/bash
# /opt/app/run.sh —— 后端 app 启动脚本(由 mini-systemctl 拉起)
#
# PORT 环境变量决定监听端口,缺省 8080。
# 用户也可以选另一条解法:把 PORT=9090 写进这里,跟 nginx 配置端口对齐。
exec env PORT="${PORT:-8080}" python3 /opt/app/server.py
