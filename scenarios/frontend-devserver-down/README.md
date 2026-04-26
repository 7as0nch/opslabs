# 本地 dev server 启动失败

## 背景

你刚入职,接手一个 React 项目,目录在 `~/webapp`。

同事跟你说:"直接 `npm run dev` 就能跑起来了。"

但你试了好几次都失败。你有 sudo 权限,可以装包 / 改配置 / kill 进程。

## 你的任务

让 React + Vite dev server 在 `http://localhost:3000/` 成功提供页面。

**验收标准**:在同一个容器里执行 `curl http://localhost:3000/`,返回 HTTP 200,且响应体是 Vite dev 注入过的 HTML(看得到 `/src/main.jsx` 这段)。

## 小贴士

- **问题不止一个**。按报错一步一步来。
- 3000 端口可能被其他进程占了,记得找找是谁。
- Vite 要前台一直跑,但前台会阻塞当前终端 —— 用 `npm run dev &` 或 `nohup npm run dev &` 把它丢到后台,再点"检查答案"。

## 禁止操作

- 不要改 `check.sh`。
- 不要改 `/etc/hosts` 把 localhost:3000 指到别处骗判题。

## 预期耗时

5-10 分钟。你能在 10 分钟之内让 dev server 起来,就算已经有手感了。
