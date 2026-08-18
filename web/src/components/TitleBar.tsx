/**
 * Nowen Video Desktop 2.0 自绘标题栏。
 *
 * 仅在 Tauri 桌面环境渲染；浏览器端直接返回 null。
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
        borderBottom: '1px solid var(--nv-border-subtle)',
        position: 'relative',
        zIndex: 1000,
        userSelect: 'none',
      }}
    >
      <div
        data-tauri-drag-region
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 12,
          fontWeight: 600,
          letterSpacing: '0.12em',
          color: 'var(--nv-text-primary)',
          pointerEvents: 'none',
        }}
      >
        <span className="text-[var(--nv-action-primary)]" style={{ fontSize: 13 }}>
          N
        </span>
        <span style={{ color: 'var(--nv-text-secondary)' }}>OWEN · VIDEO</span>
      </div>

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
            background: 'var(--nv-bg-control)',
            border: '1px solid var(--nv-border-default)',
            color: 'var(--nv-text-tertiary)',
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
              color: 'var(--nv-text-primary)',
              fontSize: 12,
            }}
          />
        </label>
      </form>

      <div
        className="nv-titlebar-controls"
        style={{ display: 'flex', flex: '0 0 auto', pointerEvents: 'auto' }}
      >
        <TitleBarButton label="最小化" onClick={onMinimize} icon={<Subtract16Regular />} />
        <TitleBarButton
          label={maximized ? '还原' : '最大化'}
          onClick={onToggleMax}
          icon={maximized ? <SquareMultiple16Regular /> : <Square16Regular />}
        />
        <TitleBarButton label="关闭" onClick={onClose} icon={<Dismiss16Regular />} danger />
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
      : 'var(--nv-bg-hover)'
    : 'transparent'
  const color = hover && danger ? '#ffffff' : 'var(--nv-text-secondary)'

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

function TitleBar() {
  if (!desktop.isDesktop) return null
  return <TitleBarImpl />
}

export default memo(TitleBar)
