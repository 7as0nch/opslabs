import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useScenarios } from '../api/scenario'
import type { ScenarioBrief } from '../types'

// 场景题列表(LeetCode 风格):顶部筛选条 + 表格行
// Landing.tsx 和 Home.tsx 共用,所有筛选都在客户端做(7 个场景全量已经下发)
export default function ScenarioTable() {
  const { data: scenarios = [], isLoading, error } = useScenarios()

  const [search, setSearch] = useState('')
  const [diffSet, setDiffSet] = useState<Set<number>>(new Set())
  const [catSet, setCatSet] = useState<Set<string>>(new Set())
  const [tagSet, setTagSet] = useState<Set<string>>(new Set())

  const { allCategories, allTags } = useMemo(() => {
    const cats = new Set<string>()
    const tags = new Set<string>()
    for (const s of scenarios) {
      cats.add(s.category)
      s.tags?.forEach((t) => tags.add(t))
    }
    return {
      allCategories: Array.from(cats).sort(),
      allTags: Array.from(tags).sort(),
    }
  }, [scenarios])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return scenarios.filter((s) => {
      if (q && !s.title.toLowerCase().includes(q) && !s.summary.toLowerCase().includes(q)) {
        return false
      }
      if (diffSet.size > 0 && !diffSet.has(s.difficulty)) return false
      if (catSet.size > 0 && !catSet.has(s.category)) return false
      if (tagSet.size > 0 && !s.tags?.some((t) => tagSet.has(t))) return false
      return true
    })
  }, [scenarios, search, diffSet, catSet, tagSet])

  const hasFilter = search !== '' || diffSet.size > 0 || catSet.size > 0 || tagSet.size > 0
  const reset = () => {
    setSearch('')
    setDiffSet(new Set())
    setCatSet(new Set())
    setTagSet(new Set())
  }

  return (
    <div>
      {/* ===== 筛选条 ===== */}
      <div className="space-y-2.5">
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索题目"
            className="flex-1 min-w-[200px] px-3 py-1.5 text-sm rounded-md border border-slate-300 focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
          />
          {hasFilter && (
            <button
              onClick={reset}
              className="px-3 py-1.5 text-xs text-slate-500 hover:text-slate-800"
            >
              重置
            </button>
          )}
        </div>

        <FilterRow label="难度">
          {[1, 2, 3, 4, 5].map((v) => (
            <DifficultyChip
              key={v}
              v={v}
              active={diffSet.has(v)}
              onClick={() => setDiffSet(toggleSet(diffSet, v))}
            />
          ))}
        </FilterRow>

        <FilterRow label="分类">
          {allCategories.map((c) => (
            <Chip
              key={c}
              active={catSet.has(c)}
              onClick={() => setCatSet(toggleSet(catSet, c))}
            >
              {c}
            </Chip>
          ))}
        </FilterRow>

        {allTags.length > 0 && (
          <FilterRow label="标签">
            {allTags.map((t) => (
              <Chip
                key={t}
                active={tagSet.has(t)}
                onClick={() => setTagSet(toggleSet(tagSet, t))}
                tone="brand"
              >
                {t}
              </Chip>
            ))}
          </FilterRow>
        )}
      </div>

      {/* ===== 表格 ===== */}
      <div className="mt-4">
        {isLoading && <div className="py-6 text-slate-400 text-sm">加载中…</div>}
        {error && (
          <div className="py-6 text-rose-600 text-sm">
            加载失败:{(error as Error).message}
          </div>
        )}
        {!isLoading && !error && filtered.length === 0 && (
          <div className="py-10 text-center text-sm text-slate-400">
            没有符合条件的场景,试试清掉几个筛选条
          </div>
        )}
        {!isLoading && !error && filtered.length > 0 && (
          <div className="overflow-x-auto rounded-md border border-slate-200 bg-white">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 text-xs uppercase tracking-wider text-slate-500">
                <tr>
                  <Th className="w-12 text-center">#</Th>
                  <Th>题目</Th>
                  <Th className="w-28">分类</Th>
                  <Th>标签</Th>
                  <Th className="w-28">难度</Th>
                  <Th className="w-24 text-right">预计</Th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((s, i) => (
                  <Row key={s.slug} s={s} idx={i + 1} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}

function Th({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <th className={`px-3 py-2 text-left font-medium ${className}`}>{children}</th>
  )
}

function Row({ s, idx }: { s: ScenarioBrief; idx: number }) {
  return (
    <tr className="border-t border-slate-100 hover:bg-slate-50 transition">
      <td className="px-3 py-3 text-center text-slate-400 tabular-nums">{idx}</td>
      <td className="px-3 py-3">
        <Link to={`/s/${s.slug}`} className="block group">
          <div className="font-medium text-slate-800 group-hover:text-brand-700">
            {s.title}
          </div>
          <div className="mt-0.5 text-xs text-slate-500 truncate max-w-[480px]">
            {s.summary}
          </div>
        </Link>
      </td>
      <td className="px-3 py-3">
        <span className="inline-block px-1.5 py-0.5 rounded text-xs bg-slate-100 text-slate-600">
          {s.category}
        </span>
      </td>
      <td className="px-3 py-3">
        <div className="flex flex-wrap gap-1">
          {s.tags?.map((t) => (
            <span
              key={t}
              className="px-1.5 py-0.5 rounded text-xs bg-brand-50 text-brand-700"
            >
              {t}
            </span>
          ))}
        </div>
      </td>
      <td className="px-3 py-3">
        <DifficultyBadge v={s.difficulty} />
      </td>
      <td className="px-3 py-3 text-right text-xs text-slate-500 tabular-nums">
        ~{s.estimatedMinutes} min
      </td>
    </tr>
  )
}

function FilterRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-2">
      <span className="shrink-0 w-10 pt-1 text-xs text-slate-400">{label}</span>
      <div className="flex flex-wrap gap-1.5">{children}</div>
    </div>
  )
}

function Chip({
  active,
  onClick,
  children,
  tone = 'slate',
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
  tone?: 'slate' | 'brand'
}) {
  const base = 'px-2 py-0.5 rounded text-xs border transition'
  const off =
    tone === 'brand'
      ? 'border-slate-200 bg-white text-slate-600 hover:border-brand-500'
      : 'border-slate-200 bg-white text-slate-600 hover:border-slate-400'
  const on =
    tone === 'brand'
      ? 'border-brand-500 bg-brand-50 text-brand-700'
      : 'border-slate-500 bg-slate-100 text-slate-800'
  return (
    <button onClick={onClick} className={`${base} ${active ? on : off}`}>
      {children}
    </button>
  )
}

function DifficultyChip({
  v,
  active,
  onClick,
}: {
  v: number
  active: boolean
  onClick: () => void
}) {
  const txt = ['?', 'easy', 'easy', 'mid', 'hard', 'hard'][v] ?? 'mid'
  const tone =
    v <= 2
      ? { on: 'border-emerald-500 bg-emerald-100 text-emerald-700', off: 'border-slate-200 bg-white text-slate-600 hover:border-emerald-400' }
      : v === 3
        ? { on: 'border-amber-500 bg-amber-100 text-amber-700', off: 'border-slate-200 bg-white text-slate-600 hover:border-amber-400' }
        : { on: 'border-rose-500 bg-rose-100 text-rose-700', off: 'border-slate-200 bg-white text-slate-600 hover:border-rose-400' }
  return (
    <button
      onClick={onClick}
      className={`px-2 py-0.5 rounded text-xs border transition ${active ? tone.on : tone.off}`}
    >
      L{v} · {txt}
    </button>
  )
}

function DifficultyBadge({ v }: { v: number }) {
  const txt = ['?', 'easy', 'easy', 'mid', 'hard', 'hard'][v] ?? 'mid'
  const color =
    v <= 2
      ? 'bg-emerald-100 text-emerald-700'
      : v === 3
        ? 'bg-amber-100 text-amber-700'
        : 'bg-rose-100 text-rose-700'
  return (
    <span className={`text-xs px-1.5 py-0.5 rounded ${color}`}>
      L{v} · {txt}
    </span>
  )
}

function toggleSet<T>(s: Set<T>, v: T): Set<T> {
  const next = new Set(s)
  if (next.has(v)) next.delete(v)
  else next.add(v)
  return next
}
