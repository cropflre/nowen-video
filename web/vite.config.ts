import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// 开发环境使用项目专属高位端口，避开常见前端开发端口和影音服务端口。
// scripts/run-dev.bat 会在冲突时自动选择后续空闲端口，并通过环境变量覆盖这里。
const apiProxyTarget = process.env.VITE_API_PROXY_TARGET || 'http://localhost:28888'
const devPort = Number(process.env.WEB_PORT) || 28889

// 历史语言包仍保存旧版本文案。构建和开发转换阶段直接删除所有退役 Pulse 文案，
// 避免已经下线的功能名称继续进入生产 JS，或被旧组件意外重新渲染。
function stripRetiredLocaleEntries(): Plugin {
  return {
    name: 'strip-retired-locale-entries',
    enforce: 'pre',
    transform(code, id) {
      const normalizedId = id.replace(/\\/g, '/')
      if (!normalizedId.includes('/src/i18n/locales/')) return null

      const stripped = code.replace(
        /^\s*'(?:nav\.pulse|pulse\.[^']+)':\s*[^\n]*\r?\n/gm,
        '',
      )
      if (stripped === code) return null
      return { code: stripped, map: null }
    },
  }
}

export default defineConfig({
  plugins: [stripRetiredLocaleEntries(), react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: devPort,
    proxy: {
      '/api': {
        target: apiProxyTarget,
        changeOrigin: true,
        ws: true, // 支持WebSocket代理
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
})
