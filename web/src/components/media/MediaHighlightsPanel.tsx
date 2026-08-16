import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Clapperboard, Clock3, Loader2, Play, RefreshCw, Sparkles, Trash2 } from 'lucide-react'
import { Button, EmptyState } from '@/components/design-system'
import { useToast } from '@/components/Toast'
import { streamApi } from '@/api'
import { mediaAnalysisApi, type MediaAnalysisTask, type MediaHighlight } from '@/api/mediaAnalysis'
import { useWebSocket, WS_EVENTS, type MediaAnalysisProgressData } from '@/hooks/useWebSocket'
import { formatErrMsg } from '@/utils/error'

interface MediaHighlightsPanelProps {
  mediaId: string
  isAdmin: boolean
}

const stageLabels: Record<string, string> = {
  queued: '等待分析',
  probe: '检查媒体文件',
  coarse_analysis: '快速采样媒体',
  refine_analysis: '精筛候选片段',
  audio_analysis: '分析音频能量',
  scene_analysis: '分析场景变化',
  ranking: '筛选精彩片段',
  thumbnail: '生成片段缩略图',
  preview: '生成动态预览',
  persist: '保存分析结果',
  completed: '分析完成',
  interrupted: '分析已中断',
}

function formatTime(seconds: number) {
  const safe = Math.max(0, Math.floor(seconds || 0))
  const h = Math.floor(safe / 3600)
  const m = Math.floor((safe % 3600) / 60)
  const s = safe % 60
  return h > 0
    ? `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
    : `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function assetUrl(url?: string) {
  return url ? streamApi.withTokenUrl(url) : ''
}

function analysisLabel(method: string) {
  if (method === 'heuristic') return '结构推断'
  if (method === 'scene') return '场景分析'
  if (method === 'sparse_audio') return '快速音频采样'
  if (method === 'sparse_audio_scene') return '快速音频 + 场景'
  return '音频 + 场景'
}

export default function MediaHighlightsPanel({ mediaId, isAdmin }: MediaHighlightsPanelProps) {
  const navigate = useNavigate()
  const toast = useToast()
  const { connected: wsConnected, on: onWS, off: offWS } = useWebSocket()
  const [highlights, setHighlights] = useState<MediaHighlight[]>([])
  const [stale, setStale] = useState(false)
  const [task, setTask] = useState<MediaAnalysisTask | null>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [hoveredId, setHoveredId] = useState<string | null>(null)
  const pollRef = useRef<number | null>(null)
  const hoverTimerRef = useRef<number | null>(null)

  const running = task?.status === 'pending' || task?.status === 'running'

  const loadHighlights = useCallback(async () => {
    const response = await mediaAnalysisApi.getHighlights(mediaId)
    setHighlights(response.data.data?.highlights || [])
    setStale(Boolean(response.data.data?.stale))
  }, [mediaId])

  const loadStatus = useCallback(async () => {
    const response = await mediaAnalysisApi.getStatus(mediaId)
    const next = response.data.data || null
    setTask(next)
    return next
  }, [mediaId])

  const refresh = useCallback(async () => {
    try {
      await Promise.all([loadHighlights(), loadStatus()])
    } finally {
      setLoading(false)
    }
  }, [loadHighlights, loadStatus])

  useEffect(() => {
    setLoading(true)
    void refresh().catch(() => setLoading(false))
  }, [refresh])

  useEffect(() => {
    const handleProgress = (data: MediaAnalysisProgressData) => {
      if (data.media_id !== mediaId) return
      setTask((previous) => ({
        id: data.task_id || previous?.id || '',
        media_id: data.media_id,
        task_type: previous?.task_type || 'media_highlight',
        status: data.status,
        stage: data.stage,
        progress: data.progress,
        error: data.error || '',
        result: previous?.result,
        started_at: previous?.started_at,
        completed_at: previous?.completed_at,
        created_at: previous?.created_at,
        updated_at: previous?.updated_at,
      }))
      if (data.status === 'completed') {
        void loadHighlights()
      }
    }

    onWS(WS_EVENTS.MEDIA_ANALYSIS_PROGRESS, handleProgress)
    onWS(WS_EVENTS.MEDIA_ANALYSIS_COMPLETE, handleProgress)
    return () => {
      offWS(WS_EVENTS.MEDIA_ANALYSIS_PROGRESS, handleProgress)
      offWS(WS_EVENTS.MEDIA_ANALYSIS_COMPLETE, handleProgress)
    }
  }, [loadHighlights, mediaId, offWS, onWS])

  useEffect(() => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
    if (!running) return

    // WebSocket is the primary progress path. HTTP stays as a low-frequency
    // recovery path for proxies/browsers where the socket is unavailable.
    const interval = wsConnected ? 15000 : 5000
    pollRef.current = window.setInterval(() => {
      void loadStatus().then((next) => {
        if (!next || (next.status !== 'pending' && next.status !== 'running')) {
          if (pollRef.current) window.clearInterval(pollRef.current)
          pollRef.current = null
          if (next?.status === 'completed') void loadHighlights()
        }
      }).catch(() => {})
    }, interval)

    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [loadHighlights, loadStatus, running, wsConnected])

  useEffect(() => () => {
    if (hoverTimerRef.current) window.clearTimeout(hoverTimerRef.current)
  }, [])

  const beginHover = (id: string) => {
    if (hoverTimerRef.current) window.clearTimeout(hoverTimerRef.current)
    hoverTimerRef.current = window.setTimeout(() => {
      setHoveredId(id)
      hoverTimerRef.current = null
    }, 300)
  }

  const endHover = (id: string) => {
    if (hoverTimerRef.current) {
      window.clearTimeout(hoverTimerRef.current)
      hoverTimerRef.current = null
    }
    setHoveredId((current) => current === id ? null : current)
  }

  const handleAnalyze = async () => {
    setSubmitting(true)
    try {
      const response = await mediaAnalysisApi.analyzeHighlights(mediaId)
      setTask(response.data.data)
      toast.success(highlights.length ? '已开始重新分析精彩片段' : '已开始生成精彩片段')
    } catch (error) {
      toast.error(formatErrMsg(error, '启动精彩片段分析失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!window.confirm('确定删除当前媒体已生成的全部精彩片段吗？原始影片不会被删除。')) return
    setDeleting(true)
    try {
      await mediaAnalysisApi.deleteHighlights(mediaId)
      setHighlights([])
      setStale(false)
      toast.success('精彩片段已删除')
    } catch (error) {
      toast.error(formatErrMsg(error, '删除精彩片段失败'))
    } finally {
      setDeleting(false)
    }
  }

  const progress = Math.max(0, Math.min(100, task?.progress || 0))
  const stageLabel = stageLabels[task?.stage || ''] || task?.stage || '本地媒体分析'
  const orderedHighlights = useMemo(
    () => [...highlights].sort((a, b) => a.start_time - b.start_time),
    [highlights],
  )

  if (loading) {
    return (
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {[0, 1, 2].map((item) => (
          <div key={item} className="overflow-hidden rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-surface-soft)]">
            <div className="skeleton aspect-video w-full" />
            <div className="space-y-2 p-4">
              <div className="skeleton h-5 w-2/3 rounded" />
              <div className="skeleton h-4 w-1/2 rounded" />
            </div>
          </div>
        ))}
      </div>
    )
  }

  if (running) {
    return (
      <div className="rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-surface-soft)] p-6 shadow-[var(--nv-shadow-card)]">
        <div className="flex items-start gap-4">
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-[var(--nv-accent-soft)] text-[var(--nv-accent)]">
            <Loader2 size={21} className="animate-spin" aria-hidden="true" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h3 className="font-semibold text-[var(--nv-text-primary)]">正在生成精彩片段 · {Math.round(progress)}%</h3>
                <p className="mt-1 text-sm text-[var(--nv-text-secondary)]">{stageLabel}。可离开当前页面，任务会在后台继续。</p>
              </div>
              <span className="rounded-full border border-[var(--nv-border-subtle)] px-3 py-1 text-xs text-[var(--nv-text-tertiary)]">本地 FFmpeg · 快速分析</span>
            </div>
            <div className="mt-5 h-2 overflow-hidden rounded-full bg-[var(--nv-surface-elevated)]">
              <div className="h-full rounded-full bg-[var(--nv-accent)] transition-[width] duration-300" style={{ width: `${progress}%` }} />
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (orderedHighlights.length === 0) {
    return (
      <EmptyState
        className="nv-detail-tab-empty-state"
        icon={<Clapperboard size={23} aria-hidden="true" />}
        title="暂无精彩片段"
        description={task?.status === 'failed'
          ? `上次分析失败：${task.error || '未知错误'}`
          : task?.status === 'interrupted'
            ? '上次分析因服务重启而中断，可以重新生成。'
            : '当前媒体尚未生成精彩片段。精彩片段完全在本机使用 FFmpeg 分析，不需要 AI。'}
        action={isAdmin ? (
          <Button type="button" variant="primary" size="sm" onClick={handleAnalyze} disabled={submitting}>
            {submitting ? <Loader2 size={15} className="animate-spin" /> : <Sparkles size={15} />}
            生成精彩片段
          </Button>
        ) : undefined}
      />
    )
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold text-[var(--nv-text-primary)]">精彩片段</h2>
            <span className="rounded-full bg-[var(--nv-surface-elevated)] px-2.5 py-1 text-xs text-[var(--nv-text-tertiary)]">{orderedHighlights.length} 个</span>
          </div>
          <p className="mt-1 text-sm text-[var(--nv-text-secondary)]">本地 FFmpeg 稀疏采样生成；动态预览只在首次悬停时按需生成。</p>
        </div>
        {isAdmin && (
          <div className="flex items-center gap-2">
            <Button type="button" variant="secondary" size="sm" onClick={handleAnalyze} disabled={submitting}>
              {submitting ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
              重新分析
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={handleDelete} disabled={deleting}>
              {deleting ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
              删除结果
            </Button>
          </div>
        )}
      </div>

      {stale && (
        <div className="rounded-[var(--nv-radius-control)] border border-amber-400/30 bg-amber-400/10 px-4 py-3 text-sm text-amber-700 dark:text-amber-300">
          源媒体文件已发生变化，当前时间点可能已经失效。建议重新分析精彩片段。
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {orderedHighlights.map((item) => {
          const duration = Math.max(0, item.end_time - item.start_time)
          const usePreview = hoveredId === item.id && item.preview_url
          const poster = assetUrl(usePreview ? item.preview_url : item.thumbnail_url)
          return (
            <button
              key={item.id}
              type="button"
              className="group overflow-hidden rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-surface-soft)] text-left shadow-[var(--nv-shadow-card)] transition duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-accent)]/40 hover:shadow-[var(--nv-shadow-elevated)]"
              onMouseEnter={() => beginHover(item.id)}
              onMouseLeave={() => endHover(item.id)}
              onFocus={() => setHoveredId(item.id)}
              onBlur={() => endHover(item.id)}
              onClick={() => navigate(`/play/${mediaId}?start=${item.start_time.toFixed(3)}&end=${item.end_time.toFixed(3)}&mode=highlight`)}
            >
              <div className="relative aspect-video overflow-hidden bg-[var(--nv-surface-elevated)]">
                {poster ? (
                  <img
                    src={poster}
                    alt=""
                    className="h-full w-full object-cover transition duration-300 group-hover:scale-[1.02]"
                    onError={() => { if (usePreview) setHoveredId(null) }}
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-[var(--nv-text-tertiary)]"><Clapperboard size={32} /></div>
                )}
                <div className="absolute inset-0 bg-gradient-to-t from-black/55 via-transparent to-transparent opacity-80" />
                <div className="absolute inset-0 flex items-center justify-center">
                  <span className="flex h-11 w-11 items-center justify-center rounded-full border border-white/25 bg-black/35 text-white shadow-lg backdrop-blur-md transition group-hover:scale-105 group-hover:bg-black/50">
                    <Play size={18} fill="currentColor" />
                  </span>
                </div>
                <span className="absolute bottom-2 right-2 rounded-full bg-black/55 px-2 py-1 text-[11px] font-medium text-white backdrop-blur-md">{duration.toFixed(0)} 秒</span>
              </div>

              <div className="p-4">
                <div className="flex items-start justify-between gap-3">
                  <h3 className="min-w-0 flex-1 truncate font-semibold text-[var(--nv-text-primary)]">{item.title}</h3>
                  <span className="shrink-0 rounded-full bg-[var(--nv-accent-soft)] px-2 py-0.5 text-xs font-semibold text-[var(--nv-accent)]">{item.score.toFixed(1)}</span>
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
                  <span className="inline-flex items-center gap-1"><Clock3 size={12} />{formatTime(item.start_time)} → {formatTime(item.end_time)}</span>
                  <span>{analysisLabel(item.analysis_method)}</span>
                </div>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
