import { useState } from 'react'
import { Link } from 'react-router-dom'
import { formatSize, formatDuration, formatDurationShort, formatDate } from '@/utils/format'
import type { Media, MediaPlayInfo, MediaPerson } from '@/types'
import {
  FileText,
  Copy,
  ChevronDown,
  ChevronUp,
  Monitor,
  Music,
  Subtitles,
  Clapperboard,
} from 'lucide-react'
import clsx from 'clsx'
import { useToast } from '@/components/Toast'

interface MediaInfoSectionProps {
  media: Media
  playInfo: MediaPlayInfo | null
  persons: MediaPerson[]
  isAdmin: boolean
}

export default function MediaInfoSection({ media, playInfo: _playInfo, persons, isAdmin }: MediaInfoSectionProps) {
  const toast = useToast()
  const [overviewExpanded, setOverviewExpanded] = useState(false)
  const isLongOverview = (media.overview?.length || 0) > 200

  const copyFilePath = () => {
    if (media.file_path) {
      navigator.clipboard.writeText(media.file_path)
        .then(() => toast.success('文件路径已复制'))
        .catch(() => {})
    }
  }

  const directors = persons.filter(p => p.role === 'director')
  const actors = persons.filter(p => p.role === 'actor')

  return (
    <>
      {/* 剧情简介 — 可展开/收起 */}
      {media.overview && (
        <section>
          <div className="relative">
            <p className={clsx(
              'text-sm leading-relaxed text-surface-300 transition-all duration-500',
              !overviewExpanded && isLongOverview && 'line-clamp-3'
            )}>
              {media.overview}
            </p>
            {isLongOverview && !overviewExpanded && (
              <div className="absolute bottom-0 left-0 right-0 h-8 bg-gradient-to-t from-surface-950 to-transparent" />
            )}
          </div>
          {isLongOverview && (
            <button
              onClick={() => setOverviewExpanded(!overviewExpanded)}
              className="mt-2 flex items-center gap-1 text-xs font-medium text-neon transition-colors hover:text-neon-blue"
            >
              {overviewExpanded ? (
                <><ChevronUp size={14} />收起</>
              ) : (
                <><ChevronDown size={14} />展开全部</>
              )}
            </button>
          )}
        </section>
      )}

      {/* 类型标签 */}
      {media.genres && (
        <section className="flex flex-wrap gap-2">
          {media.genres.split(',').map((genre) => (
            <Link
              key={genre}
              to={`/search?q=${encodeURIComponent(genre.trim())}`}
              className="rounded-xl px-4 py-1.5 text-sm text-surface-300 transition-all duration-300 hover:text-white hover:scale-[1.04]"
              style={{
                background: 'var(--neon-blue-4)',
                border: '1px solid var(--neon-blue-8)',
              }}
            >
              {genre.trim()}
            </Link>
          ))}
        </section>
      )}

      {/* 影片详情（导演/演员/国家/语言/出品方） */}
      {(media.country || media.language || media.studio || persons.length > 0) && (
        <section>
          <div className="grid gap-x-8 gap-y-3 text-sm sm:grid-cols-2">
            {directors.length > 0 && (
              <div className="flex gap-2">
                <span className="shrink-0 text-surface-500">导演：</span>
                <span style={{ color: 'var(--text-primary)' }}>
                  {directors.map(d => d.person?.name || '').filter(Boolean).join(' / ')}
                </span>
              </div>
            )}
            {actors.length > 0 && (
              <div className="flex gap-2 sm:col-span-2">
                <span className="shrink-0 text-surface-500">演员：</span>
                <span className="line-clamp-2" style={{ color: 'var(--text-primary)' }}>
                  {actors.slice(0, 8).map(a => {
                    const name = a.person?.name || ''
                    return a.character ? `${name}（饰 ${a.character}）` : name
                  }).filter(Boolean).join(' / ')}
                </span>
              </div>
            )}
            {media.country && (
              <div className="flex gap-2">
                <span className="shrink-0 text-surface-500">制片国家：</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.country}</span>
              </div>
            )}
            {media.language && (
              <div className="flex gap-2">
                <span className="shrink-0 text-surface-500">语言：</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.language}</span>
              </div>
            )}
            {media.studio && (
              <div className="flex gap-2">
                <span className="shrink-0 text-surface-500">出品公司：</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.studio}</span>
              </div>
            )}
            {/* 数据来源标识 */}
            {(media.tmdb_id > 0 || media.douban_id || media.bangumi_id > 0) && (
              <div className="flex gap-2 sm:col-span-2">
                <span className="shrink-0 text-surface-500">数据来源：</span>
                <div className="flex flex-wrap gap-1.5">
                  {media.tmdb_id > 0 && (
                    <a
                      href={`https://www.themoviedb.org/${media.media_type === 'episode' ? 'tv' : 'movie'}/${media.tmdb_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium transition-opacity hover:opacity-80"
                      style={{ background: 'rgba(1,180,228,0.12)', color: '#01b4e4' }}
                    >
                      🎬 TMDb #{media.tmdb_id}
                    </a>
                  )}
                  {media.douban_id && (
                    <a
                      href={`https://movie.douban.com/subject/${media.douban_id}/`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium transition-opacity hover:opacity-80"
                      style={{ background: 'rgba(0,180,20,0.12)', color: '#00b414' }}
                    >
                      🎯 豆瓣 #{media.douban_id}
                    </a>
                  )}
                  {media.bangumi_id > 0 && (
                    <a
                      href={`https://bgm.tv/subject/${media.bangumi_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium transition-opacity hover:opacity-80"
                      style={{ background: 'rgba(240,145,153,0.12)', color: '#f09199' }}
                    >
                      📺 Bangumi #{media.bangumi_id}
                    </a>
                  )}
                </div>
              </div>
            )}
          </div>
        </section>
      )}

      {/* 文件信息卡片 — 仅管理员可见 */}
      {isAdmin && (
        <section>
          <h3 className="mb-3 flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <FileText size={16} className="text-neon/60" />
            文件信息
          </h3>
          <div className="glass-panel rounded-xl p-5">
            {media.file_path && (
              <div className="mb-4 flex items-start gap-3">
                <span className="shrink-0 text-xs font-medium text-surface-500">文件位置：</span>
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  <code className="flex-1 truncate rounded-lg px-3 py-1.5 text-xs"
                    style={{ background: 'var(--bg-subtle)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                  >
                    {media.file_path}
                  </code>
                  <button
                    onClick={copyFilePath}
                    className="shrink-0 rounded-lg p-1.5 text-surface-500 transition-colors hover:text-neon hover:bg-neon-blue/5"
                    title="复制路径"
                  >
                    <Copy size={14} />
                  </button>
                </div>
              </div>
            )}
            <div className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
              <div>
                <span className="text-surface-500">文件大小：</span>
                <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{formatSize(media.file_size)}</span>
              </div>
              <div>
                <span className="text-surface-500">添加日期：</span>
                <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{formatDate(media.created_at)}</span>
              </div>
              {media.duration > 0 && (
                <div>
                  <span className="text-surface-500">总时长：</span>
                  <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{formatDuration(media.duration)}</span>
                </div>
              )}
            </div>
          </div>
        </section>
      )}

      {/* 视频信息 — 三栏卡片（视频/音频/字幕） */}
      <section>
        <h3 className="mb-3 flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Clapperboard size={16} className="text-neon/60" />
          视频信息
        </h3>
        <div className="grid gap-4 sm:grid-cols-3">
          {/* 视频 */}
          <div className="glass-panel-subtle rounded-xl p-4">
            <div className="mb-2 flex items-center gap-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: 'var(--neon-blue-8)' }}>
                <Monitor size={14} className="text-neon" />
              </div>
              <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>视频</span>
            </div>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              {[media.resolution, media.video_codec].filter(Boolean).join(' ')}
              {media.duration > 0 && ` · ${formatDurationShort(media.duration)}`}
            </p>
          </div>
          {/* 音频 */}
          <div className="glass-panel-subtle rounded-xl p-4">
            <div className="mb-2 flex items-center gap-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: 'var(--neon-purple-8)' }}>
                <Music size={14} className="text-purple-400" />
              </div>
              <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>音频</span>
            </div>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              {media.audio_codec || '未知编码'}
            </p>
          </div>
          {/* 字幕 */}
          <div className="glass-panel-subtle rounded-xl p-4">
            <div className="mb-2 flex items-center gap-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: 'rgba(0, 255, 136, 0.08)' }}>
                <Subtitles size={14} style={{ color: 'var(--neon-green)' }} />
              </div>
              <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>字幕</span>
            </div>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              {media.subtitle_paths ? '有外挂字幕' : '无外挂字幕'}
            </p>
          </div>
        </div>
      </section>
    </>
  )
}
