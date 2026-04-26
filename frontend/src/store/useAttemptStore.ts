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

// safeLocalStorage 给 createJSONStorage 用的容错壳
//   - 隐身模式 / 第三方 cookie 禁用 / 存储满 / 文件协议等场景下 localStorage 会抛,
//     如果不 try/catch,任何 set/patch 都会炸到整个 React 组件树
//   - 读失败视为"没存过",返回 null;写失败只 console.warn,不打断渲染
//   - 一次探测后对 MissingStorage 状态常驻,避免每次操作都重复抛异常
const safeLocalStorage = (): Storage => {
  const noop: Storage = {
    length: 0,
    clear: () => {},
    getItem: () => null,
    key: () => null,
    removeItem: () => {},
    setItem: () => {},
  }
  let real: Storage | null = null
  try {
    real = typeof window !== 'undefined' ? window.localStorage : null
  } catch {
    real = null
  }
  if (!real) return noop
  return {
    get length() {
      try {
        return real!.length
      } catch {
        return 0
      }
    },
    clear: () => {
      try {
        real!.clear()
      } catch {
        /* ignored */
      }
    },
    getItem: (k) => {
      try {
        return real!.getItem(k)
      } catch {
        return null
      }
    },
    key: (i) => {
      try {
        return real!.key(i)
      } catch {
        return null
      }
    },
    removeItem: (k) => {
      try {
        real!.removeItem(k)
      } catch {
        /* ignored */
      }
    },
    setItem: (k, v) => {
      try {
        real!.setItem(k, v)
      } catch (e) {
        // 最常见:QuotaExceededError。用 warn 而不是 error,避免把正常的存储压力
        // 事件冒泡成"看起来像 bug"的日志。用户刷新页面时会丢掉 attempt 复用,
        // 但不会白屏
        console.warn('[useAttemptStore] localStorage.setItem failed, state not persisted', e)
      }
    },
  }
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
      storage: createJSONStorage(() => safeLocalStorage()),
      partialize: (s) => ({ current: s.current, hintLevel: s.hintLevel }) as State,
      version: 1,
    },
  ),
)
