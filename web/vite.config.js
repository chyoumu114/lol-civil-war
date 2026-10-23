import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发时前端跑在 5173，API/登录代理到本地 Go 服务。
// 后端不在 8080 时可用 VITE_API_TARGET 覆盖，例如：
//   VITE_API_TARGET=http://127.0.0.1:18020 npm run dev
const apiTarget = process.env.VITE_API_TARGET || 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': apiTarget,
      '/auth': apiTarget,
      // 本地英雄头像由 Go 服务从 LCW_CHAMPION_ICON_DIR 提供；只代理子路径，避免影响 /champions 页面路由。
      '/champions/': apiTarget,
    },
  },
  build: {
    outDir: 'dist',
    chunkSizeWarningLimit: 2000,
  },
})
