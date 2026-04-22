# opslabs 变更记录

> 本文档只记录**架构级别**和**用户可见行为**的变更。
> 纯 bug 修复(typo / 样式微调)不必入档,写在 commit message 里即可。

格式:逆时序,新的在上。每条带 `[YYYY-MM-DD] [类型]` 前缀。
类型:`fix` / `feat` / `refactor` / `security` / `docs` / `breaking`。

---

## 2026-04-22

### [fix] 前端"正在分配容器…"卡死 + 刷新重复建容器

**现象**:
- 用户进入场景页,后端 `POST /v1/scenarios/{slug}/start` 已返回 200 并带 `terminalUrl`,
  但 UI 永远停在"正在分配容器…"。
- 在同一场景页按 F5 刷新,会再创建一个新容器(旧的没被复用也没被清理)。

**根因**(经过 console 日志回放后确认的真实原因):

React 18 Strict Mode 在 mount 阶段会模拟"卸载+重挂"以验证副作用幂等性。
`useMutation` 的 MutationObserver 在这次模拟卸载时被销毁、重新 new 一个,
但 `start.mutate()` 已经把请求发出去了 —— mutation 本体在 React Query 的
`MutationCache` 里继续跑到 success,所以 hook 级 `onSuccess` / `onSettled`(挂在
mutation 上)会正常触发。**但 Scenario 组件 render 拿到的是重建后的新 observer,
它从没调用过 mutate,所以 `start.status` 永远停在 `pending`、`start.data`
永远是 `undefined`**。

控制台日志能清楚看到:`mutationFn enter → request resolved → mutationFn return
→ onSuccess → onSettled` 一路跑完,但 `[opslabs/Scenario] start status=` 只从
`idle → pending` 再也没切到 `success`。这就是 observer 断联了。

早先一版改用 `start.data` 作 source of truth 正好踩中这个坑 —— observer 的
`data` 永远是 undefined,effect 不可能被触发,跟 `isPending` 卡死同一回事。

进一步地,**call-site 的 `mutate(vars, { onSuccess })` 回调也不可靠** ——
日志实测:hook-level onSuccess 触发了、但 UI 里本该由 call-site onSuccess
设置的 `startPhase` 状态仍然停在 `pending`。call-site 回调虽然理论上挂在
Mutation 上,但 observer 重建时被一起清掉了。

**唯一跨 Strict Mode observer 重建稳定触发的写入点是 hook-level `onSuccess`**,
因为它在 `useMutation({ onSuccess })` 创建 Mutation 时就落到 MutationCache 里,
即使 observer 换了一个新的,Mutation 本身走完生命周期时也会正常调用它。

**最终修复**:
- `api/attempt.ts` 的 `useStartAttempt` 在 hook-level `onSuccess` 里直接
  调用 `useAttemptStore.getState().set(...)` 把结果落进 zustand。
  API 层看似跟 store 耦合了,但对"请求成功必须落一份到 store"这种强关联场景,
  比绕 ref / context / MutationCache 订阅都干净。
- `Scenario.tsx` 调用只留 `start.mutate(slug)` 不传任何 call-site 回调;
  组件只订阅 store 变化,看到 `current.attemptId` 出现就切 UI。
- 本地 `startPhase` 改成"监听 store 变化"驱动 —— `pending → done` 的切换
  由"store 里 scenarioSlug+attemptId 齐了"决定,不依赖任何 observer/回调。
- `useAttemptStore.ts` 的 persist 写全:指定 `storage=localStorage`、
  `partialize` 只存数据字段、加 `version` 防 schema 漂移。
- 刷新策略改成"先检查 store 里同 slug 的 attemptId,有就复用、走 `/v1/attempts/{id}`
  轮询,不再盲目 start"。
- 保留诊断日志(`[opslabs/start] ...` 在 attempt.ts、`[opslabs/Scenario] start status=`
  在 Scenario.tsx),下次卡住能瞬间分清是 mutation 挂了还是 observer 又出幺蛾子。

**文件**: `frontend/src/pages/Scenario.tsx`、`frontend/src/store/useAttemptStore.ts`、
`frontend/src/api/attempt.ts`、`frontend/src/api/http.ts`(加 30s fetch 超时)。

**踩坑备忘 / 给未来的自己**: React Query v5 + React 18 Strict Mode 下,
**永远不要把 UI 关键路径绑在 `useMutation` 返回对象的 `isPending` / `data` /
`error` / `status` 字段上**,也**不要依赖 `mutate(vars, { onSuccess })` 的 call-site
回调**。只有 `useMutation({ onSuccess })` 的 hook-level 回调、或 `mutateAsync`
的返回值,才是 Strict Mode 幂等的。如果需要写外部 store,就在 hook-level onSuccess
里直接写。

---

### [fix] hello-world setup.sh Permission denied + 判题卡死

**现象**:`bash: line 6: /home/player/welcome.txt: Permission denied`,
接着 check.sh 也报错。

**根因**:
- DockerRunner 默认 `--cap-drop=ALL` 把 `CAP_DAC_OVERRIDE` / `CAP_CHOWN` / `CAP_SETUID`
  都丢掉了。
- tmpfs 挂载参数 `/home/player:rw,uid=1000,gid=1000,mode=0755` → root(容器默认用户)
  既不是 owner 也没 CAP_DAC_OVERRIDE,写不进 player 家目录。
- 接着 su-exec 切 player 也挂,因为 CAP_SETUID 被 drop。

**修复**:
- `scenarios/hello-world/Dockerfile` 加 `USER player`,容器默认以 player 启动。
- `entrypoint.sh` 去掉所有 `su-exec`,直接以 player 身份跑 setup + ttyd。
- `setup.sh` 删掉 `chown` / `chmod 644`,umask 022 默认落点就是 644。
- check.sh 由 `docker exec -u root` 从宿主端注入,不受容器内 caps 影响,继续 700+root 权限防答案泄漏。

**文件**: `scenarios/hello-world/{Dockerfile,entrypoint.sh,setup.sh}`。

---

### [fix] 前端 Strict Mode 下双启动(19998+19999)

**现象**:打开一次场景页,后端建了两个容器。

**根因**:React 18 + Vite dev 默认开 Strict Mode,`useEffect` 被跑两次,
`resetStore()` 清了 store 导致原来的 `current?.scenarioSlug === slug` 守卫失效,
第二次调用时又走了 mutate。

**修复**:Scenario.tsx 加一个 `useRef<string | null>(null)` 锁住"正在为哪个 slug
发 start",Strict Mode 的第二次调用直接早退。

---

### [refactor] DockerRunner 安全默认 + 场景级 Security 配置

- DockerRunner `Run()` 默认追加:
  `--pids-limit=256 --cap-drop=ALL --security-opt=no-new-privileges:true --ulimit=nofile=1024:2048`
- Exec 强制 `-u root`(配合 check.sh 0700 owner=root 防答案泄漏)。
- 新增 `scenario.SecurityConfig`(映射到 `runtime.SecuritySpec`):
  - `CapAdd`:按需加回(例如 ops 场景 `NET_BIND_SERVICE`)
  - `ReadonlyRootFS`:opt-in `--read-only` + tmpfs(`/tmp`、`/home/player`、`/run`)
  - `TmpfsSizeMB`:tmpfs 尺寸,默认 64
- 新增 `scenario.RuntimeConfig.NetworkMode`:`none` / `isolated` / `internet-allowed`。

**文件**: `backend/internal/runtime/{types.go,docker.go}`、
`backend/internal/scenario/types.go`、`backend/internal/biz/attempt/usecase.go`。

---

### [refactor] Mock 运行时降级为测试专用

- 默认 driver 从 `mock` 改成 `docker`(`backend/internal/server/opslabs.go`)。
- MockRunner 不再占端口,`HostPort` 固定为 0。
- service 层 `terminalURL()` 在 HostPort=0 时返回空串,前端识别并渲染"Mock 预览模式"卡片,
  不再去 iframe 一个必然失败的端口。
- `config.yaml.example` 的 `runtime.driver` 改为 `docker` 并补注释。

---

### [refactor] Alpine 轻量基础镜像

- 新增 `scenarios-build/base-minimal/`(Alpine 3.19 + ttyd + tini + su-exec),
  压缩后约 20MB(旧 Ubuntu `base` 约 150MB)。
- `hello-world` 迁到 `opslabs/base-minimal:v1`。
- 旧 Ubuntu `scenarios-build/base/` 打 `DEPRECATED` 标记,仅供后续需要 `journalctl` /
  glibc-only 工具的场景兜底。
- `scripts/build-all-scenarios.{ps1,sh}` 改为"先自动扫描 `scenarios-build/` 建 base 层,
  再构建具体场景"的两段式。

---

### [fix] 后端 repo.Create 5 秒硬超时

**原因**:远程 PG 抖动时 `gorm.Create` 会吃满 HTTP 的 1000s 超时,
请求挂死、容器变僵尸。

**修复**:`usecase.Start` 包一层 `context.WithTimeout(ctx, 5*time.Second)`,
超时返回 504 `DB_WRITE_TIMEOUT`,同时 rollback 容器。

**文件**: `backend/internal/biz/attempt/usecase.go`。

---

## 预留设计:多执行模式(execution_mode)

> **状态**:设计稿(not implemented)。V1 只实现 `sandbox` 一种;本节仅作未来规划备忘。

### 动机

不是所有场景都要起容器:
- `hello-world` 只要"touch 一个文件"就能通关 → 放在浏览器内 JS 虚拟 FS 里就够了
- 前端类场景(npm / dev-server)→ 用 StackBlitz `WebContainers` 在浏览器里跑 Node
- 基础 Linux 命令练习 → 用 WebVM / CheerpX 在浏览器里跑 WASM Linux
- 真需要 kernel 行为的(systemd、Nginx、OOM)→ 才走后端 Docker sandbox

### 四种模式

| 模式 | 场景举例 | 运行环境 | 服务端成本 |
|---|---|---|---|
| `static` | hello-world、基础命令记忆题 | 纯前端 JS 虚拟 shell + 虚拟 FS | ~0 |
| `web-container` | 前端 dev-server 类 | StackBlitz WebContainers(浏览器) | ~0 |
| `wasm-linux` | 文件系统 / shell 脚本 / 文本处理 | CheerpX / v86(浏览器) | CDN 流量 |
| `sandbox` | systemd / Nginx / PostgreSQL / OOM | 后端 Docker 容器 | 云主机 |

### Schema 变动(预留)

```yaml
# backend/internal/scenarios/<slug>/meta.yaml
execution_mode: sandbox   # static | wasm-linux | web-container | sandbox
# wasm-linux 专用
wasm:
  rootfs_url: https://cdn.opslabs.dev/rootfs/linux-101-v1.ext2
# static 专用,用声明式规则替代 check.sh,避免双份判题脚本
grading:
  rules:
    - type: file_exists
      path: /tmp/ready.flag
    - type: process_running
      name: vite
    - type: http_probe
      url: http://localhost:3000
      expect_status: 200
```

### 推进节奏

- **V1(当前)**:只做 `sandbox`,四个场景全部走 Docker。保证端到端能通。
- **V2**:加 `static` 模式,`hello-world` 先迁过去做 POC;判题用声明式规则 + `file_exists` 一种。
- **V3**:加 `web-container` 模式,覆盖 `frontend-devserver-down`;声明式规则扩展到 `process_running` / `http_probe`。
- **V4**:加 `wasm-linux` 模式(CheerpX),把基础 Linux 题批量迁过去;基础镜像走 CDN 差量分发。

### V1 阶段要预留的东西

在**不增加实现复杂度**的前提下,以下位置先埋口子,免得 V2 改骨架:

1. `scenario.Scenario` 加字段 `ExecutionMode string`(默认值 `sandbox`),
   V1 不会根据它分流,但前端 `list` 会透出来。
2. 前端 `types.ts` 加 `executionMode?: 'static'|'wasm-linux'|'web-container'|'sandbox'`。
3. 前端 `Scenario.tsx` 目前直接渲染 `<Terminal>`,V2 会改成 switch/case:
   ```tsx
   switch (scenario.executionMode ?? 'sandbox') {
     case 'static':        return <StaticRunner scenario={scenario} />
     case 'wasm-linux':    return <WasmLinuxRunner scenario={scenario} />
     case 'web-container': return <WebContainerRunner scenario={scenario} />
     case 'sandbox':
     default:              return <SandboxRunner scenario={scenario} />
   }
   ```
4. 后端 `AttemptUsecase.Start`:V2 起对 `static` / `web-container` 只返回资源包 URL,
   不分配容器(不调用 runner.Run、不占端口)。
