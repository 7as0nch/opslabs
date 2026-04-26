# ops-nginx-upstream-fail:Nginx 反代 502
#
# 构建:
#   docker build -t opslabs/base:v1                       scenarios-build/base
#   docker build -t opslabs/ops-nginx-upstream-fail:v1    scenarios/ops-nginx-upstream-fail
#
# 故障设计(对齐 registry 文案):
#   - Nginx 监听 :80,proxy_pass 到 http://127.0.0.1:9090
#   - 后端 app(python http.server)实际监听 :8080
#   - 端口对不上 → curl localhost/ → 502 Bad Gateway
#
# 排障路径:
#   1. curl http://localhost/ → 502
#   2. tail -n 50 /var/log/nginx/error.log → "connect() failed (111: Connection refused) ... upstream: http://127.0.0.1:9090"
#   3. ss -ltnp | grep -E '8080|9090' → 见 8080 在监听,9090 没有
#   4. 改 /etc/nginx/conf.d/app.conf 的 proxy_pass 端口为 8080
#   5. nginx -t && nginx -s reload
#   6. curl http://localhost/ → 200 + "Hello from app"
#
# 也允许另一条解法:把 app 改成监听 9090(改 /etc/app/run.sh + restart),check 都认。
FROM opslabs/base:v1

# ---------------- nginx + python ----------------
RUN apt-get update && apt-get install -y --no-install-recommends \
        nginx python3 \
    && rm -rf /var/lib/apt/lists/*

# ---------------- mini systemctl ----------------
COPY mini-systemctl /usr/local/bin/systemctl
RUN chmod 755 /usr/local/bin/systemctl \
    && mkdir -p /var/log/app /var/run /var/log/nginx \
    && chown -R player:player /var/log/app /var/run

# ---------------- nginx 配置(故障预埋) ----------------
# 删 default site,只保留我们的 app
RUN rm -f /etc/nginx/sites-enabled/default /etc/nginx/conf.d/default.conf
COPY nginx-app.conf /etc/nginx/conf.d/app.conf
# 把 nginx 的运行用户改成 player,避免容器里以 root 跑 daemon
RUN sed -i 's|^user .*|user player;|' /etc/nginx/nginx.conf \
    && sed -i 's|^pid .*|pid /var/run/nginx.pid;|' /etc/nginx/nginx.conf \
    && chown -R player:player /var/log/nginx /var/lib/nginx /etc/nginx/conf.d

# ---------------- app 源码(简单 python http.server) ----------------
COPY app /opt/app
RUN chmod 755 /opt/app/run.sh /opt/app/server.py \
    && chown -R player:player /opt/app

# ---------------- 场景脚本 ----------------
COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/opslabs/setup.sh
COPY check.sh      /opt/opslabs/check.sh

RUN chmod 755 /entrypoint.sh /opt/opslabs/setup.sh \
    && chmod 700 /opt/opslabs/check.sh \
    && chown root:root /entrypoint.sh /opt/opslabs/*

USER player
WORKDIR /home/player

CMD ["/entrypoint.sh"]
