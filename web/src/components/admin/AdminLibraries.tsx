import { useCallback, useEffect, useRef, useState, type SetStateAction } from 'react'
import { libraryApi } from '@/api'
import LibraryManager from '@/components/LibraryManager'
import { Button } from '@/components/design-system'
import { useWebSocket, WS_EVENTS, type ScanPhaseData, type ScanProgressData, type ScrapeProgressData } from '@/hooks/useWebSocket'
import type { Library } from '@/types'

function without<T>(entries: Record<string, T>, id: string) {
  const { [id]: _entry, ...remaining } = entries
  return remaining
}

export default function AdminLibraries() {
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loadState, setLoadState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [loadAttempt, setLoadAttempt] = useState(0)
  const [scanning, updateScanning] = useState(new Set<string>())
  const [scanProgress, setScanProgress] = useState<Record<string, ScanProgressData>>({})
  const [scrapeProgress, setScrapeProgress] = useState<Record<string, ScrapeProgressData>>({})
  const [scanPhase, setScanPhase] = useState<Record<string, ScanPhaseData>>({})
  const [scanFailures, setScanFailures] = useState<Record<string, string>>({})
  const { connected, on, off } = useWebSocket()
  const scanRevision = useRef(0)

  const setScanning = useCallback((action: SetStateAction<Set<string>>) => {
    scanRevision.current++
    updateScanning(action)
  }, [])

  useEffect(() => {
    let active = true
    setLoadState('loading')
    void libraryApi.list().then(({ data }) => {
      if (!active) return
      setLibraries(data.data || [])
      setLoadState('ready')
    }).catch(() => { if (active) setLoadState('error') })
    return () => { active = false }
  }, [loadAttempt])

  const refreshLibraries = useCallback(() => {
    void libraryApi.list().then(({ data }) => setLibraries(data.data || [])).catch(() => {})
  }, [])

  useEffect(() => {
    const started = (data: ScanProgressData) => {
      setScanning((current) => current.has(data.library_id) ? current : new Set(current).add(data.library_id))
      setScanFailures((current) => without(current, data.library_id))
      setScanProgress((current) => ({ ...current, [data.library_id]: data }))
    }
    const progress = (data: ScanProgressData) => {
      setScanProgress((current) => ({ ...current, [data.library_id]: data }))
    }
    const scrape = (data: ScrapeProgressData) => {
      if (data.library_id) setScrapeProgress((current) => ({ ...current, [data.library_id]: data }))
    }
    const clearProgress = (id: string) => {
      setScanning((current) => {
        const next = new Set(current)
        next.delete(id)
        return next
      })
      setScanProgress((current) => without(current, id))
      setScrapeProgress((current) => without(current, id))
      setScanPhase((current) => without(current, id))
    }
    const completed = (data: ScanProgressData) => {
      clearProgress(data.library_id)
      setScanFailures((current) => without(current, data.library_id))
      refreshLibraries()
    }
    const failed = (data: ScanProgressData) => {
      clearProgress(data.library_id)
      setScanFailures((current) => ({ ...current, [data.library_id]: data.message || '读取影片时出错，请重试。' }))
    }
    const phase = (data: ScanPhaseData) => {
      if (data.phase !== 'completed') {
        setScanning((current) => current.has(data.library_id) ? current : new Set(current).add(data.library_id))
        setScanPhase((current) => ({ ...current, [data.library_id]: data }))
      }
    }
    const listeners = [
      [WS_EVENTS.SCAN_STARTED, started],
      [WS_EVENTS.SCAN_PROGRESS, progress],
      [WS_EVENTS.SCAN_COMPLETED, completed],
      [WS_EVENTS.SCAN_FAILED, failed],
      [WS_EVENTS.SCAN_PHASE, phase],
      [WS_EVENTS.SCRAPE_STARTED, scrape],
      [WS_EVENTS.SCRAPE_PROGRESS, scrape],
      [WS_EVENTS.SCRAPE_COMPLETED, scrape],
    ] as const

    listeners.forEach(([event, handler]) => on(event, handler))
    return () => { listeners.forEach(([event, handler]) => off(event, handler)) }
  }, [off, on, refreshLibraries, setScanning])

  useEffect(() => {
    let active = true
    let timer: number
    const sync = async () => {
      const revision = scanRevision.current
      try {
        const { data } = await libraryApi.scanStatus()
        if (!active || revision !== scanRevision.current) return
        const phases = data.data || []
        const activeIds = new Set(phases.map(({ library_id }) => library_id))
        if (Array.from(scanning).some((id) => !activeIds.has(id))) refreshLibraries()
        updateScanning((previous) => previous.size === activeIds.size && Array.from(previous).every((id) => activeIds.has(id)) ? previous : activeIds)
        setScanPhase(Object.fromEntries(phases.map((phase) => [phase.library_id, phase])))
        setScanProgress((current) => Object.fromEntries(Object.entries(current).filter(([id]) => activeIds.has(id))))
        setScrapeProgress((current) => Object.fromEntries(Object.entries(current).filter(([id]) => activeIds.has(id))))
        if (data.failures) setScanFailures(data.failures)
      } catch {
        // 当前进度仍可通过实时消息更新。查询失败时保留上次状态。
      } finally {
        if (active && scanning.size > 0) timer = window.setTimeout(sync, 3000)
      }
    }
    // 新启动的更新由按钮先显示，稍后再向服务端核对，以覆盖实时消息断开或漏收。
    if (scanning.size > 0) timer = window.setTimeout(sync, 3000)
    else void sync()
    return () => {
      active = false
      window.clearTimeout(timer)
    }
  }, [connected, refreshLibraries, scanning])

  if (loadState === 'loading') return <p role="status" className="text-sm text-[var(--nv-text-secondary)]">正在读取影片库…</p>
  if (loadState === 'error') {
    return (
      <div role="alert" className="flex items-center gap-3 text-sm">
        暂时无法读取影片库。
        <Button variant="secondary" size="sm" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>重试</Button>
      </div>
    )
  }

  return (
    <LibraryManager
      libraries={libraries}
      setLibraries={setLibraries}
      scanning={scanning}
      setScanning={setScanning}
      scanProgress={scanProgress}
      scrapeProgress={scrapeProgress}
      scanPhase={scanPhase}
      scanFailures={scanFailures}
    />
  )
}
