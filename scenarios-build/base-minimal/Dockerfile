# opslabs 轻量基础镜像(Alpine,目标压缩后 <25MB)
#
# 用途:引导型 / shell 练习型场景(hello-world, linux-101-* 等)
#   - 只需要 bash + coreutils + ttyd,不需要联网/不需要 sudo/不需要多进程排查工具
#
# 不用 Ubuntu + 全量工具的理由:
#   - 大:Ubuntu base 压缩后仍 ~150MB,对冷启动和CDN 成本都不友好
#   - 权限超配:默认给 player NOPASSWD sudo,引导场景里完全不需要,提升了命令面
#   - 工具面过宽:lsof/ss/dig 对 hello-world 这种练 touch/ls 的场景毫无意义
#
# 需要完整排查工具的高难度场景,后面会另建 base-shell / base-net / base-node
#
# 构建:
#   docker build -t opslabs/base-minimal:v1 .
FROM alpine:3.19

ENV LANG=C.UTF-8

# ttyd 在 community 仓,3.17+ 都有;bash/coreutils 保证 shebang 一致;
# tini 做 init 进程防僵尸;su-exec 用来在 entrypoint 里从 root 切 player,比 su/gosu 都小
RUN apk add --no-cache \
    bash coreutils \
    ttyd \
    ca-certificates \
    tini su-exec \
    && rm -rf /var/cache/apk/*

# 非 root 用户;不给 sudo —— 引导场景不需要,想要 root 的场景请用 base-shell
RUN adduser -D -s /bin/bash -u 1000 player

# 场景脚本统一放 /opt/opslabs
RUN mkdir -p /opt/opslabs && chmod 755 /opt/opslabs

WORKDIR /home/player
EXPOSE 7681

# 用 tini 做 PID 1,避免 ttyd 的子 bash 变僵尸进程
ENTRYPOINT ["/sbin/tini", "--"]
