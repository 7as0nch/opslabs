import { Component, type ErrorInfo, type ReactNode } from 'react'

/**
 * RunnerErrorBoundary
 *
 * 专门裹在场景 Runner(Terminal / StaticRunner / WebContainerRunner)外层,
 * 拦截 Runner 运行期抛的异常,避免让整个 Scenario 页白屏。
 *
 * 典型触发源:
 *   - WebContainer 内部错(fs 写入 / spawn / boot 重复)
 *   - Monaco editor 在异常 language / readOnly 状态下抛
 *   - iframe onload 前组件树意外重渲染
 *
 * 设计:
 *   - 同步错误走 componentDidCatch;异步错误 React 不接,需要 Runner 自己捕获
 *   - fallback 放个"重启 Runner"按钮 —— 点击后重置 key 强行卸载重挂
 *   - 向上暴露 onReset 让 Scenario 层也可以参与(比如 terminate + 重新 start)
 */
interface Props {
  /** 出错时显示的附加提示文字,默认"Runner 内部错误" */
  label?: string
  /** 点"重新加载"按钮时调用,可用于上层做 terminate / reset 等动作 */
  onReset?: () => void
  children: ReactNode
}

interface State {
  error: Error | null
}

export class RunnerErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // 只记录控制台,不往外发 —— 产品层面有自己的埋点 / 反馈入口
    console.error('[RunnerErrorBoundary] runner crashed', error, info.componentStack)
  }

  handleReset = (): void => {
    this.setState({ error: null })
    this.props.onReset?.()
  }

  render(): ReactNode {
    const { error } = this.state
    if (!error) return this.props.children
    const label = this.props.label ?? 'Runner 内部错误'
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-rose-950/40 border border-rose-800 rounded-lg p-5 text-slate-100 text-sm">
          <div className="text-base font-medium text-rose-300 mb-2">{label}</div>
          <div className="text-slate-300 leading-relaxed">
            运行环境抛出了一个异常,当前界面已被隔离。可以点下方按钮重新加载,
            如果反复出现请把下方报错复制给我们。
          </div>
          <pre className="mt-3 p-2 text-[11px] text-rose-200 bg-black/40 rounded overflow-x-auto whitespace-pre-wrap leading-snug">
            {error.message}
          </pre>
          <button
            type="button"
            onClick={this.handleReset}
            className="mt-3 px-3 py-1.5 rounded bg-rose-700 hover:bg-rose-600 text-white text-xs"
          >
            重新加载 Runner
          </button>
        </div>
      </div>
    )
  }
}

export default RunnerErrorBoundary
