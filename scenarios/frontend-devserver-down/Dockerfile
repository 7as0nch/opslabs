# frontend-devserver-down:React + Vite 本地 dev server 启动失败
#
# 构建:
#   docker build -t opslabs/base:v1                           scenarios-build/base
#   docker build -t opslabs/frontend-devserver-down:v1        scenarios/frontend-devserver-down
#
# 这个场景 FROM ubuntu base 而不是 base-minimal,因为需要 Node.js 20 + 常用 JS 工具链,
# alpine 上的 musl + prebuilt node 混用容易踩坑。
#
# 场景故障点(2 层):
#   1. 端口 3000 被占坑进程占住(setup.sh 在后台起一个假 python http.server)
#   2. node_modules 里 @vitejs 相关插件被"意外删除"(setup.sh 删掉)
# 预期解法:
#   - 用户读错误信息 → 发现缺 @vitejs/plugin-react → npm install
#   - 启动 npm run dev → 报端口占用 → lsof / ss 定位 → kill 占坑进程
#   - npm run dev & 后台跑 → check.sh 看到 http://localhost:3000 返回 200
FROM opslabs/base:v1

# ---------------- Node.js 20 (NodeSource) ----------------
# Ubuntu 22 的 apt 自带 Node 12 太老,装 Node 20 LTS 保证 Vite 5 能跑
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && npm install -g npm@10 \
    && rm -rf /var/lib/apt/lists/*

# 全局换 npm registry 到 npmmirror —— 国内 docker build 时 registry.npmjs.org
# 偶尔 timeout,而且用户解题阶段跑 `npm install` 也吃这一份配置,体验更顺
#
# 写到 /etc/npmrc 而不是 npm config set --global,因为后者根据 npm 版本可能落到
# /usr/lib/node_modules/npm/npmrc 或 ~/.npmrc,各发行版位置不一致,player 用户
# 不一定读得到。/etc/npmrc 是 npm 默认 builtin path 之外最早的全局加载点,
# root 和 player 都生效。
ARG NPM_REGISTRY=https://registry.npmmirror.com
RUN printf 'registry=%s\nfund=false\naudit=false\n' "${NPM_REGISTRY}" > /etc/npmrc

# ---------------- 预置 webapp 项目 ----------------
# /home/player/webapp 是用户操作目录,player 拥有所有权
# 先以 root 拷贝 + npm install,再 chown 过去
COPY --chown=player:player webapp /home/player/webapp
RUN cd /home/player/webapp \
    && npm install --prefer-offline --no-audit --no-fund \
    && chown -R player:player /home/player/webapp

# ---------------- 场景脚本 ----------------
COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/opslabs/setup.sh
COPY check.sh      /opt/opslabs/check.sh

RUN chmod 755 /entrypoint.sh /opt/opslabs/setup.sh \
    && chmod 700 /opt/opslabs/check.sh \
    && chown root:root /entrypoint.sh /opt/opslabs/*

# 默认 player 身份;check.sh 仍由 docker exec -u root 从宿主端触发
USER player
WORKDIR /home/player/webapp

CMD ["/entrypoint.sh"]
