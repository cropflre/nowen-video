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
    <div className="absolute bottom-full right-0 mb-2 w-80 rounded-xl shadow-2xl"
      style={{
        background: 'rgba(11, 17, 32, 0.92)',
        border: '1px solid rgba(0, 240, 255, 0.1)',
        backdropFilter: 'blur(20px)',
      }}
    >
      {/* 头部 */}
      <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid rgba(0, 240, 255, 0.08)' }}>
        <div className="flex items-center gap-2">
          <Monitor size={18} className="text-neon-blue" />
          <h3 className="text-sm font-medium text-white">投屏</h3>
        </div>
        <button
          onClick={onClose}
          className="rounded-lg p-1 text-surface-400 transition-colors hover:text-white hover:bg-white/5"
        >
          <X size={16} />
        </button>
      </div>

      {/* 正在投屏 */}
      {session && (
        <div className="p-4" style={{ borderBottom: '1px solid rgba(0, 240, 255, 0.08)' }}>
          <div className="mb-2 flex items-center gap-2">
            <div className="h-2 w-2 animate-pulse rounded-full" style={{ background: 'var(--neon-green)', boxShadow: '0 0 6px rgba(0, 255, 136, 0.5)' }} />
            <span className="text-xs" style={{ color: 'var(--neon-green)' }}>正在投屏</span>
          </div>
          <p className="mb-1 text-sm font-medium text-white">{mediaTitle || '正在播放'}</p>
          <p className="mb-3 text-xs text-surface-500">{session.device?.name || '未知设备'}</p>

          <div className="flex items-center justify-center gap-3">
            <button
              onClick={() => controlCast(session.status === 'playing' ? 'pause' : 'play')}
              className="rounded-full p-2.5 text-white transition-all hover:scale-105"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: '0 0 15px rgba(0, 240, 255, 0.3)',
              }}
            >
              {session.status === 'playing' ? <Pause size={18} /> : <Play size={18} />}
            </button>
            <button
              onClick={() => controlCast('stop')}
              className="rounded-full p-2 text-surface-300 transition-colors hover:text-white"
              style={{ background: 'rgba(0, 240, 255, 0.06)', border: '1px solid rgba(0, 240, 255, 0.1)' }}
              title="停止投屏"
            >
              <Square size={16} />
            </button>
          </div>

          <div className="mt-3 flex items-center gap-2">
            <Volume2 size={14} className="text-surface-400" />
            <input
              type="range"
              min="0"
              max="100"
              value={Math.round(session.volume * 100)}
              onChange={(e) => controlCast('volume', parseInt(e.target.value))}
              className="h-1 flex-1 cursor-pointer appearance-none rounded-full"
              style={{
                background: `linear-gradient(to right, var(--neon-blue) ${session.volume * 100}%, rgba(255,255,255,0.1) ${session.volume * 100}%)`,
              }}
            />
          </div>
        </div>
      )}

      {/* 设备列表 */}
      {!session && (
        <div className="p-2">
          <div className="flex items-center justify-between px-2 py-1">
            <span className="text-[10px] font-bold uppercase tracking-widest text-neon-blue/40">可用设备</span>
            <button
              onClick={refreshDevices}
              disabled={refreshing}
              className="flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-surface-400 transition-colors hover:text-neon-blue hover:bg-neon-blue/5 disabled:opacity-50"
            >
              <RefreshCw size={12} className={clsx(refreshing && 'animate-spin')} />
              刷新
            </button>
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 size={20} className="animate-spin text-neon-blue/40" />
            </div>
          ) : devices.length === 0 ? (
            <div className="py-8 text-center">
              <Wifi size={32} className="mx-auto mb-2 text-surface-600" />
              <p className="text-sm text-surface-400">未发现投屏设备</p>
              <p className="mt-1 text-xs text-surface-600">请确保设备与服务器在同一网络</p>
            </div>
          ) : (
            <div className="space-y-1">
              {devices.map((device) => (
                <button
                  key={device.id}
                  onClick={() => startCast(device.id)}
                  disabled={casting}
                  className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left transition-colors hover:bg-neon-blue/5 disabled:opacity-50"
                >
                  <div className="flex h-9 w-9 items-center justify-center rounded-xl"
                    style={{ background: 'rgba(0, 240, 255, 0.06)', border: '1px solid rgba(0, 240, 255, 0.1)' }}
                  >
                    <Monitor size={16} className="text-neon-blue" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-white">{device.name || '未知设备'}</p>
                    <p className="truncate text-xs text-surface-500">
                      {device.manufacturer || device.type.toUpperCase()}
                      {device.model_name && ` · ${device.model_name}`}
                    </p>
                  </div>
                  {casting && <Loader2 size={16} className="animate-spin text-neon-blue" />}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* 错误提示 */}
      {error && (
        <div className="px-4 py-2" style={{ borderTop: '1px solid rgba(239, 68, 68, 0.1)' }}>
          <p className="text-xs text-red-400">{error}</p>
        </div>
      )}
    </div>
  )
}
