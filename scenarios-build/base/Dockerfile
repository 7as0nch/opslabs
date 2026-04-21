# ⚠️ DEPRECATED — 优先使用 scenarios-build/base-minimal/(Alpine,压缩后 <25MB)
#
# 这份 Ubuntu 22.04 基础镜像压缩后 ~150MB,用于需要完整 glibc + Ubuntu 包的
# 高难度排查场景(Week 2 的 frontend-devserver-down / backend-api-500 可能用到);
# 引导型场景(hello-world, linux-101-*)已全部迁移到 base-minimal,不要再用这里。
#
# 仍然保留的原因:
#   - 某些包(node 的 prebuilt binary、特定版本 nginx)在 alpine/musl 下不兼容
#   - 需要 journalctl / systemctl 的 ops 场景(systemd 栈目前还是 Ubuntu 顺)
#
# 如果你的场景 FROM opslabs/base:v1 只是为了 touch / ls / cat,一定改成 base-minimal。
#
# 内置:
#   - 常用排查命令 (lsof, ss, netstat, curl, tail, ping, dig, ...)
#   - ttyd 终端(7681 端口)
#   - 非 root 用户 player (有 NOPASSWD sudo)
#   - zh_CN.UTF-8 locale + Asia/Shanghai 时区
#
# 构建:
#   docker build -t opslabs/base:v1 .
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV LANG=C.UTF-8

RUN apt-get update && apt-get install -y \
    curl wget vim nano less \
    procps net-tools iputils-ping dnsutils lsof iproute2 \
    locales tzdata sudo ca-certificates \
    && locale-gen zh_CN.UTF-8 \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && rm -rf /var/lib/apt/lists/*

# ttyd: 把 shell 暴露成 websocket,前端 iframe 可直接连
RUN curl -L -o /usr/local/bin/ttyd \
    https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.x86_64 \
    && chmod +x /usr/local/bin/ttyd

# 非 root 用户;实际答题进程跑在这里,避免 root 过权
RUN useradd -m -s /bin/bash player \
    && echo "player ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# 场景脚本统一放到 /opt/opslabs
RUN mkdir -p /opt/opslabs && chmod 755 /opt/opslabs

WORKDIR /home/player
EXPOSE 7681
