import React from 'react'
import ReactDOM from 'react-dom/client'
import { LazyMotion, domAnimation } from 'framer-motion'
import App from './App'
import { initTheme } from './stores/theme'
import { initI18n } from './i18n'
import './index.css'

// 在渲染前初始化主题和国际化，避免闪烁
initTheme()
initI18n()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <LazyMotion features={domAnimation} strict>
      <App />
    </LazyMotion>
  </React.StrictMode>,
)
