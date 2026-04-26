#!/bin/bash
# backend-api-500 setup:
#   1. 篡改 /etc/app/config.yaml 的 db_password(opslabs2026 → opslabs2025)
#      → app 启动时验证密码失败,/users/<id> 永远 500,日志反复打
#         "FATAL: password authentication failed for user 'opslabs'"
#   2. 启动 app(systemctl start app)—— 让用户进容器就看到"服务在跑但 API 500"
#
# 解法:
#   - journalctl -u app -n 30  → 见 password authentication failed
#   - cat /etc/app/config.yaml → 看到 db_password: opslabs2025
#   - vi/sed 改成 opslabs2026
#   - systemctl restart app
#   - curl http://localhost:8080/users/1 → 200
set -e

# 1) 把 config.yaml 里的密码改成错的
sed -i 's/db_password: opslabs2026/db_password: opslabs2025/' /etc/app/config.yaml

# 2) 启 app 进程(以 player 身份)
systemctl start app || true

exit 0
