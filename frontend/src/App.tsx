import { Link, Route, Routes } from 'react-router-dom'
import Home from './pages/Home'
import Landing from './pages/Landing'
import Scenario from './pages/Scenario'

// 全局壳:顶部一个极简导航,路由切换 Landing / Home / Scenario
// 整体 100vh,overflow 完全交给内层子路由管理,避免页面级滚动条
//
// 路由约定:
//   /            → 落地页(项目介绍 + CTA),给第一次访问的人
//   /scenarios   → 场景目录(原 Home),熟门熟路的用户可以直接收藏
//   /s/:slug     → 场景工作区
export default function App() {
  return (
    <div className="h-full flex flex-col overflow-hidden">
      <header className="h-12 shrink-0 flex items-center px-6 border-b border-slate-200 bg-white">
        <Link to="/" className="font-semibold tracking-tight text-slate-800">
          opslabs
        </Link>
        <span className="ml-2 text-xs text-slate-400">浏览器里练真实的线上故障</span>
        <Link
          to="/scenarios"
          className="ml-auto text-xs text-slate-500 hover:text-slate-800"
        >
          场景目录
        </Link>
      </header>
      <main className="flex-1 min-h-0 overflow-hidden">
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/scenarios" element={<Home />} />
          <Route path="/s/:slug" element={<Scenario />} />
        </Routes>
      </main>
    </div>
  )
}
