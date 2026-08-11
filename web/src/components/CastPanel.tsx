import { useState, useEffect, useCallback } from 'react'
import { castApi } from '@/api'
import type { CastDevice, CastSession } from '@/types'
import { Monitor, Wifi, Play, Pause, Square, Volume2, RefreshCw, X, Loader2 } from 'lucide-react'
import clsx from 'clsx'

interface CastPanelProps {
  mediaId: string
  mediaTitle?: string
  onClose: () => void
}

export default function CastPanel({ mediaId, mediaTitle, onClose }: CastPanelProps) {
  const [devices, setDevices] = useState<CastDevice[]>([])
  const [session, setSession] = useState<CastSession | null>(null)
  const [loading, setLoading] = useState(true)
  const [casting, setCasting] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadDevices = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const res = await castApi.listDevices()
      setDevices(res.data.data || [])
    } catch {
      setError('获取设备列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  const refreshDevices = async () => {
    setRefreshing(true)
    setError(null)
    try {
      await castApi.refreshDevices()
      await new Promise(resolve => setTimeout(resolve, 2000))
      await loadDevices()
    } catch {
      setError('刷新设备失败')
    } finally {
      setRefreshing(false)
    }
  }

  const startCast = async (deviceId: string) => {
    setCasting(true)
    setError(null)
    try {
      const res = await castApi.startCast({ device_id: deviceId, media_id: mediaId })
      setSession(res.data.data)
    } catch (err: any) {
      setError(err.response?.data?.error || '投屏失败')
    } finally {
      setCasting(false)
    }
  }

  const controlCast = async (action: 'play' | 'pause' | 'stop' | 'seek' | 'volume', value?: number) => {
    if (!session) return
    try {
      await castApi.control(session.id, { action, value })
      if (action === 'stop') {
        setSession(null)
      } else {
        const res = await castApi.getSession(session.id)
        setSession(res.data.data)
      }
    } catch {
      setError('控制操作失败')
    }
  }

  useEffect(() => { loadDevices() }, [loadDevices])

  useEffect(() => {
    if (!session) return
    const timer = setInterval(async () => {
      try {
        const res = await castApi.getSession(session.id)
        setSession(res.data.data)
      } catch {
        setSession(null)
      }
    }, 5000)
    return () => clearInterval(timer)
  }, [session])

  return (
    <div
      className="player-overlay-panel absolute bottom-full right-0 mb-3 w-[360px] max-w-[calc(100vw-24px)]"
      onClick={(e) => e.stopPropagation()}
    >
      <div className="player-overlay-panel-header">
        <div className="player-overlay-panel-heading">
          <div className="player-overlay-panel-title">
            <Monitor size={17} />
            <span>投屏</span>
          </div>
          <div className="player-overlay-panel-subtitle">
            {session ? '远程控制当前播放设备' : '发现并连接同一网络中的设备'}
          </div>
        </div>

        <div className="player-overlay-inline-actions">
          {!session && (
            <button
              onClick={refreshDevices}
              disabled={refreshing}
              className="player-overlay-toolbar-btn"
              title="刷新设备"
            >
              <RefreshCw size={14} className={clsx(refreshing && 'animate-spin')} />
              <span>刷新</span>
            </button>
          )}
          <button onClick={onClose} className="player-overlay-close" title="关闭">
            <X size={16} />
          </button>
        </div>
      </div>

      <div className="player-overlay-body">
        {session ? (
          <>
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="player-overlay-status">
                <span className="player-overlay-status-dot animate-pulse" />
                <span>正在投屏</span>
              </div>
              <span className="player-overlay-item-meta">{session.device?.name || '未知设备'}</span>
            </div>

            <div className="rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] p-3">
              <p className="truncate text-sm font-semibold text-[var(--nv-player-text-primary)]">{mediaTitle || '正在播放'}</p>
              <p className="mt-1 truncate text-[11px] text-[var(--nv-player-text-tertiary)]">{session.device?.name || '未知设备'}</p>

              <div className="mt-4 flex items-center justify-center gap-8">
                <button
                  onClick={() => controlCast(session.status === 'playing' ? 'pause' : 'play')}
                  className="flex h-11 w-11 items-center justify-center rounded-full border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)] transition-[background-color,border-color,transform] hover:bg-[var(--nv-player-accent-soft-hover)] active:scale-[0.98]"
                  title={session.status === 'playing' ? '暂停' : '播放'}
                >
                  {session.status === 'playing' ? <Pause size={18} /> : <Play size={18} className="ml-0.5" />}
                </button>
                <button
                  onClick={() => controlCast('stop')}
                  className="flex h-10 w-10 items-center justify-center rounded-full border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-subtle)] text-[var(--nv-player-text-tertiary)] transition-[background-color,color,transform] hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)] active:scale-[0.98]"
                  title="停止投屏"
                >
                  <Square size={15} />
                </button>
              </div>

              <div className="mt-4 flex items-center gap-3">
                <Volume2 size={14} className="shrink-0 text-[var(--nv-player-text-tertiary)]" />
                <input
                  type="range"
                  min="0"
                  max="100"
                  value={Math.round(session.volume * 100)}
                  onChange={(e) => controlCast('volume', parseInt(e.target.value))}
                  className="player-cast-volume-slider flex-1 cursor-pointer appearance-none"
                  style={{
                    background: `linear-gradient(to right, var(--nv-player-accent) ${session.volume * 100}%, color-mix(in srgb, var(--nv-player-text-primary) 12%, transparent) ${session.volume * 100}%)`,
                  }}
                />
              </div>
            </div>
          </>
        ) : (
          <>
            <div className="player-overlay-section-label">
              <span>可用设备</span>
              {!loading && devices.length > 0 && <span className="normal-case tracking-normal">{devices.length} 台</span>}
            </div>

            {loading ? (
              <div className="player-overlay-empty">
                <div className="player-overlay-empty-inner">
                  <div className="player-overlay-empty-icon">
                    <Loader2 size={22} className="animate-spin" />
                  </div>
                  <div className="player-overlay-empty-title">正在发现设备</div>
                  <div className="player-overlay-empty-desc">正在扫描当前网络中的可投屏设备</div>
                </div>
              </div>
            ) : devices.length === 0 ? (
              <div className="player-overlay-empty">
                <div className="player-overlay-empty-inner">
                  <div className="player-overlay-empty-icon">
                    <Wifi size={25} />
                  </div>
                  <div className="player-overlay-empty-title">未发现投屏设备</div>
                  <div className="player-overlay-empty-desc">请确保设备与服务器处于同一局域网，然后点击刷新重试</div>
                </div>
              </div>
            ) : (
              <div className="player-overlay-list player-overlay-scroll max-h-[300px] overflow-y-auto pr-1">
                {devices.map((device) => (
                  <button
                    key={device.id}
                    onClick={() => startCast(device.id)}
                    disabled={casting}
                    className="player-overlay-item"
                  >
                    <div className="player-overlay-item-primary">
                      <div className="player-overlay-item-icon">
                        <Monitor size={15} />
                      </div>
                      <div className="player-overlay-item-copy">
                        <div className="player-overlay-item-title">{device.name || '未知设备'}</div>
                        <div className="player-overlay-item-desc">
                          {device.manufacturer || device.type.toUpperCase()}
                          {device.model_name && ` · ${device.model_name}`}
                        </div>
                      </div>
                    </div>
                    {casting && <Loader2 size={14} className="animate-spin text-[var(--nv-player-accent)]" />}
                  </button>
                ))}
              </div>
            )}
          </>
        )}

        {error && <div className="player-overlay-alert">{error}</div>}
      </div>
    </div>
  )
}
