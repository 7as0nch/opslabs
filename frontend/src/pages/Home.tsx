import { Link } from 'react-router-dom'
import ScenarioTable from '../components/ScenarioTable'

// 场景目录:Landing 已经收口了介绍,这里只剩纯题表
export default function Home() {
  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="max-w-6xl w-full mx-auto px-6 pt-6 shrink-0">
        <Link to="/" className="text-xs text-slate-500 hover:text-slate-800">
          ← 返回首页
        </Link>
        <h1 className="mt-2 text-xl font-semibold text-slate-800 mb-1">场景目录</h1>
        <p className="text-sm text-slate-500">从入门到线上故障,在真实 Linux 终端里练排查</p>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto scroll-thin">
        <div className="max-w-6xl mx-auto px-6 py-4 pb-10">
          <ScenarioTable />
        </div>
      </div>
    </div>
  )
}
