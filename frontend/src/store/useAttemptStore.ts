import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import { Attempt } from '../types'

// 当前 attempt 的 UI 状态:最近一次拿到的 Attempt 快照 + 已解锁提示档位
// 远程数据由 React Query 托管,这里只放"跨组件共享的当前值"
//
// 为什么要 persist 到 localStorage:
//   - 刷新后 Zustand 默认会重新初始化为 undefined
//   - Scenario.tsx 的 start effect 看到 current 为空会再次 mutate → 后端重复建容器
//   - 持久化 current.attemptId/scenarioSlug 之后,effect 命中 scenarioSlug 守卫,
//     直接走轮询 /v1/attempts/{id} 把状态拉回来,不再重新 start
//
// 注意:只把 current / hintLevel 放进去,函数引用不走持久化
interface State {
  current?: Attempt
  hintLevel: number
  set: (a: Attempt | undefined) => void
  patch: (p: Partial<Attempt>) => void
  unlockHint: (level: number) => void
  reset: () => void
}

export const useAttemptStore = create<State>()(
  persist(
    (set) => ({
      current: undefined,
      hintLevel: 0,
      set: (a) => set({ current: a }),
      patch: (p) => set((s) => ({ current: s.current ? { ...s.current, ...p } : undefined })),
      unlockHint: (level) => set((s) => ({ hintLevel: Math.max(s.hintLevel, level) })),
      reset: () => set({ current: undefined, hintLevel: 0 }),
    }),
    {
      name: 'opslabs-attempt',
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ current: s.current, hintLevel: s.hintLevel }) as State,
      version: 1,
    },
  ),
)
