<!--
 * @Author: chengjiang
 * @Date: 2026-04-20 22:53:33
 * @Description: 
-->
抽一个**统一的场景范式**出来,不然场景越加越乱,前端也没法做筛选和展示。

我从两个角度定义:**场景元信息 Schema**(描述场景是什么)+ **场景运行契约**(场景怎么跑、怎么判)。前者是前后端共享的数据结构,后者是场景开发者必须遵守的文件规范。

---

## 1. 场景元信息 Schema(Scenario Meta)

这是一份**前后端共享的核心数据结构**,既是后端数据库字段,也是前端展示字段,还是场景作者写 YAML 时的模板。

### 1.1 完整字段定义

```yaml
# scenarios/<slug>/meta.yaml —— 每个场景必须有这份文件
slug: backend-api-500
version: "1.0.0"

# ===== 基础展示信息 =====
title: "API 总是返回 500"
summary: "一个用户接口一直 500,找出原因并修好它"         # 场景列表卡片上展示
description_file: "README.md"                              # 详细背景故事,Markdown 文件

# ===== 分类维度(多维筛选核心) =====
category: backend                                           # frontend / backend / ops / devops / security / database
difficulty: 3                                              # 1-5 星
estimated_minutes: 12                                      # 预计用时

# ===== 用户画像 =====
target_personas:                                           # 适合哪类用户(多选)
  - backend-engineer
  - full-stack
experience_level: junior                                   # junior / mid / senior

# ===== 知识维度(可筛选、可统计、可推荐) =====
tech_stack:                                                # 涉及的技术栈
  - python
  - flask
  - postgresql
  - systemd

skills:                                                    # 考察的能力点(用户能力画像的依据)
  - log-analysis
  - config-troubleshooting
  - database-connectivity
  - service-management

commands:                                                  # 会用到的关键命令(做搜索和命令字典)
  - journalctl
  - systemctl
  - psql
  - curl

tags:                                                      # 自由标签,给搜索和推荐
  - interview-common
  - real-world
  - 500-error

# ===== 运行配置 =====
runtime:
  image: opslabs/backend-api-500:v1                          # Docker 镜像名
  memory_mb: 512
  cpus: 0.5
  idle_timeout_minutes: 30
  passed_grace_minutes: 10
  network_mode: isolated                                   # isolated / internet-allowed
  variants: ["db-password", "db-schema", "db-connections"] # 多变体,用户每次进入随机抽一个

# ===== 判题配置 =====
grading:
  check_script: /opt/opslabs/check.sh                        # 容器内绝对路径
  check_timeout_seconds: 10
  success_output: "OK"                                     # stdout 首行匹配即算通过

# ===== 提示(V2 启用) =====
hints:
  - level: 1
    content: "API 返回 500 但不知道为啥?去看服务的错误日志"
  - level: 2
    content: "日志显示数据库认证失败,检查配置文件里的数据库密码"
  - level: 3
    content: "cat /etc/app/config.yaml 看密码,和 psql 实际连接试一下"

# ===== 学习资源(V2 启用,复盘时展示) =====
learning_resources:
  - title: "Linux 服务日志基础"
    url: "https://..."
  - title: "PostgreSQL 连接故障排查"
    url: "https://..."

# ===== 前置/后置关系(V2 学习路径) =====
prerequisites: []                                          # 依赖先做哪些场景
recommended_next:                                          # 通关后推荐做哪些
  - ops-nginx-upstream-fail

# ===== 作者信息 =====
author: "official"
created_at: "2026-04-20"
updated_at: "2026-04-20"

# ===== 内部状态 =====
is_published: true                                         # 是否上架
is_premium: false                                          # V2 付费场景
```

### 1.2 可选字段说明

上面看着字段多,但大部分都是可选的。**MVP 场景只需要填必填字段**:

**必填(7 个)**:`slug`、`title`、`summary`、`category`、`difficulty`、`estimated_minutes`、`runtime.image`、`grading.check_script`

**强烈建议填**:`target_personas`、`tech_stack`、`skills`、`hints`

**V2 再填**:`learning_resources`、`prerequisites`、`recommended_next`、`is_premium`

---

## 2. 枚举值规范

避免场景作者写 `backend-dev` / `backend_eng` / `Backend` 这种混乱,**所有枚举值集中定义**,前后端共用。

### 2.1 Category(类别)

```go
const (
    CategoryFrontend  = "frontend"
    CategoryBackend   = "backend"
    CategoryOps       = "ops"
    CategoryDevOps    = "devops"
    CategoryDatabase  = "database"
    CategoryNetwork   = "network"
    CategorySecurity  = "security"
    CategoryGuide     = "guide"        // 引导场景
)
```

### 2.2 Target Personas(目标用户画像)

```go
const (
    PersonaFrontendEngineer = "frontend-engineer"
    PersonaBackendEngineer  = "backend-engineer"
    PersonaFullStack        = "full-stack"
    PersonaDevOps           = "devops-engineer"
    PersonaSRE              = "sre"
    PersonaOps              = "ops-engineer"
    PersonaDBA              = "dba"
    PersonaSecurityEngineer = "security-engineer"
    PersonaStudent          = "student"
    PersonaInterviewPrep    = "interview-prep"
)
```

### 2.3 Experience Level

```go
const (
    LevelIntern  = "intern"    // 实习生
    LevelJunior  = "junior"    // 1-3 年
    LevelMid     = "mid"       // 3-5 年
    LevelSenior  = "senior"    // 5+ 年
    LevelExpert  = "expert"    // 架构/专家
)
```

### 2.4 Skills(能力点分类学)

这个最关键,**决定用户能力画像的粒度**。初版建议按"动作"分类,不按"知识":

```
log-analysis              日志分析
config-troubleshooting    配置排查
service-management        服务管理
process-analysis          进程分析
resource-diagnosis        资源诊断(CPU/内存/磁盘/IO)
network-troubleshooting   网络排查
database-connectivity     数据库连接
database-performance      数据库性能
security-audit            安全审计
permission-management     权限管理
backup-recovery           备份恢复
deployment                部署相关
container-operations      容器操作
scripting                 脚本编写
monitoring                监控告警
```

Skills 的作用:用户通关多个场景后,可以生成一份"能力雷达图"(V2 功能)——"你擅长日志分析,但网络排查经验不足"。

### 2.5 Tech Stack 统一词表

维护一份**受控词表**(controlled vocabulary),避免大小写、中英文、版本号混乱:

```
操作系统: linux, ubuntu, centos, debian, windows, macos
语言:     python, nodejs, java, go, rust, php, ruby
Web:      nginx, apache, caddy, haproxy, traefik, envoy
数据库:   mysql, postgresql, redis, mongodb, elasticsearch, clickhouse
中间件:   rabbitmq, kafka, nats, consul, etcd
容器:     docker, kubernetes, containerd, podman
前端:     react, vue, vite, webpack, typescript
框架:     flask, django, express, spring, gin, kratos
工具:     systemd, cron, ssh, git, iptables, curl
监控:     prometheus, grafana, loki, jaeger
```

新场景用新技术时,PR 里需要申请新增词表项。这个约束短期有点麻烦,长期保证筛选和搜索体验。

---

## 3. 数据结构(前后端)

### 3.1 后端 Go 结构体

```go
// internal/scenario/types.go

type Scenario struct {
    // 基础
    Slug        string
    Version     string
    Title       string
    Summary     string
    Description string           // 从 description_file 读出的 markdown

    // 分类
    Category         Category
    Difficulty       uint8        // 1-5
    EstimatedMinutes uint16

    // 用户画像
    TargetPersonas  []Persona
    ExperienceLevel Level

    // 知识维度
    TechStack []string
    Skills    []string
    Commands  []string
    Tags      []string

    // 运行配置
    Runtime RuntimeConfig

    // 判题配置
    Grading GradingConfig

    // 可选
    Hints              []Hint
    LearningResources  []Resource
    Prerequisites      []string
    RecommendedNext    []string

    // 元信息
    Author      string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    IsPublished bool
    IsPremium   bool
}

type RuntimeConfig struct {
    Image             string
    MemoryMB          int64
    CPUs              float64
    IdleTimeoutMin    int
    PassedGraceMin    int
    NetworkMode       string        // isolated / internet-allowed
    Variants          []string
}

type GradingConfig struct {
    CheckScript          string
    CheckTimeoutSeconds  int
    SuccessOutput        string     // 默认 "OK"
}

type Hint struct {
    Level   uint8
    Content string
}

type Resource struct {
    Title string
    URL   string
}
```

### 3.2 前端 TypeScript 类型

**和后端 1:1 对齐**,命名转 camelCase:

```typescript
export type Category =
  | 'frontend' | 'backend' | 'ops' | 'devops'
  | 'database' | 'network' | 'security' | 'guide'

export type Persona =
  | 'frontend-engineer' | 'backend-engineer' | 'full-stack'
  | 'devops-engineer' | 'sre' | 'ops-engineer'
  | 'dba' | 'security-engineer' | 'student' | 'interview-prep'

export type Level = 'intern' | 'junior' | 'mid' | 'senior' | 'expert'

export interface Scenario {
  slug: string
  version: string
  title: string
  summary: string
  description: string

  category: Category
  difficulty: 1 | 2 | 3 | 4 | 5
  estimatedMinutes: number

  targetPersonas: Persona[]
  experienceLevel: Level

  techStack: string[]
  skills: string[]
  commands: string[]
  tags: string[]

  hints: Hint[]
  learningResources: Resource[]
  prerequisites: string[]
  recommendedNext: string[]

  isPublished: boolean
  isPremium: boolean
}

export interface Hint {
  level: 1 | 2 | 3
  content: string
}

export interface Resource {
  title: string
  url: string
}

// 列表页只需要精简版
export interface ScenarioBrief {
  slug: string
  title: string
  summary: string
  category: Category
  difficulty: number
  estimatedMinutes: number
  targetPersonas: Persona[]
  techStack: string[]
  tags: string[]
  isPremium: boolean
  // 用户相关(登录后有值)
  userPassed?: boolean
  userBestDurationMs?: number
}
```

---

## 4. 场景文件规范(运行契约)

每个场景是 `scenarios/<slug>/` 目录下的一组文件,**结构严格统一**:

```
scenarios/
  backend-api-500/
    meta.yaml           必需,场景元信息
    README.md           必需,用户看的详细背景(markdown 支持代码块、表格、图片)
    Dockerfile          必需,镜像构建
    entrypoint.sh       必需,容器启动入口
    setup.sh            必需,预埋故障的脚本
    check.sh            必需,判题脚本
    cleanup.sh          可选,销毁容器前做清理(V2)
    assets/             可选,场景依赖的文件
      config.yaml
      app.py
      ...
    variants/           可选,多变体支持
      db-password/
        setup.sh        覆盖默认 setup
        check.sh        该变体独立的判题
      db-schema/
        ...
    tests/              可选但强推荐
      solution.sh       一条通关的操作序列,CI 用来验证场景可通关
      regression.sh     校验场景一开始确实是失败状态
```

### 4.1 Dockerfile 标准

```dockerfile
FROM opslabs/base:v1

# 场景特定的依赖(如果有)
RUN apt-get update && apt-get install -y \
    python3 python3-pip postgresql \
    && rm -rf /var/lib/apt/lists/*

# 拷贝资源
COPY entrypoint.sh /entrypoint.sh
COPY setup.sh      /opt/opslabs/setup.sh
COPY check.sh      /opt/opslabs/check.sh
COPY assets/       /opt/opslabs/assets/

RUN chmod 755 /entrypoint.sh /opt/opslabs/setup.sh \
    && chmod 600 /opt/opslabs/check.sh \
    && chown root:root /opt/opslabs/*

CMD ["/entrypoint.sh"]
```

### 4.2 entrypoint.sh 标准

```bash
#!/bin/bash
set -e

# 1. 根据环境变量决定 variant(多变体场景)
VARIANT=${SCENARIO_VARIANT:-default}

# 2. 跑 setup(以 root 身份,有权限埋故障)
if [ -f "/opt/opslabs/variants/${VARIANT}/setup.sh" ]; then
    /opt/opslabs/variants/${VARIANT}/setup.sh
else
    /opt/opslabs/setup.sh
fi

# 3. 切换到普通用户启动 ttyd
exec su - player -c "ttyd -W -p 7681 --writable bash"
```

### 4.3 check.sh 约定

**判题脚本契约**(所有场景必须遵守):

- 执行身份:root(后端通过 docker exec 以 root 调用)
- 超时:`grading.check_timeout_seconds`(默认 10 秒)
- 输出约定:
  - **stdout 首行 == "OK"** 且 **exit code == 0** → 通关
  - 其他情况 → 未通关
- 必须**幂等**(多次调用结果一致,不改变状态)
- 允许多解法,只看最终状态不看过程
- 失败时可以在 stderr 打印调试信息,但不影响判题

模板:

```bash
#!/bin/bash
# 场景: <slug>
# 检查目标: <描述>

set -o pipefail

# 检查点 1: ...
if [ 条件不满足 ]; then
    echo "NO" ; echo "失败原因: xxx" >&2
    exit 0
fi

# 检查点 2: ...
if [ 条件不满足 ]; then
    echo "NO"
    exit 0
fi

echo "OK"
exit 0
```

### 4.4 tests/solution.sh(强烈建议)

每个场景**自带一个 solution.sh**,用 CI 确保场景能通关、判题正确。这是质量门槛:

```bash
#!/bin/bash
# 一条完整的通关操作序列
# CI 跑:启动容器 → 执行本脚本 → 跑 check.sh → 必须输出 OK

sed -i 's/correct_passw0rd/correct_password/' /etc/app/config.yaml
systemctl restart app
sleep 2
```

每次场景变更,CI 自动跑:
1. 启动容器 → 立即 check.sh 应该返回 NO(故障确实埋上了)
2. 启动容器 → 跑 solution.sh → check.sh 应该返回 OK(场景可通关)

这能防住 99% 的"场景上线后玩家发现没法通关"翻车。

---

## 5. API 层如何使用这个 Schema

### 5.1 场景列表(筛选)

```
GET /v1/scenarios?category=backend&difficulty=3&persona=backend-engineer&tech=python
```

前端可以组合多个维度过滤,后端按 meta.yaml 的字段索引查询。

### 5.2 场景详情

```
GET /v1/scenarios/:slug
→ 返回完整 Scenario(剥掉 hints.content 除非用户解锁过)
```

### 5.3 用户能力画像(V2)

```
GET /v1/me/skills
→ 基于用户通关的场景 × 场景的 skills 字段,聚合出能力分布
```

### 5.4 推荐下一题(V2)

三种推荐算法可以基于这个 Schema:

- 硬编码:`scenario.recommendedNext` 优先
- 同 category + 差一难度:循序渐进
- 基于能力画像:找用户薄弱的 skill,推荐带该 skill 的新场景

---

## 6. 前面三个场景统一化后长什么样

快速对照验证一下,前面设计的前端 / 后端 / 运维三个场景,都能完美套进这个 schema:

| 字段 | frontend-devserver-down | backend-api-500 | ops-nginx-upstream-fail |
|---|---|---|---|
| category | frontend | backend | ops |
| difficulty | 2 | 3 | 3 |
| target_personas | [frontend-engineer, full-stack] | [backend-engineer, full-stack] | [ops-engineer, sre, devops-engineer] |
| experience_level | junior | junior | mid |
| tech_stack | [nodejs, vite, react] | [python, flask, postgresql, systemd] | [nginx, linux] |
| skills | [dependency-management, port-conflict, config-troubleshooting] | [log-analysis, config-troubleshooting, database-connectivity, service-management] | [log-analysis, service-management, network-troubleshooting, config-troubleshooting] |
| commands | [node, npm, lsof, nvm] | [journalctl, systemctl, psql, curl] | [nginx, ss, tail, iptables] |
| tags | [interview-common, onboarding] | [interview-common, real-world, 500-error] | [interview-common, real-world, 502-error] |

你可以看到:

- 三个场景**共享 skills 中的 `config-troubleshooting`**,未来用户做完这三个,系统就知道他"擅长配置排查"
- 三个场景**共享 tags 中的 `interview-common`**,可以组装成"面试常见故障合集"
- 通过 `target_personas` 精准匹配用户,前端工程师进来默认看到的就是 frontend 场景

---

## 7. 场景作者的 PR 模板(给贡献者用)

未来开放外部贡献时,PR 模板一并检查这份 schema:

```markdown
## 场景信息
- [ ] meta.yaml 所有必填字段填写完整
- [ ] tech_stack / skills 使用了受控词表中已有的值,未自造词
- [ ] difficulty 和 estimated_minutes 合理(1 星 < 5 分钟,3 星 10-15 分钟)

## 文件规范
- [ ] Dockerfile 继承自 opslabs/base:v1
- [ ] check.sh 遵守 OK/NO 约定
- [ ] check.sh 幂等,多次调用结果一致
- [ ] tests/solution.sh 存在且能通关

## 内容质量
- [ ] README.md 背景故事真实、目标明确
- [ ] 至少支持两种解法(或在描述中说明)
- [ ] 三档 hints 阶梯合理,最强提示能让卡死的用户过

## 本地验证
- [ ] 本地起容器,手动跑通一遍
- [ ] 本地直接跑 check.sh,故障状态返回 NO
- [ ] 本地跑 solution.sh 后跑 check.sh,返回 OK
```

---

## 一句话总结

**meta.yaml 是场景的身份证**。前端靠它筛选展示,后端靠它调度和判题,用户靠它建立能力画像,CI 靠它自动验证,创作者靠它作模板。