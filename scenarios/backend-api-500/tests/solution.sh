#!/bin/bash
# backend-api-500 标准解法
# 步骤:
#   1. 看 journalctl 确认密码错误(可选,用户视角)
#   2. 把 db_password 改回 opslabs2026
#   3. systemctl restart app
#   4. 等 0.5s 让 listener 起来
set -e

# 1) (略,真实用户会看 journalctl -u app)

# 2) 改 config(player 有 sudo,但配置文件已经是 player 拥有,可以直接 sed)
sed -i 's/db_password: opslabs2025/db_password: opslabs2026/' /etc/app/config.yaml

# 3) restart
systemctl restart app

# 4) 等 listener
for i in $(seq 1 10); do
    if curl -s -m 1 -o /dev/null -w '%{http_code}' http://localhost:8080/healthz | grep -q '^2'; then
        echo "app up after ${i} attempt(s)"
        exit 0
    fi
    sleep 0.5
done

echo "app did not come up in time" >&2
exit 1
