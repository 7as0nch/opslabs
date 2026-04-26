<!--
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Week 1 骨架跑通的可落地 backlog，按"先打通骨架（mock runtime）→ 再接 Docker → 再上前端"的三段式推进。
-->

> **⚠️ 归档声明**:本文档归档于 `docs/archive/`,属于 **历史参考**。
> 项目当前进度与规划请看仓库根的 [PLAN.md](../../PLAN.md)。
> 本文里的大部分 Phase A / B / C 任务已落地或已被重新定义。

# opslabs · Week 1 Backlog(可落地版)

> 基于 `README.md`、`draft.md`、`backend/internal/scenarios/{README,draft,template}.md` 整合。
>
> 本周决策：
> - **场景注册表硬编码**（Go 代码里写，不读 yaml、不落 DB）
> - **先不管容器**：runtime 层先做 mock，跑通后端 API + DB 持久化链路
> - **docker/ttyd/前端** 留到 Phase B/C
>
> 历史文档里出现的 `opslabs` / `opslabs/*` 都按新项目名 **opslabs** 处理（镜像 `opslabs/base:v1`、网络 `opslabs-scenarios`、命名空间 `opslabs`）。待后续回头批量改。

---

## 0. 阶段划分

```
Phase A  骨架（mock runtime）         D1-D3   先拿到一条能跑通的假链路
Phase B  真实 Docker 接入              D4-D5   替换 mock，跑通 hello-world
Phase C  前端 + 并发 + GC              D6-D7   端到端体验 + 收尾
```

Phase A 结束时：`curl` 可以完整走 start → check → terminate，DB 有 attempts 记录，但 terminalUrl 是假的、"容器"是个 stub。

Phase B 结束时：浏览器能连 ttyd 真终端，hello-world 场景可通关。

Phase C 结束时：React 页面走一遍 + 3 并发 + GC 自动回收。

---

## Phase A · 骨架（mock runtime）

### A1. proto 契约定义

**路径**：`backend/api/opslabs/v1/opslabs.proto`

**内容要点**（按 `README.md` §6 + `draft.md` §5 接口定义）：
- `rpc ListScenarios(ListScenariosRequest) returns (ListScenariosReply)` — 场景列表（支持 category/difficulty/persona 过滤参数，Week 1 可先不生效）
- `rpc GetScenario(GetScenarioRequest) returns (ScenarioReply)` — 场景详情（含 description_md、hints 外壳）
- `rpc StartScenario(StartScenarioRequest) returns (StartScenarioReply)`
- `rpc GetAttempt(GetAttemptRequest) returns (AttemptReply)` — 兼心跳作用
- `rpc CheckAttempt(CheckAttemptRequest) returns (CheckAttemptReply)`
- `rpc TerminateAttempt(TerminateAttemptRequest) returns (TerminateAttemptReply)`

**proto 包名约定**：`package api.opslabs.v1;`，`go_package = "github.com/7as0nch/backend/api/opslabs/v1;v1";`，`http path` 按 `/v1/scenarios`、`/v1/scenarios/{slug}`、`/v1/scenarios/{slug}/start`、`/v1/attempts/{id}`、`/v1/attempts/{id}/check`、`/v1/attempts/{id}/terminate`。

**错误码**（新建 `backend/api/opslabs/v1/error.proto`）：
```
SCENARIO_NOT_FOUND = 0
ATTEMPT_NOT_FOUND
ATTEMPT_NOT_RUNNING
CONTAINER_START_FAIL
CHECK_EXEC_FAIL
PORT_POOL_EXHAUSTED
```

**验收**：
- `scripts/gen-api.ps1` 能生成 `*.pb.go` / `*_http.pb.go` / `*_grpc.pb.go` / `error_errors.pb.go`
- 注意：kratos errors 要写成 `enum ErrorReason` 加 `(errors.default_code) = xxx`

**依赖**：无，是整条链路的起点。

---

### A2. GORM 模型 + AutoMigrate

**路径**：`backend/internal/data/attempt.go`（新文件）

**模型定义**（严格按 `README.md` §5.1，但字段名对齐 opslabs 命名约定）：

```go
type AttemptStatus string

const (
    StatusRunning    AttemptStatus = "running"
    StatusPassed     AttemptStatus = "passed"
    StatusExpired    AttemptStatus = "expired"
    StatusTerminated AttemptStatus = "terminated"
    StatusFailed     AttemptStatus = "failed"
)

type Attempt struct {
    ID           string         `gorm:"primaryKey;type:varchar(32)"`
    ScenarioSlug string         `gorm:"type:varchar(64);index;not null"`
    ContainerID  string         `gorm:"type:varchar(128)"`
    HostPort     int            `gorm:"not null;default:0"`
    Status       AttemptStatus  `gorm:"type:varchar(16);index;not null"`
    StartedAt    time.Time      `gorm:"not null"`
    LastActiveAt time.Time      `gorm:"index;not null"`
    FinishedAt   *time.Time
    DurationMS   *int64
    CheckCount   int            `gorm:"default:0"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
    DeletedAt    gorm.DeletedAt `gorm:"index"`
}
```

**说明**：
- ID 用 snowflake 或 hex string，Week 1 不引 UserID（登录后再加）
- 注意 `db/postgres.go` 里加上 AutoMigrate 调用
- CheckCount 字段是 Week 2 用的，先预留

**验收**：启动后端，PostgreSQL 里能看到 `attempts` 表，索引完整。

**依赖**：A1（如果 repo 接口用 proto 生成的 Reply 作参数就依赖，建议不要耦合，repo 只看 domain model）。

---

### A3. 内存热缓存 + 场景注册表

**路径**：
- `backend/internal/store/attempt_store.go`（新文件）
- `backend/internal/scenario/registry.go`（新文件）

**`store/attempt_store.go`**：

```go
type CachedAttempt struct {
    ID           string
    ScenarioSlug string
    ContainerID  string
    HostPort     int
    Status       AttemptStatus
    StartedAt    time.Time
    LastActiveAt time.Time
    FinishedAt   *time.Time
}

type AttemptStore struct {
    mu   sync.RWMutex
    data map[string]*CachedAttempt
}

// Get / Put / Delete / Snapshot / UpdateLastActive
```

**`scenario/registry.go`**（硬编码 4 个场景）：

```go
type Scenario struct {
    Slug             string
    Title            string
    Summary          string
    DescriptionMd    string          // 可以从 constants 或 embed.FS 读
    Category         string
    Difficulty       uint8
    EstimatedMinutes uint16
    TargetPersonas   []string
    TechStack        []string
    Skills           []string
    Commands         []string
    Tags             []string
    Runtime          RuntimeConfig
    Grading          GradingConfig
    Hints            []Hint
}

var registry = map[string]*Scenario{
    "hello-world": {
        Slug: "hello-world",
        Title: "欢迎来到 opslabs",
        // ... 从 backend/internal/scenarios/README.md 的 meta.yaml 翻译
    },
    "frontend-devserver-down": { ... },
    "backend-api-500":         { ... },
    "ops-nginx-upstream-fail": { ... },
}

func Get(slug string) (*Scenario, bool) { ... }
func List() []*Scenario { ... }
```

**验收**：单元测试 `registry_test.go` 断言 4 个场景都能 Get 到。

**依赖**：无。

---

### A4. mock runtime

**路径**：`backend/internal/runtime/mock.go`（新文件）

**接口设计**（保持和未来真实 Docker 实现同一个 interface，方便替换）：

```go
type Runner interface {
    // Run 启动容器，返回 containerID + 宿主机端口
    Run(ctx context.Context, spec RunSpec) (containerID string, hostPort int, err error)
    // Stop 停止并删除容器
    Stop(ctx context.Context, containerID string) error
    // Exec 在容器内执行脚本，返回 stdout/stderr/exitCode
    Exec(ctx context.Context, containerID, script string, timeout time.Duration) (ExecResult, error)
}

type RunSpec struct {
    Image       string
    MemoryMB    int64
    CPUs        float64
    NetworkMode string
    Env         map[string]string
}

type ExecResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
}
```

**MockRunner**：
- `Run`：直接返回假 containerID（uuid）、从一个内置 portpool 里分配个 10000-19999 的端口
- `Stop`：释放端口
- `Exec`：内部用一个 `map[slug]func()ExecResult` 模拟通关/失败逻辑（默认直接返回 OK，方便测试）

**验收**：单元测试能 Run → Exec 返回 OK → Stop。

**依赖**：无。

---

### A5. biz 层 AttemptUsecase

**路径**：`backend/internal/biz/attempt/usecase.go`（新建子包）

**接口**：

```go
type AttemptRepo interface {
    Create(ctx context.Context, a *data.Attempt) error
    Update(ctx context.Context, a *data.Attempt) error
    FindByID(ctx context.Context, id string) (*data.Attempt, error)
}

type AttemptUsecase struct {
    repo    AttemptRepo
    runner  runtime.Runner
    store   *store.AttemptStore
    reg     scenario.Registry
    log     *log.Helper
}

// Start：
//   1. 查 scenario 存在 → 404
//   2. 生成 attemptID
//   3. runner.Run(spec) 拿到 containerID + port
//   4. 写 DB + 写 store
//   5. 返回 terminalUrl (mock 模式下返回 "http://localhost:<port>")
//
// Check：
//   1. store.Get(id)，状态必须 running
//   2. runner.Exec(containerID, scenario.Grading.CheckScript, timeout)
//   3. 判 stdout 首行 == "OK" && exitCode == 0
//   4. 通关：status=passed, 写 DB, 写 duration
//   5. 返回 CheckResult
//
// Terminate：
//   1. runner.Stop
//   2. DB + store 更新 status=terminated, finished_at=now
//
// Get: 单纯读 store，顺便 UpdateLastActive（心跳）
```

**验收**：`usecase_test.go` 用 mock runner + in-memory repo 断言 Start/Check/Terminate 三条路径。

**依赖**：A1 / A2 / A3 / A4。

---

### A6. service handler + 注册

**路径**：
- `backend/internal/service/attempt.go`（新建）
- `backend/internal/service/service.go`（把 AttemptService 注册到 ProviderSet）
- `backend/internal/server/http.go`（注册路由）
- `backend/internal/server/grpc.go`（注册 gRPC）
- `backend/cmd/backend/wire.go` + `wire.ps1`

**service 层只做 pb ↔ biz 转换**，不要塞业务逻辑。
- `ScenarioService`：ListScenarios / GetScenario → 纯查 registry
- `AttemptService`：StartScenario / GetAttempt / CheckAttempt / TerminateAttempt → 走 biz.AttemptUsecase

**验收**：
```
curl http://localhost:6039/v1/scenarios                        # 列出 4 个场景
curl http://localhost:6039/v1/scenarios/hello-world            # 拿到详情
curl -X POST http://localhost:6039/v1/scenarios/hello-world/start
  → 返回 attemptId + terminalUrl（mock）
curl http://localhost:6039/v1/attempts/:id
curl -X POST http://localhost:6039/v1/attempts/:id/check
  → 返回 passed:true
curl -X POST http://localhost:6039/v1/attempts/:id/terminate
  → status:terminated
```

**依赖**：A1 / A5。

---

### A7. 错误返回对齐 Kratos errors

**路径**：各 handler 处 `return nil, v1.ErrorScenarioNotFound("slug=%s", slug)` 这类写法。

**验收**：`curl` 不存在的 slug → 返回 HTTP 404 + JSON body 里 reason 对。

**依赖**：A1 / A6。

---

### A8. Phase A 回归脚本

**路径**：`scripts/curl-smoke.sh`

```bash
#!/bin/bash
set -e
BASE=http://localhost:6039

id=$(curl -s -X POST $BASE/v1/scenarios/hello-world/start | jq -r .attemptId)
curl -s $BASE/v1/attempts/$id | jq .
curl -s -X POST $BASE/v1/attempts/$id/check | jq .
curl -s -X POST $BASE/v1/attempts/$id/terminate | jq .
```

**验收**：`psql` 进 DB 查 `attempts` 表，有一条 status=terminated 的记录，duration_ms 有值。

---

## Phase B · 真实 Docker 接入

> 前置条件：Docker Desktop 装好，基础镜像和 hello-world 镜像 build 成功。

### B1. 基础镜像 + hello-world 镜像

**路径**：
- `scenarios-build/base/Dockerfile`（按 `scenarios/README.md` §共用基础镜像 照抄，FROM 改 ubuntu:22.04）
- `scenarios/hello-world/{Dockerfile,entrypoint.sh,setup.sh,check.sh,README.md,meta.yaml}`
- `scripts/build-all-scenarios.sh`
- `scripts/test-scenario.sh`

**注意**：这些文件目前**只存在于 markdown 里**，要真的落地到磁盘。meta.yaml 里 image 字段改成 `opslabs/hello-world:v1`。

**验收**：
```
./scripts/build-all-scenarios.sh
./scripts/test-scenario.sh hello-world   # 输出 "all pass: hello-world"
```

**依赖**：无，可以和 Phase A 并行做。

---

### B2. DockerRunner 真实实现

**路径**：`backend/internal/runtime/docker.go`

**实现 Runner interface**，关键点：
- 用 `github.com/docker/docker/client` SDK（需要 `go get`）
- `Run`：带 --memory、--cpus、--network、--cap-drop、--pids-limit 参数
- `Exec`：通过 `ContainerExecCreate` + `ContainerExecAttach`，用 root 身份
- `Stop`：`ContainerRemove(force=true)`

**Windows 开发者注意**：Docker Desktop 的 socket 是 `npipe:////./pipe/docker_engine`，go-sdk 默认能识别。

**依赖**：A4 接口定义。

---

### B3. PortPool

**路径**：`backend/internal/runtime/portpool.go`

```go
type PortPool struct {
    mu   sync.Mutex
    free []int           // 初始化 10000-19999
    used map[int]bool
}

func New(start, end int) *PortPool
func (p *PortPool) Acquire() (int, error)   // 没了返回 PORT_POOL_EXHAUSTED
func (p *PortPool) Release(port int)
```

**验收**：并发 100 次 Acquire 不重复。

---

### B4. 配置 + wire 接入

**路径**：
- `backend/internal/conf/conf.proto` 加 `message Runtime { string mode = 1; string network = 2; int32 port_start = 3; int32 port_end = 4; }`
- `config.yaml` 里：
  ```yaml
  runtime:
    mode: docker       # docker / mock
    network: opslabs-scenarios
    port_start: 10000
    port_end: 19999
  ```
- `wire` provider 根据 mode 返回 MockRunner 或 DockerRunner

**验收**：config 切 mode=docker，重启后真实起容器。

---

### B5. GC Server

**路径**：`backend/internal/task/gc.go`

```go
type GCServer struct {
    store  *store.AttemptStore
    runner runtime.Runner
    repo   AttemptRepo
    log    *log.Helper
    done   chan struct{}
}

func (g *GCServer) Start(ctx context.Context) error   // kratos transport.Server
func (g *GCServer) Stop(ctx context.Context) error

// 内部 goroutine：每 60s 扫一遍：
//   - status=running && LastActiveAt 距今 > idleTimeout → Stop + mark expired
//   - status=passed && FinishedAt 距今 > passedGrace → Stop（store 清掉）
```

**注册**：`cmd/backend/wire.go` 里 `kratos.Server(gs, hs, gcServer)`。

**依赖**：B2。

---

### B6. 进程重启时从 DB 恢复 store

**路径**：`backend/internal/store/bootstrap.go`

启动时 `repo.ListRunning()` 拿到所有 running 的 attempt，塞进 store。这样重启不丢。

注意 containerID 在 Docker 侧可能已经被杀，启动时可以顺便探活一次，失败的直接 mark expired。

---

## Phase C · 前端 + 收尾

### C1. 前端脚手架

- `npm create vite@latest frontend -- --template react-ts`
- 装 Tailwind、React Query、Zustand、React Router
- Vite 代理 `/api` → `http://localhost:6039`

### C2. 首页 + 场景详情页

按 `README.md` §7 的组件拆解落地：
- `pages/Home.tsx`、`pages/Scenario.tsx`
- `components/{Terminal,ScenarioMeta,ActionBar,PassModal}.tsx`
- `api/attempt.ts`（React Query hooks）
- `store/useAttemptStore.ts`

### C3. 并发 3 个验证

浏览器开 3 个标签，分别 start 不同 attempt，端口各不冲突。

### C4. 收尾

- Makefile（`make up` 起全家桶、`make scenarios` 全量 build）
- README 的 "10 分钟搭环境指南"
- 录一个 3 分钟完整流程视频

---

## 1. 任务依赖全景图

```
A1 proto ──┐
           ├──> A6 service ──> A7 errors ──> A8 smoke
A2 GORM ───┤                      ▲
A3 store+reg ──┐                  │
A4 mock ───────┼──> A5 biz ───────┘
               │
               ▼
        [Phase A 完成]
               │
B1 镜像 ───┐   │
           │   ▼
B2 docker ─┼──> B4 wire ──> B5 GC ──> B6 恢复
B3 port ───┘                  │
                              ▼
                       [Phase B 完成]
                              │
                   C1 ──> C2 ──> C3 ──> C4
                              │
                              ▼
                       [Week 1 交付]
```

并行策略：
- **B1（镜像）可以和整个 Phase A 并行**，放给空闲时间做
- Phase A 里 `A1/A2/A3/A4` 两两独立，可以同时开坑

---

## 2. 按日程表推荐排期

| Day | 上午 | 下午 |
|---|---|---|
| D1 | A1 proto + `gen-api.ps1` | A2 GORM + migrate 通 |
| D2 | A3 registry 硬编码 4 个场景 | A4 mock runtime |
| D3 | A5 biz + A6 service | A7 errors + A8 smoke |
| D4 | B1 基础镜像 + hello-world 镜像 | B3 PortPool + B2 DockerRunner（Run/Stop） |
| D5 | B2 Exec | B4 wire 切 docker mode，smoke 脚本全过 |
| D6 | B5 GC + B6 store 恢复 | C1 + C2 前端先跑通 start |
| D7 | C2 终端 iframe + check | C3 并发 + C4 收尾 |

如果某个 block 超时（比如 Docker SDK 卡住），**先用 mock 继续往后**，最后一天再回头接。

---

## 3. 不在本周做的事（backlog 留存）

- 用户系统（OAuth、JWT）—— Week 3
- hints 解锁接口 + 记账 —— Week 2
- 多 variant（scenarios/variants/）—— Week 2
- scenarios sync 到 DB —— V2
- adminweb 管理端 —— 暂时不用
- AI 陪练 —— V2
- 支付、Pro 场景 —— Week 8

---

## 4. 文档需要同步修正的地方

后续有空批量 sed 一下：
- `README.md` / `draft.md` / `backend/internal/scenarios/*.md` 里的 `opslabs` → `opslabs`
- 镜像前缀 `opslabs/` → `opslabs/`
- 网络名 `opslabs-scenarios` → `opslabs-scenarios`
- 命名空间 `/opt/opslabs/` 保留不变（这是容器内路径，改动成本大收益小），或一并改为 `/opt/opslabs/`，两种都行，二选一定下来就行

---

## 5. 关键决策点 checklist（需要你确认）

- [ ] attempt ID 用 uuid（32 hex chars）还是 snowflake int64 转 hex？建议 **uuid（无状态、无时钟依赖）**
- [ ] terminalUrl 的协议：Phase A mock 阶段返回 `http://localhost:<port>` 就行；Phase B 真实环境要不要上 Nginx 反代 + 子路径（`https://tty.opslabs.cn/s/<token>`）？Week 1 建议**先用裸端口**，上线前再接反代
- [ ] `/opt/opslabs` → `/opt/opslabs` 容器内路径要不要改？建议**改**，一次到位
- [ ] backend 现有的 SysUser/Menu/Dict 这套管理后台代码 **是否保留**？如果 opslabs 不需要管理后台，建议新开 `internal/biz/{attempt,scenario}` 独立命名空间，不要混进 base/

---

**下一步建议**：跑一遍这份 backlog，挑出你想先动手的 task（A1 最推荐作为起点），告诉我选了哪个，我给你直接贴可运行的代码/proto/命令。
