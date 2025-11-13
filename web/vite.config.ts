import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react()],
  base: '/nofx/',
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/nofx-api': {
        target: 'http://localhost:18080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/nofx-api/, '/api'),
      },
    },
    watch: {            // 这个配置对 Docker / WSL / 远程开发环境 特别重要。
      usePolling: true, // 开启轮询
      interval: 100,    // 检查间隔（毫秒）
    },
    hmr: {
      overlay: true,  // 出错时弹窗提示
    },
  },
})
