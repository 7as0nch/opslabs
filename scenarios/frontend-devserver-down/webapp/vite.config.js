import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Vite 配置里 server 段的 host / port 必须跟 package.json 的 "dev" 脚本一致,
// 否则 dev server 会监听跟预期不一样的地址,用户外部 curl 不通
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 3000,
  },
})
