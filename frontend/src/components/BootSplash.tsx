import { useEffect, useState } from 'react'
import { ExecutionMode } from '../types'

// Stage BootSplash 的单个阶段:显示名 + 预估耗时(毫秒)
// 预估耗时只用于心理进度条推进,不强相关后端真实耗时
export interface BootStage {
  label: string
  estMs: number
}

interface Props {
  mode: ExecutionMode
  // error 时切换成错误视图,error 为空时显示启动动画
  error?: Error | null
  // 重试回调;error 态显示时提供给用户点击
  onRetry?: () => void
  // 可选:场景标题 + 一句话描述,用于让加载页按场景定制文案
  scenarioTitle?: string
  scenarioTagline?: string
  // 可选:场景 slug,用于从 STAGES_BY_SLUG 查专属阶段文案
  // 没命中就回落到 STAGES_BY_MODE 的通用文案
  scenarioSlug?: string
  // 可选:直接注入一组阶段,完全覆盖 slug / mode 默认值
  // 场景化配置优先级 customStages > STAGES_BY_SLUG[slug] > STAGES_BY_MODE[mode]
  customStages?: BootStage[]
}

// 启动阶段的"伪进度条":前端根本没法拿到 docker pull 的真实百分比,
// 但心理学上看到阶段名在推进用户就不会抓狂。三段式:
//   1. 分配容器     ——   ~35% 权重   ~2s
//   2. 拉取镜像     ——   ~45% 权重   ~4s(首次)
//   3. 连接终端     ——   ~20% 权重   ~1s
// 三段线性叠加,总时长按 mode 不同微调:sandbox 最慢(镜像可能 120MB+),
// 非 sandbox 基本跳过镜像那步。
//
// 纯前端心理进度,不与后端状态强耦合 —— 后端真 ready 后父页面会直接替换整块 UI
const STAGES_BY_MODE: Record<ExecutionMode, BootStage[]> = {
  sandbox: [
    { label: '预留端口 · 创建容器', estMs: 1500 },
    { label: '拉取/启动 Docker 镜像', estMs: 4000 },
    { label: '连接 ttyd 终端通道', estMs: 1500 },
  ],
  static: [
    { label: '创建 attempt 记录', estMs: 600 },
    { label: '加载前端 bundle', estMs: 800 },
    { label: '初始化 Runner', estMs: 400 },
  ],
  'wasm-linux': [
    { label: '创建 attempt 记录', estMs: 600 },
    { label: '加载 v86 + BusyBox 镜像', estMs: 3000 },
    { label: '引导 Linux 内核', estMs: 1500 },
  ],
  'web-container': [
    { label: '创建 attempt 记录', estMs: 600 },
    { label: '启动 WebContainer 运行时', estMs: 2500 },
    { label: '挂载文件 · 安装依赖', estMs: 3000 },
  ],
}

// STAGES_BY_SLUG 场景级阶段文案 —— 每个场景讲自己的故事,比通用 mode 文案更具体
//
// 这份表格的规则:
//   - 必填:label(中文,≤ 14 字);estMs(阶段预估耗时毫秒)
//   - 建议阶段数 3,多了用户等不到末尾就看完阶段完成打勾反而奇怪
//   - 总时长 ≈ 场景真实首屏等待 *1.2(留一点 buffer,进度条到 95% 停住即可)
//
// 未在表中的 slug 走 STAGES_BY_MODE 通用兜底(旧行为)。
const STAGES_BY_SLUG: Record<string, BootStage[]> = {
  'hello-world': [
    { label: '预留端口 · 创建容器', estMs: 1200 },
    { label: '拉取 opslabs/hello-world:v1 镜像', estMs: 3500 },
    { label: '连接 ttyd 终端 · 准备 welcome.txt', estMs: 1300 },
  ],
  'frontend-devserver-down': [
    { label: '创建 ops 容器 · 挂载 webapp 目录', estMs: 1400 },
    { label: '拉取 opslabs/frontend-devserver-down:v1', estMs: 4500 },
    { label: '连接终端 · 等待故障现场就绪', estMs: 1500 },
  ],
  'backend-api-500': [
    { label: '创建 ops 容器', estMs: 1400 },
    { label: '拉取 opslabs/backend-api-500:v1 + 引导 PostgreSQL', estMs: 5000 },
    { label: '连接终端 · 启动 systemd 样式服务', estMs: 1600 },
  ],
  'ops-nginx-upstream-fail': [
    { label: '创建 ops 容器', estMs: 1400 },
    { label: '拉取 opslabs/ops-nginx-upstream-fail:v1', estMs: 4500 },
    { label: '连接终端 · 启动 nginx & app', estMs: 1400 },
  ],
  'css-flex-center': [
    { label: '创建 attempt 记录', estMs: 400 },
    { label: '下载 CSS 编辑器 bundle', estMs: 600 },
    { label: '初始化实时预览 + 判题器', estMs: 400 },
  ],
  'wasm-linux-hello': [
    { label: '创建 attempt 记录', estMs: 500 },
    { label: '下载 v86 + BusyBox ISO(~6MB)', estMs: 2800 },
    { label: '引导 Linux 内核 · 启动 /bin/sh', estMs: 1500 },
  ],
  'webcontainer-node-hello': [
    { label: '创建 attempt 记录', estMs: 500 },
    { label: '启动 WebContainer Node runtime', estMs: 2500 },
    { label: '挂载 handler.js · npm install', estMs: 2500 },
  ],
}

export default function BootSplash({
  mode,
  error,
  onRetry,
  scenarioTitle,
  scenarioTagline,
  scenarioSlug,
  customStages,
}: Props) {
  if (error) {
    return (
      <div className="h-full grid place-items-center p-6 bg-slate-900">
        <div className="max-w-md w-full bg-rose-900/30 border border-rose-700 rounded-lg p-5 text-slate-100 text-sm leading-relaxed">
          <div className="flex items-center gap-2 mb-3">
            <span className="inline-block w-2 h-2 rounded-full bg-rose-500" />
            <span className="text-base font-medium text-rose-300">启动场景失败</span>
          </div>
          <div className="text-slate-200 break-all">{error.message}</div>
          <div className="mt-3 text-slate-400 text-xs">
            可能原因:Docker Desktop 未启动 / 镜像未构建 / 后端 DB 写入超时 / 网络不通。
            打开浏览器控制台看 <code>[opslabs]</code> 开头的日志。
          </div>
          {onRetry && (
            <button
              onClick={onRetry}
              className="mt-4 px-3 h-8 text-xs rounded bg-rose-700 hover:bg-rose-600 text-white transition"
            >
              重试启动
            </button>
          )}
        </div>
      </div>
    )
  }

  return (
    <BootProgress
      mode={mode}
      scenarioTitle={scenarioTitle}
      scenarioTagline={scenarioTagline}
      scenarioSlug={scenarioSlug}
      customStages={customStages}
    />
  )
}

function BootProgress({
  mode,
  scenarioTitle,
  scenarioTagline,
  scenarioSlug,
  customStages,
}: {
  mode: ExecutionMode
  scenarioTitle?: string
  scenarioTagline?: string
  scenarioSlug?: string
  customStages?: BootStage[]
}) {
  // 阶段选择优先级:customStages(调用方强制) > 场景级 slug 映射 > 模式兜底
  // 加 && length>0 防止传空数组把进度条瞬间推到 95% 停住
  const stages: BootStage[] =
    (customStages && customStages.length > 0 && customStages) ||
    (scenarioSlug && STAGES_BY_SLUG[scenarioSlug]) ||
    STAGES_BY_MODE[mode] ||
    STAGES_BY_MODE.sandbox
  const total = stages.reduce((s, x) => s + x.estMs, 0)

  // 伪进度:驱动到 95% 停住,避免"到 100% 但 attempt 还没 ready"的错觉
  const [progress, setProgress] = useState(0)
  const [stageIdx, setStageIdx] = useState(0)

  useEffect(() => {
    const startAt = Date.now()
    const id = window.setInterval(() => {
      const elapsed = Date.now() - startAt
      let acc = 0
      let idx = 0
      for (let i = 0; i < stages.length; i++) {
        if (elapsed > acc + stages[i].estMs) {
          acc += stages[i].estMs
          idx = i + 1
        } else {
          idx = i
          break
        }
      }
      const pct = Math.min(95, (elapsed / total) * 100)
      setProgress(pct)
      setStageIdx(Math.min(idx, stages.length - 1))
      if (pct >= 95) window.clearInterval(id)
    }, 150)
    return () => window.clearInterval(id)
  }, [mode, total, stages])

  // 默认启动文案兜底 —— 调用方没传 scenarioTitle/tagline 时走通用文案
  const displayTitle = scenarioTitle?.trim() || '正在启动场景'
  const displayTagline =
    scenarioTagline?.trim() ||
    (mode === 'sandbox'
      ? '首次运行可能需要拉取 Docker 镜像,请稍候…'
      : '加载前端运行时,无需后端容器。')

  return (
    <div className="h-full grid place-items-center bg-gradient-to-b from-slate-900 to-slate-950 p-6">
      <div className="w-[28rem] max-w-[94vw] text-slate-100">
        {/* 标题 + 动画小圆 */}
        <div className="flex items-center gap-3">
          <Spinner />
          <h3 className="text-base font-medium truncate" title={displayTitle}>
            {displayTitle}
          </h3>
          <span className="ml-auto text-xs font-mono text-slate-400">
            {Math.round(progress)}%
          </span>
        </div>

        {/* 进度条 */}
        <div className="mt-3 h-1.5 rounded-full bg-slate-800 overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-brand-500 to-brand-400 transition-all duration-300 ease-out"
            style={{ width: `${progress}%` }}
          />
        </div>

        {/* 阶段列表:已完成打勾,当前闪烁,后续灰色 */}
        <ul className="mt-5 space-y-2 text-sm">
          {stages.map((st, i) => {
            const done = i < stageIdx
            const active = i === stageIdx
            return (
              <li key={i} className="flex items-center gap-2">
                {done ? (
                  <span className="inline-flex w-5 h-5 rounded-full bg-emerald-500/20 text-emerald-400 items-center justify-center text-xs">
                    ✓
                  </span>
                ) : active ? (
                  <span className="inline-flex w-5 h-5 rounded-full bg-brand-500/20 text-brand-400 items-center justify-center">
                    <span className="w-2 h-2 rounded-full bg-brand-400 animate-pulse" />
                  </span>
                ) : (
                  <span className="inline-flex w-5 h-5 rounded-full bg-slate-800 text-slate-600 items-center justify-center text-xs">
                    {i + 1}
                  </span>
                )}
                <span
                  className={
                    done
                      ? 'text-slate-300'
                      : active
                      ? 'text-slate-100'
                      : 'text-slate-500'
                  }
                >
                  {st.label}
                </span>
              </li>
            )
          })}
        </ul>

        <p className="mt-6 text-xs text-slate-500">{displayTagline}</p>
      </div>
    </div>
  )
}

function Spinner() {
  return (
    <span className="relative inline-flex w-5 h-5">
      <span className="absolute inset-0 rounded-full border-2 border-slate-700" />
      <span className="absolute inset-0 rounded-full border-2 border-transparent border-t-brand-400 animate-spin" />
    </span>
  )
}
