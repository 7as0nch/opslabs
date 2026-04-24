import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from 'react'
import { WebContainer } from '@webcontainer/api'
import type { FileSystemTree, WebContainerProcess } from '@webcontainer/api'
import Editor from '@monaco-editor/react'
import { ClientCheckResult } from '../../types'

/**
 * WebContainerRunner
 *
 * web-container 执行模式的前端 Runner:
 *   - StackBlitz WebContainer 在主 frame 里跑一个浏览器原生 Node.js
 *   - bundleUrl 是 project.json(FileSystemTree + entrypoint + check 命令元信息)
 *   - 启动流程:fetch project.json → boot WebContainer → mount FileSystemTree
 *     → 跑 startCmd(通常是 npm install)→ ready → 用户改代码/点检查
 *   - 判题:exec check.cmd,exit code 0 且 stdout 含 "OK" 就 passed=true
 *
 * 2026-04-23 Monaco 集成(R7-03b):
 *   - 左侧文件树 + 右侧 Monaco Editor + 底部日志的三段布局
 *   - drafts 本地编辑,"保存"后通过 wc.fs.writeFile 同步进 WebContainer
 *   - triggerCheck 前自动 flush 所有 dirty 文件,避免"改了没保存就检查"的误会
 *   - "还原"按钮回到 manifest 的原始内容(不影响 WebContainer 里已写的文件 —— 还原
 *     只回滚 draft,真要同步过去再点保存)
 *
 * **为什么不用 iframe + postMessage 协议像 Static / Wasm-Linux 那样**:
 *   - WebContainer 依赖 SharedArrayBuffer,要求 cross-origin isolation(COOP/COEP)
 *   - iframe 里的 SAB 要求更苛刻,用 `<iframe allow="cross-origin-isolated">` + sandbox
 *     配合起来有很多浏览器坑
 *   - 主 frame 一把起也就几 MB,跟页面生命周期绑定,离开页面自动释放 teardown
 *
 * **为什么用全局单例 bootPromise**:
 *   - WebContainer.boot() 一个 tab 只能调一次,重复调会抛 "WebContainer already booted"
 *   - React Strict Mode 下组件 effect 会跑两遍,没单例就炸
 *   - 用模块级 Promise 缓存 boot,切换场景时 teardown 旧 fs 但保留引擎
 *
 * **Monaco CDN 与 COEP**:
 *   - @monaco-editor/react 默认通过 jsDelivr 拉 monaco-editor 主包和 workers
 *   - Vite 侧 COEP 设为 credentialless(见 vite.config.ts),跨源子资源不要求 CORP,
 *     jsDelivr 的响应会被正常加载,SharedArrayBuffer 隔离仍然成立
 *   - 离线 / 受限网络环境可以后续切本地 loader.config({ paths: { vs: '/vendor/monaco/vs' } })
 */
export interface WebContainerRunnerHandle {
  /** 触发判题,返回 Promise<ClientCheckResult>(内部会先把未保存的 drafts flush 到 wc.fs) */
  triggerCheck: () => Promise<ClientCheckResult>
  /** 是否启动完成(mount + startCmd 都跑完) */
  ready: boolean
}

interface Props {
  /** project.json URL,形如 /v1/scenarios/{slug}/bundle/project.json */
  bundleUrl: string
}

// project.json 结构,跟 backend/internal/scenario/bundles/*/project.json 对齐
//
// 关于 editableFiles / hiddenFiles —— 为什么要有这两个字段(2026-04-23 R7-04b):
//   场景里 check 脚本(check.mjs 之类)如果允许用户改或直接看,判题就废了:
//     - 可改:用户把 check 脚本替换成 console.log('OK'); process.exit(0),直接拿通过
//     - 可看:用户照着 check 里的断言反推出答案(等于给答案),不再是"修 handler"的练习
//   两个字段做不同程度的保护:
//     - editableFiles: 白名单,只有在里面的路径 Monaco 允许编辑,其它一律 readOnly
//       (展示仍然允许,有助于用户理解 package.json / README 这种参考文件)
//     - hiddenFiles  : 黑名单,直接从文件树里拿掉,用户压根看不到(check 脚本建议列这里)
//   两个字段都没设时,保留 R7-03b 行为:所有扁平出来的文件都可编辑、全量展示
//   WebContainer 内部 fs 始终是完整 mount 的,hiddenFiles 只影响 UI,不影响 check 执行
interface ProjectManifest {
  entrypoint?: string
  startCmd?: { cmd: string; args?: string[] }
  check: { cmd: string; args?: string[] }
  files: FileSystemTree
  /** 可编辑文件白名单(路径相对 WebContainer workdir,不带前导 "/");未设时全部可编辑 */
  editableFiles?: string[]
  /** 隐藏文件黑名单(不在文件树里展示,WebContainer 里仍然存在并参与 check) */
  hiddenFiles?: string[]
  __comment?: string[] | string
}

// ============================================================
// 全局 WebContainer 单例
// ------------------------------------------------------------
// WebContainer.boot() 一个 tab 只能调一次。
// 用 module-level Promise 做单例缓存;teardown 时 resolve 的引擎仍然能复用,
// 只是 fs 要自己清理(mount 新的就会覆盖)。
// ============================================================
let bootPromise: Promise<WebContainer> | null = null

function getWebContainer(): Promise<WebContainer> {
  if (!bootPromise) {
    bootPromise = WebContainer.boot()
  }
  return bootPromise
}

// Monaco 走 @monaco-editor/react 的默认 CDN loader(jsdelivr),不再显式 config
// 离线 / 内网部署未来要切本地时,在这里导入 loader 并调用:
//   import { loader } from '@monaco-editor/react'
//   loader.config({ paths: { vs: '/vendor/monaco/vs' } })

const CHECK_TIMEOUT_MS = 20_000 // check 命令可能跑测试,给 20s
const START_TIMEOUT_MS = 60_000 // npm install 首次比较慢,给 60s

// ----------------------------------------------------------------------
// FileSystemTree 扁平化 —— 递归展开目录,只收文件节点(忽略 symlink)
// ----------------------------------------------------------------------
// 返回 { path → contents } 字典,path 不带前导 "/"(与 WebContainer workdir 相对路径一致)
//   例:{ "handler.js": "...", "src/util/log.js": "..." }
//
// 之前直接从 @webcontainer/api 导入 FileNode/DirectoryNode/SymlinkNode 类型作
// type-guard 参数;这几个命名导出在不同 @webcontainer/api 小版本里存在过重命名
// (部分版本只导出 FileSystemTree 根类型),为了避免上游包升级破坏类型检查,
// 这里用结构判定("file" / "directory" 属性存在)代替类型名,避免把类型系统
// 绑死在特定版本上。运行时语义完全一致。
function flattenTree(tree: FileSystemTree, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [name, rawNode] of Object.entries(tree)) {
    const path = prefix ? `${prefix}/${name}` : name
    // unknown 中转一次,后续按属性精确 narrow,避免和上游 exported union 类型耦合
    const node = rawNode as unknown as Record<string, unknown>
    if ('file' in node) {
      const file = node.file as { contents: string | Uint8Array }
      const contents = file.contents
      // WebContainer 的 file.contents 可以是 string | Uint8Array。
      // Monaco 只能编辑文本,二进制文件当前不支持;把它标成占位字符串
      // 让用户能看到文件存在但不误以为可以编辑。
      out[path] =
        typeof contents === 'string'
          ? contents
          : '// <binary file · 不支持在编辑器内编辑>'
    } else if ('directory' in node) {
      const sub = node.directory as FileSystemTree
      Object.assign(out, flattenTree(sub, path))
    }
    // symlink 忽略,场景里一般不会出现
  }
  return out
}

// 文件后缀 → Monaco language ID 映射
// 覆盖常见脚本/配置类型,匹配不到统一用 plaintext(Monaco 的默认降级)
function guessLanguage(path: string): string {
  const m = path.toLowerCase().match(/\.([a-z0-9]+)$/)
  if (!m) return 'plaintext'
  switch (m[1]) {
    case 'js':
    case 'cjs':
    case 'mjs':
    case 'jsx':
      return 'javascript'
    case 'ts':
    case 'tsx':
      return 'typescript'
    case 'json':
    case 'jsonc':
      return 'json'
    case 'md':
    case 'markdown':
      return 'markdown'
    case 'html':
    case 'htm':
      return 'html'
    case 'css':
      return 'css'
    case 'yaml':
    case 'yml':
      return 'yaml'
    case 'sh':
    case 'bash':
      return 'shell'
    case 'go':
      return 'go'
    case 'py':
      return 'python'
    case 'sql':
      return 'sql'
    case 'toml':
      return 'toml'
    case 'dockerfile':
      return 'dockerfile'
    default:
      return 'plaintext'
  }
}

const WebContainerRunner = forwardRef<WebContainerRunnerHandle, Props>(
  function WebContainerRunner({ bundleUrl }, ref) {
    const [status, setStatus] = useState<'idle' | 'booting' | 'installing' | 'ready' | 'error'>(
      'idle',
    )
    const [errMsg, setErrMsg] = useState<string>('')
    const [logLines, setLogLines] = useState<string[]>([])
    const logBoxRef = useRef<HTMLDivElement>(null)

    const wcRef = useRef<WebContainer | null>(null)
    const manifestRef = useRef<ProjectManifest | null>(null)

    // ------------------------------------------------------------
    // 文件编辑状态
    // ------------------------------------------------------------
    //   initialFiles : manifest 带来的原始内容,"还原"按钮基准
    //   drafts       : 当前编辑器里的内容,保存时写进 WebContainer
    //   dirty        : 已改未保存的路径集合(保存成功后从集合里拿掉)
    //   activeFile   : 当前编辑的路径
    const [initialFiles, setInitialFiles] = useState<Record<string, string>>({})
    const [drafts, setDrafts] = useState<Record<string, string>>({})
    const [dirty, setDirty] = useState<Set<string>>(() => new Set())
    const [activeFile, setActiveFile] = useState<string | null>(null)
    const [saving, setSaving] = useState(false)

    // 把最新的 drafts / dirty 挂 ref,给 triggerCheck(它在 imperativeHandle 里
    // 用的是 useImperativeHandle 内部闭包)flushDirtyFiles 读,不用靠依赖重建
    const draftsRef = useRef(drafts)
    const dirtyRef = useRef(dirty)
    useEffect(() => {
      draftsRef.current = drafts
    }, [drafts])
    useEffect(() => {
      dirtyRef.current = dirty
    }, [dirty])

    const appendLog = useCallback((s: string) => {
      setLogLines((prev) => {
        const next = [...prev, s]
        // 滚动窗口:只留最近 300 条,避免 DOM 膨胀
        return next.length > 300 ? next.slice(-300) : next
      })
    }, [])

    useEffect(() => {
      // 滚到底部,让最新日志可见
      const el = logBoxRef.current
      if (el) el.scrollTop = el.scrollHeight
    }, [logLines])

    // ==========================================================
    // 启动流程:fetch manifest → boot → mount → startCmd
    // ==========================================================
    useEffect(() => {
      let cancelled = false
      setStatus('booting')
      setLogLines([])
      setErrMsg('')
      setInitialFiles({})
      setDrafts({})
      setDirty(new Set())
      setActiveFile(null)

      const startCtl = new AbortController()
      const startTimer = setTimeout(() => {
        startCtl.abort()
      }, START_TIMEOUT_MS)

      ;(async () => {
        try {
          appendLog(`[opslabs] fetch manifest: ${bundleUrl}`)
          const resp = await fetch(bundleUrl, { signal: startCtl.signal })
          if (!resp.ok) {
            throw new Error(`fetch project.json failed: ${resp.status} ${resp.statusText}`)
          }
          const manifest = (await resp.json()) as ProjectManifest
          manifestRef.current = manifest
          if (cancelled) return

          // 扁平化文件树,初始化 drafts/initialFiles
          //   打开编辑器时:entrypoint 优先,没设就挑第一个可见路径
          //   hiddenFiles 只影响 UI 展示 —— 不加入 drafts/initialFiles,避免用户用 Ctrl+S
          //   快捷键意外保存一个不该动的文件;check 执行仍走 WebContainer.fs(mount 时写入)
          const flatAll = flattenTree(manifest.files)
          const hidden = new Set(manifest.hiddenFiles ?? [])
          const flat: Record<string, string> = {}
          for (const [p, c] of Object.entries(flatAll)) {
            if (!hidden.has(p)) flat[p] = c
          }
          setInitialFiles(flat)
          setDrafts({ ...flat })
          const entryCandidate =
            manifest.entrypoint && flat[manifest.entrypoint] != null ? manifest.entrypoint : null
          // 次选:第一个可编辑文件;再次选:第一个可见文件;都没有就 null
          const firstEditable = (manifest.editableFiles ?? []).find((p) => flat[p] != null) ?? null
          const entry = entryCandidate ?? firstEditable ?? Object.keys(flat)[0] ?? null
          setActiveFile(entry)

          appendLog('[opslabs] booting WebContainer…')
          const wc = await getWebContainer()
          wcRef.current = wc
          if (cancelled) return

          appendLog('[opslabs] mount files')
          await wc.mount(manifest.files)
          if (cancelled) return

          if (manifest.startCmd?.cmd) {
            setStatus('installing')
            appendLog(
              `[opslabs] run startCmd: ${manifest.startCmd.cmd} ${(manifest.startCmd.args || []).join(' ')}`,
            )
            const p = await wc.spawn(manifest.startCmd.cmd, manifest.startCmd.args || [])
            // 把子进程 stdout 实时塞进日志框
            p.output
              .pipeTo(
                new WritableStream({
                  write(chunk) {
                    if (cancelled) return
                    appendLog(chunk.replace(/\n+$/, ''))
                  },
                }),
              )
              .catch(() => {
                /* stream 关闭时抛 AbortError,忽略 */
              })
            const code = await p.exit
            if (cancelled) return
            if (code !== 0) {
              throw new Error(`startCmd 失败,exit=${code}`)
            }
          } else {
            appendLog('[opslabs] no startCmd,跳过')
          }

          clearTimeout(startTimer)
          if (cancelled) return
          setStatus('ready')
          appendLog('[opslabs] ready — 编辑文件后点"保存"再点"检查答案"')
        } catch (e) {
          if (cancelled) return
          clearTimeout(startTimer)
          const msg =
            (e as Error)?.name === 'AbortError'
              ? `启动超时(${START_TIMEOUT_MS}ms)`
              : (e as Error)?.message || String(e)
          setErrMsg(msg)
          setStatus('error')
          appendLog('[opslabs] ERROR: ' + msg)
        }
      })()

      return () => {
        cancelled = true
        clearTimeout(startTimer)
        startCtl.abort()
        // 不 teardown WebContainer:模块级单例,跨场景复用引擎
        // 下次 mount 会覆盖 fs,足够干净
      }
    }, [bundleUrl, appendLog])

    // ==========================================================
    // 文件编辑 / 保存 / 还原
    // ==========================================================
    // editableFiles 白名单判定(提前到所有 handler 之前声明,避免 Ctrl+S useEffect 对
     // 它形成 TDZ 引用 —— 原来放在 UI 区,JSX 渲染前 useEffect 已经跑过依赖数组求值)
    //   - manifest 没设 → 所有可见文件都可编辑(与 R7-03b 默认保持一致)
    //   - manifest 设了 → 只有列进去的路径可编辑,其它都 readOnly
    //   hiddenFiles 已经在 fetch 阶段从 drafts 里踢掉,这里不用重复过滤
    const isEditable = useCallback(
      (path: string | null): boolean => {
        if (path == null) return false
        const list = manifestRef.current?.editableFiles
        if (!list) return true
        return list.includes(path)
      },
      // manifestRef 是 ref,变化不会触发重算,但 activeFile 变化时 React 会重新
      // 调用这个 callback 读最新 ref 值,所以不用加依赖
      [],
    )

    const markDirty = useCallback((path: string) => {
      setDirty((prev) => {
        if (prev.has(path)) return prev
        const next = new Set(prev)
        next.add(path)
        return next
      })
    }, [])

    const clearDirty = useCallback((path: string) => {
      setDirty((prev) => {
        if (!prev.has(path)) return prev
        const next = new Set(prev)
        next.delete(path)
        return next
      })
    }, [])

    const handleEditorChange = useCallback(
      (val: string | undefined) => {
        if (activeFile == null) return
        // editableFiles 白名单外的路径直接忽略 onChange,防止 Monaco readOnly 被绕过
        //   (例如未来 Monaco 版本或 DevTools 强改后)脏状态扩散到 wc.fs
        const list = manifestRef.current?.editableFiles
        if (list && !list.includes(activeFile)) return
        const v = val ?? ''
        setDrafts((prev) => ({ ...prev, [activeFile]: v }))
        // 只有和 initial 不同才算脏 —— 用户原样输回去也不报脏
        if (v !== initialFiles[activeFile]) {
          markDirty(activeFile)
        } else {
          clearDirty(activeFile)
        }
      },
      [activeFile, initialFiles, markDirty, clearDirty],
    )

    // 保存单文件:draft → WebContainer 文件系统 → 清 dirty
    const saveOne = useCallback(
      async (path: string): Promise<void> => {
        const wc = wcRef.current
        if (!wc) throw new Error('WebContainer 尚未启动')
        const contents = draftsRef.current[path] ?? ''
        // fs.writeFile 接 UTF-8 文本;路径不需要前导 "/"
        await wc.fs.writeFile(path, contents)
        clearDirty(path)
      },
      [clearDirty],
    )

    // 保存所有 dirty,triggerCheck 前也会调
    const flushDirty = useCallback(async (): Promise<number> => {
      const list = Array.from(dirtyRef.current)
      if (list.length === 0) return 0
      setSaving(true)
      try {
        for (const p of list) {
          // eslint-disable-next-line no-await-in-loop
          await saveOne(p)
        }
        appendLog(`[opslabs] 保存 ${list.length} 个文件`)
        return list.length
      } finally {
        setSaving(false)
      }
    }, [saveOne, appendLog])

    // 还原当前文件到 manifest 初始内容
    const revertActive = useCallback(() => {
      if (activeFile == null) return
      const orig = initialFiles[activeFile] ?? ''
      setDrafts((prev) => ({ ...prev, [activeFile]: orig }))
      clearDirty(activeFile)
    }, [activeFile, initialFiles, clearDirty])

    // 快捷键:Ctrl/Cmd+S 保存当前文件
    //   - readOnly 文件(editableFiles 白名单外)即使通过 DevTools 把 readOnly 关了
    //     也不会走到这条路径 —— Monaco 的编辑框事件不会把文本脏状态扩散到 activeIsDirty,
    //     Ctrl+S 默认浏览器保存页面、我们 preventDefault 后再做 noop,不写 wc.fs
    useEffect(() => {
      const onKey = (e: KeyboardEvent) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 's') {
          e.preventDefault()
          if (activeFile && dirty.has(activeFile) && isEditable(activeFile)) {
            setSaving(true)
            saveOne(activeFile)
              .then(() => appendLog(`[opslabs] 已保存 ${activeFile}`))
              .catch((err) => appendLog(`[opslabs] 保存失败 ${activeFile}: ${err}`))
              .finally(() => setSaving(false))
          }
        }
      }
      window.addEventListener('keydown', onKey)
      return () => window.removeEventListener('keydown', onKey)
    }, [activeFile, dirty, saveOne, appendLog, isEditable])

    // ==========================================================
    // triggerCheck:先 flush dirty,再跑 manifest.check 命令
    // ==========================================================
    useImperativeHandle(
      ref,
      () => ({
        ready: status === 'ready',
        triggerCheck: () =>
          new Promise<ClientCheckResult>((resolve, reject) => {
            const wc = wcRef.current
            const manifest = manifestRef.current
            if (!wc || !manifest) {
              reject(new Error('WebContainer 尚未启动完成'))
              return
            }
            if (status !== 'ready') {
              reject(new Error(`Runner 还没 ready(当前:${status}),请等待`))
              return
            }

            const { cmd, args = [] } = manifest.check

            // 超时兜底:避免用户代码死循环吃掉按钮
            let timedOut = false
            let proc: WebContainerProcess | null = null
            const timer = setTimeout(() => {
              timedOut = true
              try {
                proc?.kill()
              } catch {
                /* 进程可能已退出,忽略 */
              }
              reject(new Error(`check 超时 ${CHECK_TIMEOUT_MS}ms`))
            }, CHECK_TIMEOUT_MS)

            let stdout = ''
            // WebContainer 的 output stream 把 stdout+stderr 合并(spawn 没传分流
            // 选项),全量塞 stdout;stderr 固定空串,保持 ClientCheckResult 结构完整
            const stderr = ''

            ;(async () => {
              try {
                // 判题前把当前编辑态 flush 进 WebContainer,让 check 跑到用户最新改动
                // 否则用户改了 handler.js 但没点保存,check 还在跑旧代码,UX 会误会
                const flushed = await flushDirty()
                if (flushed > 0) {
                  appendLog(`[opslabs] check 前自动保存了 ${flushed} 个文件`)
                }

                appendLog(`[opslabs] check: ${cmd} ${args.join(' ')}`)
                proc = await wc.spawn(cmd, args)
                // WebContainer 的 output 把 stdout + stderr 合并到一个 stream;
                // 要分离的话得在 spawn 里传 { stderr: ... } 选项
                // 这里简化:全塞 stdout,判题只看 exit code + 是否含 OK
                proc.output
                  .pipeTo(
                    new WritableStream({
                      write(chunk) {
                        stdout += chunk
                        appendLog(chunk.replace(/\n+$/, ''))
                      },
                    }),
                  )
                  .catch(() => {
                    /* ignore pipe abort */
                  })
                const code = await proc.exit
                clearTimeout(timer)
                if (timedOut) return
                const passed = code === 0 && /\bOK\b/.test(stdout)
                resolve({
                  passed,
                  exitCode: code,
                  stdout,
                  stderr,
                })
              } catch (e) {
                clearTimeout(timer)
                if (timedOut) return
                reject(e as Error)
              }
            })()
          }),
      }),
      [status, appendLog, flushDirty],
    )

    // ==========================================================
    // UI
    // ==========================================================
    // isEditable 在上面声明,避免和 Ctrl+S useEffect 形成 TDZ
    const files = useMemo(() => Object.keys(drafts).sort(), [drafts])

    const activeContents = activeFile != null ? drafts[activeFile] ?? '' : ''
    const activeIsDirty = activeFile != null && dirty.has(activeFile)
    const activeIsEditable = isEditable(activeFile)
    const activeLanguage = activeFile ? guessLanguage(activeFile) : 'plaintext'
    const dirtyCount = dirty.size

    return (
      <div className="w-full h-full flex flex-col bg-slate-950 text-slate-200">
        {/* 顶部状态栏 */}
        <div className="flex-shrink-0 px-3 py-2 border-b border-slate-800 bg-slate-900 text-xs font-mono flex items-center justify-between">
          <span>
            <span className="text-slate-400">WebContainer · </span>
            <StatusBadge status={status} />
            {dirtyCount > 0 && (
              <span className="ml-3 text-amber-300">● {dirtyCount} 个文件未保存</span>
            )}
          </span>
          <span className="text-slate-500 truncate max-w-[60%]" title={bundleUrl}>
            {bundleUrl}
          </span>
        </div>

        {/* 主体:左文件树 + 右 Monaco + 底日志 */}
        <div className="flex-1 min-h-0 grid grid-cols-[12rem_1fr] grid-rows-[1fr_10rem]">
          {/* 文件列表 */}
          <aside className="row-span-1 col-span-1 border-r border-slate-800 bg-slate-900/70 overflow-y-auto text-sm">
            <div className="px-3 py-2 text-[11px] uppercase tracking-wider text-slate-500 border-b border-slate-800">
              文件
            </div>
            <ul>
              {files.length === 0 && (
                <li className="px-3 py-2 text-slate-500 text-xs">
                  {status === 'error' ? '加载失败,见日志' : '加载 project.json…'}
                </li>
              )}
              {files.map((f) => {
                const isActive = f === activeFile
                const isDirty = dirty.has(f)
                const editable = isEditable(f)
                return (
                  <li key={f}>
                    <button
                      onClick={() => setActiveFile(f)}
                      className={
                        'w-full text-left px-3 py-1.5 font-mono text-xs flex items-center gap-1.5 ' +
                        (isActive
                          ? 'bg-slate-700/70 text-slate-100'
                          : editable
                            ? 'text-slate-300 hover:bg-slate-800/60'
                            : 'text-slate-500 hover:bg-slate-800/40 italic')
                      }
                      title={editable ? f : `${f}(只读,不可编辑)`}
                    >
                      <span className="truncate">{f}</span>
                      {!editable && (
                        <span className="ml-auto text-[10px] text-slate-500">只读</span>
                      )}
                      {isDirty && <span className="text-amber-400 text-[10px]">●</span>}
                    </button>
                  </li>
                )
              })}
            </ul>
            {manifestRef.current && (
              <div className="px-3 py-2 mt-2 border-t border-slate-800 text-[11px] text-slate-500 leading-relaxed">
                <div>
                  入口:
                  <code className="ml-1 px-1 rounded bg-slate-800/70 text-slate-300">
                    {manifestRef.current.entrypoint ?? '—'}
                  </code>
                </div>
                <div className="mt-1">
                  判题:
                  <code className="ml-1 px-1 rounded bg-slate-800/70 text-slate-300">
                    {manifestRef.current.check.cmd}{' '}
                    {(manifestRef.current.check.args || []).join(' ')}
                  </code>
                </div>
              </div>
            )}
          </aside>

          {/* Monaco 编辑器区 */}
          <section className="row-span-1 col-span-1 min-h-0 flex flex-col bg-slate-950">
            <div className="flex-shrink-0 h-9 px-3 flex items-center justify-between border-b border-slate-800 bg-slate-900/60 text-xs">
              <div className="flex items-center gap-2">
                <span className="font-mono text-slate-300">{activeFile ?? '未选中文件'}</span>
                {activeFile && !activeIsEditable && (
                  <span
                    className="text-[10px] px-1.5 py-0.5 rounded bg-slate-700/60 text-slate-300"
                    title="此文件不在 editableFiles 白名单,仅供参考"
                  >
                    只读
                  </span>
                )}
                {activeIsDirty && (
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300">
                    未保存
                  </span>
                )}
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => activeFile && saveOne(activeFile).catch(() => {})}
                  disabled={
                    !activeFile ||
                    !activeIsDirty ||
                    !activeIsEditable ||
                    saving ||
                    status !== 'ready'
                  }
                  className="h-7 px-3 rounded bg-slate-700 text-slate-200 hover:bg-slate-600 disabled:opacity-40 disabled:cursor-not-allowed text-xs"
                  title="保存当前文件到 WebContainer (Ctrl/Cmd+S)"
                >
                  {saving ? '保存中…' : '保存'}
                </button>
                <button
                  onClick={revertActive}
                  disabled={!activeFile || !activeIsDirty || !activeIsEditable}
                  className="h-7 px-3 rounded text-slate-400 hover:text-slate-200 hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed text-xs"
                  title="放弃对当前文件的修改,回到初始内容"
                >
                  还原
                </button>
              </div>
            </div>
            <div className="flex-1 min-h-0">
              {activeFile ? (
                <Editor
                  height="100%"
                  theme="vs-dark"
                  path={activeFile}
                  language={activeLanguage}
                  value={activeContents}
                  onChange={handleEditorChange}
                  loading={
                    <div className="w-full h-full grid place-items-center text-xs text-slate-500">
                      加载 Monaco Editor…
                    </div>
                  }
                  options={{
                    minimap: { enabled: false },
                    fontSize: 13,
                    lineNumbers: 'on',
                    scrollBeyondLastLine: false,
                    tabSize: 2,
                    wordWrap: 'on',
                    automaticLayout: true,
                    // 只读态三种来源:
                    //   1. status !== 'ready' —— 场景还没 ready 不让改(mount 会覆盖)
                    //   2. !activeIsEditable   —— editableFiles 白名单外,如 check 脚本
                    //   3. 文件来自 hiddenFiles —— 已经在 drafts 过滤阶段就不会被选中
                    // 这里的 readOnly 同时接 1 和 2,Ctrl+S 快捷键也会因为 saveOne 的
                    // disabled 条件走不通,三重保险
                    readOnly: status !== 'ready' || !activeIsEditable,
                  }}
                />
              ) : (
                <div className="w-full h-full grid place-items-center text-slate-500 text-sm">
                  左侧选一个文件开始编辑
                </div>
              )}
            </div>
          </section>

          {/* 底部日志(横跨两列) */}
          <div
            ref={logBoxRef}
            className="row-start-2 col-span-2 border-t border-slate-800 bg-black text-slate-300 text-xs font-mono p-2 overflow-y-auto whitespace-pre-wrap leading-snug"
          >
            {logLines.length === 0 ? (
              <span className="text-slate-500">等待输出…</span>
            ) : (
              logLines.map((l, i) => <div key={i}>{l}</div>)
            )}
            {status === 'error' && <div className="text-rose-400 mt-2">ERROR: {errMsg}</div>}
          </div>
        </div>
      </div>
    )
  },
)

export default WebContainerRunner

// ----------------------------------------------------------------------
// 子组件:状态徽标
// ----------------------------------------------------------------------
function StatusBadge({ status }: { status: string }) {
  const cls =
    status === 'ready'
      ? 'text-emerald-400'
      : status === 'error'
        ? 'text-rose-400'
        : 'text-amber-300'
  const label =
    status === 'idle'
      ? '待启动'
      : status === 'booting'
        ? '启动引擎'
        : status === 'installing'
          ? '安装依赖'
          : status === 'ready'
            ? '就绪'
            : status === 'error'
              ? '失败'
              : status
  return <span className={cls}>{label}</span>
}
