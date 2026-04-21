# hello-world 场景镜像(引导型,只用最小 base)
#
# 构建:
#   docker build -t opslabs/base-minimal:v1 scenarios-build/base-minimal
#   docker build -t opslabs/hello-world:v1  scenarios/hello-world
#
# 目标镜像大小:压缩后 <30MB(Alpine 基础 + 场景脚本)
FROM opslabs/base-minimal:v1

COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/opslabs/setup.sh
COPY check.sh      /opt/opslabs/check.sh

# check.sh 必须 root 可执行(judging 路径走 docker exec -u root),
# 同时对 player 不可读写(避免在终端 cat 泄露答案)。
# 0700 + root owner 正好:owner=root rwx,其他用户无权。
# entrypoint.sh / setup.sh 要给 player 读+执行,755 owner=root 即可。
RUN chmod 755 /entrypoint.sh /opt/opslabs/setup.sh \
    && chmod 700 /opt/opslabs/check.sh \
    && chown root:root /entrypoint.sh /opt/opslabs/*

# 关键:默认以 player 身份进入容器
#   - setup.sh 在 player 家目录(tmpfs,uid=1000)写文件不再踩 EACCES
#   - ttyd 直接以 player 起,不需要再 su-exec
#   - --cap-drop=ALL 下仍然能跑,因为没有任何用户切换
# 需要以 root 跑 check.sh 时,DockerRunner 用 `docker exec -u root`
# 从宿主端注入,不走容器内的 caps。
USER player

# ENTRYPOINT 已由 base 设为 tini;这里只下发业务 CMD
CMD ["/entrypoint.sh"]
