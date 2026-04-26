# 变更记录 · 2026-04-24 → 2026-04-25

> 本文记录这两天围绕 PLAN.md 落实 MVP 的所有变更,按"修复 → 优化 → 内容补齐 → 文档"四个维度整理。
> 长期维护的 PLAN.md / INTRODUCTION.md 只反映当前状态,不会累积变更细节,所以变更记录单独成文。

---

## 一、Bug 修复(2026-04-25)

用户实测反馈两个体感强的问题:**放弃做了 / 提交成功后再点检查,提示"已提交过";倒计时还是上次的**。本质是同一个根因。

### 1.0 [Bug] WebContainerRunner 切文件后内容被其他文件覆盖

**现象**:用户在 web-container 场景里点 README.md → 再点 handler.js,handler.js 的 Monaco 编辑器显示的是 README.md 的文档内容,而且 handler.js 还被标了"未保存"。

**根因**:`@monaco-editor/react` 的 `<Editor>` 组件在 `path` prop 变化时会切换内部 model,期间会触发一次 `onChange`(setValue 类型)。我们的 `handleEditorChange` 用 `useCallback` 闭包绑了 `activeFile`,但 React state 更新和 Monaco model 切换是两条**异步轨道**:onChange 触发那一刻,React 闭包里的 `activeFile` 可能已经是新值,但 monaco editor 当前的 model 还是旧的,或 value 是旧 model 的内容 —— `setDrafts` 把"上一个文件的内容"误写进了"新 activeFile 的 drafts" 槽位。下一次切回那个文件,显示的就是被串达的内容。

**修复**:[WebContainerRunner.tsx:415-455](frontend/src/components/runners/WebContainerRunner.tsx) 的 `handleEditorChange` 加守卫 —— 读 `editorRef.current.getModel()?.uri.path` 跟 `activeFile` 比对(允许前导 `/`),不一致就丢弃这次写入。同时加一个二重保险:`drafts[activeFile] === val` 时不触发 setState 链,避免 Monaco 偶发的"内容没变"flush 事件白白触发 dirty。

只读文件(README.md / package.json)的 onChange 因此也彻底无害化。

### 1.1 [Bug] 重开场景拿到老 attempt,Check 报 ATTEMPT_NOT_RUNNING / 倒计时不刷新

**根因**:[backend/internal/store/attempt_store.go:401](backend/internal/store/attempt_store.go) 的 `findActiveByOwnerKey` 把 **running + passed** 都视为可见 attempt。Start 路径 [backend/internal/biz/attempt/usecase.go:140-178](backend/internal/biz/attempt/usecase.go) 拿到 ok=true 的 passed attempt 后,默认 reusable=true 直接返回。

- **触发路径 A**:用户 PassModal 点 "重新开始" → 前端 fire-and-forget `terminate` + `fireStart`,Start 请求可能先到后端,reuse 老 passed attempt 返回 → 前端 store 拿到 status='passed' 的"新" current。
- **后果 1**(倒计时):`deadline` 由 `current.startedAt + estimatedMinutes` 算出,startedAt 还是老值 → 倒计时显示老剩余时间。
- **后果 2**(check):后端 Check 见到 `!a.IsActive()` → 抛 `ATTEMPT_NOT_RUNNING`,前端弹出"已提交过"。

**修复**(三层叠加,任一层独立工作):

1. **后端 Start 拒绝 reuse passed** —— [usecase.go](backend/internal/biz/attempt/usecase.go) 在 reusable 判定前先看 status,passed 直接 reusable=false;同时进入 stale 清理路径时**新增 `runner.Stop` 5s 超时**,避免老 sandbox 容器残留占端口。
2. **前端 restart 路径走 await + 清状态** —— [Scenario.tsx](frontend/src/pages/Scenario.tsx) `onPassModalAction` 的 'restart' 分支改成:`setCheckInfo(null) → resetStore() → await terminate.mutateAsync() → fireStart`。Race 没了,UI 立即进入"启动中"过渡态。
3. **giveUp 也清 checkInfo** —— 放弃后再次进入场景看到全新状态,不会带上次的判题残留。

修复后:重开 / 放弃 → 新 attemptId / 新 startedAt / 新 checkInfo,行为完全干净。

---

## 二、技术优化(2026-04-24)

按 PLAN.md § 六的具体条目从 High 往下推,每条都带 file:line。

### 2.1 后端

| 严重度 | 改动 | 文件 |
|---|---|---|
| High | 回滚路径的 `runner.Stop` / `repo.Update` 加 5s 硬超时,Docker daemon 异常时不再卡死请求线程 | [usecase.go](backend/internal/biz/attempt/usecase.go) |
| High | Check 路径 DB 写库 2s + Redis 500ms 独立短超时,远程组件抖动不阻塞用户拿判题结果 | [usecase.go](backend/internal/biz/attempt/usecase.go) |
| High | Docker runner 资源默认兜底:memory 512MB / CPU 1.0,场景作者忘填不会 OOM 宿主机 | [docker.go](backend/internal/runtime/docker.go) |
| High | `Stop` 失败时端口走 `MarkBad` 隔离而非回池,避免容器还在但端口被分给下一个 attempt 触发 bind 冲突 | [docker.go](backend/internal/runtime/docker.go) |
| High | `config.yaml.example` 真实远程主机名(Aliyun RDS / aihelper.chat)替换为占位符 + 头部加 SECURITY 约定 | [config.yaml.example](backend/configs/config.yaml.example) |

### 2.2 前端

| 严重度 | 改动 | 文件 |
|---|---|---|
| High | Monaco editor 卸载时清整个 model 池,切场景不再泄漏 ~50-100MB | [WebContainerRunner.tsx](frontend/src/components/runners/WebContainerRunner.tsx) |
| High | StaticRunner postMessage 严格校验:`e.origin === window.location.origin` + `typeof passed === 'boolean'`,防 XSS bundle 伪造 passed:true 绕判 | [StaticRunner.tsx](frontend/src/components/runners/StaticRunner.tsx) |
| High | `useAttemptStore` 自定义 `safeLocalStorage` 包裹所有 get/set,隐身模式 / 配额满不再白屏 | [useAttemptStore.ts](frontend/src/store/useAttemptStore.ts) |
| Medium | 新增 `RunnerErrorBoundary` 包裹三个 Runner,Runner 异常显示重试卡片不再白屏 | [RunnerErrorBoundary.tsx](frontend/src/components/RunnerErrorBoundary.tsx) + [Scenario.tsx](frontend/src/pages/Scenario.tsx) |
| Medium | `useCountdown` 监听 `visibilitychange`,后台标签页切回前台时重新对齐墙钟,不再漂移 | [useCountdown.ts](frontend/src/hooks/useCountdown.ts) |
| Medium | `runCheckRef` 从 render-phase 写改为 `useEffect` 写,符合 Concurrent React 约定 | [Scenario.tsx](frontend/src/pages/Scenario.tsx) |
| Medium | Terminal probe 指数退避(500→3000ms,最多 5 次,带 ±25% 抖动),容器冷启动 5-10s 不再硬给用户"失败"卡片 | [Terminal.tsx](frontend/src/components/Terminal.tsx) |
| Medium | Vite `manualChunks` 拆 monaco / webcontainer 独立 chunk,首屏不再被迫下载 ~3MB | [vite.config.ts](frontend/vite.config.ts) |

构建产物拆分稳定:main 256.75KB(gzip 83KB),monaco 22.20KB(独立),webcontainer 11.87KB(独立)。

---

## 三、MVP 内容补齐

### 3.1 新增 sandbox 场景三件套

`scenarios/` 目录从只有 `hello-world` 一个,补齐到 4 个:

| slug | 故障设计 | 解法 |
|---|---|---|
| [frontend-devserver-down](../scenarios/frontend-devserver-down/) | 删了 `node_modules/@vitejs/*` + python3 占坑 :3000 | `npm install` + `lsof / kill` + `npm run dev &` |
| [backend-api-500](../scenarios/backend-api-500/) | `/etc/app/config.yaml` 的 `db_password` 被改成错值 | 看 `journalctl -u app` → 改 config → `systemctl restart app` |
| [ops-nginx-upstream-fail](../scenarios/ops-nginx-upstream-fail/) | nginx `proxy_pass` 端口 9090 与 app 真实监听 8080 不一致 | 看 nginx error.log → 改配置 → `nginx -s reload` |

每个场景包含:`Dockerfile / entrypoint.sh / setup.sh / check.sh / README.md / meta.yaml / tests/{solution,regression}.sh`。后两个场景没有真 systemd,提供 `mini-systemctl` / `mini-journalctl` 薄壳让用户的肌肉记忆能用。

### 3.2 工程脚本

| 改动 | 文件 |
|---|---|
| `build-all-scenarios.{sh,ps1}` 改为扫 `scenarios/*/Dockerfile`,新增场景零脚本改动 | [scripts/build-all-scenarios.sh](../scripts/build-all-scenarios.sh) / [.ps1](../scripts/build-all-scenarios.ps1) |
| `curl-smoke.{sh,ps1}` 扩展为 4 种 execution mode 各跑一条:sandbox 发空 body,其余三种带 `clientResult` | [scripts/curl-smoke.sh](../scripts/curl-smoke.sh) / [.ps1](../scripts/curl-smoke.ps1) |

### 3.3 反馈闭环

| 改动 | 文件 |
|---|---|
| 后端 `/v1/feedback` 端点(srv.HandleFunc 直挂,不走 proto),写入服务日志含 client_id / scenarioSlug / attemptId / rating / text / UA / 时间 | [feedback.go](../backend/internal/service/opslabs/feedback.go) |
| 前端 `FeedbackPanel`(挂在 ScenarioMeta 底部),inline 展开,文字 + 可选 1-5 星;不引入 modal 库 | [FeedbackPanel.tsx](../frontend/src/components/FeedbackPanel.tsx) + [feedback.ts](../frontend/src/api/feedback.ts) |

---

## 四、文档与归档

| 改动 | 文件 |
|---|---|
| 新建对外介绍文档(供官网用) | [INTRODUCTION.md](../INTRODUCTION.md) |
| `WEEK1_BACKLOG.md` / `draft.md` 迁至 `docs/archive/`,头部加归档声明 | [docs/archive/](../docs/archive/) |
| `docs/archive/README.md` 索引页 | [docs/archive/README.md](archive/README.md) |
| PLAN.md § 三场景状态表更新为 7 行实状,§ 六.1-6.3 重写为 24 条带 file:line 的具体技术债,§ 十一 Sprint 0-3 标注完成状态 | [PLAN.md](../PLAN.md) |
| 本变更记录 | [docs/CHANGELOG-2026-04-25.md](CHANGELOG-2026-04-25.md) |

---

## 五、MVP 完成度自检(对照 PLAN.md § 五 V0.5 验收)

| V0.5 验收项 | 状态 | 备注 |
|---|---|---|
| 新人按 README 能启动前端和后端 | ⚠️ 部分 | 主流程可用;README.md 第 143 行后的 Week 1 草稿待迁出 |
| 首页能看到场景列表 | ✅ | `Home.tsx` 调 `/v1/scenarios`,registry 7 个全展示 |
| 至少 4 个场景能端到端通关 | ✅ | hello-world / css-flex-center / webcontainer-node-hello / wasm-linux-hello |
| 后端重启 / 前端刷新 / 重复进入场景不会造成明显错误 | ✅ | Redis AttemptStore + clientID 复用 + Ping 探活;本轮已修 passed reuse bug |
| 每种 execution mode 写最小冒烟检查 | ✅ | `scripts/curl-smoke.{sh,ps1}` 已扩 4 种模式 |
| 修正脚本说明,Windows / POSIX 跑法一致 | ✅ | `Makefile` / `make.ps1` / `scripts/*` 双份完整 |

**=> V0.5 主体已达成**,剩下 1 件事是:**README.md 主叙事改写**,这一项纯文档工作,不影响代码功能。

V0.8(内容生产版)进度:

| V0.8 验收项 | 状态 | 备注 |
|---|---|---|
| 落地 frontend-devserver-down / backend-api-500 / ops-nginx-upstream-fail | ✅ 代码就位 | 待 `make scenarios` 构建镜像后即可玩 |
| 每个 sandbox 场景必须有 Dockerfile / setup.sh / check.sh / README.md / tests/{solution,regression}.sh | ✅ | 4 个 sandbox 场景全部完整 |
| 场景构建脚本从硬编码改成扫描 `scenarios/*/Dockerfile` | ✅ | |
| 建立场景质量检查:初始状态必须失败,执行 solution 后必须通过 | ✅ | 每个场景的 `tests/regression.sh` 实现这个语义 |
| 5-8 个初始场景,覆盖 guide / frontend / backend / ops 四类 | ✅ 7 个 | guide(hello-world)/ frontend(devserver-down + css-flex-center + webcontainer-node-hello)/ backend(api-500)/ ops(nginx-upstream-fail + wasm-linux-hello) |

**=> V0.8 内容侧已完整**,剩下 V1.0 内测前要做:部署到公网 + 监控 + 反馈入口已备 → 可以开始组织 5 人小内测。

---

## 六、还没做的(交接清单)

按价值优先级:

1. **跑一次 `scripts/build-all-scenarios.{sh,ps1}` 实际构建 3 个新场景的镜像** —— 代码没有验证过 npm install / apt 装 nginx 在网络条件下的真实表现,首次构建可能要 5-10 分钟。
2. **跑 3 个新场景的 `tests/regression.sh`** 验证"初始 fail → solution → pass"闭环。
3. **README.md 主叙事改写**(PLAN.md § 十一 Sprint 1 第 5 条)—— 第 143 行后的 Week 1 草稿迁出,主 README 替换为反映 4 模式 + Redis 的真实快速上手。
4. **backend/README.md 改写**(Sprint 1 第 6 条)—— Kratos 模板换成 opslabs 后端说明。
5. **5 人小内测**(Sprint 3 第 12 条)—— 找熟人录屏 + 收反馈。
6. PLAN.md § 六的 Medium / Low 项还有约一半没动:Redis pipeline Lua 原子化、Snapshot 二级索引、单元测试 / vitest 配置。这些等量上来再说。

---

## 文件清单(本次提交涉及)

### 修改

```
PLAN.md
backend/configs/config.yaml.example
backend/internal/biz/attempt/usecase.go
backend/internal/runtime/docker.go
backend/internal/server/http.go
frontend/src/components/ScenarioMeta.tsx
frontend/src/components/Terminal.tsx
frontend/src/components/runners/StaticRunner.tsx
frontend/src/components/runners/WebContainerRunner.tsx
frontend/src/hooks/useCountdown.ts
frontend/src/pages/Scenario.tsx
frontend/src/store/useAttemptStore.ts
frontend/vite.config.ts
scripts/build-all-scenarios.{sh,ps1}
scripts/curl-smoke.{sh,ps1}
```

### 新增

```
INTRODUCTION.md
backend/internal/service/opslabs/feedback.go
docs/archive/README.md
docs/CHANGELOG-2026-04-25.md
frontend/src/api/feedback.ts
frontend/src/components/FeedbackPanel.tsx
frontend/src/components/RunnerErrorBoundary.tsx
scenarios/backend-api-500/                  (12 个文件)
scenarios/frontend-devserver-down/          (12 个文件)
scenarios/ops-nginx-upstream-fail/          (10 个文件)
```

### 重命名

```
WEEK1_BACKLOG.md → docs/archive/week1-backlog.md
draft.md         → docs/archive/draft-v1-design.md
```
