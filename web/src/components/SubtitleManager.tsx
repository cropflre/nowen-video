import { useCallback, useEffect, useState } from 'react'
import { subtitleApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { ExtractedSubtitleFile, SubExtractProgressData, SubtitleInfo } from '@/types'
import { Button, EmptyState, Surface, Tag } from '@/components/design-system'
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@/components/design-system/Modal'
import { Captions, Download, FileText, Image as ImageIcon, Loader2, RefreshCw, Zap } from 'lucide-react'

interface SubtitleManagerProps {
  mediaId: string
  mediaTitle?: string
  onClose: () => void
}

export default function SubtitleManager({ mediaId, mediaTitle, onClose }: SubtitleManagerProps) {
  const [subtitleInfo, setSubtitleInfo] = useState<SubtitleInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [extracting, setExtracting] = useState(false)
  const [extractFormat, setExtractFormat] = useState<'srt' | 'vtt'>('srt')
  const [selectedTracks, setSelectedTracks] = useState<Set<number>>(new Set())
  const [extractResults, setExtractResults] = useState<ExtractedSubtitleFile[]>([])
  const [asyncProgress, setAsyncProgress] = useState<SubExtractProgressData | null>(null)
  const [useAsync, setUseAsync] = useState(false)
  const { on, off } = useWebSocket()

  const loadSubtitleInfo = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await subtitleApi.getTracks(mediaId)
      setSubtitleInfo(response.data.data)
    } catch (exception: any) {
      setError(exception.response?.data?.error || '加载字幕信息失败')
    } finally {
      setLoading(false)
    }
  }, [mediaId])

  useEffect(() => {
    void loadSubtitleInfo()
  }, [loadSubtitleInfo])

  useEffect(() => {
    const handleProgress = (data: SubExtractProgressData) => {
      if (data.media_id === mediaId) setAsyncProgress(data)
    }
    const handleCompleted = (data: SubExtractProgressData) => {
      if (data.media_id !== mediaId) return
      setAsyncProgress(data)
      setExtracting(false)
      if (data.results) setExtractResults(data.results)
    }
    const handleFailed = (data: SubExtractProgressData) => {
      if (data.media_id !== mediaId) return
      setAsyncProgress(data)
      setExtracting(false)
    }

    on(WS_EVENTS.SUB_EXTRACT_STARTED as any, handleProgress)
    on(WS_EVENTS.SUB_EXTRACT_PROGRESS as any, handleProgress)
    on(WS_EVENTS.SUB_EXTRACT_COMPLETED as any, handleCompleted)
    on(WS_EVENTS.SUB_EXTRACT_FAILED as any, handleFailed)
    return () => {
      off(WS_EVENTS.SUB_EXTRACT_STARTED as any, handleProgress)
      off(WS_EVENTS.SUB_EXTRACT_PROGRESS as any, handleProgress)
      off(WS_EVENTS.SUB_EXTRACT_COMPLETED as any, handleCompleted)
      off(WS_EVENTS.SUB_EXTRACT_FAILED as any, handleFailed)
    }
  }, [mediaId, off, on])

  const toggleTrack = (index: number) => {
    setSelectedTracks((previous) => {
      const next = new Set(previous)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }

  const embeddedTracks = subtitleInfo?.embedded || []
  const externalSubs = subtitleInfo?.external || []
  const textTracks = embeddedTracks.filter((track) => !track.bitmap)
  const bitmapTracks = embeddedTracks.filter((track) => track.bitmap)

  const toggleSelectAll = () => {
    if (selectedTracks.size === textTracks.length) setSelectedTracks(new Set())
    else setSelectedTracks(new Set(textTracks.map((track) => track.index)))
  }

  const handleExtractSync = async () => {
    setExtracting(true)
    setExtractResults([])
    setAsyncProgress(null)
    try {
      const tracks = selectedTracks.size > 0 ? Array.from(selectedTracks) : undefined
      const response = await subtitleApi.extractAll(mediaId, extractFormat, tracks)
      setExtractResults(response.data.data.files)
    } catch (exception: any) {
      setError(exception.response?.data?.error || '批量提取失败')
    } finally {
      setExtracting(false)
    }
  }

  const handleExtractAsync = async () => {
    setExtracting(true)
    setExtractResults([])
    setAsyncProgress(null)
    try {
      const tracks = selectedTracks.size > 0 ? Array.from(selectedTracks) : undefined
      await subtitleApi.extractAllAsync(mediaId, mediaTitle, extractFormat, tracks)
    } catch (exception: any) {
      setError(exception.response?.data?.error || '启动异步提取失败')
      setExtracting(false)
    }
  }

  const handleDownload = async (filePath: string) => {
    const url = await subtitleApi.getDownloadUrl(filePath)
    window.open(url, '_blank')
  }

  const getLanguageLabel = (language: string) => {
    const map: Record<string, string> = {
      chi: '中文', zho: '中文', chs: '简体中文', cht: '繁体中文',
      eng: '英语', jpn: '日语', kor: '韩语', fra: '法语', deu: '德语',
      spa: '西班牙语', ita: '意大利语', por: '葡萄牙语', rus: '俄语',
      ara: '阿拉伯语', tha: '泰语', vie: '越南语', und: '未知', '': '未知',
    }
    return map[language] || language
  }

  const getCodecLabel = (codec: string) => {
    const map: Record<string, string> = {
      subrip: 'SRT', ass: 'ASS', ssa: 'SSA', webvtt: 'WebVTT', mov_text: 'MP4 Text',
      hdmv_pgs_subtitle: 'PGS', dvd_subtitle: 'VobSub', dvb_subtitle: 'DVB',
    }
    return map[codec] || codec.toUpperCase()
  }

  return (
    <Modal onClose={onClose} size="lg" ariaLabel="字幕管理" closeOnBackdrop={!extracting}>
      <ModalHeader
        title="字幕管理"
        description={mediaTitle || '查看内嵌与外挂字幕，并批量提取文本字幕'}
        icon={<Captions size={18} aria-hidden="true" />}
        onClose={onClose}
      />

      <ModalBody className="space-y-6">
        {loading ? (
          <SubtitleSkeleton />
        ) : error && !subtitleInfo ? (
          <EmptyState
            icon={<Captions size={24} />}
            title="字幕信息加载失败"
            description={error}
            action={<Button type="button" variant="secondary" onClick={loadSubtitleInfo}>重试</Button>}
          />
        ) : (
          <>
            <section className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">内嵌文本字幕</h3>
                  <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">可直接提取为 SRT 或 WebVTT</p>
                </div>
                {textTracks.length > 0 && (
                  <Button type="button" variant="ghost" size="sm" onClick={toggleSelectAll}>
                    {selectedTracks.size === textTracks.length ? '取消全选' : '全选'}
                  </Button>
                )}
              </div>

              {textTracks.length === 0 ? (
                <Surface className="p-5 text-center text-xs text-[var(--nv-text-tertiary)]">该视频不包含内嵌文本字幕</Surface>
              ) : (
                <div className="space-y-2">
                  {textTracks.map((track) => {
                    const selected = selectedTracks.has(track.index)
                    return (
                      <label
                        key={track.index}
                        className={`flex cursor-pointer items-center gap-3 rounded-[var(--nv-radius-card)] border px-4 py-3 transition-colors ${selected ? 'border-[var(--nv-action-primary)] bg-[var(--nv-bg-active)]' : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] hover:border-[var(--nv-border-hover)]'}`}
                      >
                        <input
                          type="checkbox"
                          checked={selected}
                          onChange={() => toggleTrack(track.index)}
                          className="h-4 w-4 accent-[var(--nv-action-primary)]"
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-medium text-[var(--nv-text-primary)]">轨道 #{track.index}</span>
                            <Tag tone="brand">{getCodecLabel(track.codec)}</Tag>
                            {track.default && <Tag tone="success">默认</Tag>}
                            {track.forced && <Tag tone="warning">强制</Tag>}
                          </div>
                          <div className="mt-1 truncate text-xs text-[var(--nv-text-tertiary)]">
                            {getLanguageLabel(track.language)}{track.title && ` · ${track.title}`}
                          </div>
                        </div>
                      </label>
                    )
                  })}
                </div>
              )}
            </section>

            {bitmapTracks.length > 0 && (
              <section className="space-y-3">
                <div>
                  <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">内嵌图形字幕</h3>
                  <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">PGS / VobSub 等图形字幕需要 OCR 才能转为文本</p>
                </div>
                <div className="space-y-2">
                  {bitmapTracks.map((track) => (
                    <Surface key={track.index} className="flex items-center gap-3 p-4">
                      <ImageIcon size={17} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-sm text-[var(--nv-text-secondary)]">轨道 #{track.index}</span>
                          <Tag tone="warning">{getCodecLabel(track.codec)}</Tag>
                        </div>
                        <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{getLanguageLabel(track.language)} · 图形字幕</div>
                      </div>
                    </Surface>
                  ))}
                </div>
              </section>
            )}

            <section className="space-y-3">
              <div>
                <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">外挂字幕</h3>
                <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">媒体文件旁检测到的独立字幕文件</p>
              </div>
              {externalSubs.length === 0 ? (
                <Surface className="p-5 text-center text-xs text-[var(--nv-text-tertiary)]">未发现外挂字幕文件</Surface>
              ) : (
                <div className="space-y-2">
                  {externalSubs.map((subtitle, index) => (
                    <Surface key={`${subtitle.filename}-${index}`} className="flex items-center gap-3 p-4">
                      <FileText size={17} className="shrink-0 text-[var(--nv-action-primary)]" aria-hidden="true" />
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm text-[var(--nv-text-primary)]">{subtitle.filename}</div>
                        <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{subtitle.format.toUpperCase()} · {getLanguageLabel(subtitle.language)}</div>
                      </div>
                    </Surface>
                  ))}
                </div>
              )}
            </section>

            {textTracks.length > 0 && (
              <Surface className="space-y-4 p-4 sm:p-5">
                <div className="flex items-center gap-2">
                  <Zap size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                  <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">批量提取</h3>
                </div>

                <div className="flex flex-wrap items-center gap-4">
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-[var(--nv-text-secondary)]">输出格式</span>
                    <div className="flex rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] p-1">
                      {(['srt', 'vtt'] as const).map((format) => (
                        <button
                          key={format}
                          type="button"
                          onClick={() => setExtractFormat(format)}
                          className={`rounded-[calc(var(--nv-radius-control)-2px)] px-3 py-1 text-xs font-medium transition-colors ${extractFormat === format ? 'bg-[var(--nv-bg-elevated)] text-[var(--nv-action-primary)] shadow-sm' : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)]'}`}
                          aria-pressed={extractFormat === format}
                        >
                          {format.toUpperCase()}
                        </button>
                      ))}
                    </div>
                  </div>

                  <label className="flex cursor-pointer items-center gap-2 text-xs text-[var(--nv-text-secondary)]">
                    <input type="checkbox" checked={useAsync} onChange={(event) => setUseAsync(event.target.checked)} className="h-4 w-4 accent-[var(--nv-action-primary)]" />
                    异步模式（大文件推荐）
                  </label>
                </div>

                <Button
                  type="button"
                  variant="primary"
                  onClick={useAsync ? handleExtractAsync : handleExtractSync}
                  loading={extracting}
                  className="w-fit"
                >
                  {extracting ? <Loader2 size={15} className="animate-spin" /> : <Zap size={15} />}
                  {extracting
                    ? '提取中...'
                    : selectedTracks.size > 0
                      ? `提取选中的 ${selectedTracks.size} 个轨道`
                      : `提取全部 ${textTracks.length} 个轨道`}
                </Button>

                {asyncProgress && extracting && (
                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-3 text-xs">
                      <span className="truncate text-[var(--nv-text-secondary)]">{asyncProgress.message}</span>
                      <span className="font-mono text-[var(--nv-action-primary)]">{asyncProgress.progress.toFixed(0)}%</span>
                    </div>
                    <div className="h-2 overflow-hidden rounded-full bg-[var(--nv-bg-surface-soft)]">
                      <div
                        className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-300"
                        style={{ width: `${asyncProgress.progress}%` }}
                      />
                    </div>
                  </div>
                )}
              </Surface>
            )}

            {extractResults.length > 0 && (
              <section className="space-y-3">
                <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">提取结果</h3>
                <div className="space-y-2">
                  {extractResults.map((result, index) => (
                    <Surface key={`${result.track_index}-${index}`} className="flex items-center gap-3 p-4">
                      <Tag tone={result.error ? 'danger' : 'success'}>{result.error ? '失败' : '完成'}</Tag>
                      <div className="min-w-0 flex-1">
                        <div className="text-sm text-[var(--nv-text-primary)]">轨道 #{result.track_index}</div>
                        <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">
                          {getLanguageLabel(result.language)} · {getCodecLabel(result.codec)}
                        </div>
                        {result.error && <div className="mt-1 text-xs text-[var(--nv-status-danger)]">{result.error}</div>}
                      </div>
                      {result.path && !result.error && (
                        <Button type="button" variant="secondary" size="sm" onClick={() => handleDownload(result.path!)}>
                          <Download size={13} aria-hidden="true" /> 下载 .{result.format}
                        </Button>
                      )}
                    </Surface>
                  ))}
                </div>
              </section>
            )}

            {error && subtitleInfo && (
              <div className="rounded-[var(--nv-radius-control)] border border-[color-mix(in_srgb,var(--nv-status-danger)_24%,transparent)] bg-[color-mix(in_srgb,var(--nv-status-danger)_8%,transparent)] p-3 text-xs text-[var(--nv-status-danger)]">
                {error}
              </div>
            )}
          </>
        )}
      </ModalBody>

      <ModalFooter className="justify-between">
        <div className="text-xs text-[var(--nv-text-tertiary)]">
          {subtitleInfo ? `共 ${embeddedTracks.length} 个内嵌轨道 · ${externalSubs.length} 个外挂字幕` : '字幕信息'}
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={loadSubtitleInfo} disabled={loading || extracting}>
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} aria-hidden="true" /> 刷新
          </Button>
          <Button type="button" variant="secondary" onClick={onClose}>关闭</Button>
        </div>
      </ModalFooter>
    </Modal>
  )
}

function SubtitleSkeleton() {
  return (
    <div className="space-y-5" aria-label="字幕信息加载中">
      <div className="skeleton h-5 w-40 rounded" />
      {[1, 2, 3].map((item) => <div key={item} className="skeleton h-16 rounded-[var(--nv-radius-card)]" />)}
      <div className="skeleton h-28 rounded-[var(--nv-radius-card)]" />
    </div>
  )
}
