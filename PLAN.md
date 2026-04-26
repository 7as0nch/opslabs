# opslabs 项目生命周期规划

> 本文是 opslabs 的长期产品与工程规划文档。它不是 README 的替代品:README 负责告诉别人如何快速启动项目,PLAN 负责回答“这个软件为什么做、现在做到哪一步、后面如何做成一个可运营的产品”。

---

## 一、项目背景与机会

opslabs 的核心定位是:面向中文开发者的浏览器真实故障排查训练平台。

用户不需要在本机安装复杂环境,打开浏览器就能进入一个可操作的 Linux/Node/CSS/wasm 场景,按照任务描述定位问题、修复问题,最后通过自动判题得到反馈。它对标的是 Killercoda、SadServers 这类“真实环境练习”产品,但切入点更偏中国开发者常见的学习、面试、工作排障场景。

这个方向成立的原因:

- 国内有大量 0-5 年开发者会写业务代码,但缺少真实排障练习环境。
- 面试、转岗、运维协作、线上事故复盘都需要“看日志、看端口、看进程、看配置、看资源”的手感。
- 传统教程多是文章和视频,用户看完仍然没有肌肉记忆。
- 真服务器练习成本高、风险高、配置麻烦;浏览器场景能把门槛降到最低。

一句话目标:

> 让用户在安全、可重置、可判题的环境里,练会真实生产问题的排查方法。

---

## 二、产品目的与目标用户

### 2.1 产品目的

opslabs 不是单纯的在线终端,也不是普通题库。它要做的是“排障训练闭环”:

1. 给用户一个真实问题背景。
2. 给用户一个可操作环境。
3. 用户自己探索、修改、验证。
4. 系统自动判题。
5. 通关后给出复盘、提示、推荐下一题。

长期目标是形成三个价值层:

- 学习价值:让新人补齐 Linux、服务、网络、配置、日志、数据库等排障能力。
- 面试价值:提供贴近工作场景的实战题,比纯八股更能检验能力。
- 企业价值:未来可以作为候选人实操面试、团队训练、事故演练工具。

### 2.2 初始目标用户

优先服务这几类人:

- 1-5 年经验的后端、运维、SRE、DevOps。
- 计算机专业高年级学生、转行到开发/运维的人。
- 前端/全栈工程师中想补 Linux、Node、本地环境排查能力的人。
- 准备面试,希望练真实“服务挂了怎么办”的人。

暂时不优先服务:

- 完全零基础用户。需要另做非常强的新手教程。
- 企业大客户。当前还没有权限、审计、团队管理、SLA 能力。
- 高危安全攻防用户。当前产品主线是排障训练,不是攻防靶场。

---

## 三、当前项目已经做到哪一步

对照最初的 `README.md`、[docs/archive/week1-backlog.md](docs/archive/week1-backlog.md)、[docs/archive/draft-v1-design.md](docs/archive/draft-v1-design.md) 和当前代码,项目已经从“Week 1 骨架验证”推进到“多执行模式 MVP 原型”。

### 3.1 已经完成的主干能力

后端已经具备:

- Go 1.25 + Kratos v2.8,HTTP `:6039` / gRPC `:9000`。
- 6 个核心 API:`ListScenarios` / `GetScenario` / `StartScenario` / `GetAttempt` / `CheckAttempt` / `TerminateAttempt`,另有 `POST /v1/attempts/{id}/heartbeat` 和 `GET /v1/scenarios/{slug}/bundle/{file*}`(embed.FS bundle 下发,带 Range / 304)。
- `AttemptUsecase` 管理场景生命周期,`backend/internal/biz/attempt/usecase.go` 注释明确写"V1 四模式都已接入":sandbox 走 `startSandbox`(Docker + ttyd + 端口池),static / wasm-linux / web-container 统一走 `startBundleless`(不分配容器,前端 Runner 拉 bundle 自己跑,判题时前端上报结果)。
- Docker runner 抽象 + mock runner + 真实 Docker 路径(端口池 10000-19999)。
- Redis-backed `AttemptStore`,TTL 60 分钟,按 clientID 做 owner 索引,GC 每 60s 扫描、30min idle 回收。
- 匿名用户 clientID 维度的 attempt 复用(带 Ping 探活)。
- 场景注册表硬编码在 `backend/internal/scenario/registry.go`,`meta.yaml` 扫描方案已预留。

前端已经具备:

- Vite 5.4 + React 18 + TS 5.5,依赖含 Monaco、`@webcontainer/api`、Zustand、React Query、react-router。
- 场景目录页(`src/pages/Home.tsx`)+ 场景详情页(`src/pages/Scenario.tsx`)。
- **4 种 Runner UI 全部落地**:
  - `Terminal.tsx` —— sandbox 模式 ttyd iframe,带 probe 重试。
  - `StaticRunner.tsx` —— static + wasm-linux 共用,postMessage 判题协议。
  - `WebContainerRunner.tsx`(~745 行)—— StackBlitz WebContainer + Monaco 编辑器 + 文件树 + 日志面板 + Ctrl+S + 编辑白名单。
- Zustand `useAttemptStore`(localStorage 持久化)+ React Query hooks(30s 轮询 + 20s 心跳)+ `useCountdown`(颜色分级,超时不强杀)+ `PassModal`(通关/失败/复盘/重开/放弃)。
- Vite 中间件注入 COOP / COEP(WebContainer 必需)。

### 3.2 场景真实落地情况

registry 里声明 7 个场景。以下是 `2026-04-25` 的实际状态:

| slug | 模式 | registry | 磁盘资源 | bundle | 镜像构建 | 可端到端通关 |
|---|---|---|---|---|---|---|
| hello-world | sandbox | ✅ | ✅ `scenarios/hello-world/` | - | ✅ | ✅ |
| frontend-devserver-down | sandbox | ✅ | ✅ `scenarios/frontend-devserver-down/`(Vite + React 脚手架 + tests) | - | ⚠️ 待 build | ⚠️ 镜像构建后即可 |
| backend-api-500 | sandbox | ✅ | ✅ `scenarios/backend-api-500/`(Flask + mini-systemctl/journalctl + tests) | - | ⚠️ 待 build | ⚠️ 镜像构建后即可 |
| ops-nginx-upstream-fail | sandbox | ✅ | ✅ `scenarios/ops-nginx-upstream-fail/`(Nginx + python http.server + tests) | - | ⚠️ 待 build | ⚠️ 镜像构建后即可 |
| css-flex-center | static | ✅ | - | ✅ `backend/internal/scenario/bundles/css-flex-center/`(embed) | n/a | ✅ |
| webcontainer-node-hello | web-container | ✅ | - | ✅ `bundles/webcontainer-node-hello/project.json` | n/a | ✅ |
| wasm-linux-hello | wasm-linux | ✅ | - | ✅ `bundles/wasm-linux-hello/`(v86 + BusyBox) | n/a | ✅ |

**=> 7 个场景目录全部就位**;4 个非 sandbox 已即开即用,3 个新 sandbox 场景需要先跑一次 `scripts/build-all-scenarios.{sh,ps1}` 把镜像构建出来,之后 7 个场景全部端到端可玩。这是 V0.5 → V0.8 的最大一块内容补齐。

工程脚本已经具备:

- Windows 与 POSIX 两套启动/构建/冒烟脚本(`Makefile` / `make.ps1` / `scripts/*.sh` / `scripts/*.ps1`)。
- 场景镜像构建脚本(目前硬编码只构建 hello-world,V0.8 要改成扫描 `scenarios/*/Dockerfile`)。
- API 冒烟脚本(`scripts/curl-smoke.{sh,ps1}`)。
- v86 资源拉取脚本(`scripts/fetch-v86.{sh,ps1}`)。

### 3.3 四模式分工速记

整个架构里最有辨识度的设计 —— 给未来自己和合作者一张对照表:

| 模式 | 适合的题材 | 技术代价 | 判题方式 |
|---|---|---|---|
| **sandbox** | Linux / 服务 / 网络 / 配置类排障 | 需要起 Docker 容器,资源最重 | 后端 `docker exec check.sh` |
| **static** | 纯前端视觉题(CSS / HTML / 纯 JS) | 无容器,最轻量 | 前端 iframe 内 `postMessage` 上报 |
| **web-container** | Node 工程类排障,需要改源码 | 浏览器跑 StackBlitz,需 COOP/COEP | Runner 跑 `check` 命令,前端上报结果 |
| **wasm-linux** | 轻量 Linux 引导题 / 离线演示 | v86 + BusyBox,浏览器吃 CPU | 前端 iframe 内 `postMessage` 上报 |

关键设计:**sandbox 走后端判题,其他三种前端上报**。后端 Check() 对两类输入都兼容(sandbox 看 exec 结果,bundleless 看 `ClientCheckResult`),场景作者在 registry 里选模式决定走哪条路。

### 3.4 当前真实阶段判断

当前项目不再是"第一周骨架没跑通"的阶段,而是:

> 技术路线已经验证,但还没有完成产品化、内容化、上线化、运营化。

下一阶段的重点不是继续堆架构,而是把它变成一个别人能稳定使用、能理解、愿意反馈、愿意复访的产品。内容侧的关键缺口是 3 个 sandbox 场景的 Dockerfile,工程侧的关键缺口在 § 六的具体技术债清单。

---

## 四、当前代码与文档差距

这个项目目前最大的问题不是“没有做”,而是“做得比旧文档快,文档还停在早期设想”。

### 4.1 已过时或不一致的地方

- `README.md` 主体仍是 Week 1 MVP 叙事(hello-world + sandbox + ttyd),没覆盖 4 模式、Redis store、WebContainer、wasm-linux;且第 143 行之后混入了一整段早期 Week 1 开发文档草稿,建议把这段迁到 `docs/archive/week1.md`,主 README 保留 1-138 行的快速上手 + 目录结构部分,再补足现状。
- [docs/archive/week1-backlog.md](docs/archive/week1-backlog.md) / [docs/archive/draft-v1-design.md](docs/archive/draft-v1-design.md) 是早期设计 / 待办文档,基本任务都已被重构或落地;已于 2026-04-24 迁入 `docs/archive/`,不代表当前计划。
- `backend/README.md` 仍是 Kratos template + `aichat-backend-deploy` 时期残留,需要从模板说明改成 opslabs 后端说明。
- `scripts/build-all-scenarios.{sh,ps1}` 默认只构建 `hello-world`,新增场景必须改脚本,人工维护成本高,要改成扫描 `scenarios/*/Dockerfile`。
- `backend/configs/config.yaml.example` 里带有真实远程主机名(Aliyun RDS / aihelper.chat),作为公开示例会暴露基础设施拓扑,需要换成占位符。
- 部分地方提到"7 个场景已落地",但实际只有 4 个能端到端通关(见 § 3.2 场景快照表),文档口径要统一。

### 4.2 当前最需要补齐的能力

优先级从高到低:

1. 文档与实现同步。
2. 场景内容真实落地。
3. 本地一键回归稳定。
4. 上线部署与监控方案。
5. 用户反馈与数据采集。
6. 登录、通关记录、个人主页。
7. 商业化与公司化运营。

---

## 五、版本路线图

这里按版本目标规划,不按周拆。每个版本都应该有明确验收标准。

### V0.5 本地可验证 MVP

目标:让你自己和技术朋友能稳定跑通核心体验。

重点任务:

- 更新 README,把当前真实能力讲清楚。
- 清理配置示例,提供本地开发版 `config.yaml.example`。
- 确保 `hello-world` Docker 场景完整可构建、可启动、可判题。
- 确保 `css-flex-center`、`webcontainer-node-hello`、`wasm-linux-hello` 三个非 sandbox 场景可完整通关。
- 修正脚本说明,让 Windows 和 POSIX 跑法一致。
- 为每种 execution mode 写最小冒烟检查。

验收标准:

- 新人按 README 能启动前端和后端。
- 首页能看到场景列表。
- 至少 4 个场景能端到端通关。
- 后端重启、前端刷新、重复进入场景不会造成明显错误。

### V0.8 内容生产版

目标:把 opslabs 从技术 demo 变成“有内容体系的训练产品”。

重点任务:

- 落地 `frontend-devserver-down`、`backend-api-500`、`ops-nginx-upstream-fail` 的真实场景目录。
- 每个 sandbox 场景必须有 `Dockerfile`、`setup.sh`、`check.sh`、`README.md`、`tests/solution.sh`、`tests/regression.sh`。
- 场景构建脚本从硬编码改成扫描 `scenarios/*/Dockerfile`。
- 建立场景质量检查:初始状态必须失败,执行 solution 后必须通过。
- 将 registry 与真实场景目录的字段对齐,避免“registry 有,文件没有”。
- 完成 5-8 个初始场景,覆盖 guide、frontend、backend、ops 四类。

验收标准:

- 所有上架场景都有真实可运行资源。
- 一条命令能构建全部可构建镜像。
- 一条命令能跑所有场景回归。
- 场景文案、提示、判题标准一致。

### V1.0 公开内测版

目标:能给 20-50 个真实用户试用,收集有效反馈。

重点任务:

- 部署到公网测试环境。
- 增加基础监控:服务存活、API 错误率、活跃 attempt 数、容器数、Redis/DB 状态。
- 增加反馈入口:通关后评分、文字反馈、问题上报。
- 增加最小用户身份:先支持匿名记录,后续接 GitHub/微信登录。
- 增加通关记录页:用户能看到自己完成过哪些题。
- 增加基础运营页面:关于、反馈方式、更新日志。
- 增加数据埋点:访问、开始场景、检查次数、通关、放弃、停留时间。

验收标准:

- 20 个内测用户可以自主进入、开始、做题、判题、反馈。
- 完成率、失败点、用户反馈可统计。
- 出现容器泄漏或服务错误时,你能在 30 分钟内发现。

### V1.5 学习闭环版

目标:让用户不仅能做题,还能知道自己哪里弱、下一步练什么。

重点任务:

- 登录系统正式接入。
- 个人主页:通关记录、最佳用时、提示使用次数、做题历史。
- 提示解锁接口和提示使用记录。
- 场景推荐:根据 prerequisites、recommended_next、skills 推荐下一题。
- 能力画像:按 skills 聚合用户已通关场景。
- 场景复盘页:显示正确思路、常见误区、相关命令。
- 管理端最小化:查看反馈、场景状态、用户通关数据。

验收标准:

- 用户做完一题后知道下一题做什么。
- 你能根据数据判断哪些场景太难、太简单、误判多。
- 内容迭代不再靠感觉,而是靠完成率、平均检查次数、反馈评分。

### V2.0 商业验证版

目标:验证 opslabs 是否值得长期投入和公司化经营。

重点任务:

- 做 3-5 个 Pro 场景,贴近面试、线上事故、企业实战。
- 增加付费意向测试:锁定场景、解锁按钮、价格页。
- 接入合规支付前,可以先做“点击解锁/预约购买/人工开通”的轻验证。
- 增加 AI 复盘助手:通关后解释用户做过的关键动作、推荐命令学习材料。
- 增加企业版原型:面试题包、候选人结果页、限时练习。

验收标准:

- Pro 场景解锁点击率超过 5%。
- 实际付费或强意向用户出现。
- 有用户主动要求更多场景或团队使用。

---

## 六、技术优化清单

本章不再写"要补单元测试""要加监控"这种纲领式语言 —— 那样的条目做不完、也不知从哪开始。下面每一条都带有文件路径 + 行号 + 严重度,可以直接排进 sprint。严重度分三档:

- **High** —— 在有真实用户之前就该修,影响 correctness / availability / security。
- **Medium** —— 内测前修,影响 UX 稳定性或小概率一致性。
- **Low** —— 进度允许就修,属于"量级上来之后必要,现阶段不紧急"。

条目格式:

```
- [严重度] 一句话问题 — file_path:line_range
  Why: 为什么是问题
  How: 怎么修
```

### 6.1 后端(按严重度排序)

- **[High] 回滚路径用 `context.Background()` 没有超时** — [backend/internal/biz/attempt/usecase.go:187,243,249,298](backend/internal/biz/attempt/usecase.go)
  Why: Docker daemon 卡住时,`runner.Stop` / `repo.Update` 可能无限等,把请求线程冻住。
  How: 统一改成 `context.WithTimeout(context.Background(), 5*time.Second)`。宁可偶尔泄一个容器,也不能让 API hang。

- **[High] Check 路径 Redis / DB 调用没有独立短超时** — [backend/internal/biz/attempt/usecase.go:491-498](backend/internal/biz/attempt/usecase.go)
  Why: Check 用的是请求级 ctx(可能几十秒),Redis 一抖整个 Check 就慢,并发下会级联。
  How: 关键路径用 500ms 短超时包一层,失败只 Warn,不阻塞用户看到判题结果,落库可以异步补偿。

- **[High] Docker runner 没有资源上限兜底** — [backend/internal/runtime/docker.go:140-172](backend/internal/runtime/docker.go)
  Why: 场景作者忘了写 `MemoryMB` 就默认无限,一个死循环能 OOM 整个宿主机。
  How: `if spec.MemoryMB <= 0 { spec.MemoryMB = 512 }`,CPU 默认 1.0。`pids-limit` 256 已硬编码保留。

- **[High] Stop 失败时端口被错误释放,可能导致端口冲突** — [backend/internal/runtime/docker.go:366-391](backend/internal/runtime/docker.go)
  Why: `docker rm -f` 因权限等非"不存在"错误失败时,容器还在,但端口已回池,下一个 `Acquire` 拿到会 bind 冲突。
  How: 只在 rm 成功或确认"not found"时释放端口,其他错误调用 `pool.MarkBad(port)` 隔离。

- **[High] 没有速率限制,容易被刷爆端口池** — [backend/internal/server/http.go:56-84](backend/internal/server/http.go)
  Why: `while true; curl /start` 一秒钟能耗尽 10000-19999 整个端口池。
  How: 接 `kratos/middleware/ratelimit`,按 X-Client-ID 做 5 req/s + 10 并发 attempt 上限。

- **[High] `config.yaml.example` 里有真实远程主机名** — [backend/configs/config.yaml.example:11,13,20](backend/configs/config.yaml.example)
  Why: `rm-*.mysql.rds.aliyuncs.com` / `sshjd.aihelper.chat` 暴露基础设施拓扑,属于侦察面。
  How: 改成 `example-rds.region.rds.aliyuncs.com` / `postgres.example.com` 之类占位符,再加 SECURITY 注释。

- **[Medium] TTL extend 和 GC terminate 之间存在数据一致性竞态** — [backend/internal/biz/attempt/usecase.go:586-595](backend/internal/biz/attempt/usecase.go)
  Why: `repo.Update` 和 `store.UpdateLastActive` 不在同一个原子块,GC 中途插入可能造成 DB terminated / store running 的短暂不一致。
  How: 用 Lua 脚本把 Redis 那边的两次写合成一个原子操作。

- **[Medium] `AttemptStore.Put` 的 pipeline 部分失败后没有补偿** — [backend/internal/store/attempt_store.go:140-159,207-222](backend/internal/store/attempt_store.go)
  Why: SET 主键成功 + SADD active 失败 → `Get(id)` 能找到但 `Snapshot` 找不到 → 僵尸 attempt。
  How: 整个 Put 包 Lua 脚本,all-or-nothing。

- **[Medium] `serveBundle` 没校验 slug 字符集** — [backend/internal/service/opslabs/bundle.go:52-76](backend/internal/service/opslabs/bundle.go)
  Why: `cleaned` 过了 `path.Clean`,但 `slug` 没校验;大小写不敏感文件系统上可能跨场景访问。
  How: `regexp.MustCompile("^[a-z0-9-]+$").MatchString(slug)`,不匹配直接 400。

- **[Medium] web-container 模式 `.tar/.tgz` 没注册 MIME** — [backend/internal/service/opslabs/bundle.go:107-138](backend/internal/service/opslabs/bundle.go)
  Why: StackBlitz 依赖正确 MIME 加载项目,浏览器 sniff 可能判成 `application/octet-stream`。
  How: 补 `.tar` / `.gz` / `.tgz` 三条 case。

- **[Medium / 前置于登录] `TerminateAttempt` 没做 owner 校验** — [backend/internal/service/opslabs/attempt.go:216-229](backend/internal/service/opslabs/attempt.go)
  Why: V1 无登录,但任意客户端可以 terminate 别人的 attempt,恶意脚本能刷爆别人会话。
  How: 先加 TODO 注释,等登录接入后补 `if ctx clientID != a.ClientID && != anon { Forbidden }`。

- **[Medium] check.sh 的 stderr 可能进日志,有泄密风险** — [backend/internal/biz/attempt/usecase.go:512-516](backend/internal/biz/attempt/usecase.go)
  Why: 用户在容器里 echo 了密码,stderr 被日志原文记录。
  How: sanitize + truncate 200 字符,只在错误路径记录。

- **[Low] Redis 没有按 client / slug 的二级索引,`Snapshot` 是全库扫描** — [backend/internal/store/attempt_store.go](backend/internal/store/attempt_store.go)
  Why: 10k 活跃 attempt 时 GC 每 60s 要 MGet 10k 条,浪费带宽。
  How: 维护 `opslabs:attempt:owner:client:<c>` 有序集,GC 分桶扫描。**10 用户量级之前不用做**。

- **[Low] 核心路径无单元测试** — `backend/internal/biz/attempt/`、`backend/internal/runtime/portpool.go` 等
  Why: race 类 bug 最容易在这几个模块发生,但当前零覆盖。
  How: 先补 reuse + ping 失败路径、TTL extend 竞态、PortPool MarkBad 并发三条,CI 加 `go test -race ./...`。

### 6.2 前端(按严重度排序)

- **[High] `WebContainerRunner` 的 Monaco editor 从未 dispose** — [frontend/src/components/runners/WebContainerRunner.tsx:688-722](frontend/src/components/runners/WebContainerRunner.tsx)
  Why: 每次切场景泄漏 ~50-100 MB,切 3-4 次肉眼可见卡顿。
  How: `editorRef = useRef(null)`,`onChange` 存 ref,unmount 时 `editorRef.current?.dispose()`。

- **[High] `StaticRunner` 的 `postMessage` 没校验 origin + payload 结构** — [frontend/src/components/runners/StaticRunner.tsx:85-108](frontend/src/components/runners/StaticRunner.tsx)
  Why: 当前代码注释写"宽松校验只认 type",bundle 被 XSS 时攻击者能 post 伪造的 `passed:true` 绕过判题。
  How: 校验 `e.origin === window.location.origin || e.origin === bundleOrigin`,且 `typeof data.passed === 'boolean'`。

- **[High] `useAttemptStore` 的 localStorage 读写没有 try/catch** — [frontend/src/store/useAttemptStore.ts:24-41](frontend/src/store/useAttemptStore.ts)
  Why: localStorage 满或隐身模式下直接抛,用户打开就白屏。
  How: 自定义 storage 工厂把 getItem / setItem / removeItem 都 try-catch。

- **[Medium] 没有 ErrorBoundary 包 Runner** — [frontend/src/pages/Scenario.tsx:560-585](frontend/src/pages/Scenario.tsx)
  Why: WebContainer 运行时抛异常,整个详情页白屏,用户只能刷新。
  How: 包一层 `<ErrorBoundary fallback={<RetryPanel/>}>`,fallback 带"重启 Runner"按钮。

- **[Medium] `useCountdown` 后台标签页切回来时会漂移** — [frontend/src/hooks/useCountdown.ts:39-51](frontend/src/hooks/useCountdown.ts)
  Why: `setInterval(1000)` 在 hidden tab 被浏览器节流,回到前台时 `remaining` 跟真实时间差几秒。
  How: 监听 `visibilitychange`,可见时从 deadline 重新计算。

- **[Medium] `Scenario.tsx` 的 `runCheck` 有 stale closure 风险** — [frontend/src/pages/Scenario.tsx:280-365](frontend/src/pages/Scenario.tsx)
  Why: `useImperativeHandle` 依赖数组可能不完整,超时触发 `onExpire` 时可能拿到旧 attemptId。
  How: 把 `runCheck` 存 ref,每次 render 写,`onExpire` 调用 `runCheckRef.current()`。

- **[Medium] `Terminal` 的 probe 重试没有指数退避** — [frontend/src/components/Terminal.tsx:32-63](frontend/src/components/Terminal.tsx)
  Why: 容器启动慢 5-10s 时,用户只能看到"正在连接…"手动重试,体验差。
  How: 自动退避重试 `min(500 * 2^n + jitter, 5000)`,5 次失败后才显示"失败"。

- **[Medium] 没有代码分割,Monaco + WebContainer 全打一个 bundle** — [frontend/vite.config.ts](frontend/vite.config.ts)
  Why: 两个加起来压缩后 ~3 MB,首次进详情页才用得到,但首页也被迫下载。
  How: `rollupOptions.output.manualChunks` 拆 `monaco` / `webcontainer` 两个 chunk,或走 `React.lazy` 动态 import。

- **[Low] `Home.tsx` 没有筛选 / 搜索** — [frontend/src/pages/Home.tsx](frontend/src/pages/Home.tsx)
  Why: 场景 >20 个后找起来吃力,PLAN V1.5 里本来就要补。
  How: 加 category / difficulty 下拉 + 文字搜索,卡片加 `loading="lazy"`。

- **[Low] 没有任何前端测试配置** — 全局
  Why: 手测还跑得动,但 Runner 逻辑复杂后回归难。
  How: 引入 vitest + `@testing-library/react`,先测 `useCountdown` / `useAttemptStore` 这种纯函数 hook。

### 6.3 场景系统

原有的 6 条规范继续保留,并新增 1 条脚本优化:

- **[High] `scripts/build-all-scenarios.{sh,ps1}` 当前硬编码只构建 hello-world** — [scripts/build-all-scenarios.sh](scripts/build-all-scenarios.sh)
  Why: 新增场景的 Dockerfile 不会被自动构建,人工维护脚本列表成本高,违背了"场景加一个就加一个"的目标。
  How: 扫描 `scenarios/*/Dockerfile`,每个产出 `opslabs/<slug>:v1` 的镜像。
- 每个场景必须有自动化验收。
- `check.sh` 必须幂等,失败时输出可诊断信息。
- `README.md` 必须说明背景、任务、验收标准、约束。
- `hints` 必须分三档:方向提示、关键线索、近似答案。
- 场景尽量允许多解法,判题只看最终状态。
- 场景时长控制在 3-15 分钟,不要一开始就做 30 分钟大题。

### 6.4 运维

在拿到内测流量前价值有限,保留为"内测前后要做"的提醒:

- 增加部署文档:服务器规格、Docker、Nginx、TLS、数据库、Redis、日志目录。
- 增加备份策略:数据库每日备份,配置单独备份。
- 增加监控:HTTP 健康检查、容器数量、端口池占用、Redis/DB 延迟。
- 增加故障手册:容器泄漏、端口耗尽、Redis 连接失败、Docker daemon 异常时怎么处理。

---

## 七、内容与场景生产流程

以后每个场景都按同一生命周期生产。

### 7.1 场景立项

先写清楚:

- 这个故障真实工作中会不会发生。
- 目标用户是谁。
- 用户做完能学会什么。
- 预计 5 分钟、10 分钟还是 15 分钟完成。
- 有哪些可能误解或误判。

### 7.2 场景设计

固定包含:

- 背景故事。
- 初始故障状态。
- 用户目标。
- 禁止操作或约束。
- 标准解法。
- 可接受的其他解法。
- 判题标准。
- 三档提示。

### 7.3 场景实现

固定文件:

- `meta.yaml`
- `README.md`
- `Dockerfile` 或 bundle 配置。
- `setup.sh`
- `check.sh`
- `tests/regression.sh`
- `tests/solution.sh`

### 7.4 场景验收

每次合并前必须验证:

- 初始状态不能直接通过。
- solution 执行后必须通过。
- check 多次执行结果一致。
- 用户用合理替代方案也能通过。
- 场景文案不泄露答案。
- 最强提示能让卡死用户过关。

---

## 八、上线、运营与数据指标

### 8.1 内测前必须准备

- 一个稳定域名。
- HTTPS。
- 基础监控。
- 反馈入口。
- 简短用户说明:这是排障练习平台,容器会自动回收,不要输入真实密码。
- 隐私说明:收集哪些数据、用于什么。

### 8.2 内测数据指标

最重要的不是访问量,而是行为质量:

- 场景开始率:进入详情页的人有多少点击开始。
- 通关率:开始场景的人有多少通过。
- 平均检查次数:太高说明题目不清楚或太难。
- 平均用时:是否接近预计时间。
- 放弃率:在哪些场景最高。
- 反馈评分:1-5 星平均分。
- 复访率:第二天或一周后是否回来。

### 8.3 冷启动建议

先不要大范围投放。建议顺序:

1. 找 5 个熟悉的技术朋友试玩。
2. 修掉明显 bug 和题目误导。
3. 找 20 个内测用户。
4. 根据录屏和反馈调整场景。
5. 再去掘金、V2EX、即刻、B 站、小红书等渠道公开发。

公开发布时不要只说“我做了个平台”,要说清楚:

- 你解决了什么痛点。
- 用户 3 分钟能玩到什么。
- 目前有哪些场景。
- 你希望大家反馈什么。

---

## 九、一个人的公司如何经营

这里的“公司”不是一开始就租办公室、招人、融资,而是你作为一个人把产品持续经营起来。

### 9.1 早期不要急着公司化

在没有真实用户之前,最重要的是验证需求:

- 用户是否愿意打开。
- 用户是否能做完。
- 用户是否觉得有用。
- 用户是否愿意分享。
- 用户是否愿意为更好的场景付费。

这阶段不要过早投入:

- 复杂 Logo。
- 大规模管理后台。
- 完整支付系统。
- 企业销售材料。
- 过度精美的首页。

### 9.2 一个人公司的核心循环

每周只盯一个循环:

1. 做一个可用改进。
2. 发布给用户。
3. 收集反馈和数据。
4. 修掉最影响体验的问题。
5. 再发布。

你要同时扮演四个角色:

- 产品经理:决定做什么、不做什么。
- 工程师:把东西稳定做出来。
- 内容作者:持续生产高质量场景。
- 运营者:找到用户、回应反馈、建立信任。

### 9.3 收入模式可以逐步验证

不要一开始就做重订阅。推荐路径:

1. 免费场景吸引用户。
2. Pro 场景测试付费意愿。
3. 面试题包或学习路径小额付费。
4. AI 复盘或个人能力报告增值。
5. 企业面试/团队训练版本。

适合 opslabs 的可能商业模式:

- 单个 Pro 场景包。
- 月度会员。
- 面试实战训练营。
- 企业候选人实操测评。
- 团队故障演练环境。

### 9.4 经营纪律

- 每次上线都写 changelog。
- 每周至少和 3 个用户交流。
- 每个场景都看完成率和反馈。
- 不让用户数据丢失。
- 不在未验证需求前做大功能。
- 每月复盘一次成本、收入、用户增长。

---

## 十、中国内地 Web 运营合规清单

> 说明:以下是产品经营学习清单,不是法律意见。正式上线、收费、广告投放或企业服务前,建议咨询云服务商备案专员、当地通信管理局或专业律师。

### 10.1 域名、服务器与备案

如果网站使用中国内地服务器并对公众开放,通常需要完成 ICP 备案。

根据工信部《互联网信息服务管理办法》:

- 经营性互联网信息服务实行许可制度。
- 非经营性互联网信息服务实行备案制度。
- 未取得许可或者未履行备案手续的,不得从事互联网信息服务。

官方参考:

- 工信部《互联网信息服务管理办法》: https://www.miit.gov.cn/jgsj/zfs/xzfg/art/2020/art_0bf50f171cbf4e5aaca30553d8762d64.html
- 工信部非经营性互联网信息服务备案办事指南: https://ynca.miit.gov.cn/bsfw/bszn/art/2020/art_1e2c69e668b74b59b2572d2a8138b454.html

实践建议:

- 早期技术验证可以先本地或海外服务器测试。
- 如果面向中国内地公网长期运营,建议尽早准备备案。
- 备案主体可以是个人或企业,但如果涉及收费、商业经营、企业服务,建议用公司主体。
- 云服务商通常会提供备案入口,按它们的流程提交资料。

### 10.2 ICP 备案与 ICP 许可证

大致理解:

- ICP 备案:通常对应非经营性网站,用于说明这个网站由谁主办。
- ICP 许可证:通常对应经营性互联网信息服务,比如通过互联网向用户有偿提供信息或网页制作等服务。

opslabs 的风险点:

- 如果只是免费学习平台,通常先考虑备案。
- 如果开始收费、会员、Pro 场景、企业服务,需要进一步确认是否涉及经营性互联网信息服务许可。
- 如果接入支付、开票、企业合同,公司主体会更合适。

### 10.3 公司注册

如果你要长期经营 opslabs,建议最终注册有限责任公司,原因:

- 方便备案、签合同、收款、开票。
- 个人财务与业务财务分离。
- 企业客户更容易合作。
- 后续接支付、招聘、商标、软件著作权更顺。

注意注册资本不要虚高。根据 2024 年后的公司法配套规则,有限责任公司股东认缴出资期限原则上自公司成立之日起 5 年内缴足。

官方参考:

- 中国政府网《国务院关于实施〈中华人民共和国公司法〉注册资本登记管理制度的规定》: https://www.gov.cn/zhengce/content/202407/content_6960376.htm
- 中国政府网《公司登记管理实施办法》: https://www.gov.cn/gongbao/2025/issue_11826/202501/content_7001287.html

建议:

- 注册资本按实际能力写,不要为了“看起来大”写很高。
- 经营范围可覆盖软件开发、技术服务、互联网信息服务等,具体以当地登记机关建议为准。
- 收费前准备好财务账户、发票、合同模板、用户协议、隐私政策。

### 10.4 数据与隐私

opslabs 会天然收集一些用户行为数据:

- 用户 ID 或匿名 clientID。
- 场景开始、检查、通关、放弃。
- 用时、提示使用次数、反馈内容。
- 可能的日志、IP、User-Agent。

上线前需要:

- 写隐私政策。
- 写用户协议。
- 明确不让用户在容器里输入真实密码、密钥、公司机密。
- 限制日志采集范围,不要记录用户终端完整输入,除非明确告知并获得同意。
- 数据库备份加密或至少做好访问控制。

### 10.5 商标、著作权与开源

建议:

- 确认产品名是否可注册商标。
- 核查 v86、WebContainer、Monaco、Kratos、Docker SDK 等依赖的许可证义务。
- 自己写的场景、文案、图片保留版权说明。
- 如果未来开放外部贡献场景,需要贡献者协议或至少 PR 声明。

---

## 十一、近期最优先行动清单

按"有用户之前必须做的"从前往后排,每一条都对应一个具体文件/脚本/动作。前 3 条从 § 六 挑 High 级技术债,后面是内容 / 文档 / 反馈闭环。

**Sprint 0 · 零架构改动,本地小改(半天就能做完):**

1. ✅ 已完成(2026-04-24):**后端回滚路径加超时兜底** —— [backend/internal/biz/attempt/usecase.go](backend/internal/biz/attempt/usecase.go) 的 3 处回滚分支已改成 `context.WithTimeout(..., 5s)`,同时 Check 路径 DB 2s + Redis 500ms 独立短超时。
2. ✅ 已完成(2026-04-24):**Docker runner 资源上限兜底** —— [backend/internal/runtime/docker.go](backend/internal/runtime/docker.go) 默认 memory 512MB + CPU 1.0;同时 `Stop` 失败时端口走 `MarkBad` 隔离而非回池。
3. ✅ 已完成(2026-04-24):**Monaco editor 加 dispose** —— [WebContainerRunner.tsx](frontend/src/components/runners/WebContainerRunner.tsx) `onMount` 拿 monaco 引用,卸载时清整个 model 池。

**Sprint 1 · 文档和配置对齐真实状态(半天):**

4. ✅ 已完成(2026-04-24):清理 [backend/configs/config.yaml.example](backend/configs/config.yaml.example),真实远程主机名已替换为占位符,头部加 SECURITY 约定。
5. 把 [README.md](README.md) 第 143 行之后的 Week 1 草稿迁到 `docs/archive/week1-dev.md`,主 README 改写为反映 4 模式 + Redis 的真实快速上手。**待办**。
6. 把 [backend/README.md](backend/README.md) 从 Kratos 模板改成 opslabs 后端说明。**待办**。
7. ✅ 已完成(2026-04-24):`WEEK1_BACKLOG.md` / `draft.md` 已迁至 [docs/archive/](docs/archive/),头部加归档声明。

**Sprint 2 · 内容补齐,让能玩的场景从 4 个 → 5 个(1-2 天):**

8. ✅ 已完成(2026-04-24):补齐 [scenarios/frontend-devserver-down/](scenarios/frontend-devserver-down/) 全套文件(Dockerfile / setup.sh / check.sh / entrypoint.sh / webapp 脚手架 / tests / README / meta.yaml)。镜像构建通过后该场景即可端到端通关。
9. ✅ 已完成(2026-04-24):改造 [scripts/build-all-scenarios.{sh,ps1}](scripts/) 扫描 `scenarios/*/Dockerfile`,不再硬列场景名。
10. ✅ 已完成(2026-04-24):[scripts/curl-smoke.{sh,ps1}](scripts/) 扩展为 4 种模式各一条冒烟,sandbox 发空 body、其他三种带 `clientResult`。

**Sprint 3 · 反馈与内测(视进度):**

11. ✅ 已完成(2026-04-24):详情页最小反馈入口 —— 后端 [feedback.go](backend/internal/service/opslabs/feedback.go) HandleFunc `/v1/feedback` 写日志,前端 [FeedbackPanel.tsx](frontend/src/components/FeedbackPanel.tsx) 挂在 ScenarioMeta 底部。
12. 组织一次 5 人小内测,录屏 + 收反馈。**待办**。
13. 根据反馈决定 V1.0 前还要补哪些场景。**待办**。

**前端 Medium 级项(A 类优化,已在 2026-04-24 落地):**

- ✅ `RunnerErrorBoundary` 包 Runner,Runner 崩溃不再白屏(带"重新加载"按钮)
- ✅ `useCountdown` 监听 `visibilitychange` 防后台标签页漂移
- ✅ `runCheckRef` 从 render-phase 写改成 `useEffect` 写,符合 Concurrent React 约定
- ✅ `Terminal.tsx` probe 指数退避重试(500→3000ms,最多 5 次)
- ✅ `vite.config.ts` `manualChunks` 拆出 monaco / webcontainer 独立 chunk

第 4-13 项和 § 五 V0.5/V0.8/V1.0 版本路线图一一对应,不重复列。

---

## 十二、你需要学习的知识路线

为了把 opslabs 从项目做成产品,你需要同时补四条线。

### 12.1 工程能力

- Go Kratos 服务治理。
- Docker SDK、容器资源限制、网络隔离。
- Redis 缓存和过期策略。
- 前端状态管理、React Query、错误边界。
- 自动化测试与 CI。
- 日志、监控、告警、备份。

### 12.2 内容能力

- 如何把真实事故抽象成 10 分钟训练题。
- 如何写清晰的背景、目标、约束。
- 如何设计不泄露答案但能判题的 check。
- 如何控制难度曲线。
- 如何从用户反馈中改题。

### 12.3 产品能力

- 用户画像。
- MVP 验证。
- 数据指标。
- 留存与转化。
- 用户访谈。
- 反馈分类。
- 定价实验。

### 12.4 公司经营能力

- ICP 备案与许可证基础。
- 公司注册、财税、合同、发票。
- 用户协议与隐私政策。
- 支付与退款。
- 商标、版权、开源许可证。
- 企业客户沟通。

---

## 十三、最终判断

opslabs 现在最有价值的不是“再加一个复杂架构”,而是把已经跑通的技术能力压实成可复用的内容生产系统。

短期最关键的胜负手:

- 场景质量。
- 启动稳定性。
- 判题可靠性。
- 用户反馈速度。
- 文档可信度。

只要这五件事做好,opslabs 就不只是一个代码项目,而会变成一个可以持续运营、持续增长、甚至未来商业化的训练产品。
