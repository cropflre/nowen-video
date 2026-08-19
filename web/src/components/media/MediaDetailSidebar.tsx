import type { FileDetail, Media, MediaPlayInfo, PlaybackStatsInfo, TechSpecs } from '@/types'
import { formatSize } from '@/utils/format'
import { Captions, Check, Database, Film, PlayCircle } from 'lucide-react'

interface MediaDetailSidebarProps {
  media: Media
  playInfo: MediaPlayInfo | null
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
  playbackStats: PlaybackStatsInfo | null
  isAdmin: boolean
  onManageSubtitles: () => void
}

function parseExternalSubtitlePaths(value: string): string[] {
  const raw = value?.trim()
  if (!raw) return []

  if (raw.startsWith('[')) {
    try {
      const parsed = JSON.parse(raw) as unknown
      if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean)
    } catch {
      // Fall back to delimiter parsing for legacy values.
    }
  }

  return raw.split(/[\n,;|]+/).map((item) => item.trim()).filter(Boolean)
}

function formatCodec(codec?: string): string {
  if (!codec) return '未知编码'
  const normalized = codec.toLowerCase()
  if (normalized === 'hevc' || normalized === 'h265') return 'H.265'
  if (normalized === 'h264') return 'H.264'
  if (normalized === 'av1') return 'AV1'
  return codec.toUpperCase()
}

function formatScrapeStatus(status?: Media['scrape_status']): string {
  const map: Record<string, string> = {
    scraped: '已匹配',
    partial: '部分匹配',
    failed: '匹配失败',
    pending: '待处理',
    manual: '手动维护',
  }
  return status ? (map[status] || status) : '未记录'
}

export default function MediaDetailSidebar({
  media,
  playInfo,
  techSpecs,
  fileInfo,
  playbackStats: _playbackStats,
  isAdmin,
  onManageSubtitles,
}: MediaDetailSidebarProps) {
  const sourceSize = fileInfo?.file_size || media.file_size || 0
  const sourceCodec = playInfo?.video_codec || media.video_codec
  const sourceResolution = media.resolution || '原始画质'
  const embeddedSubtitleCount = (techSpecs?.streams || []).filter((stream) => stream.codec_type === 'subtitle').length
  const externalSubtitleCount = parseExternalSubtitlePaths(media.subtitle_paths).length
  const subtitleCount = embeddedSubtitleCount + externalSubtitleCount

  const metadataSources = [
    media.tmdb_id > 0 ? { name: 'TMDb', value: String(media.tmdb_id), href: `https://www.themoviedb.org/${media.media_type === 'episode' ? 'tv' : 'movie'}/${media.tmdb_id}` } : null,
    media.douban_id ? { name: '豆瓣', value: media.douban_id, href: `https://movie.douban.com/subject/${media.douban_id}/` } : null,
    media.bangumi_id > 0 ? { name: 'Bangumi', value: String(media.bangumi_id), href: `https://bgm.tv/subject/${media.bangumi_id}` } : null,
  ].filter((item): item is { name: string; value: string; href: string } => item !== null)

  const playbackTitle = [
    `${sourceResolution} · ${formatCodec(sourceCodec)}`,
    sourceSize > 0 ? formatSize(sourceSize) : null,
    playInfo?.can_direct_play ? '支持直接播放' : '自动兼容播放',
  ].filter(Boolean).join(' · ')

  const subtitleTitle = subtitleCount > 0
    ? `${subtitleCount} 个字幕来源（${embeddedSubtitleCount} 内嵌 · ${externalSubtitleCount} 外挂）`
    : '暂未检测到字幕轨道'

  return (
    <aside className="nv-detail-sidebar nv-detail-summary-strip" aria-label="媒体状态">
      <div className="nv-detail-summary-tags">
        <div className="nv-detail-status-tag nv-detail-status-tag--playback" title={playbackTitle}>
          <Film size={13} aria-hidden="true" />
          <strong>{sourceResolution}</strong>
          <span>{formatCodec(sourceCodec)}</span>
          {playInfo?.can_direct_play && (
            <span className="nv-detail-status-tag-accent">
              <PlayCircle size={11} aria-hidden="true" />
              直放
            </span>
          )}
        </div>

        {isAdmin ? (
          <button
            type="button"
            className="nv-detail-status-tag nv-detail-status-tag--interactive"
            title={`${subtitleTitle}，点击管理字幕`}
            onClick={onManageSubtitles}
          >
            <Captions size={13} aria-hidden="true" />
            <strong>{subtitleCount > 0 ? `${subtitleCount} 字幕` : '无字幕'}</strong>
            <span className="nv-detail-status-tag-hint">管理</span>
          </button>
        ) : (
          <div className="nv-detail-status-tag" title={subtitleTitle}>
            <Captions size={13} aria-hidden="true" />
            <strong>{subtitleCount > 0 ? `${subtitleCount} 字幕` : '无字幕'}</strong>
          </div>
        )}

        {metadataSources.length > 0 ? metadataSources.map((source) => (
          <a
            key={source.name}
            href={source.href}
            target="_blank"
            rel="noopener noreferrer"
            className="nv-detail-status-tag nv-detail-status-tag--interactive"
            title={`${source.name} ${source.value} · ${formatScrapeStatus(media.scrape_status)}`}
          >
            <Check size={12} aria-hidden="true" />
            <strong>{source.name}</strong>
          </a>
        )) : (
          <div className="nv-detail-status-tag" title={`本地资料 · ${formatScrapeStatus(media.scrape_status)}`}>
            <Database size={13} aria-hidden="true" />
            <strong>本地资料</strong>
          </div>
        )}
      </div>
    </aside>
  )
}
