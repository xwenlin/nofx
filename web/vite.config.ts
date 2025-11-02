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
  },
})
