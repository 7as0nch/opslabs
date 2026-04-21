import { create } from 'zustand'
import { Attempt } from '../types'

// 当前 attempt 的 UI 状态:最近一次拿到的 Attempt 快照 + 已解锁提示档位
// 远程数据由 React Query 托管,这里只放"跨组件共享的当前值"
interface State {
  current?: Attempt
  hintLevel: number
  set: (a: Attempt | undefined) => void
  patch: (p: Partial<Attempt>) => void
  unlockHint: (level: number) => void
  reset: () => void
}

export const useAttemptStore = create<State>((set) => ({
  current: undefined,
  hintLevel: 0,
  set: (a) => set({ current: a }),
  patch: (p) => set((s) => ({ current: s.current ? { ...s.current, ...p } : undefined })),
  unlockHint: (level) => set((s) => ({ hintLevel: Math.max(s.hintLevel, level) })),
  reset: () => set({ current: undefined, hintLevel: 0 }),
}))
