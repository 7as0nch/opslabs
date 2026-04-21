<!--
 * @Author: chengjiang
 * @Date: 2026-04-20 17:45:37
 * @Description: 各种场景设计。
-->

好,我把 `hello-world` 引导场景 + 前面设计的三个入门场景(前端/后端/运维)都按 schema 完整做出来。每个场景给一套完整文件。

---

## 场景 0:hello-world(引导场景)

### `scenarios/hello-world/meta.yaml`

```yaml
slug: hello-world
version: "1.0.0"

title: "欢迎来到 FixIt"
summary: "熟悉平台操作,3 分钟上手第一个任务"
description_file: "README.md"

category: guide
difficulty: 1
estimated_minutes: 3

target_personas:
  - student
  - frontend-engineer
  - backend-engineer
  - ops-engineer
experience_level: intern

tech_stack:
  - linux
skills:
  - basic-shell
commands:
  - cat
  - touch
  - ls
tags:
  - onboarding
  - tutorial

runtime:
  image: fixit/hello-world:v1
  memory_mb: 256
  cpus: 0.3
  idle_timeout_minutes: 30
  passed_grace_minutes: 10
  network_mode: isolated
  variants: []

grading:
  check_script: /opt/fixit/check.sh
  check_timeout_seconds: 5
  success_output: "OK"

hints:
  - level: 1
    content: "不知道如何查看文件?试试 cat ~/welcome.txt"
  - level: 2
    content: "创建空文件可以用 touch 命令,格式:touch /路径/文件名"
  - level: 3
    content: "在终端执行:touch /tmp/ready.flag"

learning_resources: []
prerequisites: []
recommended_next:
  - frontend-devserver-down
  - backend-api-500
  - ops-nginx-upstream-fail

author: official
created_at: "2026-04-20"
updated_at: "2026-04-20"
is_published: true
is_premium: false
```

### `scenarios/hello-world/README.md`

```markdown
# 欢迎来到 FixIt

你好,欢迎!

在你开始真正的故障排查之前,先用 3 分钟熟悉一下这个环境。

## 你面前的工作台

- **左侧**:这份任务说明(你现在正在读的)
- **右侧**:一个真实的 Linux 终端,你可以在里面执行任意命令

## 你的第一个任务

1. 查看你 home 目录下的 `welcome.txt` 文件
2. 按照文件里的指示完成一个小操作
3. 点击右下角的「检查答案」按钮

完成后你就正式上路了。

## 小贴士

- 遇到不会的命令别慌,下方有三档提示可以看
- 提示越强会暴露越多答案,建议先自己试试
- 每个场景都有时间限制,空闲太久容器会回收
```

### `scenarios/hello-world/setup.sh`

```bash
#!/bin/bash
set -e

cat > /home/player/welcome.txt <<'EOF'
欢迎来到 FixIt。

你的第一个任务:
在 /tmp 下创建一个名为 ready.flag 的空文件,
然后点击界面上的「检查答案」按钮。

提示:
  - 创建空文件可以用 touch 命令
  - 例如: touch /tmp/example.txt
EOF

chown player:player /home/player/welcome.txt
chmod 644 /home/player/welcome.txt

cat >> /home/player/.bashrc <<'EOF'

echo ""
echo "========================================"
echo "  欢迎!请查看 ~/welcome.txt"
echo "========================================"
echo ""
EOF
chown player:player /home/player/.bashrc

exit 0
```

### `scenarios/hello-world/check.sh`

```bash
#!/bin/bash
set -o pipefail

if [ ! -f /tmp/ready.flag ]; then
    echo "NO"
    echo "flag 文件还未创建" >&2
    exit 0
fi

echo "OK"
exit 0
```

### `scenarios/hello-world/entrypoint.sh`

```bash
#!/bin/bash
set -e
/opt/fixit/setup.sh
exec su - player -c "ttyd -W -p 7681 --writable bash"
```

### `scenarios/hello-world/Dockerfile`

```dockerfile
FROM fixit/base:v1

COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/fixit/setup.sh
COPY check.sh      /opt/fixit/check.sh

RUN chmod 755 /entrypoint.sh /opt/fixit/setup.sh \
    && chmod 600 /opt/fixit/check.sh \
    && chown root:root /opt/fixit/*

CMD ["/entrypoint.sh"]
```

### `scenarios/hello-world/tests/solution.sh`

```bash
#!/bin/bash
touch /tmp/ready.flag
```

### `scenarios/hello-world/tests/regression.sh`

```bash
#!/bin/bash
if [ -f /tmp/ready.flag ]; then
    echo "FAIL: flag 不应该一开始就存在"
    exit 1
fi
echo "PASS"
```

---

## 场景 1:frontend-devserver-down(前端)

### `scenarios/frontend-devserver-down/meta.yaml`

```yaml
slug: frontend-devserver-down
version: "1.0.0"

title: "本地 dev server 启动失败"
summary: "接手项目跑不起来,排查 Node 版本、依赖、端口、配置"
description_file: "README.md"

category: frontend
difficulty: 2
estimated_minutes: 8

target_personas:
  - frontend-engineer
  - full-stack
  - student
experience_level: junior

tech_stack:
  - nodejs
  - vite
  - react
skills:
  - dependency-management
  - port-conflict
  - config-troubleshooting
  - version-management
commands:
  - node
  - npm
  - nvm
  - lsof
  - ss
tags:
  - onboarding
  - real-world
  - common

runtime:
  image: fixit/frontend-devserver-down:v1
  memory_mb: 1024
  cpus: 0.5
  idle_timeout_minutes: 30
  passed_grace_minutes: 10
  network_mode: internet-allowed
  variants: []

grading:
  check_script: /opt/fixit/check.sh
  check_timeout_seconds: 10
  success_output: "OK"

hints:
  - level: 1
    content: "npm run dev 的报错信息别忽略,它通常会告诉你缺什么。同时注意端口占用、Node 版本、依赖安装这几件事"
  - level: 2
    content: "依次检查:Node 版本是否符合 package.json 的要求、node_modules 是否存在、3000 端口是否被占用、配置文件里有没有拼写错误"
  - level: 3
    content: "nvm use 20 切 Node 版本 → lsof -i:3000 找占用进程并 kill → npm install 装依赖 → cat vite.config.js 看 host 字段拼写"

learning_resources: []
prerequisites:
  - hello-world
recommended_next:
  - backend-api-500

author: official
created_at: "2026-04-20"
updated_at: "2026-04-20"
is_published: true
is_premium: false
```

### `scenarios/frontend-devserver-down/README.md`

```markdown
# 本地 dev server 启动失败

## 背景

你刚入职,接手一个 React 项目,目录在 `~/webapp`。

同事跟你说:"直接 `npm run dev` 就能跑起来了。"

但你试了好几次都失败。

## 你的任务

排查问题,让开发服务器成功启动。

**验收标准**:在另一个 shell 里执行 `curl http://localhost:3000`,返回 HTTP 200 且响应体是 HTML 内容。

## 提示

- 可能不止一个问题
- 别忽略报错信息,它通常会给你方向
- 你有 sudo 权限,可以装包、改配置、kill 进程
```

### `scenarios/frontend-devserver-down/setup.sh`

```bash
#!/bin/bash
set -e

# 准备项目目录
mkdir -p /home/player/webapp
cd /home/player/webapp

# 创建一个最小 Vite + React 项目(用预构建的资源,避免网络依赖)
cat > package.json <<'EOF'
{
  "name": "webapp",
  "version": "0.1.0",
  "type": "module",
  "engines": {
    "node": ">=20"
  },
  "scripts": {
    "dev": "vite"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.2.0",
    "vite": "^5.0.0"
  }
}
EOF

# 故障 1: vite.config.js 里 host 拼错
cat > vite.config.js <<'EOF'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: 'localhsot',
    port: 3000
  }
})
EOF

cat > index.html <<'EOF'
<!DOCTYPE html>
<html>
  <head><title>Webapp</title></head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.jsx"></script>
  </body>
</html>
EOF

mkdir -p src
cat > src/main.jsx <<'EOF'
import React from 'react'
import ReactDOM from 'react-dom/client'

ReactDOM.createRoot(document.getElementById('root')).render(
  <h1>Hello from app</h1>
)
EOF

chown -R player:player /home/player/webapp

# 故障 2: node_modules 不存在(用户需要 npm install)
# 已经不存在,不用做什么

# 故障 3: 3000 端口被占用
nohup python3 -m http.server 3000 --directory /tmp > /dev/null 2>&1 &
echo $! > /var/run/port-squatter.pid

# 故障 4: 默认 Node 版本是 16(nvm 预装了 16 和 20,当前激活 16)
# nvm 在基础镜像里已经装好,默认指向 16
# 用户必须 nvm use 20

# 提示信息
cat >> /home/player/.bashrc <<'EOF'

echo ""
echo "================================================"
echo "  项目在 ~/webapp,同事说 npm run dev 就能跑起来"
echo "  但你试了几次都起不来。排查一下吧。"
echo "================================================"
echo ""
cd ~/webapp
EOF
chown player:player /home/player/.bashrc

exit 0
```

### `scenarios/frontend-devserver-down/check.sh`

```bash
#!/bin/bash
set -o pipefail

# 连续请求 3 次,确认稳定
for i in 1 2 3; do
    code=$(curl -s -o /tmp/check_resp.html -w "%{http_code}" \
        --max-time 3 http://localhost:3000/)
    if [ "$code" != "200" ]; then
        echo "NO"
        echo "http status: $code" >&2
        exit 0
    fi
    sleep 0.3
done

# 确认返回的是 HTML(不是 python 的目录索引)
if ! grep -qi "<html\|<!DOCTYPE" /tmp/check_resp.html; then
    echo "NO"
    echo "response is not HTML" >&2
    exit 0
fi

# 确认是 vite 在跑(而不是其他 http server workaround)
if ! pgrep -f "vite" > /dev/null; then
    echo "NO"
    echo "vite process not found" >&2
    exit 0
fi

echo "OK"
exit 0
```

### `scenarios/frontend-devserver-down/Dockerfile`

```dockerfile
FROM fixit/base:v1

RUN apt-get update && apt-get install -y python3 \
    && rm -rf /var/lib/apt/lists/*

ENV NVM_DIR=/usr/local/nvm
RUN mkdir -p $NVM_DIR \
    && curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash \
    && . $NVM_DIR/nvm.sh \
    && nvm install 16 \
    && nvm install 20 \
    && nvm alias default 16

RUN echo 'export NVM_DIR="/usr/local/nvm"' >> /home/player/.bashrc \
    && echo '[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"' >> /home/player/.bashrc \
    && chown -R player:player /home/player/.bashrc $NVM_DIR

COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/fixit/setup.sh
COPY check.sh      /opt/fixit/check.sh

RUN chmod 755 /entrypoint.sh /opt/fixit/setup.sh \
    && chmod 600 /opt/fixit/check.sh \
    && chown root:root /opt/fixit/*

CMD ["/entrypoint.sh"]
```

### `scenarios/frontend-devserver-down/entrypoint.sh`

```bash
#!/bin/bash
set -e
/opt/fixit/setup.sh
exec su - player -c "ttyd -W -p 7681 --writable bash"
```

### `scenarios/frontend-devserver-down/tests/solution.sh`

```bash
#!/bin/bash
set -e

export NVM_DIR=/usr/local/nvm
. $NVM_DIR/nvm.sh

cd /home/player/webapp

# 1. 切 Node 版本
nvm use 20

# 2. 杀掉占 3000 的进程
if [ -f /var/run/port-squatter.pid ]; then
    kill $(cat /var/run/port-squatter.pid) 2>/dev/null || true
fi

# 3. 修配置拼写错误
sed -i "s/localhsot/localhost/" vite.config.js

# 4. 装依赖
npm install --silent

# 5. 后台启动
nohup npm run dev > /tmp/vite.log 2>&1 &

# 等 vite 起来
for i in $(seq 1 30); do
    if curl -s http://localhost:3000 > /dev/null; then
        exit 0
    fi
    sleep 1
done

echo "vite failed to start"
exit 1
```

### `scenarios/frontend-devserver-down/tests/regression.sh`

```bash
#!/bin/bash
code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 http://localhost:3000/)

if curl -s http://localhost:3000 | grep -qi "<html\|<!DOCTYPE"; then
    echo "FAIL: dev server 不应该一开始就能正常返回 HTML"
    exit 1
fi

if [ -d /home/player/webapp/node_modules ]; then
    echo "FAIL: node_modules 不应该一开始就存在"
    exit 1
fi

echo "PASS"
```

---

## 场景 2:backend-api-500(后端)

### `scenarios/backend-api-500/meta.yaml`

```yaml
slug: backend-api-500
version: "1.0.0"

title: "API 总是返回 500"
summary: "用户接口一直 500,看日志、查配置、验证数据库连接"
description_file: "README.md"

category: backend
difficulty: 3
estimated_minutes: 12

target_personas:
  - backend-engineer
  - full-stack
experience_level: junior

tech_stack:
  - python
  - flask
  - postgresql
  - systemd
skills:
  - log-analysis
  - config-troubleshooting
  - database-connectivity
  - service-management
commands:
  - journalctl
  - systemctl
  - psql
  - curl
  - tail
tags:
  - interview-common
  - real-world
  - 500-error

runtime:
  image: fixit/backend-api-500:v1
  memory_mb: 768
  cpus: 0.5
  idle_timeout_minutes: 30
  passed_grace_minutes: 10
  network_mode: isolated
  variants:
    - db-password

grading:
  check_script: /opt/fixit/check.sh
  check_timeout_seconds: 10
  success_output: "OK"

hints:
  - level: 1
    content: "API 返回 500 但你不知道为啥?先看服务的错误日志,别盲猜"
  - level: 2
    content: "日志会告诉你数据库相关的错。检查配置文件里的数据库密码是否正确"
  - level: 3
    content: "看 /var/log/app/error.log 发现 password authentication failed。cat /etc/app/config.yaml 对比 PostgreSQL 的实际密码。改对后 systemctl restart app"

learning_resources: []
prerequisites:
  - hello-world
recommended_next:
  - ops-nginx-upstream-fail

author: official
created_at: "2026-04-20"
updated_at: "2026-04-20"
is_published: true
is_premium: false
```

### `scenarios/backend-api-500/README.md`

```markdown
# API 总是返回 500

## 背景

一个用户 API 部署在这台服务器上。产品反馈:

> "访问 `http://localhost:8080/users/1` 一直返回 500,你看看咋回事"

你登上服务器检查:

- 服务进程在跑(`systemctl status app` 显示 active)
- PostgreSQL 也在跑
- 但接口就是不通

## 你的任务

找出问题,让 `GET /users/1` 返回 200 和正常的 JSON 响应。

**验收标准**:连续 3 次 `curl http://localhost:8080/users/1` 都返回 200,且响应体包含 `"id"` 字段。

## 提示

- 不要直接改代码,先看日志
- 这个服务依赖 PostgreSQL 数据库
- 修改配置后记得重启服务
```

### `scenarios/backend-api-500/assets/app.py`

```python
#!/usr/bin/env python3
import os
import yaml
import logging
import psycopg2
from flask import Flask, jsonify

logging.basicConfig(
    filename='/var/log/app/error.log',
    level=logging.ERROR,
    format='%(asctime)s %(levelname)s %(message)s'
)

app = Flask(__name__)

with open('/etc/app/config.yaml') as f:
    config = yaml.safe_load(f)

def get_db():
    return psycopg2.connect(
        host=config['db']['host'],
        port=config['db']['port'],
        user=config['db']['user'],
        password=config['db']['password'],
        dbname=config['db']['name'],
        connect_timeout=3
    )

@app.route('/users/<int:user_id>')
def get_user(user_id):
    try:
        conn = get_db()
        cur = conn.cursor()
        cur.execute("SELECT id, name FROM users WHERE id = %s", (user_id,))
        row = cur.fetchone()
        cur.close()
        conn.close()
        if not row:
            return jsonify({"error": "not found"}), 404
        return jsonify({"id": row[0], "name": row[1]})
    except Exception as e:
        app.logger.error(f"request failed: {e}")
        return jsonify({"error": "internal server error"}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
```

### `scenarios/backend-api-500/assets/config.yaml`

```yaml
db:
  host: localhost
  port: 5432
  user: app
  password: correct_passw0rd
  name: appdb
```

### `scenarios/backend-api-500/assets/app.service`

```ini
[Unit]
Description=User API
After=postgresql.service

[Service]
Type=simple
ExecStart=/usr/bin/python3 /opt/app/app.py
Restart=on-failure
StandardOutput=append:/var/log/app/stdout.log
StandardError=append:/var/log/app/error.log

[Install]
WantedBy=multi-user.target
```

### `scenarios/backend-api-500/setup.sh`

```bash
#!/bin/bash
set -e

mkdir -p /etc/app /opt/app /var/log/app
cp /opt/fixit/assets/app.py         /opt/app/app.py
cp /opt/fixit/assets/config.yaml    /etc/app/config.yaml
cp /opt/fixit/assets/app.service    /etc/systemd/system/app.service
chmod 755 /opt/app/app.py

pg_ctlcluster 14 main start || service postgresql start

sudo -u postgres psql <<'EOF'
CREATE USER app WITH PASSWORD 'correct_password';
CREATE DATABASE appdb OWNER app;
\c appdb
CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(100));
INSERT INTO users (name) VALUES ('alice'), ('bob'), ('charlie');
GRANT ALL ON TABLE users TO app;
GRANT USAGE, SELECT ON SEQUENCE users_id_seq TO app;
EOF

systemctl daemon-reload
systemctl enable app
systemctl start app

sleep 2

cat >> /home/player/.bashrc <<'EOF'

echo ""
echo "================================================"
echo "  任务:访问 http://localhost:8080/users/1 一直 500"
echo "  找出问题并修好,让接口返回 200"
echo "================================================"
echo ""
EOF
chown player:player /home/player/.bashrc

exit 0
```

### `scenarios/backend-api-500/check.sh`

```bash
#!/bin/bash
set -o pipefail

for i in 1 2 3; do
    resp_file=/tmp/check_resp_$i.json
    code=$(curl -s -o "$resp_file" -w "%{http_code}" \
        --max-time 5 http://localhost:8080/users/1)

    if [ "$code" != "200" ]; then
        echo "NO"
        echo "request $i: http $code" >&2
        exit 0
    fi

    if ! grep -q '"id"' "$resp_file"; then
        echo "NO"
        echo "request $i: missing id field" >&2
        exit 0
    fi

    sleep 0.3
done

echo "OK"
exit 0
```

### `scenarios/backend-api-500/Dockerfile`

```dockerfile
FROM fixit/base:v1

RUN apt-get update && apt-get install -y \
    python3 python3-pip \
    postgresql postgresql-contrib \
    systemd \
    && rm -rf /var/lib/apt/lists/*

RUN pip3 install flask psycopg2-binary pyyaml --break-system-packages

COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/fixit/setup.sh
COPY check.sh      /opt/fixit/check.sh
COPY assets/       /opt/fixit/assets/

RUN chmod 755 /entrypoint.sh /opt/fixit/setup.sh \
    && chmod 600 /opt/fixit/check.sh \
    && chown root:root /opt/fixit/*

CMD ["/entrypoint.sh"]
```

### `scenarios/backend-api-500/entrypoint.sh`

```bash
#!/bin/bash
set -e
/opt/fixit/setup.sh
exec su - player -c "ttyd -W -p 7681 --writable bash"
```

### `scenarios/backend-api-500/tests/solution.sh`

```bash
#!/bin/bash
set -e

sed -i "s/correct_passw0rd/correct_password/" /etc/app/config.yaml
systemctl restart app

for i in $(seq 1 10); do
    if curl -s http://localhost:8080/users/1 | grep -q '"id"'; then
        exit 0
    fi
    sleep 1
done

echo "service did not recover"
exit 1
```

### `scenarios/backend-api-500/tests/regression.sh`

```bash
#!/bin/bash
code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://localhost:8080/users/1)
if [ "$code" = "200" ]; then
    echo "FAIL: API 不应该一开始就正常"
    exit 1
fi
echo "PASS (http $code)"
```

---

## 场景 3:ops-nginx-upstream-fail(运维)

### `scenarios/ops-nginx-upstream-fail/meta.yaml`

```yaml
slug: ops-nginx-upstream-fail
version: "1.0.0"

title: "Nginx 反代 502 排查"
summary: "Nginx 返回 502,后端明明活着,找出 upstream 问题"
description_file: "README.md"

category: ops
difficulty: 3
estimated_minutes: 12

target_personas:
  - ops-engineer
  - sre
  - devops-engineer
  - backend-engineer
experience_level: mid

tech_stack:
  - nginx
  - linux
  - systemd
skills:
  - log-analysis
  - service-management
  - network-troubleshooting
  - config-troubleshooting
  - port-conflict
commands:
  - nginx
  - ss
  - netstat
  - tail
  - curl
  - systemctl
tags:
  - interview-common
  - real-world
  - 502-error

runtime:
  image: fixit/ops-nginx-upstream-fail:v1
  memory_mb: 512
  cpus: 0.5
  idle_timeout_minutes: 30
  passed_grace_minutes: 10
  network_mode: isolated
  variants: []

grading:
  check_script: /opt/fixit/check.sh
  check_timeout_seconds: 10
  success_output: "OK"

hints:
  - level: 1
    content: "502 意味着 Nginx 连不上后端。先看 Nginx 错误日志,它会告诉你 Nginx 在尝试连哪个端口"
  - level: 2
    content: "日志里的端口和后端 app 实际监听的端口对得上吗?用 ss -tlnp 看 app 真实监听在哪"
  - level: 3
    content: "tail /var/log/nginx/error.log 看到 connect() failed → ss -tlnp | grep python 发现 app 在 8081 → 改 /etc/nginx/conf.d/default.conf 的 proxy_pass → nginx -s reload"

learning_resources: []
prerequisites:
  - hello-world
recommended_next: []

author: official
created_at: "2026-04-20"
updated_at: "2026-04-20"
is_published: true
is_premium: false
```

### `scenarios/ops-nginx-upstream-fail/README.md`

```markdown
# Nginx 反代 502 排查

## 背景

客服反馈网站打不开,你登上服务器检查。架构很简单:

```
Client → Nginx (:80) → app service (:8080)
```

当前情况:

- `curl http://localhost/` 返回 **502 Bad Gateway**
- Nginx 进程在跑
- app 服务进程也在跑

## 你的任务

找出问题并修复,让 `curl http://localhost/` 返回 200,响应体包含 `Hello from app`。

**约束**:

- 不要重装任何组件
- 不要改 app 的代码
- 只调整配置和服务状态即可

## 提示

- 有多种解法,能让服务恢复就行
- 修改 nginx 配置后用 reload,不要直接 restart
```

### `scenarios/ops-nginx-upstream-fail/assets/app.py`

```python
#!/usr/bin/env python3
from http.server import HTTPServer, BaseHTTPRequestHandler

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'text/plain')
        self.end_headers()
        self.wfile.write(b'Hello from app\n')
    def log_message(self, *args): pass

HTTPServer(('127.0.0.1', 8081), Handler).serve_forever()
```

### `scenarios/ops-nginx-upstream-fail/assets/default.conf`

```nginx
server {
    listen 80;
    server_name _;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### `scenarios/ops-nginx-upstream-fail/assets/app.service`

```ini
[Unit]
Description=Backend App

[Service]
Type=simple
ExecStart=/usr/bin/python3 /opt/app/app.py
Restart=always

[Install]
WantedBy=multi-user.target
```

### `scenarios/ops-nginx-upstream-fail/setup.sh`

```bash
#!/bin/bash
set -e

mkdir -p /opt/app
cp /opt/fixit/assets/app.py      /opt/app/app.py
cp /opt/fixit/assets/app.service /etc/systemd/system/app.service
chmod 755 /opt/app/app.py

rm -f /etc/nginx/sites-enabled/default
cp /opt/fixit/assets/default.conf /etc/nginx/conf.d/default.conf

systemctl daemon-reload
systemctl enable app
systemctl start app
systemctl restart nginx

sleep 2

cat >> /home/player/.bashrc <<'EOF'

echo ""
echo "================================================"
echo "  任务:curl http://localhost/ 一直返回 502"
echo "  Nginx 和后端服务都在跑,找出原因并修好它"
echo "================================================"
echo ""
EOF
chown player:player /home/player/.bashrc

exit 0
```

### `scenarios/ops-nginx-upstream-fail/check.sh`

```bash
#!/bin/bash
set -o pipefail

resp_file=/tmp/check_resp.txt
code=$(curl -s -o "$resp_file" -w "%{http_code}" \
    --max-time 5 http://localhost/)

if [ "$code" != "200" ]; then
    echo "NO"
    echo "http status: $code" >&2
    exit 0
fi

if ! grep -q "Hello from app" "$resp_file"; then
    echo "NO"
    echo "response body mismatch" >&2
    exit 0
fi

if ! pgrep -x nginx > /dev/null; then
    echo "NO"
    echo "nginx not running" >&2
    exit 0
fi

if ! pgrep -f "app.py" > /dev/null; then
    echo "NO"
    echo "app.py not running" >&2
    exit 0
fi

echo "OK"
exit 0
```

### `scenarios/ops-nginx-upstream-fail/Dockerfile`

```dockerfile
FROM fixit/base:v1

RUN apt-get update && apt-get install -y \
    nginx python3 systemd \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/fixit/setup.sh
COPY check.sh      /opt/fixit/check.sh
COPY assets/       /opt/fixit/assets/

RUN chmod 755 /entrypoint.sh /opt/fixit/setup.sh \
    && chmod 600 /opt/fixit/check.sh \
    && chown root:root /opt/fixit/*

CMD ["/entrypoint.sh"]
```

### `scenarios/ops-nginx-upstream-fail/entrypoint.sh`

```bash
#!/bin/bash
set -e
/opt/fixit/setup.sh
exec su - player -c "ttyd -W -p 7681 --writable bash"
```

### `scenarios/ops-nginx-upstream-fail/tests/solution.sh`

```bash
#!/bin/bash
set -e

sed -i "s|proxy_pass http://127.0.0.1:8080;|proxy_pass http://127.0.0.1:8081;|" \
    /etc/nginx/conf.d/default.conf
nginx -s reload

for i in $(seq 1 10); do
    if curl -s http://localhost/ | grep -q "Hello from app"; then
        exit 0
    fi
    sleep 1
done

echo "service did not recover"
exit 1
```

### `scenarios/ops-nginx-upstream-fail/tests/regression.sh`

```bash
#!/bin/bash
code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://localhost/)
if [ "$code" = "200" ]; then
    echo "FAIL: 不应该一开始就正常"
    exit 1
fi
echo "PASS (http $code)"
```

---

## 共用基础镜像(所有场景依赖)

### `scenarios-build/base/Dockerfile`

```dockerfile
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

RUN curl -L -o /usr/local/bin/ttyd \
    https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.x86_64 \
    && chmod +x /usr/local/bin/ttyd

RUN useradd -m -s /bin/bash player \
    && echo "player ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

RUN mkdir -p /opt/fixit && chmod 755 /opt/fixit

WORKDIR /home/player
EXPOSE 7681
```

---

## 构建与测试的一键脚本

### `scripts/build-all-scenarios.sh`

```bash
#!/bin/bash
set -e

BASE_DIR=$(cd "$(dirname "$0")/.." && pwd)

docker build -t fixit/base:v1 "$BASE_DIR/scenarios-build/base"

for slug in hello-world frontend-devserver-down backend-api-500 ops-nginx-upstream-fail; do
    docker build -t "fixit/$slug:v1" "$BASE_DIR/scenarios/$slug"
done

echo "all images built"
```

### `scripts/test-scenario.sh`

```bash
#!/bin/bash
set -e

SLUG=$1
if [ -z "$SLUG" ]; then
    echo "usage: $0 <scenario-slug>"
    exit 1
fi

IMAGE="fixit/$SLUG:v1"
NAME="fixit-test-$SLUG-$$"

cleanup() {
    docker rm -f "$NAME" > /dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name "$NAME" --privileged "$IMAGE" > /dev/null
sleep 3

echo "[1/3] regression check (故障应已预埋)..."
docker cp "scenarios/$SLUG/tests/regression.sh" "$NAME:/tmp/regression.sh"
docker exec "$NAME" bash /tmp/regression.sh

echo "[2/3] running solution..."
docker cp "scenarios/$SLUG/tests/solution.sh" "$NAME:/tmp/solution.sh"
docker exec "$NAME" bash /tmp/solution.sh

echo "[3/3] check after solution (应返回 OK)..."
result=$(docker exec "$NAME" bash /opt/fixit/check.sh | head -1)
if [ "$result" != "OK" ]; then
    echo "FAIL: check.sh after solution = '$result'"
    exit 1
fi

echo "all pass: $SLUG"
```

用法:

```bash
./scripts/build-all-scenarios.sh
./scripts/test-scenario.sh hello-world
./scripts/test-scenario.sh frontend-devserver-down
./scripts/test-scenario.sh backend-api-500
./scripts/test-scenario.sh ops-nginx-upstream-fail
```

---

## 几个落地时的关键提醒

1. **systemd 在容器里需要 `--privileged`**:backend-api-500 和 ops-nginx-upstream-fail 都依赖 systemd,启动容器时要加这个参数,或者改用 supervisord(更轻,但不如 systemd 真实)。V1 先用 privileged,V2 再考虑 supervisord 替代。

2. **frontend 场景需要联网**:npm install 要拉包,所以 `network_mode: internet-allowed`。后端启动容器时看这个字段决定是否走隔离网络。

3. **PostgreSQL 初始化要等一下**:setup.sh 里 `pg_ctlcluster` 后最好 `sleep 1`,避免 psql 连接太快失败。

4. **tests/solution.sh 是 CI 的命根子**:每次场景变更,CI 必须自动跑一遍 `test-scenario.sh`。任何一步挂了就不许合并。这是场景质量的唯一靠谱保障。

5. **场景之间独立**:每个场景自成一个镜像,没有交叉依赖。这样 V2 要拆出独立仓库、接外部贡献者都很容易。

这四个场景跑通后,你就有了一个**完整的内容体系雏形**——从最简单的引导到三个典型用户画像的入门题,每个都遵循统一 schema,每个都有自动化测试兜底。按这个模板再加 10 个场景,就够 MVP 上线了。
