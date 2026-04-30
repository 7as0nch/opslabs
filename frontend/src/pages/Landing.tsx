import { Link } from 'react-router-dom'
import ScenarioTable from '../components/ScenarioTable'

// 落地页:顶部一段产品定位 + CTA,下方直接铺题表
// 题表组件 ScenarioTable 是 Landing 和 /scenarios 共用的
export default function Landing() {
  return (
    <div className="h-full overflow-y-auto scroll-thin bg-gradient-to-b from-slate-50 to-white">
      <div className="max-w-6xl mx-auto px-6 py-8 md:py-10">
        <section>
          <h1 className="text-2xl md:text-3xl font-semibold tracking-tight text-slate-900">
            opslabs · 在浏览器里练真实的线上故障
          </h1>
          <p className="mt-2 text-sm md:text-base text-slate-600 max-w-2xl">
            不用装环境,打开网页就有真实 Linux 终端、在线编辑器、自动判分。下面挑一题开始。
          </p>
          <div className="mt-4 flex flex-wrap gap-2">
            <Link
              to="/s/hello-world"
              className="inline-flex items-center gap-2 px-4 py-2 rounded-md bg-brand-600 text-white text-sm font-medium hover:bg-brand-700 transition"
            >
              开始练习 <span aria-hidden>→</span>
            </Link>
            <Link
              to="/s/hello-world"
              className="inline-flex items-center gap-2 px-4 py-2 rounded-md border border-slate-300 text-slate-700 text-sm font-medium hover:bg-slate-50 transition"
            >
              先看 3 分钟 Demo
            </Link>
          </div>
        </section>

        <section className="mt-8">
          <ScenarioTable />
        </section>

        <footer className="mt-10 pt-5 border-t border-slate-200 text-xs text-slate-400">
          opslabs · 浏览器里练真实的线上故障
        </footer>
      </div>
    </div>
  )
}
