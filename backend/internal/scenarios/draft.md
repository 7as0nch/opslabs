<!--
 * @Author: chengjiang
 * @Date: 2026-04-20 22:48:50
 * @Description: 
-->

好,我按"前端工程师会遇到"、"后端工程师会遇到"、"运维/SRE 会遇到"各给一个简单真实的场景。都是能在一个容器里跑、能自动判题的那种。

---

## 场景 A:前端场景 —— 本地 dev server 启动失败

**slug**: `frontend-devserver-down`
**难度**: ★★
**预计用时**: 5-8 分钟
**画像**: 前端工程师,熟悉 npm,不太碰 Linux

### 背景故事

> 你刚入职,接手一个 React 项目。同事说"`npm run dev` 就能跑起来了",但你跑起来一直失败。
>
> 项目在 `~/webapp` 目录下。请排查原因,让开发服务器成功启动,浏览器里 `curl http://localhost:3000` 能返回 HTML。
>
> 可能有不止一个问题需要修。

### 预埋故障(setup.sh)

容器启动时做这几件事,制造一组**前端工程师真实会遇到的坑**:

1. **Node 版本不对**:项目 `package.json` 里 `"engines": { "node": ">=20" }`,但容器装的是 Node 16。需要用 nvm 切到 20(提前装好 nvm 和多版本)
2. **依赖没装**:`node_modules` 是空的,需要 `npm install`
3. **端口被占**:后台启动一个 `python3 -m http.server 3000` 占用 3000 端口,要用户先找到并 kill 掉
4. **配置错一行**:`vite.config.js` 里 `host` 写成 `'localhsot'`(故意拼错),导致 vite 启动时绑定失败

为了控制难度,这 4 个坑可以 setup.sh 里**只开启其中 2-3 个**,通过环境变量 `DIFFICULTY=1/2/3` 控制。V1 先固定全开。

### 考点

- Node 版本管理(nvm 使用)
- npm install / 依赖检查
- 端口占用排查(`lsof -i:3000` 或 `ss -tlnp`)
- 配置文件细节阅读(vite.config.js)
- 读懂报错信息

### Check 逻辑

```bash
#!/bin/bash
# 1. 检查 3000 端口有服务响应
resp=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000 --max-time 3)
if [ "$resp" != "200" ]; then
    echo "NO"
    exit 0
fi

# 2. 确认是 vite 在跑(而不是用户用其他东西 workaround)
if ! pgrep -f "vite" > /dev/null; then
    echo "NO"
    exit 0
fi

echo "OK"
```

### 三档提示

- 弱:`npm run dev` 的报错信息别忽略,它会告诉你缺什么
- 中:注意 Node 版本、依赖、端口这三件事分别可能出问题
- 强:`nvm use 20` → 检查 3000 端口 `lsof -i:3000` → `npm install` → 看 vite.config.js 有没有拼写错误

### 设计意图

前端工程师平时**只点击"运行"按钮,很少面对真实 Linux 问题**。这个场景把"同事 3 分钟能解决,我 3 小时解决不了"的常见情况打包起来,让他们学会:看报错、版本、端口、配置文件这四板斧。做完一次,下次自己电脑上遇到类似问题就不慌了。

---

## 场景 B:后端场景 —— API 总是返回 500

**slug**: `backend-api-500`
**难度**: ★★★
**预计用时**: 10-15 分钟
**画像**: 后端工程师,熟悉 HTTP、数据库,但没做过线上排障

### 背景故事

> 一个简单的用户 API 部署在这台服务器上。产品同学反馈:访问 `http://localhost:8080/users/1` 一直返回 500。
>
> 服务看起来在运行(systemctl status app 是 active),但接口就是不通。请修好它,让 `GET /users/1` 返回正常的 200 JSON 响应。
>
> 提示:这个服务依赖一个数据库。

### 预埋故障(setup.sh)

搭建一个最小的 Python Flask 服务连 PostgreSQL,预埋**后端最经典的 3 个坑之一**(随机选或固定):

**Variant 1:数据库密码错了**
- `/etc/app/config.yaml` 里的 DB password 和实际 PostgreSQL 里的不一致
- Flask 日志会打 `FATAL: password authentication failed`

**Variant 2:数据库连接数打满**
- PostgreSQL `max_connections=5`,预先起 5 个 idle 连接占着
- 新连接建立失败

**Variant 3:表不存在 / schema 没初始化**
- 数据库是空的,`users` 表根本没建
- Flask 日志会打 `relation "users" does not exist`

V1 建议先做 **Variant 1**,判题最简单、教学价值最大。

具体实现:

- 用 supervisor 管理 Flask 进程,确保用户 kill 掉会被拉起(或者就用 systemd)
- Flask 代码固定,有详尽的异常捕获和日志(`/var/log/app/error.log`)
- config.yaml 密码写错一个字符
- PostgreSQL 正常运行,密码是 `correct_password`,但 config.yaml 写的是 `correct_passw0rd`

### 考点

- 看服务日志(`journalctl -u app` 或 `/var/log/app/error.log`)
- 理解"服务活着"不等于"服务可用"
- 数据库连接排查(`psql -U ... -W`)
- 配置文件修改 + 服务重启

### Check 逻辑

```bash
#!/bin/bash
# 1. 请求 API 返回 200
resp=$(curl -s -o /tmp/resp.json -w "%{http_code}" http://localhost:8080/users/1 --max-time 5)
if [ "$resp" != "200" ]; then
    echo "NO"
    exit 0
fi

# 2. 返回体包含预期字段
if ! grep -q '"id"' /tmp/resp.json; then
    echo "NO"
    exit 0
fi

# 3. 连续成功 3 次(排除偶然)
for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/users/1)
    if [ "$code" != "200" ]; then
        echo "NO"
        exit 0
    fi
done

echo "OK"
```

### 三档提示

- 弱:API 返回 500 但不知道为啥?去看服务的错误日志
- 中:日志显示数据库认证失败,检查配置文件里的数据库密码
- 强:`cat /etc/app/config.yaml` 看密码 → `psql -h localhost -U app -W` 手动试能不能连上 → 改配置 → `systemctl restart app`

### 设计意图

后端工程师最常见的真实故障就是"接口 500 了"。这个场景教会三件事:

1. **不要直接改代码,先看日志**——日志几乎总能告诉你根因
2. **配置 != 代码**,运行环境的配置文件是真相
3. **改完要重启服务**——很多新人改完 config 忘记重启,还以为自己没改对

这个场景可以做成**3 个 variant 随机抽取**,重复游玩不会腻。

---

## 场景 C:运维场景 —— Nginx 上游连接不上

**slug**: `ops-nginx-upstream-fail`
**难度**: ★★★
**预计用时**: 10-15 分钟
**画像**: 运维/SRE,熟悉服务架构和常见组件

### 背景故事

> 客服反馈网站打不开,你登上服务器。架构很简单:**Nginx 监听 80,反代到后端 8080 的 app 服务**。
>
> 当前现象:
> - `curl http://localhost/` 返回 **502 Bad Gateway**
> - Nginx 本身是活的
> - app 服务据说也是活的
>
> 请找出问题并修复,让 `curl http://localhost/` 返回 200 且响应体包含 `Hello from app`。
>
> **不要重装任何组件,不要改代码,只调整配置和服务状态**。

### 预埋故障(setup.sh)

这是个**复合故障**,模拟真实线上的典型"502 排查":

1. **app 服务实际监听在 8081,但 Nginx upstream 写的是 8080**
   - app 是一个简单的 Python HTTP server,返回固定字符串 "Hello from app"
   - `/etc/nginx/conf.d/default.conf` 里 `proxy_pass http://127.0.0.1:8080;`
   - 用 supervisor 托管 app 进程,保证活着
2. **同时,iptables 里有一条 DROP 规则挡住 8080**(即便用户发现端口不对,如果直接 8080 也连不上,增加迷惑性)
   - 这条规则是"看起来像曾经有人加的"
   - 用户需要决定:改 Nginx 配置指向 8081 即可,不一定要处理 iptables

**允许多种解法**:
- 解法 1(推荐):改 Nginx 配置 `proxy_pass http://127.0.0.1:8081;` → `nginx -s reload`
- 解法 2:把 app 迁移到 8080(但 iptables 挡着还得处理防火墙)
- 解法 3:处理 iptables + 把 app 改到 8080

只要最终 `curl localhost` 通,都算通关。这种**多解法**是好场景的特征。

### 考点

- Nginx 错误日志阅读(`/var/log/nginx/error.log`)
- upstream 配置(`/etc/nginx/conf.d/`)
- 端口实际监听检查(`ss -tlnp` / `netstat -tlnp`)
- 服务与配置的对应关系
- 配置修改后 reload 而非 restart
- 可选的 iptables 诊断(`iptables -L -n`)

### Check 逻辑

```bash
#!/bin/bash
# 1. 从 80 端口能访问
resp=$(curl -s -o /tmp/resp.txt -w "%{http_code}" http://localhost/ --max-time 5)
if [ "$resp" != "200" ]; then
    echo "NO"
    exit 0
fi

# 2. 响应体是 app 返回的内容(而不是某个静态页)
if ! grep -q "Hello from app" /tmp/resp.txt; then
    echo "NO"
    exit 0
fi

# 3. Nginx 确实在跑
if ! pgrep -x nginx > /dev/null; then
    echo "NO"
    exit 0
fi

# 4. app 服务确实在跑
if ! pgrep -f "app.py\|python.*http" > /dev/null; then
    echo "NO"
    exit 0
fi

echo "OK"
```

### 三档提示

- 弱:502 的意思是"Nginx 找不到后端"。先看 Nginx 的错误日志在哪
- 中:错误日志会告诉你 Nginx 尝试连的端口,然后检查后端实际在哪个端口上
- 强:`cat /var/log/nginx/error.log` → 看到 `connect() failed to 127.0.0.1:8080` → `ss -tlnp | grep LISTEN` 看 app 实际端口 → 改 nginx conf → `nginx -s reload`

### 设计意图

这是运维/SRE 最经典的入门题之一,覆盖"三分钟服务器体检"的核心动作:

- 看进程、看端口、看日志、看配置
- 理解"反向代理"这个概念的具体含义
- 学会"多解法"——真实故障没有标准答案,能恢复服务就行
- 认识"reload vs restart"的区别

做完这一题,用户面对生产环境的 502、503 就有了方向感,而不是只会喊"重启试试"。

---

## 三个场景的难度曲线与关联

```
场景 A (前端 devserver)    ★★     偏应用层,Linux 知识要求低
场景 B (后端 API 500)       ★★★    需要结合日志 + 配置 + 依赖组件
场景 C (Nginx 502)          ★★★    需要理解多组件协作 + 多解法思维
```

三个场景**覆盖三种典型用户**,都能在一个 Ubuntu 容器里跑、判题逻辑都是简单的 curl + pgrep + grep,不需要复杂的判题框架。setup 难度都在合理范围(不需要跨容器、不需要真正的 K8s 集群)。

## 场景制作的通用检查清单

以后写新场景,建议对照这 7 条自检:

1. **故事真实**:1-2 句话描述,像工作中真的会遇到
2. **目标明确**:用户知道"做到什么就算成功",不含糊
3. **多解法友好**:至少有 2 条路能通关,判题只看结果不看过程
4. **Check 可自动化**:5 行以内 shell 脚本能判,不依赖用户手动报告
5. **Check 不误伤**:用户用正当方式解决不会被误判
6. **Setup 可重复**:每次启动容器故障一致,不随机(除非刻意设计 variant)
7. **10 分钟能做完**:超过 20 分钟的题不要做,用户会弃坑
