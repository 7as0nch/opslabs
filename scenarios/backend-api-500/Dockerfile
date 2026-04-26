# backend-api-500:Python Flask app 持续返回 500 错误
#
# 构建:
#   docker build -t opslabs/base:v1                 scenarios-build/base
#   docker build -t opslabs/backend-api-500:v1      scenarios/backend-api-500
#
# 故障设计:
#   - app 启动时读 /etc/app/config.yaml 里的 db_password
#   - app.py 里有一个伪造的 verify_password() 模拟"PG auth"行为:
#     不等于 hardcoded 的预期密码就抛 "FATAL: password authentication failed"
#   - setup.sh 把 config.yaml 的 db_password 改成错的(off-by-one)
#   - 用户访问 /users/1 → 500
#
# 排障路径(对齐 registry 文案):
#   1. journalctl -u app (实际 tail /var/log/app/error.log) 看到 password authentication failed
#   2. cat /etc/app/config.yaml 看到 db_password: opslabs2025(错)
#   3. 改成 opslabs2026
#   4. systemctl restart app
#   5. curl localhost:8080/users/1 → 200
#
# 为什么不真接 PostgreSQL:
#   PG server 启动 + initdb 让镜像膨胀 200MB+,冷启动 5-10s,sandbox 体验差;
#   故障语义("配置写错 → 看日志 → 改 → 重启")对训练目的足够,真实 PG 留给 V2。
FROM opslabs/base:v1

# ---------------- Python + Flask ----------------
RUN apt-get update && apt-get install -y --no-install-recommends \
        python3 python3-pip python3-flask python3-yaml \
    && rm -rf /var/lib/apt/lists/*

# ---------------- mini systemctl / journalctl wrapper ----------------
# 容器里没 systemd,但用户的肌肉记忆是 systemctl/journalctl。提供一层薄壳让
# "systemctl restart app" / "systemctl status app" / "journalctl -u app" 可用。
# 状态文件 /var/run/app.pid,日志 /var/log/app/error.log。
COPY mini-systemctl /usr/local/bin/systemctl
COPY mini-journalctl /usr/local/bin/journalctl
RUN chmod 755 /usr/local/bin/systemctl /usr/local/bin/journalctl \
    && mkdir -p /var/log/app /etc/app /var/run \
    && chown -R player:player /var/log/app /etc/app /var/run

# ---------------- app 源码 + 配置 ----------------
COPY app /opt/app
RUN chmod 755 /opt/app/app.py \
    && chown -R player:player /opt/app

# 初始 config.yaml(setup.sh 会篡改 db_password)
RUN cat > /etc/app/config.yaml <<'EOF'
# /etc/app/config.yaml
# 后端 user-api 服务的运行配置。修改后 systemctl restart app 生效。
listen_port: 8080
db_path: /var/lib/app/users.db
db_user: opslabs
db_password: opslabs2026
log_level: INFO
EOF
RUN chown player:player /etc/app/config.yaml \
    && chmod 644 /etc/app/config.yaml

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
