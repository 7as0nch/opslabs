import { Link } from 'react-router-dom'
import { useScenarios } from '../api/scenario'

// 场景目录:按难度/分类排好,点开任意场景进入终端
// 外壳 h-full + overflow-hidden,卡片列表区单独 overflow-y-auto,避免页面级滚动条
export default function Home() {
  const { data: scenarios = [], isLoading, error } = useScenarios()

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="max-w-5xl w-full mx-auto px-6 pt-6 shrink-0">
        <h1 className="text-xl font-semibold text-slate-800 mb-1">场景目录</h1>
        <p className="text-sm text-slate-500">
          从入门到线上故障,在真实 Linux 终端里练排查
        </p>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto scroll-thin">
        <div className="max-w-5xl mx-auto px-6 py-4 pb-10">
          {isLoading && <div className="text-slate-400">加载中…</div>}
          {error && (
            <div className="text-rose-600">
              加载失败:{(error as Error).message}
            </div>
          )}

          <ul className="grid gap-3 sm:grid-cols-2">
            {scenarios.map((s) => (
              <li key={s.slug}>
                <Link
                  to={`/s/${s.slug}`}
                  className="block bg-white rounded-lg border border-slate-200 p-4 hover:border-brand-500 hover:shadow-sm transition"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="font-medium text-slate-800">{s.title}</div>
                    <DifficultyBadge v={s.difficulty} />
                  </div>
                  <div className="mt-1 text-sm text-slate-500 line-clamp-2">
                    {s.summary}
                  </div>
                  <div className="mt-3 flex flex-wrap gap-1.5 text-xs">
                    <span className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600">
                      {s.category}
                    </span>
                    <span className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600">
                      ~{s.estimatedMinutes}min
                    </span>
                    {s.tags?.slice(0, 3).map((t) => (
                      <span
                        key={t}
                        className="px-1.5 py-0.5 rounded bg-brand-50 text-brand-700"
                      >
                        {t}
                      </span>
                    ))}
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
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
