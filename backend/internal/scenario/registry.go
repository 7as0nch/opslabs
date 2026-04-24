/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 场景注册表(硬编码),Week 2 可改为扫描 scenarios//meta.yaml
**/
package scenario

import (
	"errors"
	"sort"
	"sync"
)

// ErrScenarioNotFound slug 不存在
var ErrScenarioNotFound = errors.New("scenario not found")

// Registry 场景注册表接口
type Registry interface {
	// Get 按 slug 获取场景,不存在返回 ErrScenarioNotFound
	Get(slug string) (*Scenario, error)
	// List 列出所有已上架的场景(按 difficulty 升序,同难度按 slug 字典序)
	List() []*Scenario
}

type memRegistry struct {
	mu   sync.RWMutex
	data map[string]*Scenario
}

// NewRegistry 构造硬编码的场景注册表
func NewRegistry() Registry {
	r := &memRegistry{data: make(map[string]*Scenario)}
	for _, s := range builtinScenarios() {
		r.data[s.Slug] = s
	}
	return r
}

func (r *memRegistry) Get(slug string) (*Scenario, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.data[slug]
	if !ok {
		return nil, ErrScenarioNotFound
	}
	return s, nil
}

func (r *memRegistry) List() []*Scenario {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Scenario, 0, len(r.data))
	for _, s := range r.data {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Difficulty != out[j].Difficulty {
			return out[i].Difficulty < out[j].Difficulty
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// ============================================================
// 内置场景定义 - 对应 backend/internal/scenarios/README.md
// 镜像前缀从 opslabs/* 统一改为 opslabs/*
// ============================================================
func builtinScenarios() []*Scenario {
	return []*Scenario{
		scenarioHelloWorld(),
		scenarioFrontendDevserverDown(),
		scenarioBackendAPI500(),
		scenarioOpsNginxUpstreamFail(),
		scenarioCSSFlexCenter(),
		scenarioWebContainerNodeHello(),
		scenarioWasmLinuxHello(),
	}
}

func scenarioHelloWorld() *Scenario {
	return &Scenario{
		Slug:    "hello-world",
		Version: "1.0.0",
		Title:   "欢迎来到 opslabs",
		Summary: "熟悉平台操作,3 分钟上手第一个任务",
		DescriptionMd: `# 欢迎来到 opslabs

你好,欢迎!

在你开始真正的故障排查之前,先用 3 分钟熟悉一下这个环境。

## 你面前的工作台

- **左侧**:这份任务说明(你现在正在读的)
- **右侧**:一个真实的 Linux 终端,你可以在里面执行任意命令

## 你的第一个任务

1. 查看你 home 目录下的 ` + "`welcome.txt`" + ` 文件
2. 按照文件里的指示完成一个小操作
3. 点击右下角的「检查答案」按钮

完成后你就正式上路了。

## 小贴士

- 遇到不会的命令别慌,下方有三档提示可以看
- 提示越强会暴露越多答案,建议先自己试试
- 每个场景都有时间限制,空闲太久容器会回收
`,
		Category:         "guide",
		Difficulty:       1,
		EstimatedMinutes: 3,
		TargetPersonas:   []string{"student", "frontend-engineer", "backend-engineer", "ops-engineer"},
		ExperienceLevel:  "intern",
		TechStack:        []string{"linux"},
		Skills:           []string{"basic-shell"},
		Commands:         []string{"cat", "touch", "ls"},
		Tags:             []string{"onboarding", "tutorial"},
		Runtime: RuntimeConfig{
			Image:              "opslabs/hello-world:v1",
			MemoryMB:           128, // 引导场景够用,降下来省内存
			CPUs:               0.2,
			IdleTimeoutMinutes: 30,
			PassedGraceMinutes: 10,
			// 注意:不能用 none —— 用了 --network=none 后 --publish 映射失效,
			// 前端 iframe 会连不到 ttyd。用 isolated 走 opslabs-scenarios 自定义网络,
			// 后续若要真正断网,应在 docker network 创建时加 --internal
			NetworkMode: "isolated",
			// 开启最严:只读根文件系统 + tmpfs,用户的写入只落 tmpfs,重启即消失
			Security: SecurityConfig{
				ReadonlyRootFS: true,
				TmpfsSizeMB:    32, // 只是 touch 一个空文件,32MB 绰绰有余
			},
		},
		Grading: GradingConfig{
			CheckScript:         "/opt/opslabs/check.sh",
			CheckTimeoutSeconds: 5,
			SuccessOutput:       "OK",
		},
		Hints: []Hint{
			{Level: 1, Content: "不知道如何查看文件?试试 cat ~/welcome.txt"},
			{Level: 2, Content: "创建空文件可以用 touch 命令,格式:touch /路径/文件名"},
			{Level: 3, Content: "在终端执行:touch /tmp/ready.flag"},
		},
	}
}

func scenarioFrontendDevserverDown() *Scenario {
	return &Scenario{
		Slug:    "frontend-devserver-down",
		Version: "1.0.0",
		Title:   "本地 dev server 启动失败",
		Summary: "接手项目跑不起来,排查 Node 版本、依赖、端口、配置",
		DescriptionMd: `# 本地 dev server 启动失败

## 背景

你刚入职,接手一个 React 项目,目录在 ` + "`~/webapp`" + `。

同事跟你说:"直接 ` + "`npm run dev`" + ` 就能跑起来了。"

但你试了好几次都失败。

## 你的任务

排查问题,让开发服务器成功启动。

**验收标准**:在另一个 shell 里执行 ` + "`curl http://localhost:3000`" + `,返回 HTTP 200 且响应体是 HTML 内容。

## 提示

- 可能不止一个问题
- 别忽略报错信息,它通常会给你方向
- 你有 sudo 权限,可以装包、改配置、kill 进程
`,
		Category:         "frontend",
		Difficulty:       2,
		EstimatedMinutes: 8,
		TargetPersonas:   []string{"frontend-engineer", "full-stack", "student"},
		ExperienceLevel:  "junior",
		TechStack:        []string{"nodejs", "vite", "react"},
		Skills:           []string{"dependency-management", "port-conflict", "config-troubleshooting", "version-management"},
		Commands:         []string{"node", "npm", "nvm", "lsof", "ss"},
		Tags:             []string{"onboarding", "real-world", "common"},
		Runtime: RuntimeConfig{
			Image:              "opslabs/frontend-devserver-down:v1",
			MemoryMB:           1024,
			CPUs:               0.5,
			IdleTimeoutMinutes: 30,
			PassedGraceMinutes: 10,
			NetworkMode:        "internet-allowed",
		},
		Grading: GradingConfig{
			CheckScript:         "/opt/opslabs/check.sh",
			CheckTimeoutSeconds: 10,
			SuccessOutput:       "OK",
		},
		Hints: []Hint{
			{Level: 1, Content: "npm run dev 的报错信息别忽略,它通常会告诉你缺什么。同时注意端口占用、Node 版本、依赖安装这几件事"},
			{Level: 2, Content: "依次检查:Node 版本是否符合 package.json 的要求、node_modules 是否存在、3000 端口是否被占用、配置文件里有没有拼写错误"},
			{Level: 3, Content: "nvm use 20 切 Node 版本 → lsof -i:3000 找占用进程并 kill → npm install 装依赖 → cat vite.config.js 看 host 字段拼写"},
		},
	}
}

func scenarioBackendAPI500() *Scenario {
	return &Scenario{
		Slug:    "backend-api-500",
		Version: "1.0.0",
		Title:   "API 总是返回 500",
		Summary: "用户接口一直 500,看日志、查配置、验证数据库连接",
		DescriptionMd: `# API 总是返回 500

## 背景

一个用户 API 部署在这台服务器上。产品反馈:

> "访问 ` + "`http://localhost:8080/users/1`" + ` 一直返回 500,你看看咋回事"

你登上服务器检查:

- 服务进程在跑(` + "`systemctl status app`" + ` 显示 active)
- PostgreSQL 也在跑
- 但接口就是不通

## 你的任务

找出问题,让 ` + "`GET /users/1`" + ` 返回 200 和正常的 JSON 响应。

**验收标准**:连续 3 次 ` + "`curl http://localhost:8080/users/1`" + ` 都返回 200,且响应体包含 ` + "`\"id\"`" + ` 字段。

## 提示

- 不要直接改代码,先看日志
- 这个服务依赖 PostgreSQL 数据库
- 修改配置后记得重启服务
`,
		Category:         "backend",
		Difficulty:       3,
		EstimatedMinutes: 12,
		TargetPersonas:   []string{"backend-engineer", "full-stack"},
		ExperienceLevel:  "junior",
		TechStack:        []string{"python", "flask", "postgresql", "systemd"},
		Skills:           []string{"log-analysis", "config-troubleshooting", "database-connectivity", "service-management"},
		Commands:         []string{"journalctl", "systemctl", "psql", "curl", "tail"},
		Tags:             []string{"interview-common", "real-world", "500-error"},
		Runtime: RuntimeConfig{
			Image:              "opslabs/backend-api-500:v1",
			MemoryMB:           768,
			CPUs:               0.5,
			IdleTimeoutMinutes: 30,
			PassedGraceMinutes: 10,
			NetworkMode:        "isolated",
			Variants:           []string{"db-password"},
		},
		Grading: GradingConfig{
			CheckScript:         "/opt/opslabs/check.sh",
			CheckTimeoutSeconds: 10,
			SuccessOutput:       "OK",
		},
		Hints: []Hint{
			{Level: 1, Content: "API 返回 500 但你不知道为啥?先看服务的错误日志,别盲猜"},
			{Level: 2, Content: "日志会告诉你数据库相关的错。检查配置文件里的数据库密码是否正确"},
			{Level: 3, Content: "看 /var/log/app/error.log 发现 password authentication failed。cat /etc/app/config.yaml 对比 PostgreSQL 的实际密码。改对后 systemctl restart app"},
		},
	}
}

// scenarioCSSFlexCenter V1 首个 static 执行模式场景 —— 纯前端判题,不起容器
//
// 场景目标:教会学员用 Flex 在两个轴上把子元素居中。判题逻辑写在 bundle 的 HTML 里
// (view-source 即可见),适合"学原理"题型,不打算防爆解 —— 真要防爆解走 sandbox。
//
// 为什么挂在 frontend 分类 + difficulty 1:
//   - 前端新人第一周常踩的坑(margin: auto? transform? 还是 flex?)
//   - 放 difficulty 1 让学员在 hello-world 之后很快遇到,建立对"题型"的信心
func scenarioCSSFlexCenter() *Scenario {
	return &Scenario{
		Slug:    "css-flex-center",
		Version: "1.0.0",
		Title:   "CSS Flexbox 水平垂直居中",
		Summary: "用 flex 把一个元素在容器里居中 —— 最基础但最常问的 CSS 面试题",
		DescriptionMd: `# CSS Flexbox 水平垂直居中

## 背景

这是前端八股里问烂了,但实战里又天天写错的题:

> **给定一个容器和一个子元素,让子元素在容器里水平 + 垂直居中。**

你会在右边看到一个在线 CSS 编辑器 + 实时预览。容器 ` + "`.container`" + ` 是一个黄色盒子,
里面有一个蓝色 ` + "`.box`" + `,现在 box 被挤在左上角。

## 你的任务

1. 让 ` + "`.container`" + ` 成为 flex 布局
2. 让 ` + "`.box`" + ` 在容器里**水平且垂直居中**
3. **不要**改 ` + "`.box`" + ` 自己的宽高

## 验收标准

判题时会:

- 校验 ` + "`.container`" + ` 的 computedStyle.display === 'flex'(或 'inline-flex')
- 计算 ` + "`.box`" + ` 与 ` + "`.container`" + ` 的中心点,两个轴偏移都 ≤ 2px 视为通过

## 提示

- 主轴用什么属性控制对齐?交叉轴又是哪个?
- ` + "`justify-content`" + ` 和 ` + "`align-items`" + ` 分别管谁
- 亚像素取整:2px 容忍已经给得够宽了
`,
		Category:         "frontend",
		Difficulty:       1,
		EstimatedMinutes: 5,
		TargetPersonas:   []string{"frontend-engineer", "full-stack", "student"},
		ExperienceLevel:  "intern",
		TechStack:        []string{"css", "flexbox"},
		Skills:           []string{"flexbox", "layout", "centering"},
		Commands:         []string{}, // static 模式不跑 shell
		Tags:             []string{"interview-common", "frontend-basics", "static"},
		ExecutionMode:    ExecutionModeStatic,
		// Runtime 在 static 模式下不起作用,留空即可
		// Grading 同样不走后端 check.sh,判题在 bundle 内 postMessage 回传
		Hints: []Hint{
			{Level: 1, Content: "主轴默认是水平方向,justify-content 管主轴对齐。要让子元素在主轴居中,用 center"},
			{Level: 2, Content: "交叉轴用 align-items 控制。两个都设为 center,box 自然就居中了"},
			{Level: 3, Content: ".container { display: flex; justify-content: center; align-items: center; }"},
		},
	}
}

// scenarioWebContainerNodeHello V1 首个 web-container 执行模式场景
//
// 原理:浏览器里用 StackBlitz WebContainer 起一个 Node.js runtime,
// 挂载 bundle 下发的 project.json 文件树,用户直接在浏览器里改代码 + 跑 npm install + node check.mjs,
// 后端完全不碰代码。
//
// 相比 sandbox 容器方案的取舍:
//   - 优点:冷启动快(不用拉镜像)、资源占用低(所有算力挪到用户浏览器)、
//     任意多用户并发零成本、部署零依赖(不需要 Docker)
//   - 缺点:浏览器要支持 SharedArrayBuffer(需要 COOP/COEP 跨域隔离头),
//     Node 版本跟着 WebContainer 走,不能随便指定
//
// 适合题型:纯前端 / Node 小题,不涉及原生依赖(如 sharp、node-gyp)
func scenarioWebContainerNodeHello() *Scenario {
	return &Scenario{
		Slug:    "webcontainer-node-hello",
		Version: "1.0.0",
		Title:   "Node.js 入门:修复 greeting handler",
		Summary: "浏览器里直接跑 Node,修一个对象字段名拼错的 bug",
		DescriptionMd: `# Node.js 入门:修复 greeting handler

## 背景

你面前是一个极简 Node 项目,跑在**浏览器里的 Node.js runtime**(StackBlitz WebContainer)。
完全不需要后端容器,连 npm install 都在你的浏览器里跑。

## 你的任务

打开 ` + "`handler.js`" + `,里面有一个默认导出函数:

` + "```js" + `
export default function greet(name) {
  return { greet: 'hello ' + name }   // 这里故意写错了
}
` + "```" + `

判题脚本 ` + "`check.mjs`" + ` 会用 ` + "`name='world'`" + ` 调它,期望返回值是
` + "`{ greeting: 'hello world' }`" + `。

**注意字段名是 ` + "`greeting`" + `,不是 ` + "`greet`" + `**。

修好后点"检查答案",Runner 会在 WebContainer 里 ` + "`node check.mjs`" + `,
退出码 0 即通关。

## 提示

- 打开 ` + "`check.mjs`" + ` 看一眼判题逻辑,比盲改更快
- 终端面板会显示命令输出,报错信息就在那里
- 改完记得保存(Runner 会把编辑器内容同步回 WebContainer)
`,
		Category:         "backend",
		Difficulty:       1,
		EstimatedMinutes: 5,
		TargetPersonas:   []string{"backend-engineer", "full-stack", "student"},
		ExperienceLevel:  "intern",
		TechStack:        []string{"nodejs", "javascript"},
		Skills:           []string{"debugging", "basic-js"},
		Commands:         []string{}, // web-container 模式 UI 自带命令,不需要裸终端
		Tags:             []string{"interview-common", "web-container", "nodejs-basics"},
		ExecutionMode:    ExecutionModeWebContainer,
		Hints: []Hint{
			{Level: 1, Content: "check.mjs 里那句 `out.greeting !== expected.greeting` 就是全部线索"},
			{Level: 2, Content: "把 handler.js 里的字段名 `greet` 改成 `greeting` 即可"},
			{Level: 3, Content: "return { greeting: 'hello ' + name }"},
		},
	}
}

// scenarioWasmLinuxHello V1 首个 wasm-linux 执行模式场景
//
// 原理:浏览器里用 v86(GPL 开源 x86 模拟器,WebAssembly 版)跑一个极简 Linux(BusyBox),
// 判题和终端交互都在 iframe 内通过 v86 的 serial adapter 完成,
// 跟 Static 模式共用 opslabs:ready / opslabs:check 这套 postMessage 协议。
//
// 为什么不用 CheerpX:
//   - CheerpX 需要商业 license,二次分发受限
//   - v86 GPL 可以自由 embed,首屏大小约 300KB(压缩后),磁盘镜像可以用精简 BusyBox ~4MB
//
// 为什么走 iframe + postMessage 而不是像 WebContainer 那样 main frame:
//   - v86 不依赖 SharedArrayBuffer,没有 cross-origin isolation 要求
//   - iframe 隔离之后 main thread 不会被模拟器 JIT 卡顿(v86 只是单线程 JS,放 iframe 会好一点)
//   - 协议一致 → 前端 BundleRunner 一份实现就能带两种模式
//
// V1 Round 3 先实装骨架 + 一个 hello-world 题型,后续再扩到真 Linux 运维题
func scenarioWasmLinuxHello() *Scenario {
	return &Scenario{
		Slug:    "wasm-linux-hello",
		Version: "1.0.0",
		Title:   "wasm Linux 欢迎:touch 一个文件",
		Summary: "浏览器本地跑 wasm Linux(v86 + BusyBox),touch 一个文件即通关",
		DescriptionMd: `# wasm Linux 欢迎

## 背景

你右边看到的是一个**完全跑在你浏览器里的 Linux**:

- **引擎**:[v86](https://github.com/copy/v86)(GPL 开源的 x86 模拟器,WebAssembly 版)
- **系统**:BusyBox + musl libc 精简镜像,开机约 3 秒
- **没有后端容器**:你敲的每一条命令都在你当前这个浏览器 tab 里执行,
  我们的服务器完全不知情

## 你的任务

在 ` + "`/tmp`" + ` 目录下创建一个名为 ` + "`ready.flag`" + ` 的空文件即可通关。

## 验收

Runner 每次点"检查答案"会跑:

` + "```bash" + `
[ -f /tmp/ready.flag ] && echo OK
` + "```" + `

看到 ` + "`OK`" + ` 即通过。

## 提示

- 最常用的命令是 ` + "`touch`" + `
- 看 ` + "`ls /tmp`" + ` 能不能看到你新建的文件
`,
		Category:         "ops",
		Difficulty:       1,
		EstimatedMinutes: 3,
		TargetPersonas:   []string{"ops-engineer", "backend-engineer", "student"},
		ExperienceLevel:  "intern",
		TechStack:        []string{"linux", "busybox", "wasm"},
		Skills:           []string{"basic-shell"},
		Commands:         []string{"touch", "ls", "cat"},
		Tags:             []string{"onboarding", "wasm-linux", "v86"},
		ExecutionMode:    ExecutionModeWasmLinux,
		Hints: []Hint{
			{Level: 1, Content: "touch 命令可以创建空文件:`touch 文件名`"},
			{Level: 2, Content: "在 /tmp 目录下创建,完整命令:`touch /tmp/ready.flag`"},
			{Level: 3, Content: "`touch /tmp/ready.flag`,然后点检查答案"},
		},
	}
}

func scenarioOpsNginxUpstreamFail() *Scenario {
	return &Scenario{
		Slug:    "ops-nginx-upstream-fail",
		Version: "1.0.0",
		Title:   "Nginx 反代 502 排查",
		Summary: "Nginx 返回 502,后端明明活着,找出 upstream 问题",
		DescriptionMd: `# Nginx 反代 502 排查

## 背景

客服反馈网站打不开,你登上服务器检查。架构很简单:

` + "```" + `
Client → Nginx (:80) → app service (:8080)
` + "```" + `

当前情况:

- ` + "`curl http://localhost/`" + ` 返回 **502 Bad Gateway**
- Nginx 进程在跑
- app 服务进程也在跑

## 你的任务

找出问题并修复,让 ` + "`curl http://localhost/`" + ` 返回 200,响应体包含 ` + "`Hello from app`" + `。

**约束**:

- 不要重装任何组件
- 不要改 app 的代码
- 只调整配置和服务状态即可

## 提示

- 有多种解法,能让服务恢复就行
- 修改 nginx 配置后用 reload,不要直接 restart
`,
		Category:         "ops",
		Difficulty:       3,
		EstimatedMinutes: 12,
		TargetPersonas:   []string{"ops-engineer", "sre", "devops-engineer", "backend-engineer"},
		ExperienceLevel:  "mid",
		TechStack:        []string{"nginx", "linux", "systemd"},
		Skills:           []string{"log-analysis", "service-management", "network-troubleshooting", "config-troubleshooting", "port-conflict"},
		Commands:         []string{"nginx", "ss", "netstat", "tail", "curl", "systemctl"},
		Tags:             []string{"interview-common", "real-world", "502-error"},
		Runtime: RuntimeConfig{
			Image:              "opslabs/ops-nginx-upstream-fail:v1",
			MemoryMB:           512,
			CPUs:               0.5,
			IdleTimeoutMinutes: 30,
			PassedGraceMinutes: 10,
			NetworkMode:        "isolated",
		},
		Grading: GradingConfig{
			CheckScript:         "/opt/opslabs/check.sh",
			CheckTimeoutSeconds: 10,
			SuccessOutput:       "OK",
		},
		Hints: []Hint{
			{Level: 1, Content: "502 意味着 Nginx 连不上后端。先看 Nginx 错误日志,它会告诉你 Nginx 在尝试连哪个端口"},
			{Level: 2, Content: "日志里的端口和后端 app 实际监听的端口对得上吗?用 ss -tlnp 看 app 真实监听在哪"},
			{Level: 3, Content: "tail /var/log/nginx/error.log 看到 connect() failed → ss -tlnp | grep python 发现 app 在 8081 → 改 /etc/nginx/conf.d/default.conf 的 proxy_pass → nginx -s reload"},
		},
	}
}
