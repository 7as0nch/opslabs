import { useState } from 'react'

export default function App() {
  const [count, setCount] = useState(0)
  return (
    <main style={{ fontFamily: 'system-ui, sans-serif', padding: 24, maxWidth: 640 }}>
      <h1>hello from dev server</h1>
      <p>如果你看到这一页,说明 Vite dev server 已经成功跑起来了。</p>
      <button onClick={() => setCount((c) => c + 1)}>点了 {count} 次</button>
    </main>
  )
}
