import React from 'react'
import ReactDOM from 'react-dom/client'
import { LazyMotion, domAnimation } from 'framer-motion'
import App from './App'
import FluentAppProvider from './components/FluentAppProvider'
import { initTheme } from './stores/theme'
import { initI18n } from './i18n'
import './index.css'
import './styles/fluent.css'

// 在渲染前初始化主题和国际化，避免闪烁
initTheme()
initI18n()

// 标记桌面端运行时（供 CSS 切换透明/Mica 样式）
if (typeof window !== 'undefined') {
  const w = window as unknown as { __TAURI_INTERNALS__?: unknown; __TAURI__?: unknown }
  if (w.__TAURI_INTERNALS__ || w.__TAURI__) {
    document.documentElement.setAttribute('data-runtime', 'tauri')
  }
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <FluentAppProvider>
      <LazyMotion features={domAnimation} strict>
        <App />
      </LazyMotion>
    </FluentAppProvider>
  </React.StrictMode>,
)
