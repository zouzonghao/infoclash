import { fileURLToPath, URL } from 'node:url'

import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  // 根据当前工作目录和模式加载 .env 文件
  // process.cwd() 是项目根目录（/Users/macm4/code/infoclash/frontend）
  const env = loadEnv(mode, process.cwd(), '')

  return {
    plugins: [
      vue(),
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      }
    },
    server: {
      proxy: {
        '/api': {
          target: env.VITE_API_BASE_URL || 'http://localhost:8081',
          changeOrigin: true,
        },
      },
    },
  }
})
