<!--
 * @Author: chengjiang
 * @Date: 2026-04-20 22:33:41
 * @Description: 
-->

> **⚠️ 归档声明**:本文档归档于 `docs/archive/`,属于 **历史参考**(V1 初版产品设计草稿)。
> 项目当前进度与规划请看仓库根的 [PLAN.md](../../PLAN.md),对外介绍请看 [INTRODUCTION.md](../../INTRODUCTION.md)。
> 本文里提到的场景 / 接口 / 页面设计可能已经演进,不代表当前实现。

---

# 第一部分:初版产品设计方案

## 1. 产品定位

**一句话定位**:在浏览器里玩的 Linux 真实故障排查练习平台。

**目标用户**(初版只服务这一类):
- 1-5 年经验的后端/运维/SRE,想提升排障能力或准备面试
- 次要群体:计算机专业高年级学生、转行到运维的人

**用户核心诉求**:"我想在安全环境里练手真实故障,练完知道自己哪里菜。"

**产品名**:建议朗朗上口、能联想到场景的,比如 `opslabs`、`ServerDown`、`排障岛`、`修服务`——先起一个占位名就行,上线前再定。文档里先用 **opslabs** 代称。

## 2. 核心功能清单(V1 上线范围)

| 功能 | 是否包含 | 说明 |
|---|---|---|
| 场景列表页 | ✅ | 展示 5 个初始场景,难度分级 |
| 场景详情页 | ✅ | 左侧题目描述,右侧 Web 终端 |
| Web 终端 | ✅ | 基于 ttyd,连到独立容器 |
| 判题按钮 | ✅ | 调 check.sh,返回通过/未通过 |
| 分级提示 | ✅ | 3 档提示,依次解锁,解锁后扣成绩 |
| 场景通关展示 | ✅ | 通关后显示用时、提示使用次数 |
| 微信/GitHub 扫码登录 | ✅ | 最小用户系统,只存 id、昵称、头像 |
| 个人主页 | ✅ | 我通关了哪些场景 |
| 反馈入口 | ✅ | 每个场景通关后弹反馈窗 |
| AI 助手 | ❌ | V1 不做,V2 再上 |
| Windows/Mac 场景 | ❌ | V1 不做 |
| 支付 | ❌ | V1 不做 |
| 场景创作者后台 | ❌ | V1 场景全部你自己写,手动加到 Git |

## 3. 初始 5 个场景设计

按"真实 × 易做 × 有教学价值"选的:

**场景 1:磁盘被谁打满了**(★ 入门)
- 背景:服务器告警磁盘 95%,请找出哪个进程在疯狂写日志并停掉它(不要删日志)
- Setup:启动一个 bash 脚本在后台 while true 写 /var/log/bad.log
- Check:检查该进程是否还在,且 bad.log 文件依然存在
- 考点:`df`、`du`、`lsof`、`ps`、`kill`

**场景 2:端口被占了启动不起来**(★ 入门)
- 背景:Nginx 启动失败说端口 80 被占,找出凶手并让 Nginx 成功启动
- Setup:先启动一个 python -m http.server 80 占着,Nginx 配置已就绪但没跑
- Check:curl localhost:80 返回 nginx 默认页
- 考点:`netstat`/`ss`、`lsof -i`、`systemctl`

**场景 3:定时任务悄悄跑不动了**(★★ 中等)
- 背景:一个每分钟执行的 cron 任务从昨晚开始没产出,查原因并修复
- Setup:cron 脚本里引用了一个权限错误的文件 / 路径写错 / 环境变量缺失(三选一随机)
- Check:最近 2 分钟内产出文件存在且内容正确
- 考点:`crontab`、cron 日志、权限、环境变量

**场景 4:CPU 飙到 100% 的神秘进程**(★★ 中等)
- 背景:监控告警 CPU 长期 100%,找到罪魁祸首并终止
- Setup:一个伪装成 `sshd-worker` 名字的死循环程序
- Check:该进程不存在,CPU idle > 80%
- 考点:`top`、`pidstat`、`ps`、线程级分析

**场景 5:服务能启动但请求全 502**(★★★ 进阶)
- 背景:Nginx 跑着、后端服务也跑着,但访问全是 502,修好它
- Setup:Nginx upstream 指向 127.0.0.1:8080,但后端实际监听 8081;或防火墙把 8080 挡了
- Check:curl localhost 返回 200 且内容正确
- 考点:Nginx 配置、upstream、端口检查、防火墙、日志分析

这 5 个场景覆盖了"资源类、进程类、配置类、网络类"四大类故障,足够展示产品能力。

## 4. 产品页面结构

```
/                    首页(产品介绍 + 场景入口)
/scenarios           场景列表(按难度筛选)
/scenarios/:id       场景详情 + 终端 + 判题
/me                  我的主页(通关记录)
/login               扫码登录
/about               关于 + 反馈
```

**关键界面布局(场景详情页)**:

```
┌──────────────────────────────────────────────────┐
│ Logo   场景列表   我的   登录头像                 │
├─────────────────────┬────────────────────────────┤
│                     │                            │
│  场景背景故事       │                            │
│  你的任务           │       Web 终端             │
│  ────────           │       (ttyd iframe)        │
│                     │                            │
│  💡 提示 1 (解锁)   │                            │
│  💡 提示 2 (锁)     │                            │
│  💡 提示 3 (锁)     │                            │
│                     │                            │
│  [ 检查答案 ]       │                            │
│  已用时: 03:24      │                            │
│                     │                            │
└─────────────────────┴────────────────────────────┘
```

左侧 40%,右侧 60%,移动端上下堆叠(但 V1 优先保证桌面体验,移动端能看就行)。

## 5. 技术架构(务实版)

**前端**
- Next.js 或 Vue + Vite(选你顺手的)
- xterm.js 不用自己搞,直接 iframe 嵌 ttyd 的页面最省事
- Tailwind CSS 快速出 UI
- 部署:Vercel 免费额度够用,或者自己服务器 Nginx

**后端**
- Node.js (NestJS / Express) 或 Go (Gin),选你熟的
- PostgreSQL(用户、通关记录)+ Redis(会话、限流)
- 核心 API:用户登录、场景列表、创建场景实例、判题、记录结果

**容器/场景运行时**
- 单机 Docker 起步,**不要上 K8s**,前 1000 个用户一台 8C16G 机器够
- 每个用户进入场景 → 后端起一个容器 → 返回 ttyd 的 URL → 前端 iframe 加载
- 每个容器带一个 check.sh 脚本,判题 API 通过 `docker exec` 调用
- 空闲 30 分钟自动销毁容器(定时任务扫一遍)
- 基础镜像用 Ubuntu 22.04,每个场景一个 Dockerfile 继承

**场景 Git 仓库结构**
```
/scenarios
  /01-disk-full
    README.md          # 场景描述(给用户看)
    Dockerfile         # 构建镜像
    setup.sh           # 容器启动时执行,埋故障
    check.sh           # 判题脚本,输出 OK/NO
    hints.json         # 3 条分级提示
    meta.yaml          # 难度、标签、预计用时
  /02-port-conflict
    ...
```

**路由/代理**
- 单机阶段:Nginx + 动态端口映射就够了
- 每个容器启动时分配一个宿主机端口(10000-20000 范围),Nginx 配置通过模板生成

**数据库表(最小集)**
```
users          id, provider, provider_uid, nickname, avatar, created_at
scenarios      id, slug, title, difficulty, tags, version (从 Git 同步)
attempts       id, user_id, scenario_id, started_at, finished_at, 
               hints_used, status (running/passed/failed/expired)
feedback       id, user_id, scenario_id, rating, content, created_at
```

就这四张表,足够 V1 跑通。

## 6. 单场景生命周期

```
1. 用户点 "开始场景"
      ↓
2. 后端创建 attempt 记录,docker run 一个容器(带 setup.sh 自动执行)
      ↓
3. 分配一个端口,起 ttyd 把容器 shell 暴露出来
      ↓
4. 返回 ttyd URL 给前端,前端 iframe 加载
      ↓
5. 用户在终端里操作
      ↓
6. 用户点 "检查答案" → 后端 docker exec check.sh → 返回 OK/NO
      ↓
7. 通过则 attempt 标记 passed,弹通关页
   未过则显示 "还没好哦",用户继续
      ↓
8. 30 分钟空闲或通关 10 分钟后,容器自动销毁
```

## 7. 成本与容量预估

- 一台 8C16G 云服务器(阿里云/腾讯云约 ¥600/月)
- 每个场景容器限制 512MB 内存、0.5 CPU
- 理论同时 ~20 个用户,实际 10 个以内稳定
- V1 目标:500 注册用户,日活 50,成本可控在 ¥1000/月内

---

# 第二部分:完整开发流程

## 阶段划分总览

```
Week 0      准备与决策
Week 1-2    骨架搭建(能跑通 1 个场景)
Week 3      内容与打磨(5 个场景 + 基础 UI)
Week 4      小范围内测(朋友 20 人)
Week 5      公开冷启动 + 数据收集
Week 6-7    基于反馈迭代
Week 8      第一次付费点试探
```

**总共 8 周拿到关键决策数据**,然后决定投入 V2 还是转向。

## Week 0:准备与决策(3-5 天)

**做的事**:
- 定产品名、买域名(.com 或 .cn,别省这点钱,百来块)
- 注册 GitHub 组织,建 3 个仓库:`frontend`、`backend`、`scenarios`
- 买一台云服务器(8C16G,Ubuntu 22.04)
- 画出 5 个场景详细设计(背景故事、setup、check、提示),在纸上或 Notion 里,不写代码
- 画产品核心 3 个页面的线框图(手画即可,别用 Figma 浪费时间)

**交付物**:一个 Notion 页面,包含产品定位、场景设计稿、页面线框图。

**不要做**:不要先注册公司、不要设计 Logo、不要研究融资。

## Week 1:骨架跑通(最难的一周)

**目标:一个能用的场景端到端跑起来,哪怕 UI 很丑。**

**Day 1-2:容器运行时**
- 写第一个场景(磁盘打满)的 Dockerfile + setup.sh + check.sh
- 本地 `docker run` 能起来,手动 exec 进去能操作,手动跑 check.sh 能返回对错
- **这一步完成 = 证明技术路线可行**

**Day 3-4:ttyd 接入**
- 在容器里装 ttyd,启动时暴露 7681 端口
- 浏览器直接访问 `http://server:port` 能看到终端并操作
- 到这一步你应该能说:"浏览器里能玩 Linux 了"

**Day 5-7:最简后端**
- 写 3 个 API:`POST /scenarios/:id/start`(起容器)、`GET /scenarios/:id/status`、`POST /scenarios/:id/check`
- 不要用户系统,先用 cookie 里的随机 ID 跟踪会话
- 用内存或者单文件 JSON 存状态都行,先别碰数据库

**周末自测**:打开一个网页 → 自动起容器 → 在终端里操作 → 点检查 → 看结果。这个流程跑通,Week 1 就成功了。

## Week 2:最小前端 + 第 2-3 个场景

**Day 1-3:前端页面**
- 首页 + 场景列表 + 场景详情 3 个页面
- 场景详情页左侧描述 + 右侧 iframe 嵌 ttyd + 判题按钮
- UI 用 Tailwind 默认样式,丑但能用,**不要花时间调 CSS**

**Day 4-5:分级提示逻辑**
- hints.json 格式定好
- 提示解锁后前端显示,并在后端 attempt 里记录使用次数

**Day 6-7:第 2、3 个场景**
- 按第 1 个场景的模板套出来
- 这时你会发现 Dockerfile 可以抽公共 base 镜像,setup/check 可以抽公共工具函数——**开始做模板化**

**周末自测**:3 个场景都能独立完成。找一个信任的技术朋友,让他试玩,全程你不说话只看。**他哪里卡住了,哪里是问题。**

## Week 3:5 个场景齐全 + 最小用户系统

**Day 1-2:第 4、5 个场景**
- 进阶场景(CPU 飙高、502 排查),这两个坑更多,留出时间调

**Day 3-4:用户系统**
- 接 GitHub OAuth(最简单)或微信扫码(更接地气但麻烦,先做 GitHub)
- 建 PostgreSQL,表按上面设计建
- 通关记录落库

**Day 5:个人主页 + 反馈入口**
- 个人主页显示通关列表、用时、提示次数
- 每个场景通关后弹反馈框(1-5 星 + 可选文字)

**Day 6-7:打磨与测试**
- 修 Bug,优化容器回收(别让服务器被僵尸容器撑满)
- 加基础监控(容器数、CPU、内存),能在命令行看到就行
- 写一个最简运维脚本(重启服务、清理容器、看日志)

**Week 3 末里程碑:产品可以公开给陌生人用了。**

## Week 4:小范围内测(最关键的一周)

**不要急着公开发布,先把 20 个熟人拉进来。**

**Day 1:招募 20 个内测**
- 朋友圈发:"做了个 Linux 排障练习平台,招 20 个内测,限免,帮我找 Bug"
- 建一个微信群,把他们都拉进去

**Day 2-5:盯着看**
- 装上 Microsoft Clarity(免费),看**每一个用户的录屏**
- 在群里每天问一个具体问题:"第 3 题有人做出来吗?卡在哪?"
- 记下每一条反馈,分类:Bug / 体验问题 / 功能建议 / 内容问题

**Day 6-7:紧急修复**
- 只修 Bug 和严重体验问题,不加新功能
- 如果有场景明显太难或太简单,立即调整

**Week 4 里程碑的 3 个数据**:
- 完成率:进入场景的人里,多少通关?**目标 > 40%**
- NPS/满意度:1-5 星平均分。**目标 > 3.8**
- 意愿分享数:多少人主动问"能不能分享给朋友"。**目标 > 20%**

达标,进入 Week 5。不达标,**停下来复盘,别公开**,否则冷启动浪费。

## Week 5:公开冷启动

**Day 1:写 3 篇介绍内容**
- 一篇掘金/少数派长文:"我做了个 Linux 排障练习平台,来聊聊为什么"
- 一条 V2EX/即刻短帖:简洁 + 附体验链接 + 求反馈
- 一条 B 站视频(3 分钟):录屏 + 旁白,演示一个场景的完整通关

**Day 2:集中发布**
- 早上 9 点掘金,中午 12 点 V2EX,晚上 8 点即刻——避开撞车
- B 站视频当天发,标题带"面试题""实战""真实故障"之类关键词

**Day 3-7:响应一切反馈**
- 每一条评论都回
- 记录流量来源:哪个渠道 ROI 最高?
- 盯着服务器别挂——流量高峰可能把容器撑爆,准备好临时加机器

**Week 5 目标**:500 次访问、100 注册、50 通关、10 条深度反馈。

## Week 6-7:基于数据迭代

这时你手里有了真实数据,**不再靠拍脑袋决策**。

**三个最常见的发现和对应动作**:

- **发现 1:很多人卡在某个场景** → 要么降难度,要么加引导,要么改提示
- **发现 2:大家反复问"还有更多场景吗"** → 这是最强信号,加紧做 V2 场景包
- **发现 3:大家不想注册就退出了** → 移除注册门槛,前 N 题免注册可玩

这两周也是你**决定下一步战略**的关键:
- 用户爱这个产品 → 加速做 AI 助手、做付费
- 用户用完就走 → 产品黏性有问题,研究为什么,考虑转型
- 用户极少 → 需求可能是伪需求,认真评估要不要继续

## Week 8:付费点小步试探

**不要一上来搞订阅制,先做最轻的验证**:
- 做 3 个"Pro 场景"(更难、更真实、更贴近面试),打上锁头图标
- 解锁价 ¥19 或 ¥29,用个人微信/支付宝收款码先手动处理(用 Stripe/支付宝官方接口留到有量再接)
- 看点击率和付费转化率

**关键指标**:
- 点击"解锁"按钮的用户比例 > 5% = 有付费意愿
- 实际付费转化 > 1% = 值得认真做付费体系

数据好 → 进入 V2 规划(AI 助手、更多场景、企业版)。数据一般 → 调整定价或增值点再试。数据差 → 重新思考产品价值主张。

## 开发过程中的几个硬性纪律

1. **每周必须上线一次**,哪怕是一个小改动。养成迭代节奏。
2. **每天至少看一次用户录屏或反馈**,断了手感产品就做歪了。
3. **不做任何超出当周计划的功能**。想到的好点子写到 Backlog,别插队。
4. **代码可以烂,数据不能丢**。数据库每天备份,这是你未来分析和决策的底气。
5. **上线就监控**。至少有:服务是否存活、容器数、错误日志。出问题能半小时内发现。

## 最后一个最关键的提醒

**你会在 Week 3 或 Week 4 有一个强烈的冲动想加新功能**——AI 助手、更多场景、更漂亮的 UI、排行榜、社交分享……全都忍住。**先把 MVP 发出去,让真实用户告诉你接下来做什么**。

90% 的独立产品死在"再加一点就上线"。**丑也好、简陋也好,Week 5 必须公开发布**。这是硬指标,是这套方案的灵魂。

---

方案先到这里。开工后一定会遇到具体技术问题(容器隔离、ttyd 配置、判题脚本怎么写得靠谱),随时来聊。祝你顺利,我看好这个方向。

---

# opslabs 排障练习平台 · 初版技术方案

## 1. 项目概述

### 1.1 项目背景

国内开发者缺少实战化的 Linux/运维排障练习平台,现有产品(Killercoda、SadServers)均为英文,且不覆盖中文常见业务场景。本项目提供浏览器内真实容器环境,让用户通过解决预埋故障练习排障能力。

### 1.2 设计目标

- 浏览器即开即用,零本地配置
- 真实故障,真实系统响应,非脚本模拟
- 分级提示 + 自动判题,自学闭环
- 架构可扩展,后期支持 AI 助手、Windows 场景、企业面试等

## 2. 业务功能需求

### 2.1 核心功能(V1 范围)

- 场景列表浏览、难度筛选
- 场景进入、终端操作、自动判题
- 分级提示(3 档渐进)
- 用户登录、通关记录、个人主页
- 场景通关反馈收集

### 2.2 非功能需求

- 单场景启动延迟 < 5 秒
- 单台 8C16G 服务器支持 20 并发场景
- 空闲 30 分钟自动回收容器资源
- 每日数据库自动备份

## 3. 流程设计

### 3.1 用户核心流程图(V1)

### 3.2 场景生命周期时序图

#### 3.2.1 启动与判题时序

#### 3.2.2 资源回收时序

## 4. 数据库设计 (PostgreSQL)

### 4.1 users (用户表)
### 4.2 scenarios (场景元信息表)
### 4.3 attempts (通关尝试记录表)
### 4.4 hints_usage (提示使用记录表)
### 4.5 feedback (反馈表)

## 5. 接口定义 (API Definition)

### 5.1 用户登录 (`POST /v1/auth/github`)
### 5.2 获取场景列表 (`GET /v1/scenarios`)
### 5.3 获取场景详情 (`GET /v1/scenarios/:slug`)
### 5.4 启动场景实例 (`POST /v1/scenarios/:slug/start`)
### 5.5 检查答案 (`POST /v1/attempts/:id/check`)
### 5.6 解锁提示 (`POST /v1/attempts/:id/hints/:level`)
### 5.7 结束场景 (`POST /v1/attempts/:id/terminate`)
### 5.8 获取个人通关记录 (`GET /v1/me/attempts`)
### 5.9 提交反馈 (`POST /v1/attempts/:id/feedback`)

## 6. 场景内容体系

### 6.1 引导场景三部曲(Hello World 系列)
### 6.2 场景文件规范
### 6.3 判题脚本约定

## 7. 安全与资源控制

### 7.1 容器资源限制
### 7.2 网络隔离策略
### 7.3 判题脚本防篡改
### 7.4 用户行为限流

## 8. 方案路线图 (Roadmap)

### 8.1 V1(Week 1-8)功能清单
### 8.2 V2(AI 助手 + 付费)
### 8.3 V3(企业版 + 多系统)

---

# Week 1-2 详细设计

## Week 1:骨架跑通

### 目标
一个真实可用的场景(引导场景 1)从浏览器端到端跑通。

### 任务拆解

| Day | 任务 | 验收标准 |
|---|---|---|
| D1 | 服务器准备、Docker 安装、基础镜像构建 | `docker run opslabs-base` 能进入一个带 ttyd 的 shell |
| D2 | 场景 1 Dockerfile + setup.sh + check.sh | 本地手动跑通,故障能复现,check.sh 判题准确 |
| D3 | ttyd 集成,浏览器直连容器终端 | 浏览器访问 `http://server:port` 能操作容器 |
| D4 | Go 后端脚手架(gin + gorm)搭建 | `/ping` 接口可访问 |
| D5 | 场景启动/查询/判题 3 个核心 API | Postman 能完整走通一次流程 |
| D6 | 容器端口动态分配 + Nginx 动态代理 | 并发启动 3 个场景不冲突 |
| D7 | 容器超时回收后台任务 | 30 分钟无操作容器自动销毁 |

## Week 2:前端 + 场景 2、3 + 用户系统

### 目标
完整三个引导场景的前后端联通,用户能登录并记录通关。

### 任务拆解

| Day | 任务 | 验收标准 |
|---|---|---|
| D1-D2 | Next.js 前端:首页、场景列表、场景详情页 | 左侧描述 + 右侧终端 iframe 布局完成 |
| D3 | 分级提示 UI + 解锁逻辑联调 | 点击解锁后提示展开,服务端记录使用 |
| D4 | 场景 2、场景 3 内容制作 | 两个场景手动测试通过 |
| D5 | GitHub OAuth 登录 + 用户表落库 | 扫码能登录并拿到 token |
| D6 | attempts/feedback 表联调,个人主页 | 通关后记录写入,个人页能看到 |
| D7 | 全流程回归测试 + Bug 修复 | 邀请 2 个朋友走完全流程无阻塞 |

---

## 三个引导场景设计

像编程从 Hello World 起步一样,排障的"Hello World"应该是**教用户怎么用这个平台,同时学会最基本的排障动作**——看进程、看日志、看资源。

### 场景 1:欢迎来到 opslabs(★ 引导)

**slug**: `hello-world`
**预计用时**: 3 分钟
**考点**: 熟悉终端、基本文件命令

**背景故事**
> 欢迎来到 opslabs!在你开始真正的故障排查之前,先让你熟悉一下这个环境。
>
> 你当前登录的是一台 Linux 服务器。在你的 `home` 目录下有一个 `welcome.txt` 文件。请读取它的内容,并按文件里的指示完成一个简单任务。

**隐藏任务**(welcome.txt 内容):
> 欢迎来到 opslabs。你的第一个任务:在 /tmp 下创建一个名为 `ready.flag` 的空文件,然后点击"检查答案"按钮。

**Setup 逻辑**
- 在 `/root` 下生成 welcome.txt 文件
- 初始化一个标准 bash 环境,带中文 locale

**Check 逻辑**
- 判断 `/tmp/ready.flag` 是否存在

**三档提示**
- 弱:不知道如何查看文件?试试 `cat` 命令
- 中:创建空文件可以用 `touch /路径/文件名`
- 强:直接执行 `touch /tmp/ready.flag` 即可

**设计意图**:让用户"通过一次",获得第一次成就感,熟悉"描述→操作→检查"的产品节奏。

---

### 场景 2:谁在偷偷吃 CPU(★★ 入门)

**slug**: `cpu-hog`
**预计用时**: 5-8 分钟
**考点**: `top` / `ps` / `kill`,学会"找到异常进程"

**背景故事**
> 凌晨 3 点,监控告警:服务器 CPU 持续 100%。
>
> 你被电话叫醒,打开终端一看确实如此。请找出占用 CPU 的罪魁祸首并终止它。
>
> 注意:可能有多个进程看起来都很忙,请找到真正异常的那个。

**Setup 逻辑**
- 启动一个名为 `data-sync` 的 bash 进程,内部是 `while true; do :; done` 死循环
- 同时启动 2-3 个正常但活跃的进程(nginx、sleep 等)作为干扰项

**Check 逻辑**
- 检查 `data-sync` 进程已不存在
- 检查 CPU idle > 80%(用 mpstat 或 /proc/stat 计算)

**三档提示**
- 弱:从看当前谁在占 CPU 开始,试试 `top`
- 中:找到占用最高的进程后,记下它的 PID
- 强:用 `kill -9 <PID>` 终止异常进程

**设计意图**:第一个"真实故障",让用户体会"看见问题→定位原因→执行动作"的完整排障循环。

---

### 场景 3:磁盘被悄悄填满了(★★★ 基础进阶)

**slug**: `disk-full`
**预计用时**: 10-15 分钟
**考点**: `df` / `du` / `lsof` / 日志分析,学会"顺藤摸瓜"

**背景故事**
> 客服刚刚反馈线上服务异常,你检查发现服务器磁盘使用率 98%。
>
> 请找出是什么在疯狂写入数据,并**停止这个写入行为**。
>
> 注意事项:
> 1. 不要直接删除日志文件,因为运维规范要求保留现场
> 2. 写入源可能不止一个入口,确保你真的找到了根因

**Setup 逻辑**
- 启动一个 Python 脚本,伪装成 `log-collector`,持续往 `/var/log/app/collector.log` 写入
- 该脚本由一个 systemd-like 的 supervisor 管理(如 supervisord),直接 kill 会被拉起来
- 另外预先填一个大的历史日志文件 `/var/log/old-backup.log`(干扰项)

**Check 逻辑**
- 检查 `collector.log` 最近 30 秒内没有新增
- 检查 `log-collector` 进程不存在且不会被自动拉起
- 检查 `/var/log/app/collector.log` 文件**仍然存在**(不能删掉)

**三档提示**
- 弱:先用 `df -h` 看磁盘,然后找找哪个目录最大
- 中:进程被杀后又起来?看看是不是有父进程或 supervisor 管着它
- 强:停 supervisor(`systemctl stop supervisor` 或修改其配置后重启)

**设计意图**:引入"假相"和"防御式设计"——用户会发现 kill 了进程会被自动拉起,从而学到在真实环境里"停服务"不是 kill 那么简单。这是真实生产的精髓。

---

## 4. 数据库设计(Golang struct 伪代码)

用 gorm tag 直接贴近实现:

```go
// User 用户表
type User struct {
    ID           uint64    `gorm:"primaryKey"`
    Provider     string    `gorm:"size:20;index:idx_provider_uid"`  // github/wechat
    ProviderUID  string    `gorm:"size:64;index:idx_provider_uid"`  // 第三方 uid
    Nickname     string    `gorm:"size:64"`
    AvatarURL    string    `gorm:"size:255"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
    LastLoginAt  *time.Time
}

// Scenario 场景元信息(由 Git 仓库同步,DB 只存索引信息)
type Scenario struct {
    ID             uint64    `gorm:"primaryKey"`
    Slug           string    `gorm:"size:64;uniqueIndex"`     // hello-world / cpu-hog
    Title          string    `gorm:"size:128"`
    Summary        string    `gorm:"size:512"`                // 场景列表页展示
    Difficulty     uint8     `gorm:"index"`                   // 1-5 星
    Tags           string    `gorm:"size:255"`                // 逗号分隔: linux,process,cpu
    EstimatedMin   uint16                                     // 预计用时(分钟)
    DockerImage    string    `gorm:"size:128"`                // opslabs/cpu-hog:v1
    Version        string    `gorm:"size:32"`                 // 场景版本号,便于追溯
    IsPublished    bool      `gorm:"default:false"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// Attempt 一次场景尝试
type Attempt struct {
    ID              uint64      `gorm:"primaryKey"`
    UserID          uint64      `gorm:"index:idx_user_scenario"`
    ScenarioID      uint64      `gorm:"index:idx_user_scenario"`
    ScenarioSlug    string      `gorm:"size:64;index"`       // 冗余存一份便于查询
    ContainerID     string      `gorm:"size:64"`             // docker container id
    ProxyURL        string      `gorm:"size:255"`            // ttyd 代理地址
    Status          string      `gorm:"size:16;index"`       // running/passed/failed/expired/terminated
    HintsUsed       uint8       `gorm:"default:0"`
    StartedAt       time.Time
    FinishedAt      *time.Time
    LastActiveAt    time.Time   `gorm:"index"`               // 用于空闲回收判断
    DurationSeconds *uint32                                  // 通关用时(只在 passed 时有值)
    CheckCount      uint16      `gorm:"default:0"`           // 点了几次"检查答案"
}

// HintUsage 提示解锁记录(可选,也可以直接 Attempt.HintsUsed 累计)
type HintUsage struct {
    ID          uint64    `gorm:"primaryKey"`
    AttemptID   uint64    `gorm:"index"`
    HintLevel   uint8                                      // 1/2/3
    UnlockedAt  time.Time
}

// Feedback 反馈
type Feedback struct {
    ID          uint64    `gorm:"primaryKey"`
    UserID      uint64    `gorm:"index"`
    AttemptID   uint64    `gorm:"index"`
    ScenarioID  uint64    `gorm:"index"`
    Rating      uint8                                      // 1-5
    Content     string    `gorm:"type:text"`
    CreatedAt   time.Time
}
```

**索引策略**
- `attempts (user_id, scenario_id)` 查询"某用户在某场景上的尝试历史"
- `attempts (status, last_active_at)` 用于清理任务扫描
- `scenarios (is_published, difficulty)` 场景列表分级筛选

---

## 5. API 接口定义(第一版)

统一返回格式:

```go
type APIResponse struct {
    Code    int         `json:"code"`     // 0 成功,其他为错误码
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```

### 5.1 GitHub 登录

```
POST /v1/auth/github
Body: { "code": "github_oauth_code" }
Resp: {
  "code": 0,
  "data": {
    "token": "jwt_token_here",
    "user": { "id": 1, "nickname": "...", "avatar_url": "..." }
  }
}
```

```go
type GithubLoginReq struct {
    Code string `json:"code" binding:"required"`
}

type LoginResp struct {
    Token string    `json:"token"`
    User  UserBrief `json:"user"`
}

type UserBrief struct {
    ID        uint64 `json:"id"`
    Nickname  string `json:"nickname"`
    AvatarURL string `json:"avatar_url"`
}
```

### 5.2 场景列表

```
GET /v1/scenarios?difficulty=2&tag=linux
Resp: {
  "code": 0,
  "data": {
    "scenarios": [
      {
        "slug": "hello-world",
        "title": "欢迎来到 opslabs",
        "summary": "熟悉你的排障工作台",
        "difficulty": 1,
        "tags": ["guide"],
        "estimated_min": 3,
        "is_passed": true    // 当前登录用户是否通关过
      }
    ]
  }
}
```

```go
type ScenarioListReq struct {
    Difficulty *uint8   `form:"difficulty"`
    Tag        string   `form:"tag"`
}

type ScenarioBrief struct {
    Slug         string   `json:"slug"`
    Title        string   `json:"title"`
    Summary      string   `json:"summary"`
    Difficulty   uint8    `json:"difficulty"`
    Tags         []string `json:"tags"`
    EstimatedMin uint16   `json:"estimated_min"`
    IsPassed     bool     `json:"is_passed"`
}
```

### 5.3 场景详情

```
GET /v1/scenarios/:slug
Resp: {
  "code": 0,
  "data": {
    "slug": "cpu-hog",
    "title": "谁在偷偷吃 CPU",
    "description_md": "# 背景故事\n凌晨 3 点...",  // markdown
    "difficulty": 2,
    "tags": ["linux", "process"],
    "estimated_min": 6,
    "hints_preview": [
      { "level": 1, "unlocked": false },
      { "level": 2, "unlocked": false },
      { "level": 3, "unlocked": false }
    ]
  }
}
```

### 5.4 启动场景

```
POST /v1/scenarios/:slug/start
Resp: {
  "code": 0,
  "data": {
    "attempt_id": 12345,
    "terminal_url": "https://tty.opslabs.cn/s/abc123def",
    "expires_at": "2026-04-20T15:30:00Z"
  }
}
```

```go
type StartScenarioResp struct {
    AttemptID   uint64    `json:"attempt_id"`
    TerminalURL string    `json:"terminal_url"`
    ExpiresAt   time.Time `json:"expires_at"`
}
```

**注意**:同一用户同一场景若已有 `running` 态 attempt,复用而不新建;如果是其他场景,先 terminate 旧 attempt 释放资源。

### 5.5 检查答案

```
POST /v1/attempts/:id/check
Resp: {
  "code": 0,
  "data": {
    "passed": true,
    "message": "恭喜通关!",
    "duration_seconds": 324,
    "hints_used": 1
  }
}
```

```go
type CheckResp struct {
    Passed          bool    `json:"passed"`
    Message         string  `json:"message"`
    DurationSeconds *uint32 `json:"duration_seconds,omitempty"`
    HintsUsed       uint8   `json:"hints_used"`
}
```

### 5.6 解锁提示

```
POST /v1/attempts/:id/hints/:level
Resp: {
  "code": 0,
  "data": {
    "level": 1,
    "content": "从看当前谁在占 CPU 开始,试试 `top`"
  }
}
```

规则:级别必须按 1→2→3 顺序解锁;重复解锁同级别不计数。

### 5.7 结束场景

```
POST /v1/attempts/:id/terminate
Resp: { "code": 0, "data": { "status": "terminated" } }
```

### 5.8 我的通关记录

```
GET /v1/me/attempts?status=passed&page=1&page_size=20
Resp: {
  "code": 0,
  "data": {
    "total": 12,
    "attempts": [
      {
        "id": 100,
        "scenario_slug": "cpu-hog",
        "scenario_title": "谁在偷偷吃 CPU",
        "status": "passed",
        "duration_seconds": 324,
        "hints_used": 1,
        "finished_at": "2026-04-19T10:00:00Z"
      }
    ]
  }
}
```

### 5.9 提交反馈

```
POST /v1/attempts/:id/feedback
Body: { "rating": 5, "content": "题目很真实" }
Resp: { "code": 0 }
```

---

## 6. 核心流程图

### 6.1 用户完整流程(场景生命周期)

![alt text](image-1.png)

![alt text](image.png)

## 7. 安全与资源控制要点

**容器限制**(docker run 参数):
```
--memory=512m --memory-swap=512m --cpus=0.5
--network=opslabs-scenarios   (独立网络,禁止访问宿主和外网)
--cap-drop=ALL --cap-add=SETUID --cap-add=SETGID --cap-add=NET_BIND_SERVICE
--pids-limit=200
--security-opt=no-new-privileges
--read-only=false  (场景内需要写,但挂载只读根 + tmpfs)
```

**判题脚本防篡改**
- `check.sh` 放在 `/opt/opslabs/` 目录,属主 root 且只读(镜像构建时 `chmod 0400`)
- 用户默认 user 权限,无法读取 check.sh 内容
- 执行判题时由外部 agent 用 root 身份 exec

**接口限流**(Redis)
- 启动场景:每用户每分钟最多 1 次
- 判题:每用户每分钟最多 10 次
- 提示解锁:每用户每场景每级别只能解锁一次

## 8. 方案路线图

**V1(Week 1-8)**:3 个引导场景 → 5-8 个实战场景,公开冷启动,收集数据
**V1.5(Week 9-12)**:AI 复盘助手(场景结束后生成复盘报告),解锁付费场景
**V2(Q3)**:NPC 陪练 AI(场景进行中的实时提示)、团队协作、企业面试版
**V3(Q4+)**:Windows 场景(Windows Server Container)、场景创作者开放平台

---

# 可直接贴到 README 的里程碑速查表

```
Week 1  骨架        容器 + ttyd + 3个核心 API            ✓ 场景 1 本地可玩
Week 2  全联通      前端 + 用户系统 + 场景 2/3            ✓ 3 场景端到端
Week 3  打磨        场景 4/5 + 反馈 + 基础监控            ✓ 5 场景齐全
Week 4  内测        20 人邀请制 + 录屏 + 快速迭代         ✓ 完成率/满意度达标
Week 5  公开        掘金/V2EX/即刻/B站 冷启动             ✓ 500 访问
Week 6-7  迭代       按数据优化场景难度 + UX              ✓ 留存 > 15%
Week 8  付费验证    3 个 Pro 场景 + 点击率测试            ✓ 解锁点击 > 5%
```