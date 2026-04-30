/**
 * TitleBar
 *
 * Hills Lite 风格自绘标题栏：
 *   [拖拽区：Logo + 标题] ... [全局搜索] ... [最小化 / 最大化 / 关闭]
 *
 * - 仅在 Tauri 桌面端渲染（浏览器 / 移动端返回 null）
 * - 高度 32px（紧凑模式），与 Windows 11 系统标题栏一致
 * - 拖拽使用 Tauri 的 `data-tauri-drag-region` 约定
 * - 三键图标采用 Fluent Icons，符合 WinUI 3 设计
 */
import { useEffect, useState, useCallback, memo } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  Subtract16Regular,
  Square16Regular,
  SquareMultiple16Regular,
  Dismiss16Regular,
  Search20Regular,
} from '@fluentui/react-icons'
import { desktop } from '@/desktop/bridge'

function TitleBarImpl() {
  const [maximized, setMaximized] = useState(false)
  const [search, setSearch] = useState('')
  const navigate = useNavigate()
  const location = useLocation()

  // 初始化并定时同步最大化状态（窗口可以通过系统快捷键/拖边改变）
  useEffect(() => {
    let alive = true
    const sync = async () => {
      const isMax = await desktop.windowIsMaximized()
      if (alive) setMaximized(isMax)
    }
    sync()
    const id = window.setInterval(sync, 800)
    return () => {
      alive = false
      clearInterval(id)
    }
  }, [])

  const onMinimize = useCallback(() => {
    desktop.windowMinimize()
  }, [])

  const onToggleMax = useCallback(async () => {
    const next = await desktop.windowToggleMaximize()
    setMaximized(next)
  }, [])

  const onClose = useCallback(() => {
    desktop.windowClose()
  }, [])

  const onSearchSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault()
      const q = search.trim()
      if (!q) return
      navigate(`/search?q=${encodeURIComponent(q)}`)
    },
    [search, navigate],
  )

  // 在播放页隐藏标题栏（沉浸模式）
  const isPlayer = location.pathname.startsWith('/play/')
  if (isPlayer) return null

  return (
    <div
      data-tauri-drag-region
      className="nv-titlebar"
      style={{
        height: 36,
        display: 'flex',
        alignItems: 'center',
        flex: '0 0 auto',
        paddingLeft: 12,
        paddingRight: 0,
        background: 'transparent',
        borderBottom: '1px solid var(--border-default)',
        position: 'relative',
        zIndex: 1000,
        userSelect: 'none',
      }}
    >
      {/* Logo 区（拖拽） */}
      <div
        data-tauri-drag-region
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 12,
          fontWeight: 600,
          letterSpacing: '0.12em',
          color: 'var(--text-primary)',
          pointerEvents: 'none',
        }}
      >
        <span className="text-neon text-neon-glow" style={{ fontSize: 13 }}>
          N
        </span>
        <span style={{ color: 'var(--text-secondary)' }}>OWEN · VIDEO</span>
      </div>

      {/* 中间搜索条（Hills 的核心视觉之一） */}
      <form
        onSubmit={onSearchSubmit}
        style={{
          flex: 1,
          display: 'flex',
          justifyContent: 'center',
          padding: '0 16px',
          pointerEvents: 'auto',
        }}
      >
        <label
          className="nv-titlebar-search"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            width: 'min(480px, 70%)',
            height: 26,
            padding: '0 10px',
            borderRadius: 999,
            background: 'rgba(11, 17, 32, 0.6)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-tertiary)',
            fontSize: 12,
            transition: 'all .2s ease',
          }}
        >
          <Search20Regular style={{ width: 14, height: 14 }} />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索影片、剧集、演员..."
            style={{
              flex: 1,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: 'var(--text-primary)',
              fontSize: 12,
            }}
          />
        </label>
      </form>

      {/* 三键（不拖拽） */}
      <div
        className="nv-titlebar-controls"
        style={{ display: 'flex', flex: '0 0 auto', pointerEvents: 'auto' }}
      >
        <TitleBarButton
          label="最小化"
          onClick={onMinimize}
          icon={<Subtract16Regular />}
        />
        <TitleBarButton
          label={maximized ? '还原' : '最大化'}
          onClick={onToggleMax}
          icon={maximized ? <SquareMultiple16Regular /> : <Square16Regular />}
        />
        <TitleBarButton
          label="关闭"
          onClick={onClose}
          icon={<Dismiss16Regular />}
          danger
        />
      </div>
    </div>
  )
}

interface TitleBarButtonProps {
  label: string
  icon: React.ReactNode
  onClick: () => void
  danger?: boolean
}

function TitleBarButton({ label, icon, onClick, danger }: TitleBarButtonProps) {
  const [hover, setHover] = useState(false)
  const bg = hover
    ? danger
      ? '#e81123'
      : 'rgba(255, 255, 255, 0.08)'
    : 'transparent'
  const color = hover && danger ? '#ffffff' : 'var(--text-secondary)'
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={onClick}
      style={{
        width: 46,
        height: 36,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        border: 'none',
        background: bg,
        color,
        cursor: 'pointer',
        transition: 'background .12s ease, color .12s ease',
      }}
    >
      {icon}
    </button>
  )
}

const isTauri =
  typeof window !== 'undefined' &&
  Boolean((window as any).__TAURI_INTERNALS__ || (window as any).__TAURI__)

function TitleBar() {
  // 浏览器环境不渲染
  if (!isTauri) return null
  return <TitleBarImpl />
}

export default memo(TitleBar)
