import type { FileDetail, Media, MediaPlayInfo, PlaybackStatsInfo, TechSpecs } from '@/types'
import { Button, Surface, Tag } from '@/components/design-system'
import { formatSize } from '@/utils/format'
import {
  Captions,
  Check,
  Clock3,
  Database,
  Film,
  HardDrive,
  PlayCircle,
} from 'lucide-react'

interface MediaDetailSidebarProps {
  media: Media
  playInfo: MediaPlayInfo | null
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
  playbackStats: PlaybackStatsInfo | null
  isAdmin: boolean
  onManageSubtitles: () => void
}

interface SubtitleSummary {
  label: string
  detail: string
  default?: boolean
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

function compactPath(path: string): string {
  if (path.length <= 54) return path
  const parts = path.replace(/\\/g, '/').split('/').filter(Boolean)
  if (parts.length < 3) return `…${path.slice(-50)}`
  return `…/${parts.slice(-3).join('/')}`
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
  playbackStats,
  isAdmin,
  onManageSubtitles,
}: MediaDetailSidebarProps) {
  const sourcePath = media.file_path || [fileInfo?.file_dir, fileInfo?.file_name].filter(Boolean).join('/')
  const sourceSize = fileInfo?.file_size || media.file_size || 0
  const sourceCodec = playInfo?.video_codec || media.video_codec
  const sourceResolution = media.resolution || '原始画质'

  const embeddedSubtitles = (techSpecs?.streams || [])
    .filter((stream) => stream.codec_type === 'subtitle')
    .slice(0, 4)
    .map<SubtitleSummary>((stream, index) => ({
      label: `内嵌字幕 ${index + 1}`,
      detail: `${stream.codec_name?.toUpperCase() || 'SUB'} · Stream #${stream.index}`,
      default: index === 0,
    }))

  const externalSubtitles = parseExternalSubtitlePaths(media.subtitle_paths)
    .slice(0, Math.max(0, 4 - embeddedSubtitles.length))
    .map<SubtitleSummary>((path) => ({
      label: path.split(/[\\/]/).pop() || '外挂字幕',
      detail: '外挂字幕',
    }))

  const subtitleItems = [...embeddedSubtitles, ...externalSubtitles]
  const metadataSources = [
    media.tmdb_id > 0 ? { name: 'TMDb', value: String(media.tmdb_id), href: `https://www.themoviedb.org/${media.media_type === 'episode' ? 'tv' : 'movie'}/${media.tmdb_id}` } : null,
    media.douban_id ? { name: '豆瓣', value: media.douban_id, href: `https://movie.douban.com/subject/${media.douban_id}/` } : null,
    media.bangumi_id > 0 ? { name: 'Bangumi', value: String(media.bangumi_id), href: `https://bgm.tv/subject/${media.bangumi_id}` } : null,
  ].filter((item): item is { name: string; value: string; href: string } => item !== null)

  const stats = playbackStats as unknown as {
    total_play_count?: number
    unique_viewers?: number
    last_played_at?: string
  } | null

  return (
    <aside className="nv-detail-sidebar" aria-label="媒体摘要">
      <Surface className="nv-detail-side-card">
        <div className="nv-detail-side-card-header">
          <div>
            <span className="nv-detail-side-eyebrow">Playback</span>
            <h2>播放版本</h2>
          </div>
          <Film size={16} aria-hidden="true" />
        </div>

        <div className="nv-detail-version-list">
          <div className="nv-detail-version-item is-active">
            <div className="nv-detail-version-dot" />
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 items-center gap-2">
                <strong className="truncate">{sourceResolution} 原始文件</strong>
                <Tag tone="brand">当前</Tag>
              </div>
              <div className="nv-detail-version-meta">
                <span>{formatCodec(sourceCodec)}</span>
                {fileInfo?.file_ext && <span>{fileInfo.file_ext.replace(/^\./, '').toUpperCase()}</span>}
                {sourceSize > 0 && <span>{formatSize(sourceSize)}</span>}
              </div>
              {sourcePath && <code title={sourcePath}>{compactPath(sourcePath)}</code>}
            </div>
          </div>

          {playInfo?.is_preprocessed && (
            <div className="nv-detail-version-item">
              <div className="nv-detail-version-dot" />
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-2">
                  <strong className="truncate">预处理 HLS</strong>
                  <Tag tone="success">秒开</Tag>
                </div>
                <div className="nv-detail-version-meta">
                  <span>自适应播放</span>
                  <span>{playInfo.preprocess_status || 'ready'}</span>
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="nv-detail-side-footnote">
          <PlayCircle size={13} aria-hidden="true" />
          <span>{playInfo?.can_direct_play ? '当前源支持直接播放' : '播放器将自动选择兼容播放方式'}</span>
        </div>
      </Surface>

      <div id="detail-subtitles" className="scroll-mt-24">
        <Surface className="nv-detail-side-card">
          <div className="nv-detail-side-card-header">
            <div>
              <span className="nv-detail-side-eyebrow">Subtitles</span>
              <h2>字幕</h2>
            </div>
            <Captions size={16} aria-hidden="true" />
          </div>

          {subtitleItems.length > 0 ? (
            <div className="nv-detail-subtitle-list">
              {subtitleItems.map((item, index) => (
                <div key={`${item.label}-${index}`} className="nv-detail-subtitle-item">
                  <div className="min-w-0 flex-1">
                    <strong className="truncate">{item.label}</strong>
                    <span>{item.detail}</span>
                  </div>
                  {item.default && <Tag tone="brand">默认</Tag>}
                </div>
              ))}
              {(embeddedSubtitles.length + externalSubtitles.length) >= 4 && (
                <div className="nv-detail-subtitle-more">更多字幕可在字幕管理中查看</div>
              )}
            </div>
          ) : (
            <div className="nv-detail-side-empty">暂未检测到字幕轨道</div>
          )}

          {isAdmin && (
            <Button type="button" variant="ghost" size="sm" className="mt-3 w-full" onClick={onManageSubtitles}>
              <Captions size={14} aria-hidden="true" />
              管理字幕
            </Button>
          )}
        </Surface>
      </div>

      <Surface className="nv-detail-side-card">
        <div className="nv-detail-side-card-header">
          <div>
            <span className="nv-detail-side-eyebrow">Metadata</span>
            <h2>元数据来源</h2>
          </div>
          <Database size={16} aria-hidden="true" />
        </div>

        <div className="nv-detail-source-list">
          {metadataSources.length > 0 ? metadataSources.map((source) => (
            <a key={source.name} href={source.href} target="_blank" rel="noopener noreferrer" className="nv-detail-source-item">
              <span className="nv-detail-source-status"><Check size={11} aria-hidden="true" /></span>
              <strong>{source.name}</strong>
              <code>{source.value}</code>
            </a>
          )) : (
            <div className="nv-detail-side-empty">暂无外部元数据匹配</div>
          )}
        </div>

        <div className="nv-detail-source-meta">
          <div>
            <span>刮削状态</span>
            <strong>{formatScrapeStatus(media.scrape_status)}</strong>
          </div>
          {media.last_scrape_at && (
            <div>
              <span>最近更新</span>
              <strong>{new Date(media.last_scrape_at).toLocaleString()}</strong>
            </div>
          )}
          {stats?.total_play_count !== undefined && (
            <div>
              <span>播放次数</span>
              <strong>{stats.total_play_count}</strong>
            </div>
          )}
        </div>
      </Surface>

      <Surface className="nv-detail-side-card nv-detail-side-card--quiet">
        <div className="nv-detail-side-mini-row">
          <HardDrive size={14} aria-hidden="true" />
          <span>{sourceSize > 0 ? formatSize(sourceSize) : '文件大小未知'}</span>
        </div>
        {stats?.last_played_at && (
          <div className="nv-detail-side-mini-row">
            <Clock3 size={14} aria-hidden="true" />
            <span>最近播放 {new Date(stats.last_played_at).toLocaleDateString()}</span>
          </div>
        )}
      </Surface>
    </aside>
  )
}
