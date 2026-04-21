import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发端:5173
// 后端:6039(见 backend/configs/config.yaml)
// 代理 /v1 -> backend,前端 api/*.ts 里直接 fetch('/v1/...')
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/v1': {
        target: 'http://localhost:6039',
        changeOrigin: true,
      },
    },
  },
})
