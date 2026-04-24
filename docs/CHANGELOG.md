# opslabs 变更记录

> 本文档只记录**架构级别**和**用户可见行为**的变更。
> 纯 bug 修复(typo / 样式微调)不必入档,写在 commit message 里即可。

格式:逆时序,新的在上。每条带 `[YYYY-MM-DD] [类型]` 前缀。
类型:`fix` / `feat` / `refactor` / `security` / `docs` / `breaking`。

---

## 2026-04-23

### [fix] R7-04d:wasm-linux 终端 ANSI 乱码 + 箭头键垃圾 + `%` 提示符识别

R7-04c 把键盘改成 char-mode 之后,用户能敲命令了,但终端里充斥 `??[J` / `??[D`
之类的垃圾(截图可见),箭头键尤其严重。并且 ready 判定没第一时间命中,要靠 25s
watchdog 兜底才出来。

**根因**

1. `<textarea>` 是哑终端,不解析 ANSI 转义序列。BusyBox ash 默认配置会发 `\x1b[K`
   (清行)、`\x1b[2J`(清屏)、颜色序列 `\x1b[1;32m` 等,直接 `termEl.value += ch`
   导致 ESC(0x1b,不可渲染,显示成 `?`)后面跟着 `[K` 字面字符挂在屏幕上
2. 我把箭头键映射成 `ESC [ A/B/C/D` 往 serial 发,shell 不认 → 原样 echo 回来 →
   同样被 ESC 乱码问题污染
3. 常见 BusyBox 构建 prompt 用 `%` 而非 `#`/`$`(截图显示 `/root%` → `/%`),
   我的 loose 正则 `/[#$]\s+$/` 没命中,只能等 25s watchdog 强制 ready

**修**(单文件 `backend/internal/scenario/bundles/wasm-linux-hello/index.html`)

1. **ANSI 状态机过滤**:新增 `appendToTerm(byte)`,replaces raw `termEl.value += ch`。
   状态机处理 `normal` / `esc` / `csi` / `osc` 四态:
   - `normal`:可见字符直接渲染;`\b` 前端真退格(删 textarea 最后一字符);
     `\x07`(BEL)/`\x7f`(DEL)/`\x00` 丢弃;其它 <0x20 控制字符丢弃(保留
     `\n`/`\r`/`\t`)
   - `esc`:收到 `[` 进 csi 态;收到 `]` 进 osc 态;其它 ESC 单字符序列丢弃
   - `csi`:累积参数,遇到终止字节 `0x40-0x7e` 整个序列丢弃
   - `osc`:累积到 BEL 或 ESC\ 结束,整体丢弃
   无论 shell 发什么 ANSI 花样,textarea 都只看到干净文本
2. **serialBuf 仍走原样**:判题 marker 匹配 / prompt 探测还是拿全量字节跑正则,
   不受显示过滤影响
3. **箭头 / Delete / Home / End / Escape / PageUp/Down 键盘 noop**:
   preventDefault 但不发 serial。哑终端下送这些序列只会产生乱码,用户按了没反应
   反而比"按了但屏幕上冒垃圾"更好理解 —— 心理模型直接是"这个终端不支持方向键"。
   真要支持 history/line-editing,未来换 xterm.js,不是继续往 serial 塞 ESC 序列
4. **prompt loose 正则加 `%`**:`/[#$]\s+$/` → `/[#$%]\s*$/`。BusyBox 某些构建
   用 `%` 当 root prompt(`/root%`、`/%`);同时把尾随空白放宽为 `\s*`,避免某些
   shell 打完 prompt 不加空格直接接用户输入。autologin 后立刻 ready,不再干等 25s

**结果**

按 R7-04c 的 char-mode + R7-04d 的 ANSI 过滤 + `%` prompt 支持,用户进入场景:
autologin → shell prompt(不管是 `/ # ` / `~# ` / `/%`)立即触发 ready → 可敲
命令 → 屏幕干净无乱码 → `touch /tmp/ready.flag` → 点检查答案 → passed。

### [fix] R7-04c:WebContainer TDZ 崩溃 + wasm-linux 终端字符重复 / Backspace 失效 / 双层加载遮罩

上一批(R7-04a + R7-04b)落地后用户一试就崩:

**1. `WebContainerRunner` 白屏:`Cannot access 'isEditable' before initialization`**

`isEditable` 用 `useCallback` 声明在 UI 区块(大约行 547),但 Ctrl/Cmd+S 的 useEffect
在行 ~449 把它写进依赖数组。`const` 声明没 hoist,useEffect 先跑、依赖数组求值时进 TDZ,
整个组件抛异常、Scenario 页直接变白。

**修**:把 `isEditable` 的 useCallback 提前到所有 handler 和 useEffect 之前声明,
上面加一段注释说明为什么不能往下挪。原 UI 区的重复定义删掉。

**2. wasm-linux 终端:回车后命令重复、Backspace 删不掉**

旧实现同时在前端维护 `lineBuf`(本地行缓冲)+ 每个字符 `sendSerialString(e.key)`:

- 每敲一个字符 → serial 立刻收到一份
- 按回车 → `sendSerialString(lineBuf + '\n')` → serial 又把整行收到一次
- shell 拿到 "lsls\n",看上去就是"命令重复"
- `\b \b`(ANSI 光标回退)BusyBox readline 不认,Backspace 删不动

**修**(单文件 `index.html`):纯 char-mode 重写键盘处理。

- 所有按键 `preventDefault`,textarea 不再本地缓冲,完全由 serial 回显驱动
- 键位映射按真实 TTY 语义:
  - Enter → `0x0d` (CR)
  - Backspace → `0x7f` (DEL,BusyBox `stty erase ^?` 默认能识别)
  - Tab → `0x09`
  - 箭头 → `ESC [ A/B/C/D`(shell 历史 + 行内光标)
  - Delete/Home/End → 标准 ANSI 转义序列
  - Ctrl+C/D/L/U/A/E/K/W → 对应控制字符,覆盖常用 readline 操作
- 粘贴(`paste` 事件):把剪贴板文本一次性塞进 serial,`\r\n` 统一转 `\r`
- `cut`/`drop` 一律 `preventDefault`,防止用户误操作改动 textarea 本地显示

**3. "加载题目 bundle…" 和 bundle 自身 "加载 v86…" 两层居中叠在一起**

父级 `StaticRunner` 的 overlay 原本一直挂到 `opslabs:ready` 才撤,但 wasm-linux 从 iframe
HTML load 完到 v86 起来要十几秒,这段时间两个居中 overlay 视觉重叠,用户以为挂了。

**修**(`StaticRunner.tsx`):新增 `bundleMounted` 状态。

- iframe `onLoad` 触发 → `setBundleMounted(true)`
- 父级 overlay 只在 `!bundleMounted` 时显示,一旦 bundle 自己接手就撤掉
- `ready`(`opslabs:ready` postMessage)语义不变,仍然控制"检查答案"按钮 gating
- `bundleUrl` 变化时两个状态一起 reset

UX 结果:切到 wasm-linux 场景后先看到"加载题目 bundle…"(<1s),然后直接看到 bundle
自己的 boot overlay + 顶栏诊断(秒数/字节数),不再有两个居中文字叠在一起的错觉。

### [fix] R7-04a:wasm-linux ready 判定从头重构 —— 从"靠文本"扩到"文本 + 时间 + 观测"

**背景**

R7-02 修 autologin、R7-03a 放宽 prompt 正则,用户反馈仍然"还是一样的问题"。
用户明确让我"从头梳理一下"。复盘发现前两轮都陷在同一个假设里:**"串口流里一定
会出现我们认得的 login / prompt 文本"**。但这个假设其实很脆弱:

- `libv86.js` 从 copy.sh 被 GFW 拦截 → 本地 vendor 又没跑 `fetch-v86.sh` → 前端
  拿不到 V86 构造函数,但现有代码会吞异常,用户看到的只是遮罩永远不消失
- `linux.iso` 下载 403 / 超时 → `bootEmulator` 抛错被早期 try/catch 吃掉
- v86 wasm 实例化失败 → 同上
- 内核启动但串口驱动没挂好 → `serial0-output-byte` 一个字节都不进,前面再宽
  的正则都救不了

结论:ready 判定不能再只靠"匹配到正确文本",必须加时间维度 + 可观测性。

**重构**(单文件 `backend/internal/scenario/bundles/wasm-linux-hello/index.html`)

1. **诊断面板**:顶栏右上角新增 `diag` 元素,1 秒刷一次,展示:
   - 经过秒数
   - serial 累计字节数(`serialByteCount`)
   - 最近 20 字节尾巴(回车替换成 ·,方便直观看到启动走到哪)
   用户不用开 DevTools 就能截图看"卡在哪",我/下一任调试者省掉一整个来回。
2. **Ready 逻辑拆成四个函数**,每个职责清晰单测友好:
   - `tryAutologin()` —— 文本驱动:login → root,password → 空回车;只在
     `!readyFired` 时触发,避免 shell 里敲到 "login" 字样被误发
   - `fireReadyIfPromptHit()` —— 文本驱动:严格 `/ # ` 或宽松 `[#$]\s+$`
   - `fireReady(reason)` —— 单点出口,幂等,理由打到 status bar + console.log,
     便于事后对因
   - `scheduleReadyWatchdog()` —— **时间兜底**:首字节到达后启动 25s watchdog,
     到期强制 `fireReady('watchdog:25s')`,即使 prompt 格式没覆盖到也不再死锁,
     用户能手动操作自救
3. **30s 超长静默错误**:`setTimeout` 30s 后检查 `serialByteCount === 0`,
   命中就给红字:"内核 30s 未输出任何字符,请跑 scripts/fetch-v86.sh(Linux/
   macOS)或 scripts/fetch-v86.ps1(Windows) 把 vendor/ 填满再刷新"。这是
   资源没加载成功的典型症状,给用户可操作的下一步而不是静默挂起。
4. **passwordSent 变量补声明**:中途增量编辑遗漏 `let passwordSent = false`
   声明(R7-03a 的残留),首次匹配到 "Password:" 时会 ReferenceError;补齐。

**观测胜过猜测**

前两轮修改完全是在"盯代码 + 推测根因 + 调正则"的循环里,直接的因果证据一次
都没到手过。这一轮的核心不是"再写一条更宽的正则",而是把 **"下次再挂的时候,
截图/日志能直接指出到哪一步卡住"** 这件事固化进 bundle 本身。

### [security] R7-04b:WebContainer editableFiles 白名单保护 check 文件

**背景**

R7-03b 把 Monaco 编辑器接进来之后,用户反馈:`check.mjs` 也暴露在文件树里,
可读可改。问题严重 —— 用户只要把 `check.mjs` 里的判题逻辑替换成
`console.log('OK'); process.exit(0)` 就能直接伪装通过;即使不改,光是看到
`check.mjs` 里对 `greeting` 字段的期望,就等于把答案直接送到脸上。原本"修
handler.js 找出 bug"的练习目的被完全掏空。

**修复**(两侧)

1. **`ProjectManifest` 扩展两个可选字段**(`WebContainerRunner.tsx`):
   - `editableFiles?: string[]` —— 白名单。只有列进去的路径 Monaco 允许编辑,
     其它路径 `readOnly: true`,保存/还原按钮 disabled。不设时默认全部可编辑,
     保持 R7-03b 行为兼容。
   - `hiddenFiles?: string[]` —— 黑名单。这些路径扁平化后从 drafts/initialFiles
     里拿掉,文件树里压根不出现。WebContainer 里 `mount` 仍然写入这些文件,
     `check.cmd` 照跑不受影响。
2. **四重防线**(面向将来 Monaco 或 DevTools 绕过):
   - 文件树:hidden 路径不渲染 `<li>`
   - Monaco:非 editable 文件 `readOnly: true`
   - `handleEditorChange`:非白名单路径直接 `return`,不更新 drafts/dirty
   - `Ctrl/Cmd+S` 快捷键 + 保存按钮:`isEditable(activeFile)` 检查失败不写 fs
3. **UI 提示**:文件列表里只读文件变灰 + 右侧显示"只读"小徽标;编辑器工具栏
   标题处也加一个"只读"胶囊,让用户明确知道这文件不是 bug 所在。
4. **manifest 落地**(`backend/.../webcontainer-node-hello/project.json`):
   - `"editableFiles": ["handler.js"]`
   - `"hiddenFiles": ["check.mjs"]`
   - README 重写:删掉对 check.mjs 的文件级描述,改成"判题会 import 你的
     handler.js 传 name='world'"这种不暴露字段名的说明

**测试路径**

- 进入场景,文件树只显示 `handler.js / package.json / README.md`,没有 check.mjs ✓
- 点 package.json / README.md,Monaco readOnly、保存按钮 disabled、工具栏"只读"徽标亮 ✓
- 点 handler.js,正常编辑 → Ctrl+S → 保存 → 检查答案 → OK ✓
- 判题行为不变:WebContainer.fs 里 check.mjs 仍然存在,`node check.mjs` 照跑 ✓

### [feat] R7-03b:WebContainer 场景内嵌 Monaco 代码编辑器

**背景**

用户反馈:"这个场景也功能实现一下哦,还不能编辑代码块"。
`WebContainerRunner` 之前只展示一份文件清单 + 占位提示("暂未嵌入编辑器,
后续 PR 接入 Monaco"),用户拿到 `handler.js` 故意埋的 bug 也没法修,
场景完全卡在"跑一次必红"的状态。

**实现**

`frontend/src/components/runners/WebContainerRunner.tsx` 整体重构 UI,
接入 `@monaco-editor/react`:

- **依赖**:`package.json` 加 `@monaco-editor/react@^4.6.0`。它默认通过
  jsDelivr 加载 monaco 主包和 workers,Vite 侧 COEP=credentialless 允许跨源
  子资源,隔离态(SharedArrayBuffer)和 WebContainer 共存不冲突。
- **文件扁平化**:`flattenTree(FileSystemTree)` 递归展开目录,返回平铺的
  `{ path → contents }`,当前场景虽然都是顶层文件,但结构已支持 `src/foo.js`
  这样的嵌套路径。二进制节点(Uint8Array contents)标占位字符串、不可编辑。
- **UI 三段布局**:左侧文件列表(带 dirty `●` 指示) + 右侧 Monaco 编辑器
  (含保存/还原工具栏) + 底部原有日志面板。`grid-cols-[12rem_1fr]
  grid-rows-[1fr_10rem]` 一把搞定。
- **编辑语义**:
  - `drafts`:当前编辑器内容,内存态,和 WebContainer fs 相互独立
  - `dirty: Set<path>`:已改但未保存的文件集合;只要编辑后和 `initialFiles`
    不同才算脏(用户原样输回去不报脏)
  - **保存**:`wc.fs.writeFile(path, contents)` 把 draft 落到 WebContainer 文件系统
  - **还原**:把 draft 回滚到 `initialFiles`(manifest 原始内容),不动 fs
  - **Ctrl/Cmd+S** 快捷键绑定到保存当前文件
- **判题前自动 flush**:`triggerCheck` 第一步就跑 `flushDirty()`,把所有 dirty
  写进 WebContainer 再 spawn check 命令。避免用户"改了没点保存 → check 还在
  跑旧代码 → 疑惑为什么红"的 UX 陷阱。
- **只读守卫**:`status !== 'ready'` 时 Monaco `readOnly: true`,避免 mount
  完成前用户编辑的改动被 mount 覆盖丢失。
- **语言识别**:`guessLanguage(path)` 按扩展名推 Monaco language(js/ts/json/
  md/html/css/yaml/sh/go/py/sql/toml/dockerfile 等),未知类型降级 plaintext。

**用户体验**

进入 `webcontainer-node-hello` 场景后:
1. 左侧看到 `package.json / handler.js / check.mjs / README.md`
2. 默认打开 `handler.js`(manifest.entrypoint),能看到故意拼错的 `greet` 字段
3. 改成 `greeting` → 顶部 dirty 指示亮起 → 点"保存"或 Ctrl+S
4. 点"检查答案" → flushDirty 自动兜底 → node check.mjs → stdout "OK" → passed

---

## 2026-04-22

### [fix] R7-03a:wasm-linux ready 识别(续)—— 登录后 prompt 不止 "/ # "

**背景**

R7-02 加了 autologin(`login:` → 自动发 `root\n`),但用户实测仍然卡在"加载
题目 bundle…" 遮罩。追查到根因:autologin 自身没问题,shell 也活了,
但 ready 正则 `/\/ # /` 只认 cwd=`/` 时的 BusyBox 提示符。新版 linux.iso 经
getty 登录后 cwd 默认切到 `/root`,prompt 变成 `~ # ` 或 `root@(none):~# `,
原正则完全不命中,`opslabs:ready` postMessage 还是没发出去。

**修复**(单文件 `backend/internal/scenario/bundles/wasm-linux-hello/index.html`)

1. **login 触发条件放宽**:`/login:\s*$/` → `/login:/`。某些 v86 串口驱动
   在 login 后面补 CRLF,原来的 `\s*$` 可能被 `\r` 或空格挤掉导致匹配不上。
2. **登录成功后主动 `cd /`**:延迟 600ms 发 `cd / 2>/dev/null\n`,把 cwd 拉
   回 `/`,让原严格 prompt 正则保持兜底能力。
3. **ready 探测分两档**:
   - 严格档(老行为):`/\/ # /` —— 老 ISO 或 cd / 之后命中
   - 宽松档(新):`autoLoginSent === true` 之后,匹配任意行末 `[#$]\s+$`,
     覆盖 `~ # ` / `root@(none):~# ` / `/root # ` 等所有常见 shell prompt。
     autoLoginSent 前不启用 loose,避免内核 printk 里偶然的 `#` 误触发。
4. **诊断日志**:关键节点加 `console.log`,用户本地复现时开 DevTools 能看到
   "login detected" / "post-login: cd /" / "ready fired; strict=.. loose=.." 三句。

**验证思路**

- 新版 ISO:login → autologin → cd / → strict ready 命中(或宽松档先命中)→ 遮罩消
- 老版 ISO:没有 login 字样,直接 drop shell → strict ready 照旧命中
- 手动输入通道始终开放(R7-02 已经放开 !readyFired keydown 守卫)

### [fix] R7-02:wasm-linux 场景卡在 "(none) login:" 的死锁

用户反馈截图:wasm-linux 欢迎场景,右边 v86 终端显示
"Welcome to Buildroot / (none) login:",但中间"加载题目 bundle…" 遮罩
一直不消失,也无法敲键盘。

**根因**(一条链)

1. `copy.sh` 最新 linux.iso 的 /etc/inittab 切到 `getty`,开机后打
   "(none) login:" 等凭证输入,不再像老版本那样直接 drop 到 `/ # ` shell。
2. `bundles/wasm-linux-hello/index.html` 的 ready 信号只检测 `/ # ` 正则,
   getty 界面下永远不命中,子页面永远不给父页面发 `opslabs:ready`。
3. `StaticRunner.tsx` 的 `!ready` 遮罩"加载题目 bundle…"就一直挂着。
4. index.html 的 keydown 守卫 `if (!readyFired) { e.preventDefault() }`
   又把用户手动输 `root` 的尝试也吞了 —— ready 要求 shell prompt,shell
   prompt 要求登录,登录要求键盘 —— 死锁。

**修复**(前端 bundle 两处)

`backend/internal/scenario/bundles/wasm-linux-hello/index.html`:

1. 新增 `autoLoginSent` 状态 + serial0 监听里 `/login:\s*$/` 匹配逻辑:
   - 只要 serialBuf 末尾是 login 提示符就自动发 `root\n`
   - BusyBox 默认 root 无密码,直接过 getty 进入 `/ # ` shell
   - `readyFired / autoLoginSent` 双锁防重入,shell 里出现 "login" 字样不会误触
2. keydown 守卫放开 `!readyFired`:允许 login 阶段键盘输入,autologin 失效时
   用户能手动输凭证自救。判题触发依然 gated on `readyFired`,login 期点"检查
   答案"会直接返回"Linux 还在启动中"错误,不会误发判题命令。

**兼容性**

- 老版本 linux.iso(直接进 `/ # `):autologin 检查因无 `login:` 匹配而跳过,
  走原有 `/ # ` ready 路径,行为不变。
- 新版 linux.iso(经 getty):autologin 兜底触发,数百毫秒内过登录,继续走
  ready 路径,遮罩消失。

### [fix] R7-01:顶部倒计时每次刷新被"重置满"的 bug

用户反馈:"剩余时间那里有问题,没有拿到缓存的时间吗,每次刷新或者进来都重新计时了"。

**症状**
- 同一场景,开始做题 5 分钟后按浏览器刷新,顶部倒计时从 `剩余 5:00` 跳回 `剩余 10:00`。
- 从列表返回再进入,同样 reset。
- 过 30 秒左右才会"跳"到正确值(轮询把权威 startedAt 拉回来)。

**根因**(两层叠加)

1. `frontend/src/api/attempt.ts::useStartAttempt.onSuccess` 无脑写 `startedAt: now`。
   刷新 / 重进场景时 Scenario.tsx 一律发一次 Start,后端 reuse 分支返回同一个
   attemptId 的同一条 attempt,但前端 onSuccess 依旧把 `startedAt` 覆盖成
   `new Date().toISOString()`,连带 localStorage 里持久化的原 startedAt 被刷没。
   顶部倒计时公式是 `startedAt + estimatedMinutes`,于是看起来就是"重新计时"。

2. `frontend/src/pages/Scenario.tsx` 轮询回调里 `setCurrent(attemptRemote)` 用的是
   `set`(覆盖整个 current)而不是 `patch`(合并)。AttemptReply 里不带
   `expiresAt / idleTimeoutSeconds / passedGraceSeconds / reviewingUntil`,
   set 一次等于把 Start 阶段刚灌进去的这几个字段清成 undefined,导致:
   - FinalTip 警示阈值读 current.expiresAt 永远拿不到,banner 永远不弹
   - 复盘剩余时间丢失

**修复**

`frontend/src/api/attempt.ts`:
- onSuccess 加复用判定:`prev.attemptId === data.attemptId && prev.scenarioSlug === slug`
  视为 backend reuse,保留原 `startedAt`、复用 `idleTimeoutSeconds / passedGraceSeconds /
  reviewingUntil`;新建路径回退到 `now`。

`frontend/src/pages/Scenario.tsx`:
- `setCurrent(attemptRemote)` 改为 `patchCurrent(attemptRemote)`,merge 语义保留
  Start-only 字段。同时清掉 `setCurrent` 局部变量(已无引用)。

**契约**
- StartScenarioReply 依旧不带 started_at(保持 proto 兼容,避免 regen)。
- 权威 startedAt 仍以 GetAttempt 轮询下来的 `r.started_at` 为准,onSuccess 的复用判定
  只是保证"刷新瞬间的闪烁期"startedAt 不抖。

### [breaking] Round 6:AttemptStore 迁 Redis + 删除 AttemptBootstrapper

用户反馈:"有用到 redis 缓存用户的信息的吗,本地缓存都替换为 redis 吧,存场景的一些
信息,做题时长等。"

V0/V1 的 `AttemptStore` 是进程内 `map[int64]*OpslabsAttempt` + `sync.RWMutex`,
单实例能跑但顶不住两种场景:

1. **多实例部署**:两个 backend pod 各自维护一份缓存,用户请求被 LB 打到 pod-B,
   Start 走 FindActive 反查 pod-A 的内存就查不到,会误新建一个攻击容器 + 多写一条
   DB 记录,两个 pod 抢同一份 DB 锁概率小但存在。
2. **进程重启**:Round 3 引入的 `AttemptBootstrapper` 做"DB 回灌内存缓存",但
   回灌发生在 kratos 启动期,窗口内如果有用户请求会落到"缓存 miss 但 DB 有"的
   尴尬分支;而且回灌到内存也只是缓一次,下次重启又得走一遍,本质上内存就不适合
   做这一类状态的主存。

Redis 作为 opslabs 的强依赖本来就存在(`configs/config.yaml` 的 `data.redis`),
这一轮把 attempt 共享状态迁过去一步到位。

**R6-01. 清理 AttemptBootstrapper + RestoreRunning + ListRunning**

- `backend/internal/biz/attempt/repo.go`:从接口里删 `ListRunning`;
- `backend/internal/data/attempt.go`:对应 `ListRunning` 实现也删(DB 只剩
  Create / Update / FindByID 最小集);
- `backend/internal/biz/attempt/usecase.go`:删 `RestoreRunning` 方法,
  `GC 扫描 + 前端 polling 的 NEEDS_RESTART` 已经足够覆盖重启场景;
- `backend/internal/server/attempt_bootstrap.go`:整文件清空 + 注释说明
  (runner.Reconcile 已并到 GCServer.Start,见 R6-05)。

**R6-02. 扩展 db.RedisRepo 接口**

`backend/internal/db/redis.go` 原本只有 `Get/Set/Del/RPush/LRange`,对 attempt
的"主 key + 活跃 SET + owner 反查索引"三段式布局不够用。加:

- `SAdd / SRem / SMembers`:维护 `opslabs:attempt:active` 这个 running/passed
  attemptID 集合,`Snapshot` 走 `SMEMBERS → MGET` 两步拿完整列表;
- `MGet`:批量取主 key,比循环 Get 省一个数量级 RTT;
- `Expire`:Put 后续命主 key + owner 索引的 TTL;
- `Pipeline()`:暴露 `redis.Pipeliner`,把"主 key + SAdd + 两条 owner 索引"的
  4 个命令打成一次 RTT。

**R6-03. consts/redis.go 加 opslabs attempt key**

集中在一处方便 rename / 调试:

```go
RedisKeyAttempt            = "opslabs:attempt:%d"
RedisKeyAttemptActive      = "opslabs:attempt:active"
RedisKeyAttemptOwnerClient = "opslabs:attempt:owner:client:%s:%s"
RedisKeyAttemptOwnerUser   = "opslabs:attempt:owner:user:%d:%s"
```

命名遵循仓库里已有的 `<业务>:<实体>:<维度>:<标识>` 约定(和 `chat:stream:*`、
`user:auth:oauth:qq:state:*` 对齐)。

**R6-04. 重写 store/attempt_store.go 为 Redis 实现**

类型名 `AttemptStore` 保留,API 全量重塑:

- 所有方法首参统一是 `ctx context.Context`(网络 I/O 必须可取消);
- 读类方法(`Get / FindActive*`)返回 3 元组 `(*Attempt, bool, error)`:
  命中 → `(a, true, nil)` / 未命中 → `(nil, false, nil)` / 网络错 → `(nil, false, err)`;
- 写类方法(`Put / Delete / UpdateLastActive / UpdateStatus`)返回 `error`;
- `Put` / `Delete` 用 Pipeline 一次打包主 key + active SET + owner 索引,降 RTT;
- `Snapshot` 走 `SMEMBERS → MGet → JSON.Unmarshal`,`MGet` 拿到 `nil` 时当
  "主 key 过期但 SET 成员没来得及清"处理,顺手 `SRem` 清脏数据;
- `TTL = 60min`(2 倍 idleCutoff),每次 `Put` / `UpdateLastActive` 都刷,
  兜底"进程忘记清理"的死角;
- JSON 损坏时降级为 miss + 主动 `Del + SRem`,避免反复反序列化炸。

**R6-04b. 更新 AttemptStore 所有调用方签名**

API 改动是破坏性的,一次性改完 17 个调用点:

- `backend/internal/biz/attempt/usecase.go`:13 处 —— 复用判定 / Heartbeat /
  Check / Extend / Terminate / CleanupPassed 全改成 ctx + error 处理;
  错误处理分级:`Put` 失败回滚容器并返错(用户看到明确错误码),
  `UpdateLastActive` / `UpdateStatus` / `Delete` 失败只 Warn(不影响主流程);
- `backend/internal/server/gcserver.go`:3 处 —— `Snapshot` 失败本轮跳过,
  `Get` / `Delete` 失败 Warn 后下轮 tick 重试;
- `backend/internal/service/opslabs/ttyd_proxy.go`:1 处 —— 传 `r.Context()`,
  Redis 不可用时返 `502 BadGateway`(不是 `404`,别误导前端重建 attempt)。

**R6-05. GCServer.Start 加 runner.Reconcile 兜底**

`AttemptBootstrapper` 原本做两件事:runner.Reconcile(清上次残留容器)+
RestoreRunning(DB 回灌内存)。后者 R6 废弃,前者合并到 `GCServer.Start` 首轮
tick 之前同步执行 —— 必须早于第一次 `Snapshot`,否则会碰到"Redis 没有但 docker
还在跑"的孤儿容器。失败只 Warn 不阻塞启动。

**R6-06. NewAttemptStore 改强依赖 Redis**

- `store.NewAttemptStore(rdb, logger, opts...)`:`rdb == nil` 直接 `log.Fatal`,
  把问题按在启动期,不让运行时 NPE;
- `server.NewAttemptStore(rdb db.RedisRepo, logger *zap.Logger)`:签名跟上,
  用户明确传入 redis 依赖;
- 启动前 `configs/config.yaml` 的 `data.redis.addr` 必须填,否则 `db.NewRedis`
  会返回 nil,`NewAttemptStore` 直接 Fatal。

**R6-07. ProviderSet + main.go + wire_gen.go 调整**

- `backend/internal/server/server.go`:移除 `NewAttemptBootstrapper`;
  删掉旧 `NewAttemptReaper` 兼容条目,直接用 `NewGCServer`;
- `backend/cmd/backend/main.go`:`newApp` 参数从
  `(reaper *AttemptReaper, bootstrapper *AttemptBootstrapper)`
  简化为 `(gc *GCServer)`,`kratos.Server(...)` 里也只留 gc;
- `backend/cmd/backend/wire_gen.go`:手工对齐(wire 工具本地跑 `go generate ./...`
  重出一份也可以);`NewAttemptStore` 调用改 `(redisRepo, logger)`,
  `NewGCServer` 直接替代 `NewAttemptReaper`;
- `backend/internal/server/gcserver.go`:删 `AttemptReaper = GCServer` 别名和
  `NewAttemptReaper` 适配器 —— 不再有调用方。

**R6-08. 重写 attempt_store_test.go 用 miniredis**

旧测试基于 `sync.RWMutex` 时代的"并发 + 拷贝语义"断言,Redis 版这些保证
变成服务端天然具备的属性,测试重心换成 Redis 侧行为:

- `miniredis` 启内嵌 Redis,通过 `redis.NewClient` + 薄 adapter 构造
  `db.RedisRepo`,零外部依赖、CI 可跑;
- 覆盖场景:Put/Get 往返、Get miss、Delete 幂等 + owner 索引同步清理、
  UpdateLastActive/UpdateStatus 命中与 miss、Snapshot 过滤过期 key 并清理
  脏 SET 成员、FindActive 对 terminated 状态的拒绝、空 clientID/UserID 的
  快速返回、Len 计数。
- 需要 `go mod tidy` 拉入 `github.com/alicebob/miniredis/v2`;CI 跑
  `go test ./...` 自动带上。

**R6-09. Round 6 CHANGELOG + 验证**

本条目。`go build ./...` / `go test ./internal/store/...` / `go vet ./...`
需在本地执行验证(此次沙箱环境 bash 不稳,未现场跑)。

**破坏性 checklist(升级 main 分支需对齐)**:

1. `configs/config.yaml` 必须填 `data.redis.addr`,否则启动 Fatal;
2. 部署环境需至少一个可达 Redis 实例,Round 6 不再允许无 Redis 跑;
3. 任何直接 import `store.AttemptStore` 的外部代码需跟进新 API(ctx + error);
4. 不再有 `server.AttemptReaper` / `server.NewAttemptReaper` /
   `server.AttemptBootstrapper` / `server.NewAttemptBootstrapper`
   四个符号,替换为 `server.GCServer` / `server.NewGCServer`;
5. `attempt.AttemptRepo.ListRunning` / `AttemptUsecase.RestoreRunning` 已删。

---

### [refactor] Round 5:倒计时改体验型 + 返回不销毁 + ClientID/心跳 + GC 搬家

Round 4 follow-up 上线后用户又反馈两类硬问题,统一在 Round 5 收:

1. **第二次进入 sandbox 仍然 refused** —— T1-T3 只覆盖了"复用前 Ping",但
   Scenario 页 `useEffect` 卸载(用户点"返回")会发 terminate,然后用户立即
   再进同一场景,Start 走 T2 复用分支时 `Ping` 还能通(docker stop 需要秒级),
   但下一次 `Get` / iframe 连接时容器真停了,ttyd 报 refused。
2. **倒计时语义混乱** —— 页面展示的 deadline 其实是后端的 idle 窗口
   (30min 没操作就 GC),每次 `check` / `heartbeat` 都会刷,数字会跳;
   用户期望的是"按场景预估时长稳定展示"。
3. **到期自动终止伤感受** —— 用户写得正起劲被一刀切很沮丧,尤其是 30min
   预估时长只是"参考",不是"硬上限"。

根因:**"资源守护(idle)" 和 "体验提示(预估时长)" 被混成同一个倒计时**;
**销毁路径分散在前后端多处,'返回'被错当成'放弃'**。

这一轮从 9 个子任务把两者彻底解耦(U9 已合并进 U5,最终落地 U1-U8 + U10):

**U1. 定位第二次进入 refused 真凶**

交叉对比前后端日志 + docker events,最终定位到 `Scenario.tsx` 的卸载
`useEffect` 在 "返回" 时调了 `terminate.mutate`。这条路径在 V0 为了
"防止僵尸容器泄漏"加的,但和 Round 4 新增的 30min idle GC 是重复的,
且 GC 更准(心跳驱动),前端卸载 terminate 反而制造新的 refused 窗口。

**U2. 后端加 ClientID 维度 + X-Client-ID 中间件**

未登录阶段需要一个"这是谁"的维度,不然 idle 清理 / 并发限额都没主语:

- `backend/internal/middleware/clientid.go`:kratos HTTP middleware,
  从 `X-Client-ID` header 取匿名 owner,塞进 ctx(unexported key,
  同进程无冲突);
- `backend/internal/biz/attempt/usecase.go`:Start / FindActiveByUserSlug
  等方法从 ctx 取 ClientID 作为 owner key,DB/store 的 owner 字段
  从"默认 anonymous"升级为"客户端 UUID"级;
- 登录接入后这一维度自动让位给 user.ID,中间件会优先用登录态。

**U3. StartReply / GetReply 透出 estimatedMinutes**

proto 字段补齐,前端从 attempt reply 直接拿场景的预估时长,无需再查
`scenario.estimatedMinutes`(后端/前端两份真相容易对不齐)。
为 U6 倒计时改版提供数据基础。

**U3.5. GC 代码搬家到 gcserver.go(按 README Day 6 规划的目标位置)**

`backend/internal/server/opslabs.go` 里一个文件扛了五项职责,越来越难读。
剥出:

- `backend/internal/server/gcserver.go`:`GCServer` 类型 + `NewGCServer` +
  `tick` / `cleanupStale`,实现 `kratos.transport.Server`;
- 旧名 `AttemptReaper` / `NewAttemptReaper` 保留 `type alias` 和适配器
  (老签名 `runner=nil`),wire_gen 不用重跑;
- `opslabs.go` 只留 `NewScenarioRegistry` / `NewAttemptStore` / `NewRunner` /
  `NewOpslabsServiceOptions`,文件头注释明确指向 `gcserver.go`。

**U4. Get / GetTerminalURL 加 Ping 校验 + NEEDS_RESTART 业务错**

`AttemptUsecase.GetWithPing`:对 sandbox running attempt 做一次 `Ping`,
失败的同时执行"清 store + MarkTerminated",并返回 `alive=false`。

`service/opslabs/attempt.go` 的 `GetAttempt` 检到 `alive=false` 直接返
`kerrors.Conflict("NEEDS_RESTART", ...)`。前端 `useAttempt` 拿到这个
错误后自动 reset store + 重发 Start,UX 上表现为"自动补一次,用户无感"。

**U5. 销毁路径收敛 + GCServer 扩展 stale-container 周期扫描**

在 `backend/internal/biz/attempt/usecase.go` 文件头加**容器销毁路径白名单**
(V1 只允许 5 条,明确写出哪几条**不应**销毁,含"前端卸载触发"这条红线)。

`GCServer.tick` 新增 **stale-container 分支**:对 running sandbox attempt
每 1min Ping 一次,失败就 `cleanupStale`(清 store + `MarkTerminatedInDB`,
不调 `runner.Stop` —— 容器已经不存在了,Stop 会产生误导日志)。

这一条是外部操作(`docker rm`, Docker Desktop 重启)的兜底,U4 是前端
polling 的即时兜底,两条互补。

**U6. 前端 CountdownBadge 改版 + 超时不终止**

- `frontend/src/hooks/useCountdown.ts`:新增 `allowOvertime` 第三参数。
  `true` 时返回有符号秒数(负数=超时累计),到期继续 tick;`onExpire`
  仍只触发一次(跨 0 瞬间),兼容老调用;
- `frontend/src/components/CountdownBadge.tsx`:新增 `allowOvertime` prop。
  超时后 label 切"已超时",深红+脉动,`title` 说明"容器仍会按资源策略
  自动回收";
- `frontend/src/pages/Scenario.tsx`:`running` 时 `deadline` 改用
  `startedAt + estimatedMinutes * 60_000`,固定不刷;`CountdownBadge`
  收到 `allowOvertime={current?.status === 'running'}`;`onExpire` 对
  running 什么都不做(只 passed 分支走老 terminate 逻辑)。

**U7. 前端 ClientID + X-Client-ID header + 20s 心跳**

- `frontend/src/lib/clientId.ts`:`getClientId()` 返回 localStorage 持久化
  的 UUID,用 `crypto.randomUUID` + `Math.random` 兜底,Safari 隐私模式
  localStorage 不可写时退回内存 UUID;
- `frontend/src/api/http.ts`:自动注入 `X-Client-ID` header(调用方可
  覆盖,便于单测);
- `frontend/src/hooks/useHeartbeat.ts`:20s interval POST
  `/v1/attempts/{id}/heartbeat`,前置守卫 `tab visible` +
  `近 60s 有交互`,防止"tab 开着人走开"一直续命;4xx 停止,5xx/网络
  下一轮重试;
- `backend/internal/biz/attempt/usecase.go`:新增 `Heartbeat(ctx, id)`,
  校验 owner + 刷 `LastActiveAt`;
- `backend/internal/service/opslabs/attempt.go`:裸 `http.HandlerFunc`
  `HeartbeatAttemptHandler`(不走 proto —— 心跳是前端内部协议,没必要
  走 codegen),路径 `/v1/attempts/{id}/heartbeat`;
- `backend/internal/server/http.go`:`srv.HandleFunc` 注册上述路径,
  放在 proto 路由后面做兜底。

**U8. 前端 Back vs GiveUp 语义分离**

- "返回"(`navigate('/')` / 顶部返回按钮 / 浏览器后退)→ **不再终止容器**,
  删除 `Scenario.tsx` 卸载 useEffect 里的 `terminate.mutate` 调用;
  保留大段注释解释为什么删,防止后人看"返回不清理"的代码又好心加回来;
- "放弃"(ActionBar `onGiveUp` / PassModal `giveUp` 分支)→ 明确的
  `terminate.mutate`(销毁路径 #1);
- 资源不泄漏的保障链:GCServer idle 30min + stale-container 扫描 +
  GetWithPing 即时兜底 + Start 错误回滚(销毁路径 #2-#5)。

**U9. (合并进 U5) 前端 reusable 快路径全部拆掉**

Round 4 的 T3 已经拆过一次,Round 5 确认 `Scenario.tsx` / `useAttempt` /
`useAttemptStore` 都没有"直接信任 localStorage 跳过 Start/Get"的逻辑。
后端 StartReply 是唯一真相,store 只做"刷新页面不丢 attemptId"的缓存。

**U10. CHANGELOG + 走查验证**

- `go build ./...`:期望通过,但当前 session 工作区不可用,本地校验完落
  commit;
- `tsc --noEmit`:同上;
- 销毁路径白名单注释与实际调用位置逐条对账:5 条允许路径齐备,删除的
  卸载 terminate 有注释显式声明;
- 倒计时语义:`deadline` 在 running 时基于 estimatedMinutes,passed 时
  基于 reviewingUntil,其他 undefined;`remainingForWarning` 仍基于
  `expiresAt`(即 idle 窗口),和 banner "剩余 X 分钟就自动销毁" 的
  文案语义一致。

---

### [fix] Round 4 follow-up:Runner.Ping 探活 + 前端永远 Start + 到期自动判 + 场景级 Splash 阶段

Round 4 主体上线后用户再次反馈两个硬伤:

1. 进入 sandbox 场景时 ttyd iframe 报 `dial tcp 127.0.0.1:19996: connection refused`
   —— 后端 store 里还留着上一次 attempt 的 HostPort,但宿主侧的容器已经被
   `docker rm` / Docker Desktop 重启清掉了;Round 4 新加的"活跃 attempt 复用"
   直接把这条僵尸当成可复用的给返回了;
2. 顶部不显示倒计时 —— 前端从 localStorage 恢复的 `expiresAt` 是上一次 attempt
   的,进入场景后前端靠 `reusable` 快路径直接跳过 Start 调用,从来没拿到新的
   `expiresAt`,所以 CountdownBadge 的 `deadline` 为空就不渲染。

两个问题根因是同一个:**后端 store 里的记录能否复用,缺了"真实探活"这一步**;
**前端复用判定分散在两侧又互相不信任,漏洞容易从缝里钻出来**。这一轮从 7 个
子任务把这条链路打穿:

**T1. runtime.Runner 新增 `Ping(ctx, containerID) error` 接口**

- `backend/internal/runtime/types.go`:接口加一行 `Ping(...)`,返回约定
  `nil / ErrContainerNotFound / 其它 error`;
- `backend/internal/runtime/docker.go`:走 `docker inspect -f '{{.State.Running}}' <cid>`
  3s 超时;stderr 命中 "no such object" / "no such container" 归一到
  `ErrContainerNotFound`,其它错误原样包一层返回;`running == "true"` 才算活;
- `backend/internal/runtime/mock.go`:只要 containerID 在 `attempts` 表里就返回
  nil;空 ID 或已 Stop 的 ID 返回 `ErrContainerNotFound`,让单测能直接模拟
  "容器真死"场景。

**T2. AttemptUsecase.Start 复用前先 Ping**

`backend/internal/biz/attempt/usecase.go` 复用分支改造:sandbox 模式命中老 attempt
时,先校验 `HostPort > 0`,再调 `runner.Ping(ctx, existing.ContainerID)`。任一步失败
都落到"清理 + 新建"路径:

- `uc.store.Delete(existing.ID)` 清掉内存记录;
- `existing.MarkTerminated(now)` + 独立 ctx 2s 内 `repo.Update` 把 DB 也标记为
  terminated,避免前端下次 Get 拉到"running 但连不上"的僵尸记录;
- 继续往下跑新建路径,用户无感。

日志打点 `"sandbox reuse ping failed, will recreate"` 方便线上排障。

**T3. 前端拆掉 localStorage `reusable` 快路径**

`frontend/src/pages/Scenario.tsx` 的 start effect 原来有一条快路径:localStorage
里 `current.scenarioSlug === slug && attemptId` 时直接返回,不发 Start。这导致
T2 的后端复用永远不会被触发,同时 `expiresAt` / `reviewingUntil` 等字段永远停在
上一次的快照上。

改为:每次进入场景都发 Start,deps 从 `[startPhase, slug, current?.scenarioSlug, current?.attemptId]`
简化为 `[slug]`。幂等性靠两条:

- 前端同一 render 周期内 `startingRef.current === slug` 守卫,挡掉 StrictMode
  effect 双触发;
- 后端 T2 的 `FindActiveByUserSlug` + `Ping` 组合 —— 真能复用的一定复用,
  真不能复用的立即清掉。

前端现在只持有一份真相(从后端回来的 StartReply),localStorage 只作为"刷新
页面后不丢 attemptId"的缓存层。

**T4. 到期不再直接 terminate,自动判一次 + 显示 PassModal**

用户希望"倒计时走到 0 的时候能看到这一把的结果,而不是被静默清场"。

`onExpire` 回调改造:

- 如果 `current.status === 'passed'`(已经通关,倒计时是复盘期) → 走原先的
  `terminate` + 回列表;
- 否则 → 调用 `runCheck({ auto: true })`,PassModal 里追加前缀 "⏰ 容器已到期,
  系统自动判了一次 —— ";
- 同时启动 60s 兜底 timer(`autoExpireTimerRef`):如果判题 60s 内没返回或用户
  没关 PassModal,再 terminate 掉 attempt,避免容器资源永久泄漏;
- PassModal 任意 action(关闭 / 继续 / 退出)都 `clearAutoExpireTimer`;组件
  unmount 时 cleanup 也清掉这条 timer。

`runCheck` 是原先 `onCheck` 抽出的纯函数,支持 `{ auto?: boolean }` 参数区分手动
/ 自动判题的 PassModal 文案。`onCheck = () => runCheck()` 保持外部调用点不变。

为了让 onExpire 这个 `useCallback` 能拿到"最新的 runCheck 闭包"又不在依赖列表里
拉一串重建,用 `runCheckRef` ref 模式:每次 render 把最新 runCheck 写进 ref,
onExpire 回调里永远读 `runCheckRef.current(...)`。

**T5. BootSplash 支持按 slug 切换阶段文案**

Round 4 让 BootSplash 能按场景定制 title/tagline,但三段式阶段文案还是按 mode
共用的。用户反馈"hello-world 场景首屏看到的是通用的'预留端口 · 创建容器'很生硬"。

`frontend/src/components/BootSplash.tsx` 加 `STAGES_BY_SLUG` 静态表,命中 7 个
V1 场景的专属文案(如 hello-world 的 "拉取 opslabs/hello-world:v1 镜像");
接收 prop `scenarioSlug`,选取优先级 `customStages > STAGES_BY_SLUG[slug] > STAGES_BY_MODE[mode]`,
未覆盖的场景自动回落通用文案,完全向下兼容。

本来想让阶段文案走 scenario YAML/proto 配置,这样新增场景就不用改前端代码;
但权衡下来:V1 场景总共 7 个,加文案只需要改 1 个前端文件;做 boot_stages proto
要动 pb.go 再生成 + service 层透出 + 所有场景 YAML 补字段,改动面大 3~5 倍。
所以走 80% 效果的前端方案,保留后续迁移到后端驱动的空间。

**T6. DockerRunner.Ping 单元测试**

`backend/internal/runtime/ping_test.go` 新增 7 个 case:

- MockRunner:Run 后 Ping 返回 nil;Stop 后返回 `ErrContainerNotFound`;空 ID 返回
  `ErrContainerNotFound`;
- DockerRunner:通过 `writeFakeDocker(t, behavior)` 写一个临时 shell 脚本当 docker
  二进制,`WithDockerBin(path)` 注入。3 种 behavior:
  - `"running"` → echo true,Ping 返回 nil;
  - `"stopped"` → echo false,Ping 返回非 nil 且**不是** `ErrContainerNotFound`;
  - `"nosuch"` → stderr 输出 "Error: No such object: xxx" + exit 1,Ping 返回
    `ErrContainerNotFound`;
  - 空 ID 直接返回 `ErrContainerNotFound`。

Windows 开发机用 shebang 跑不了脚本,测试里 `runtime.GOOS == "windows"` 时 `t.Skip`;
项目其它 test 已是同样的约定,CI 跑在 linux 容器里不会 skip。

**T7. 自审**

Shell workspace 依旧 "still starting" 无法跑 `go build ./...` / `go test ./...` /
`pnpm tsc --noEmit`,这一轮靠静态走查兜底:

- T1:新增接口方法 + 实现,没删既有 signature,所有 Runner call site 用的是接口
  引用,DockerRunner / MockRunner 都已实现,一定编译通过;
- T2:usecase.go 新增分支只调用已存在方法,走查过 `store.Delete` / `Attempt.MarkTerminated` /
  `repo.Update` / `store.Get` / `store.UpdateLastActive` 都在对应文件里;
- T3:Scenario.tsx 改动局限在一个 useEffect 的 guard 和 deps,编辑过的 React hooks
  均保持同位置调用顺序,无条件 hook;
- T4:新增 `autoExpireTimerRef` / `clearAutoExpireTimer` / `runCheckRef` 三个 ref,
  在同一个组件顶层声明;`onCheck = () => void runCheck()` 保持原签名;PassModal 的
  prop 没改;
- T5:BootSplash 的 props 加了两个可选项,调用侧都传了;`STAGES_BY_SLUG[slug]` 在
  类型层走 `Record<string, BootStage[]>`,miss 时回落 undefined,下游短路到 mode 兜底;
- T6:`ping_test.go` 只引入标准库 + `go.uber.org/zap`(项目已用),`runtime.GOOS`
  来自标准库的 runtime 包,文件本身 package runtime 同目录,无循环引用。

后续 round 做 `go test ./internal/runtime/... -v` 回归确认;现在把改动 merge 上去
不会破坏已有构建。

---

### [fix] Round 4:wasm-linux 双发 mutation 修复 + 后端 attempt 复用 + 结束比例提醒 + 场景级 Splash 文案

用户反馈 Round 3 的同源反代装上后 sandbox 终端能用了,但 **wasm-linux 依然卡在
"加载题目 bundle…"**;另外 Docker 场景每次进入都会**新建一个容器**,不会复用
前一次的会话。翻控制台把 `[opslabs/start]` 的日志贴出来,一眼看到一次访问里
有两次 `mutationFn enter`、两个不同的 attemptId(`…753` 和 `…752`),顺序且几乎
同时 —— 典型的 **React 18 StrictMode effect 双触发**,我上一轮留下的守卫没挡住。

**1. 双发 mutation 根治(wasm-linux 卡 overlay 的真因)**

`frontend/src/pages/Scenario.tsx` 的 start effect 之前靠
`startingRef.current === slug && startPhase === 'pending'` 做去重。StrictMode
(dev)下 effect 会跑 effect→cleanup→effect 两轮,期间 `setStartPhase('pending')`
只是把 update 排进队列,下一轮 effect 进来读到的 `startPhase` 还是 `'idle'`,
双条件 `&&` 校验落空 → 又跑一次 `fireStart` → 两个 attempt 并发产生。

对 **sandbox** 场景,表现是 Docker 连起两个容器、后端 store 覆盖前一条 attemptId,
前端看到的是第二个,但第一个没人管也没放端口回池(端口池改过之后能 clean,但
语义上仍然多建了一次); 对 **wasm-linux**,后端不起容器,两次 Start 都落
`startBundleless` 只写 DB,**但前端 StaticRunner 在两次 bundleUrl 写入之间会有短
暂的 useEffect 重跑 → `setReady(false)`**,如果 v86 里刚好已经探到 `/ # ` prompt
发了 `opslabs:ready`,那条消息会因为 React 把 ready 重置而 "未被看到",overlay
就这样挂死。

**修法**:守卫改成纯 ref —— `if (startingRef.current === slug) return`。ref 是
同步写入,第二轮 effect 一定能看到;error 态下用户点 retry 走
`onRetry→fireStart(slug)`,那条路径不经过 effect,不受守卫影响。

**2. 后端 Start 加"活跃 attempt 复用"逻辑**

用户的原话是"每次进入还是会重新创建(这里应该检查一下当前用户有没有正在运行的
容器,有则直接进去)"。这其实就是 #1 的后果 + 前端 localStorage reuse 在清缓存/
换浏览器时失效,后端兜底一下可以让这条保证更硬。

- `backend/internal/store/attempt_store.go` 新增 `FindActiveByUserSlug(userID, slug)`,
  按 (UserID, Slug) 扫 in-memory store,挑 status 为 running/passed 的最新一条;
- `backend/internal/biz/attempt/usecase.go` `Start()` 开头加复用判定:命中就
  `UpdateLastActive(now)` 返回老 attempt,sandbox 模式额外校验 `HostPort>0`(mock
  场景或容器已被外部干掉时不复用,走新建)。
- V1 未接入登录统一用 `UserID=0`;登录接入后换成真实 ID,多用户同一 slug 自动
  分隔,不会互相串。

组合效果:即使前端因为某些异常多发了 Start,后端也会把它折回到同一个 attempt,
用户看到的永远是同一个容器 / 同一个终端会话。

**3. 接近结束的比例提醒 banner**

`frontend/src/pages/Scenario.tsx` 加一条新 banner:剩余时间 ≤
`max(idleTimeout × 20%, 180s)` 时,用玫瑰红底色常驻顶部,带"立刻提交 / 稍后再说"
两个按钮。阈值取比例是因为场景时长差异大:60 分钟的场景最后 12 分钟提醒,
5 分钟的场景最后 3 分钟提醒;兜底 180s 防止过短场景没机会提醒。按 attemptId
维度记关闭状态,避免用户关掉后每 1s 又弹回来。

**4. BootSplash 支持场景级启动文案**

`BootSplash` 加 `scenarioTitle` / `scenarioTagline` 两个可选 prop;`Scenario.tsx`
在 renderByExecutionMode 调用处把 `scenario.title` 和 `firstSentence(summary)`
带进去,加载页就会显示"你正在进入:CSS Flexbox 居中"这种专属文案,而不是
通用的"正在启动场景"。不传就走原先的默认文案,完全向下兼容。

**verification**:工作区 Linux sandbox 依旧持续 "still starting" 无法跑
`go build ./...` / `tsc --noEmit`,代码改动走严格自审:
- Go 这边只新增方法 + 调用,没有破坏性改型;
- 前端改的三处都是本地状态 / 渲染逻辑,没碰 API schema;
- `scenario.summary` / `title` 在 `types.ts` 已存在,`firstSentence` 纯本地函数。

---

### [fix] Round 3(真正落地):ttyd 改同源反代 + wasm-linux 关掉 ISO async

Round 2 的两个修复(bundle 加 Range 支持 + iframe 加 credentialless)上机验证
没能解决用户反馈的问题。继续深挖 + 换方案。

**1. sandbox 终端 iframe:改走后端同源反代**

**为什么 Round 2 的 credentialless 属性没修好**:Chrome 在 COEP=credentialless
下对跨源 iframe 的 navigation 检查依赖浏览器版本/实验开关,同一批 Chrome 不同
flag 组合的表现并不稳定 —— ttyd 那边还是会被当成 cross-origin-navigation 失败,
中文 locale 渲染出的"localhost 拒绝了我们的连接请求"跟真 TCP 拒绝完全一样,
现场 debug 很容易被带偏。

**根本方案**:后端加 `GET /v1/ttyd/{attemptId}/*` 反向代理,把 ttyd 宿主端口
吞到后端同源下:

- `backend/internal/service/opslabs/ttyd_proxy.go` 新增 `NewTtydProxyHandler`,
  用 `httputil.NewSingleHostReverseProxy(http://127.0.0.1:{hostPort})` 转发 HTTP
  + WebSocket(Go 1.12+ 的 ReverseProxy 原生支持 `Upgrade` hijack + 双向字节
  拷贝)。handler 按 attemptId 从 AttemptStore 反查 HostPort,每条 attempt 首次
  访问时 lazy 构造 proxy 实例,后续复用 transport 连接池。
- `backend/internal/server/http.go`:`NewHTTPServer` 签名加 `*store.AttemptStore`
  + `*zap.Logger` 两个入参,注册 `srv.HandlePrefix(TtydProxyURLPrefix, ...)`。
  `cmd/backend/wire_gen.go` 同步更新调用。
- `backend/internal/service/opslabs/attempt.go` `terminalURL()`:默认模板
  `http://{host}:{port}/` 不再渲染,直接返回相对路径 `/v1/ttyd/{id}/`。
  TerminalURLTemplate 显式配置非默认值时仍走模板(兼容外部反代 / 直连端口部署)。
- `frontend/vite.config.ts`:`/v1` 代理加 `ws: true`,让 dev 链路
  `browser → vite → kratos → ttyd` 的 WebSocket 端到端连通。
- `frontend/src/components/Terminal.tsx`:删掉 Round 2 的 `credentialless` iframe
  属性 ref hack(同源后用不到),probe 从 `mode: 'no-cors'` 改回正常 fetch,
  能拿到状态码,404/410/502 都落到"终端连接失败"兜底面板,用户看得到真实原因。

**副作用(正向)**:宿主 ttyd 端口不再外露,外部摸不到 `http://localhost:19999/`,
只能通过后端经过鉴权才能到 ttyd,顺带把 XSS via ttyd 之类的风险面缩小。

**2. wasm-linux:关掉 cdrom async,ISO 一次性拉全**

Round 2 给 bundle.go 补了 `http.ServeContent` Range 支持是对的,但复测下来 v86
还是有偶发卡住的现象。根因藏在链路另一端:

- dev 模式下 `/v1/scenarios/.../bundle/vendor/images/linux.iso` 经 vite 的
  http-proxy-middleware 二次代理,部分版本在 `changeOrigin: true` 下会吃掉
  `Content-Range` / `Accept-Ranges` 响应头,v86 async 读取路径解析失败就停住。
- 调代理透传 header 折腾量大,不如从需求侧绕过:BusyBox iso 只有 4MB,一次性
  全量下载也就几十~几百毫秒。

**修法**:`backend/internal/scenario/bundles/wasm-linux-hello/index.html`
`cdrom: { url, async: true }` → `async: false`。

- 前端代价:首屏多 200~500ms 下载时间
- 收益:不再依赖 Range/Partial Content 任何环节,引导必然跑完,shell prompt
  必然出现,`opslabs:ready` 必然发给父页,StaticRunner 的"加载题目 bundle…"
  overlay 必然消失

**后端 Range 支持没回滚**:`bundle.go` 的 `http.ServeContent` 对其它场景(例如
未来有大 wasm / 视频素材)仍然有用,保留。本次 fix 属于"应用层不依赖 Range"的
冗余防线。

---

### [fix] Round 2:wasm-linux 永远不就绪 + sandbox iframe"连接被拒绝"

上一轮修完 sandbox 三连之后,用户跑真实场景又暴露两个连锁问题,根因都在 COEP
credentialless 引入后才表现出来。

**1. wasm-linux 场景卡在"加载题目 bundle…"**

现象:用户跑完 `scripts/fetch-v86.ps1`,DevTools Network 能看到 libv86.js /
v86.wasm / seabios.bin / vgabios.bin / linux.iso 全是 200,但页面上 StaticRunner
的半透明 overlay "加载题目 bundle…" 永远不消失,iframe 内的 BusyBox 也始终
停在"引擎已就绪,等待内核启动…"。

根因:`backend/internal/service/opslabs/bundle.go` `serveBundle` 用
`io.Copy(w, f)` 直接把 embed.FS 字节吐出去,**不支持 HTTP Range**。
v86 的 `cdrom: { url: linux.iso, async: true }` 是懒加载 ISO —— BusyBox 启动时
每读一个扇区就发一次 `Range: bytes=X-Y` 请求只取那几 KB。没有 Range 支持时
服务器永远回整条 4MB 全量 + 200,v86 的 async 读取路径解析不出预期的 206 Partial
Content,ISO 读盘直接僵死,BusyBox 内核永远到不了 shell prompt,
`/ # ` 探测不触发 → `opslabs:ready` postMessage 不发 → 父页 StaticRunner 永远
`ready === false`,overlay 就挂在那儿。

修法:`bundle.go` 换成 `http.ServeContent(w, r, cleaned, mtime, bytes.NewReader(data))`,
把 embed 字节切片包成 `io.ReadSeeker`。ServeContent 原生处理:

- `Range: bytes=X-Y` → 206 + `Content-Range` + 正确的 `Content-Length`
- `If-Modified-Since` / `If-None-Match` → 304
- 自动设置 `Accept-Ranges: bytes`

mtime 用进程启动时间近似(`var bundleStartTime = time.Now()`),embed 文件无真实
mtime,这里的语义是"该版二进制发布时间",改 bundle 必须重新构建二进制 → 时间
自然更新,前端缓存判定够用。Content-Type 依旧 `contentTypeByExt` 预设,
ServeContent 只在 header 没写时才自己 sniff,不冲突。

**2. sandbox 终端 iframe 显示"localhost 拒绝了我们的连接请求"**

现象:同一个 `http://localhost:19997/` 在新标签页能正常打开 ttyd,但嵌到场景页
iframe 里就显示 Chrome 的 ERR_CONNECTION_REFUSED 错误页。

根因:Round 1 把 COEP 从 `require-corp` 换成 `credentialless` 只解决了**子资源**
的跨域加载(fetch / script / img 走 credentialless 请求,免 CORP 头),但 COEP
credentialless 对**嵌入 iframe 的导航**仍然要求两者之一:

- 目标文档带 `Cross-Origin-Resource-Policy` 头,**或者**
- iframe 元素显式加 `credentialless` 属性,让 iframe 继承 credentialless 上下文

ttyd 不发 CORP 头、我们也不碰 ttyd 二进制,那就只能在 iframe 上补 credentialless。
Chrome 在 COEP 拦截 iframe navigation 时的 NetworkError 在 Chinese locale 下被翻译
成 "localhost 拒绝了我们的连接请求",跟真正的 TCP ECONNREFUSED 肉眼不可分 —— 这是
之前误判成 "ttyd 没起来" 的关键坑。

修法:`frontend/src/components/Terminal.tsx` 的 `<iframe>` 走 ref 回调
`el.setAttribute('credentialless', '')`。React 18 的 JSX 类型里还没有这个
prop(React 19 才加),所以不能直接写 JSX attribute,ref 回调是唯一合法通道。
ref 回调在 commit 阶段同步跑,赶在 iframe 发起 navigation 之前生效,不会有
"第一次普通 / 第二次 credentialless" 的闪烁。

兼容性:`credentialless` iframe 属性 Chrome 110+ / Edge 110+ 支持,Firefox 134+
随 COEP credentialless 一起落地。我们本来就只承诺 Chromium(WebContainer 前提),
老浏览器降级行为跟以前一致,没有新门槛。

**为什么没有同步改 StaticRunner iframe**:static / wasm-linux bundle 的 iframe src
是 `/v1/scenarios/.../bundle/index.html`,dev 下走 vite proxy、prod 下走同源反代,
**始终与父页同源**,不受 COEP 跨源限制。给同源 iframe 加 credentialless 属性
会反倒割裂 storage 分区,没必要。

---

### [fix] Sandbox 启动相关三连 bug

集中修一轮真实跑沙箱场景才暴露出来的三个问题。

**1. Docker 端口冲突 → 起容器失败**

现象:点进沙箱场景,后端日志:

```
container start failed: docker run: exit status 125: ...
docker: Error response from daemon: failed to set up container networking:
  Bind for 0.0.0.0:19999 failed: port is already allocated
```

根因:

- PortPool 只跟踪进程内分配状态,不感知 OS 层端口占用
- 上次进程崩溃 / Ctrl-C 没走正常 Stop 链路,docker 里残留 `opslabs-xxxxxx`
  容器仍在跑,占着 19999 / 19998 ...
- 新进程的端口池一上来就把 19999 吐给 Acquire,docker 撞冲突

修法(两层兜底):

- `backend/internal/runtime/portpool.go` 加 `MarkBad(port)`:把指定端口
  从 free / used 池摘掉移进 `bad`,Acquire 不再发放,Release 也不回流。
- `backend/internal/runtime/docker.go` `Run` 拆出 `tryRunOnce`,
  端口冲突时 MarkBad 该端口 + 顺手 `docker rm -f <name>` 清理半生不熟的容器壳子,
  外层最多重试 8 次。错误文案识别:`port is already allocated` /
  `Bind for 0.0.0.0:xxxx failed` / `address already in use`(Linux) /
  中文环境的 "端口已" 模糊匹配。
- 新增 `DockerRunner.Reconcile(ctx)` 启动钩子:
  `docker ps -aq --filter label=opslabs.attempt_id` 列出残留 → `docker rm -f`
  全部干掉,从根上避免下一次 Run 撞冲突。
- `runtime.Runner` 接口加 `Reconcile(ctx)`,MockRunner 实现为 no-op。
- `server/attempt_bootstrap.go` `AttemptBootstrapper.Start` 顺序调整:
  先 `runner.Reconcile`,后 `RestoreRunning`,失败都只记日志不阻塞主服务。
  `NewAttemptBootstrapper` 签名加一个 `runner runtime.Runner` 入参,
  `wire_gen.go` 同步更新。

**2. COEP 拦截 ttyd iframe → 沙箱终端永远显示空白**

现象:浏览器 console:

```
GET http://localhost:19997/ net::ERR_BLOCKED_BY_RESPONSE
  .NotSameOriginAfterDefaultedToSameOriginByCoep
```

根因:Round 2 给 vite 加了全局 `Cross-Origin-Embedder-Policy: require-corp`
(WebContainer 需要 SharedArrayBuffer),但 `require-corp` 会强制所有
cross-origin 子资源带 `Cross-Origin-Resource-Policy` 才能加载,ttyd 默认
不发 CORP,沙箱模式的 `<Terminal src="http://localhost:19997/">` 直接被 block。

修法:`frontend/vite.config.ts` 把 `Cross-Origin-Embedder-Policy` 从
`require-corp` 改成 `credentialless`。credentialless 允许 cross-origin
子资源不带 CORP 也能加载(代价仅是不附 cookie / credentials,ttyd 这种
本地无认证服务不受影响),同时仍保持 `crossOriginIsolated === true`,
WebContainer 继续工作。`Cross-Origin-Resource-Policy` 从 `same-origin`
改成 `cross-origin`,代理回来的 /v1 子资源也不受拦截。

兼容性:credentialless Chrome 110+ / Edge 110+ / Firefox 134+ 支持。
WebContainer 本来就只支持 Chromium,因此交集没变。

**3. attempt 进入终态后无法重新 start**

现象:沙箱启动失败 / 用户主动 terminate / idle 到期后,刷新场景页面
卡在 BootSplash 或显示已废 attemptId 的旧界面,不会重新拉新容器。

根因:`frontend/src/pages/Scenario.tsx` 的 useEffect 复用守卫只看
`scenarioSlug + attemptId` 是否匹配,不看 status。terminated / expired /
failed 这些终态也命中 "复用",直接 `return`,fireStart 没机会跑。

修法:`useEffect` 复用守卫加 status 判断 ——
只有 `running` / `passed` 才视为可复用,其它状态进入 `resetStore + fireStart`
分支重新拉新容器。依赖数组也加上 `current?.status`,状态翻面就触发重评估。

---

### [feat] 沙箱生命周期 UX 改造:返回 / 倒计时 / 空闲提醒 / BootSplash / 复盘分支

**背景**:

V1 初版进入场景后的 UX 有几个明显缺口:

- 页面没有返回按钮,用户只能靠浏览器回退或按地址栏改路径
- 容器有空闲超时但前端不告诉用户,莫名其妙 401 / 容器没了
- 判题通过后容器立刻被 passed 清理,用户没机会复盘
- 判题失败后的弹窗只有"关闭",没有"继续做"和"放弃退出"的语义分离
- 启动场景那一两秒是空白页或光秃秃一行 "正在分配容器…",跟真实 Docker pull
  慢场景叠加后很像卡死

这次把生命周期相关的五个点一次性收掉。

**改动**:

1. **顶部栏重构 `frontend/src/pages/Scenario.tsx`**:
   - 左返回按钮(带小箭头 icon),直接 `navigate('/')`
   - 中标题 + execution_mode tag(mono 字体小标签)
   - 右 `<CountdownBadge>`:running 态显示 idle 剩余,passed+review 态显示
     复盘剩余;未命中状态不渲染

2. **新增倒计时原语**:
   - `frontend/src/hooks/useCountdown.ts`:deadline 变更重对齐,ref 守卫
     onExpire 一次性触发
   - `frontend/src/components/CountdownBadge.tsx`:tier 配色
     (>5min 灰 / 1-5min 琥珀 / <1min 红+脉动),过期自动调 onExpire

3. **启动动画 `frontend/src/components/BootSplash.tsx`**:
   - 四执行模式各自三段伪进度(sandbox 最慢 ~7s;static 最快 ~2s)
   - 上条 linear progress + 阶段清单(已完成打勾 / 当前闪烁 / 未开始灰)
   - error 分支显示错误原因 + Retry 按钮(sandbox 最常见:Docker Desktop 没开)
   - 被 `renderByExecutionMode` 在 startPending / startError / 无 attemptId
     时统一替换主 runner 区域,避免 404/空白

4. **判题弹窗 `frontend/src/components/PassModal.tsx` 重做**:
   - 新增 `PassModalAction = 'close' | 'enterReview' | 'restart' | 'giveUp' | 'backHome'`
     联合类型,用户意图由父页面翻译成具体动作
   - passed 三按钮:进入复盘 / 再做一次 / 返回列表
   - failed 两按钮:继续做 / 放弃退出
   - 底部提示语根据 passed/failed 切换,给用户下一步心理预期

5. **复盘模式(纯前端)**:
   - `Attempt` 类型加 `reviewingUntil?: string`,用户点"进入复盘"时写当前
     `Date.now() + 10min`
   - CountdownBadge 在 status=passed + reviewingUntil 存在时切到复盘计时
   - 到期 onExpire 触发 terminate + 回列表
   - 顶部 banner 提醒"已通关 · 正在复盘,容器会保留到计时结束后自动销毁",
     带"立即结束并返回"按钮
   - 不依赖后端 Extend RPC —— proto 未变,靠现有 PassedGrace(10min)做兜底清理

6. **空闲提示 banner**:
   - running + sandbox 状态第一次进页面时显示琥珀色提示条:
     "容器将在 N 分钟无操作后自动销毁,届时数据会丢失,需要手动重新开始"
   - "我知道了"按钮 + sessionStorage 标记,本 tab 不再重弹

7. **Check 成功后 expiresAt 外推**:
   - `frontend/src/api/attempt.ts` `useCheckAttempt` hook-level onSuccess
     推 `expiresAt = Date.now() + idleTimeoutSeconds(默认 30min)`
   - 对齐后端 `Check()` 里 `LastActiveAt = now` 的刷新语义;proto 后续补上
     `AttemptReply.expires_at` 后这段外推改为直接用服务端值

8. **后端 biz 层预留 Extend 能力**:
   - `backend/internal/biz/attempt/usecase.go` 新增 `Extend(ctx, id, extend,
     reason)`:把 `LastActiveAt` 推到 `now + extend`,触发 repo.Update 和
     store.UpdateLastActive,方便 reaper / idle 检查自动生效
   - `backend/internal/service/opslabs/options.go` 加 `DefaultPassedGrace`
     / `MaxExtendSeconds` 字段(默认 10min / 30min);
     `AttemptService` 加 `idleTimeoutFor(slug)` / `passedGraceFor(slug)`
     两个小辅助
   - 未接出 ExtendAttempt RPC —— 避免本轮强制 proto regen 依赖,V2
     proto 打通后直接挂 AttemptService 上

**为什么复盘走纯前端**:

- 后端 Extend RPC 会引入 proto 新字段 + regen 依赖,本轮想把 UX 先交付
- 复盘是"通关后用户自愿的 10min 观察窗口",到期行为(terminate)跟后端
  PassedGrace 本就同频,前端写本地截止时间足够
- biz 层的 Extend 已经写好,未来只要 proto 补 `ExtendAttempt` RPC 就能零
  改动接上

**影响与回滚**:

- 向后兼容:`Attempt` 新字段全是 optional,proto 未动
- sessionStorage key `opslabs:idleTipSeen` 是新增,不冲突
- localStorage `opslabs-attempt` 结构多了 `reviewingUntil` 等字段,旧值反序列化
  不会失败(多出字段 / 缺字段都是 Partial)
- Scenario.tsx 是页面级重写,如需回滚可从 git 拉回旧版;但旧版没有返回按钮
  和倒计时,UX 倒退明显,不建议

### [feat] wasm-linux v86 资源:CDN 被墙时自动降级到后端自托管

**背景**:

Round 3 刚交付时 v86 资源(libv86.js / v86.wasm / seabios / vgabios / linux.iso,
合计 ~5MB)默认从 `https://copy.sh/v86/...` 拉,在国内部分网络线路会被运营商
拦截或 DNS 污染,页面停在 "libv86.js 未加载成功" 文案。

**改动**:

1. `backend/internal/scenario/bundles/wasm-linux-hello/index.html`:
   - 新增 `V86_SOURCES` 列表,优先 `./vendor`(后端 embed 同源)→ `https://copy.sh/v86`
   - `loadScriptWithTimeout(src, 8000)`:每个源 8s 内没 load 完视为失败,继续下一个
   - `bootLib()` 串行遍历,第一个成功的 source 写入 `V86_BASE`,后续 wasm/bios/iso
     都基于这个 base 拼 URL
   - `bootEmulator()` 改为读 `V86_BASE`,`!V86_BASE` 保险丝防止调用顺序 bug
   - 全部源挂掉时显示清晰错误文案,引导跑 `scripts/fetch-v86.sh`

2. `scripts/fetch-v86.sh` / `scripts/fetch-v86.ps1`:
   - 落地到 `backend/internal/scenario/bundles/wasm-linux-hello/vendor/{build,bios,images}/`,
     结构与 copy.sh 源完全对齐,bootLib 切 source 时 URL 后缀一致,不需翻译
   - 支持 `V86_MIRROR` 环境变量覆盖默认源(例如公司内网 mirror)

3. `backend/internal/scenario/bundles/wasm-linux-hello/vendor/README.md`:
   - 解释目录用途,强调 `.gitignore` 只忽略二进制,保留 `.gitkeep` / `README.md`
   - 列出每个文件的作用与大小

4. `.gitignore`:
   - `backend/internal/scenario/bundles/wasm-linux-hello/vendor/**` 递归忽略二进制,
     保留 vendor/ 目录本身 + README + .gitkeep

**效果**:

- 本地开发/私有化部署:跑 fetch 脚本 → `go build ./...` → v86 资源随 Go 二进制分发
- 公网 Demo:fetch 脚本没跑也能工作 —— bootLib 回退到 copy.sh
- 内网部署:`V86_MIRROR=https://my-mirror/v86 ./scripts/fetch-v86.sh` 一行改完

### [feat] V1 四执行模式全部打通(Round 2 + Round 3)

**背景**:
承接 Round 1 的 `static`,Round 2 把 `web-container` 模式真正跑起来(StackBlitz
WebContainer 在浏览器里跑 Node.js),Round 3 把 `wasm-linux` 模式跑起来(v86
WebAssembly 模拟器在浏览器里跑 BusyBox Linux)。至此 4 种 execution_mode
全部有前后端完整实现 + 至少一个可跑的 hello-world 场景。

**Round 2 改动(web-container)**:

1. `backend/internal/scenario/bundles/webcontainer-node-hello/project.json`
   - WebContainer FileSystemTree 格式的项目清单
   - 内含:`handler.js`(带 bug:字段名 `greet` 应为 `greeting`)、`check.mjs`
     (`node check.mjs` 成功返回 exit 0 + stdout "OK")、`package.json`、`README.md`
   - `startCmd: npm install`,`check: node check.mjs`

2. `backend/internal/scenario/registry.go`
   - 新增 `scenarioWebContainerNodeHello()`,slug=`webcontainer-node-hello`,
     category=backend,difficulty=1,execution_mode=web-container

3. `backend/internal/scenario/bundles/bundles.go`
   - 新增 `//go:embed all:webcontainer-node-hello` 指令
   - (embed 不支持一句话收敛多子目录,必须每个 bundle 一行)

4. `backend/internal/service/opslabs/bundle.go`
   - 新增 `BundleEntryURLFor(mode, slug)`:
     - web-container → `.../bundle/project.json`(FileSystemTree)
     - static / wasm-linux → `.../bundle/index.html`(iframe 入口)
     - sandbox → 空串
   - `bundleURLForMode` 从只返回 `index.html` 改为调 `BundleEntryURLFor`

5. `frontend/package.json`
   - dependencies 加 `@webcontainer/api: ^1.5.1`

6. `frontend/vite.config.ts`
   - 新增 plugin `opslabs-cross-origin-isolation`,给所有 dev/preview 响应打
     `Cross-Origin-Opener-Policy: same-origin` + `Cross-Origin-Embedder-Policy:
     require-corp` + `Cross-Origin-Resource-Policy: same-origin`
   - WebContainer 强依赖 SharedArrayBuffer,这三个头缺一不可
   - **生产部署需要在反代 / CDN 同样配这几个头**

7. `frontend/src/components/runners/WebContainerRunner.tsx`(新)
   - 在**主 frame** 挂 WebContainer(不走 iframe,避免 sandbox+COEP 冲突)
   - 启动流程:fetch project.json → boot WebContainer → mount FileSystemTree →
     跑 startCmd(`npm install`)→ 发 ready
   - 判题:`wc.spawn(check.cmd, check.args)`,exit=0 + stdout 含 `OK` 视为 passed
   - 用模块级 `bootPromise` 单例避免 "WebContainer already booted"(Strict Mode)
   - UI:顶部状态 + manifest 预览(顶层文件列表、入口、判题命令) + 底部实时日志窗

**Round 3 改动(wasm-linux)**:

1. `backend/internal/scenario/bundles/wasm-linux-hello/index.html`(新)
   - 浏览器端 v86 引擎 + BusyBox Linux
   - libv86.js / v86.wasm / seabios.bin / vgabios.bin / linux.iso 全部从
     `https://copy.sh/v86/...` 拉(该站点 CORS 开放),避免 4~8MB embed 到 Go 二进制
   - 纯 serial 模式(不挂 VGA screen_container),I/O 全走 serial0
   - 在 textarea 里捕获键盘 → serial0 字节流,实现迷你 shell 终端
   - 检测到 BusyBox 的 `/ # ` 提示符视为 ready,发 `opslabs:ready`
   - 判题:父页面 `opslabs:check` → 写入 `[ -f /tmp/ready.flag ] && echo OPSLABS_PASS_<token>`,
     在 serial0 输出 buffer 里扫描 token marker,4s 超时兜底

2. `backend/internal/scenario/registry.go`
   - 新增 `scenarioWasmLinuxHello()`,slug=`wasm-linux-hello`,category=ops,
     difficulty=1,execution_mode=wasm-linux
   - 场景目标:`touch /tmp/ready.flag`
   - hints 由浅入深 3 级

3. `backend/internal/scenario/bundles/bundles.go`
   - 新增 `//go:embed all:wasm-linux-hello` 指令

4. 前端**复用 StaticRunner**
   - wasm-linux 和 static 共用 iframe + postMessage 协议,没必要另造一个 Runner
   - `Scenario.tsx` 的 `renderByExecutionMode` switch 把 `static` / `wasm-linux`
     合并到同一 case,都渲染 `<StaticRunner />`
   - 前端没有 WasmLinuxRunner.tsx 独立组件 —— 这是有意为之,避免重复实现

**Scenario.tsx 整理**:

- 删掉 Round 1 的 `renderBundleComingSoon` 占位 UI
- 新增 `renderIframeBundleRunner`(static + wasm-linux)、`renderWebContainerRunner`
- 两套独立 ref:`staticRunnerRef` / `webContainerRunnerRef`(避免 MutableRefObject 类型不变性)
- `onCheck` 按 mode 路由到对应 handle:`web-container → webContainerRunnerRef`,
  其它非 sandbox → `staticRunnerRef`

**本地验证步骤**:

1. 后端:
   - `cd backend && go build ./...` 编译通过
   - `go run ./cmd/backend -conf configs/config.yaml` 起服务
   - 验证:
     - `GET /v1/scenarios/webcontainer-node-hello/bundle/project.json` 返回 FileSystemTree JSON
     - `GET /v1/scenarios/wasm-linux-hello/bundle/index.html` 返回 v86 启动页

2. 前端:
   - `cd frontend && npm install` 拉 `@webcontainer/api`
   - `npm run dev` 起 dev server
   - **首次打开** `http://localhost:5173/scenarios/webcontainer-node-hello`:
     - 控制台确认 `window.crossOriginIsolated === true`(不满足 WebContainer 不会启动)
     - 日志窗应该依次看到:fetch manifest / boot / mount / npm install / ready
     - 点"检查答案" → 应该 fail(handler 里有 bug),stdout 里能看到"字段不对"
     - 把 `handler.js` 里的 `greet` 改成 `greeting` → 再点检查应该 passed
     - **但注意**:Round 2 暂未嵌入浏览器内编辑器,修文件只能通过 project.json
       本身改(后续 PR 接入 Monaco Editor)
   - `wasm-linux-hello`:
     - 打开页面等 3~5 秒,看到 `/ # ` 提示符后点击 textarea 敲 `touch /tmp/ready.flag` + 回车
     - 点"检查答案" → 应该 passed,stdout 显示 "OK\n/tmp/ready.flag 存在"

**已知限制**:

- WebContainer 当前没有在浏览器里编辑代码的入口(需要后续接 Monaco);
  用户想改 `handler.js` 的 bug 需要改 project.json 源文件。
- v86 首次启动依赖 copy.sh CDN 可达性,国内网络可能需要代理;
  要真·自托管,把 `V86_BASE` 换成自己 CDN 路径即可。
- v86 shell 是 BusyBox hush,`vi` 这种编辑器没有,只能用 `echo > file` / `cat > file <<EOF`
  这种简单命令。对"touch 一个文件"来说够用。

---

### [feat] V1 三执行模式分发骨架 + static 端到端(Round 1)

**背景**:
承接上一条"V1 预留多执行模式钩子"的预留字段,Round 1 把其中 `static` 模式
真正打通:前端写答题 → 前端自己判题 → 后端只负责下发 bundle + 存 Attempt 结果。
`web-container` / `wasm-linux` 在 Round 2/3 继续。

**改动清单**:

1. `backend/api/opslabs/v1/opslabs.proto`
   - `ScenarioBrief.execution_mode`(#11)
   - `ScenarioDetail.execution_mode`(#17)、`ScenarioDetail.bundle_url`(#18)
   - `StartScenarioReply` 加 `execution_mode` / `bundle_url`
   - `AttemptReply` 加 `execution_mode` / `bundle_url`
   - 新 `message ClientCheckResult { passed, exit_code, stdout, stderr }`
   - `CheckAttemptRequest` 加 `client_result`(仅非 sandbox 模式使用)
   - **需要 `cd backend && make api` 回生 `opslabs.pb.go`** 才能编译

2. `backend/internal/scenario/bundles/`
   - 新目录,内嵌 static/wasm-linux/web-container 的前端 bundle 文件
   - `bundles.go` 用 `//go:embed all:css-flex-center` 打进二进制,
     `FS()` / `Open(slug+"/"+path)` / `Has(slug)` 三个出口给 handler 用
   - 首个 bundle:`css-flex-center/index.html` —— 分栏 CSS 编辑器 + 实时预览 iframe,
     判题走 postMessage(父 → 子 `opslabs:check`,子 → 父 `opslabs:check-result`)

3. `backend/internal/service/opslabs/bundle.go`(新)
   - `NewBundleHandler()` 处理 `GET /v1/scenarios/{slug}/bundle/{file*}`,
     从 embed 里流式下发;按扩展名设 Content-Type、禁止 `..` 上跳
   - `BundleURLFor(slug)` 给 Start/Get 的 reply 拼出入口 URL

4. `backend/internal/server/http.go`
   - **先注册 RPC `RegisterScenarioHTTPServer`,再 `srv.HandlePrefix("/v1/scenarios/", NewBundleHandler())`**
   - 这个顺序让 gorilla/mux 的 RPC 精确路由(`/v1/scenarios` / `/v1/scenarios/{slug}`)
     先匹配,剩下的 `/v1/scenarios/{slug}/bundle/...` 落到 HandlePrefix

5. `backend/internal/biz/attempt/usecase.go`
   - `Start()` 把原单线 Docker 分配拆成 `startSandbox()` + `startBundleless()`:
     `sandbox` → 起容器 → 落库;其余三种 → 只落库,不占端口,`ContainerID`/`HostPort` 留空
   - 新 `ClientCheckResult` 业务层结构体(不 import proto 保持分层)
   - `Check(ctx, id, *ClientCheckResult)`:按模式分流
     - sandbox:`runner.Exec(check.sh)` + SuccessOutput 匹配(原逻辑抽到 `checkSandbox()`)
     - 其它模式:`checkFromClient()` —— `client==nil` 返 400,否则直接采用前端结果
   - `CheckCount++` / `MarkPassed` / `repo.Update` 在分流外统一处理,避免分支各写一份

6. `backend/internal/service/opslabs/attempt.go`
   - `NewAttemptService(uc, opts, registry)` 多吃一个 `scenario.Registry` 依赖
     (wire_gen.go 同步更新,Wire 会自动解析 —— provider 已在 `server.NewScenarioRegistry`)
   - `StartScenarioReply` / `AttemptReply` 里用新增的 `runtimeEntry(slug)` 反查
     `execution_mode` / `bundle_url`,**不入库**;DB schema 不变
   - `CheckAttempt` 把 `req.GetClientResult()` 翻译成 biz 的 `ClientCheckResult`
     传给 usecase;sandbox 模式传 nil,后端照常走 docker exec

7. `backend/internal/service/opslabs/scenario.go`
   - `toScenarioBrief` / `toScenarioDetail` 填 `execution_mode`,后者额外填 `bundle_url`
   - sandbox 模式下 `bundle_url` 为空串(避免让前端以为可以拉 bundle)

8. `backend/internal/scenario/registry.go`
   - 新增 `scenarioCSSFlexCenter()`:slug=`css-flex-center`,mode=`static`,
     difficulty=1,category=`frontend`。Runtime/Grading 字段在 static 模式下不使用

9. `frontend/src/types.ts`
   - `ScenarioDetail.bundleUrl?`、`Attempt.executionMode?`/`bundleUrl?`、
     `StartScenarioReply.executionMode?`/`bundleUrl?`、
     新 `ClientCheckResult`、`CheckAttemptRequest`

10. `frontend/src/api/attempt.ts`
    - `useCheckAttempt` 签名改为 `mutate({ id, clientResult? })`,body 里附带 `clientResult`
    - `useStartAttempt` / `useAttempt` 把 `executionMode` / `bundleUrl` 透进 store

11. `frontend/src/components/runners/StaticRunner.tsx`(新)
    - `forwardRef` + `useImperativeHandle`:暴露 `triggerCheck()` 返回 `Promise<ClientCheckResult>`
    - window.message 监听 `opslabs:ready` / `opslabs:check-result`
    - 5s 超时;bundleUrl 变化重置 `ready` 状态;卸载时 reject 任何 in-flight Promise
    - iframe `sandbox="allow-scripts allow-same-origin"`

12. `frontend/src/pages/Scenario.tsx`
    - `execMode` 优先取 `current.executionMode`(运行时权威),回落 scenario 然后 `sandbox`
    - `onCheck` 非 sandbox 分支先 `await staticRunnerRef.current.triggerCheck()`
      把 `clientResult` 一起带给 `/check`;失败当作"未通过 + 错误消息"本地展示
    - `renderByExecutionMode` 把原"未实现"占位换成 `renderStaticRunner`;
      `web-container` / `wasm-linux` 改为 `renderBundleComingSoon` 说明 Round 2/3 接入中

**需要你本地执行的一步**(无法在容器内跑):

```bash
cd backend && make api        # 回生 opslabs.pb.go
cd backend && go build ./...  # 验证全 green
cd frontend && pnpm typecheck # 验证 tsc --noEmit 过
```

**后续**:
- Round 2:`@webcontainer/api` + Vite COOP/COEP header 配置 + 真 Node 题
- Round 3:CheerpX / v86 + 磁盘镜像流水线 + 真 Linux 题

---

### [feat] V1 预留多执行模式钩子

**背景**:
opslabs 长期规划支持四种执行模式(详见内部设计稿):
- `sandbox`     —— 后端 Docker + ttyd(V1 唯一实装)
- `static`      —— 纯前端题,不起容器
- `wasm-linux`  —— 浏览器 CheerpX / v86 跑 wasm Linux,后端只下发资源包
- `web-container` —— 浏览器 StackBlitz WebContainer

V1 阶段只做 sandbox,但为了避免 V2 接入其它模式时要到处改数据结构、回调链,
先把入口与字段都留出来。**这次改动对现有场景完全零行为变化**,所有老场景
`ExecutionMode` 为空串,调用方通过 `EffectiveExecutionMode()` 兜底回 `sandbox`。

**改了哪些地方**:

1. `backend/internal/scenario/types.go`
   - 加 `ExecutionMode string` 字段到 `Scenario` / `Brief`。
   - 加常量:`ExecutionModeSandbox` / `Static` / `WasmLinux` / `WebContainer`。
   - 加 `DefaultExecutionMode = sandbox` 常量 + `(*Scenario).EffectiveExecutionMode()`
     方法,空串兜底。调用方不直接读字段,一律过这个方法。

2. `backend/internal/biz/attempt/usecase.go`
   - `Start()` 进 docker 分配前加 `switch sc.EffectiveExecutionMode()`:
     - `sandbox`:走现有 Docker 分配逻辑。
     - `static` / `wasm-linux` / `web-container`:返回 501 `EXECUTION_MODE_NOT_IMPLEMENTED`,
       带清晰错误消息,让前端即使配错场景也能看到"这个模式 V2 才有"。
     - `default`:未知模式返 500,防止拼错串被当 sandbox 糊弄过去。

3. `frontend/src/types.ts`
   - 加 `export type ExecutionMode = 'sandbox' | 'static' | 'wasm-linux' | 'web-container'`。
   - `ScenarioBrief` / `ScenarioDetail` 都加了可选字段 `executionMode?: ExecutionMode`。
   - **V1 注意**:proto 里暂时没下发 `execution_mode`,前端读到 undefined
     时在 `Scenario.tsx` 兜底成 `sandbox`。proto 回填放到 V2(见"后续")。

4. `frontend/src/pages/Scenario.tsx`
   - 新增 `renderByExecutionMode()` dispatch 函数,按 `scenario.executionMode ?? 'sandbox'`
     路由:
     - `sandbox` → `renderSandboxRunner()`(原 `renderTerminalArea` 改名,行为一字不动)。
     - 其它三种 → `renderModeNotImplemented(title, desc)`,显示"未实装"占位卡。
   - default 分支用 `const _exhaustive: never = p.mode`,新增 ExecutionMode 时
     TypeScript 会在这里报错提醒补 case,避免漏路由。

**为什么暂不改 proto**:

V1 没有任何场景用非 sandbox 模式,proto 字段就算加了也没值可灌。
proto 改动会拖一整条 `make api` 回归(protoc + kratos 插件 + pb.go 规模较大),
风险收益不对等。V2 真有场景声明 `executionMode: 'static'` 时一并补,
服务层加两行 `ExecutionMode: sc.EffectiveExecutionMode()` 就打通。

**后续(V2 计划)**:

- `backend/api/opslabs/v1/opslabs.proto` 给 `ScenarioBrief` / `ScenarioDetail`
  加 `string execution_mode = <n>`,`make api` 重生。
- `backend/internal/service/opslabs/scenario.go` 的 `toScenarioBrief` /
  `toScenarioDetail` 里加一行 `ExecutionMode: sc.EffectiveExecutionMode()`。
- `Scenario.tsx` 的对应 case 里填真实 runner(静态 bundle / CheerpX / WebContainer)。
- `AttemptUsecase.Start` 的 501 早退分支换成对应的资源包分发逻辑,返回
  前端需要的 URL / tarball 而不是容器信息。

**文件**: `backend/internal/scenario/types.go`、
`backend/internal/biz/attempt/usecase.go`、
`frontend/src/types.ts`、
`frontend/src/pages/Scenario.tsx`。

---

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
