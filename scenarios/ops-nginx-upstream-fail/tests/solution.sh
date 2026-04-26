#!/bin/bash
# ops-nginx-upstream-fail 标准解法(走解法 A:改 nginx 配置)
set -e

# 1) 把 proxy_pass 端口改回 8080
sudo sed -i 's|proxy_pass *http://127.0.0.1:9090|proxy_pass http://127.0.0.1:8080|' /etc/nginx/conf.d/app.conf

# 2) reload
nginx -s reload

# 3) 等 listener 起来
for i in $(seq 1 10); do
    if curl -s -m 1 http://localhost/ | grep -q 'Hello from app'; then
        echo "nginx upstream restored after ${i} attempt(s)"
        exit 0
    fi
    sleep 0.3
done

echo "nginx upstream still failing" >&2
tail -n 30 /var/log/nginx/error.log >&2 || true
exit 1
