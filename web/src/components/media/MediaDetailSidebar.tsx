import type { FileDetail, Media, MediaPlayInfo, PlaybackStatsInfo, TechSpecs } from '@/types'
import { Button, Surface, Tag } from '@/components/design-system'
import { formatSize } from '@/utils/format'
import { Captions, Check, Database, Film } from 'lucide-react'

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
  if (normalized === 'hevc' || normalized === 'h265') return 'HEVC / H.265'
  if (normalized === 'h264') return 'H.264 / AVC'
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

  return (
    <aside className="nv-detail-sidebar nv-detail-summary-strip" aria-label="媒体状态">
      <Surface className="nv-detail-summary-surface">
        <section className="nv-detail-summary-item" aria-label="播放信息">
          <div className="nv-detail-summary-icon" aria-hidden="true"><Film size={15} /></div>
          <div className="nv-detail-summary-copy">
            <span className="nv-detail-summary-label">播放</span>
            <div className="nv-detail-summary-primary">
              <strong>{sourceResolution} · {formatCodec(sourceCodec)}</strong>
              <Tag tone="brand">当前</Tag>
            </div>
            <span className="nv-detail-summary-secondary">
              {sourceSize > 0 ? formatSize(sourceSize) : '文件大小未知'}
              {' · '}
              {playInfo?.can_direct_play ? '支持直接播放' : '自动兼容播放'}
            </span>
          </div>
        </section>

        <section className="nv-detail-summary-item" aria-label="字幕信息">
          <div className="nv-detail-summary-icon" aria-hidden="true"><Captions size={15} /></div>
          <div className="nv-detail-summary-copy">
            <span className="nv-detail-summary-label">字幕</span>
            <div className="nv-detail-summary-primary">
              <strong>{subtitleCount > 0 ? `${subtitleCount} 个字幕来源` : '暂无字幕'}</strong>
            </div>
            <span className="nv-detail-summary-secondary">
              {subtitleCount > 0
                ? `${embeddedSubtitleCount} 内嵌 · ${externalSubtitleCount} 外挂`
                : '播放时仍可使用播放器兼容字幕'}
            </span>
          </div>
          {isAdmin && (
            <Button type="button" variant="ghost" size="sm" className="nv-detail-summary-action" onClick={onManageSubtitles}>
              管理
            </Button>
          )}
        </section>

        <section className="nv-detail-summary-item" aria-label="元数据信息">
          <div className="nv-detail-summary-icon" aria-hidden="true"><Database size={15} /></div>
          <div className="nv-detail-summary-copy">
            <span className="nv-detail-summary-label">元数据</span>
            <div className="nv-detail-summary-sources">
              {metadataSources.length > 0 ? metadataSources.map((source) => (
                <a key={source.name} href={source.href} target="_blank" rel="noopener noreferrer" className="nv-detail-summary-source" title={`${source.name} ${source.value}`}>
                  <Check size={10} aria-hidden="true" />
                  <span>{source.name}</span>
                </a>
              )) : (
                <strong className="nv-detail-summary-local">本地资料</strong>
              )}
            </div>
            <span className="nv-detail-summary-secondary">{formatScrapeStatus(media.scrape_status)}</span>
          </div>
        </section>
      </Surface>
    </aside>
  )
}
