import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { Media, MediaPlayInfo, MediaPerson } from '@/types'
import {
  ChevronDown,
  ChevronUp,
  ExternalLink,
} from 'lucide-react'
import clsx from 'clsx'
import { useTranslation } from '@/i18n'

interface MediaInfoSectionProps {
  media: Media
  playInfo: MediaPlayInfo | null
  persons: MediaPerson[]
}

// 格式化分钟为 Xh Ym
function formatRuntime(minutes: number): string {
  if (!minutes || minutes <= 0) return ''
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  if (h === 0) return `${m}min`
  if (m === 0) return `${h}h`
  return `${h}h ${m}min`
}

export default function MediaInfoSection({ media, playInfo: _playInfo, persons }: MediaInfoSectionProps) {
  const { t } = useTranslation()
  const [plotExpanded, setPlotExpanded] = useState(false)
  const [origPlotExpanded, setOrigPlotExpanded] = useState(false)
  const isLongPlot = (media.overview?.length || 0) > 200
  const isLongOrigPlot = (media.original_plot?.length || 0) > 120

  const directors = persons.filter(p => p.role === 'director')
  const actors = persons.filter(p => p.role === 'actor')

  // 从标题里提取番号兜底（后端无 num 时）
  const extractedNum = (() => {
    if (media.num) return media.num
    const m = media.title?.match(/\b([A-Z]{2,6})-?(\d{2,5})\b/i)
    return m ? `${m[1].toUpperCase()}-${m[2]}` : ''
  })()

  // 标签拆分：tags 字段优先（NFO 中用户标签），否则回退 genres
  const tagList = (media.tags || '').split(',').map(s => s.trim()).filter(Boolean)
  const genreList = (media.genres || '').split(',').map(s => s.trim()).filter(Boolean)

  // 是否存在元数据表格内容
  const hasMetaTable = !!(
    extractedNum ||
    media.maker ||
    media.publisher ||
    media.label ||
    media.studio ||
    media.release_date ||
    media.premiered ||
    media.mpaa ||
    media.country ||
    media.language ||
    media.runtime ||
    media.website ||
    directors.length > 0 ||
    actors.length > 0
  )

  return (
    <>
      {/* 顶部徽标行：番号、分级、分辨率 */}
      {(extractedNum || media.mpaa || media.resolution) && (
        <section className="flex flex-wrap items-center gap-2">
          {extractedNum && (
            <span
              className="inline-flex items-center rounded-md px-2.5 py-1 text-xs font-semibold tracking-wider"
              style={{
                background: 'rgba(255,140,0,0.12)',
                border: '1px solid rgba(255,140,0,0.35)',
                color: '#ff8c00',
              }}
              title="番号"
            >
              {extractedNum}
            </span>
          )}
          {media.mpaa && (
            <span
              className="inline-flex items-center rounded-md px-2.5 py-1 text-xs font-semibold"
              style={{
                background: 'rgba(220,38,38,0.12)',
                border: '1px solid rgba(220,38,38,0.35)',
                color: '#ef4444',
              }}
              title={t('mediaInfo.mpaa')}
            >
              {media.mpaa}
            </span>
          )}
          {media.resolution && (
            <span
              className="inline-flex items-center rounded-md px-2.5 py-1 text-xs font-medium"
              style={{
                background: 'var(--neon-blue-4)',
                border: '1px solid var(--neon-blue-8)',
                color: 'var(--text-secondary)',
              }}
            >
              {media.resolution}
            </span>
          )}
        </section>
      )}

      {/* 原始标题（与主标题分开的独立副标题） */}
      {media.orig_title && media.orig_title !== media.title && (
        <section>
          <p className="text-sm italic" style={{ color: 'var(--text-muted)' }}>
            {media.orig_title}
          </p>
        </section>
      )}

      {/* 宣传语 */}
      {media.tagline && (
        <section>
          <p className="text-sm italic" style={{ color: 'var(--text-secondary)' }}>
            "{media.tagline}"
          </p>
        </section>
      )}

      {/* 剧情摘要 (outline)：短摘要，总是全量展示 */}
      {media.outline && media.outline !== media.overview && (
        <section>
          <h4 className="mb-1 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
            {t('mediaInfo.outline')}
          </h4>
          <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
            {media.outline}
          </p>
        </section>
      )}

      {/* 详细剧情 (plot = overview)：可展开/收起 */}
      {media.overview && (
        <section>
          {media.outline && media.outline !== media.overview && (
            <h4 className="mb-1 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
              {t('mediaInfo.plot')}
            </h4>
          )}
          <div className="relative">
            <p className={clsx(
              'text-sm leading-relaxed transition-all duration-500',
              !plotExpanded && isLongPlot && 'line-clamp-3'
            )} style={{ color: 'var(--text-secondary)' }}>
              {media.overview}
            </p>
            {isLongPlot && !plotExpanded && (
              <div className="absolute bottom-0 left-0 right-0 h-8" style={{ background: `linear-gradient(to top, var(--bg-base), transparent)` }} />
            )}
          </div>
          {isLongPlot && (
            <button
              onClick={() => setPlotExpanded(!plotExpanded)}
              className="mt-2 flex items-center gap-1 text-xs font-medium text-neon transition-colors hover:text-neon-blue"
            >
              {plotExpanded ? (
                <><ChevronUp size={14} />{t('mediaInfo.collapse')}</>
              ) : (
                <><ChevronDown size={14} />{t('mediaInfo.expandAll')}</>
              )}
            </button>
          )}
        </section>
      )}

      {/* 原文剧情 (originalplot) */}
      {media.original_plot && (
        <section>
          <h4 className="mb-1 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
            {t('mediaInfo.originalPlot')}
          </h4>
          <div className="relative">
            <p className={clsx(
              'text-sm leading-relaxed italic transition-all duration-500',
              !origPlotExpanded && isLongOrigPlot && 'line-clamp-2'
            )} style={{ color: 'var(--text-muted)' }}>
              {media.original_plot}
            </p>
          </div>
          {isLongOrigPlot && (
            <button
              onClick={() => setOrigPlotExpanded(!origPlotExpanded)}
              className="mt-2 flex items-center gap-1 text-xs font-medium text-neon transition-colors hover:text-neon-blue"
            >
              {origPlotExpanded ? (
                <><ChevronUp size={14} />{t('mediaInfo.collapse')}</>
              ) : (
                <><ChevronDown size={14} />{t('mediaInfo.expandAll')}</>
              )}
            </button>
          )}
        </section>
      )}

      {/* 分类 (genres) */}
      {genreList.length > 0 && (
        <section>
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
            {t('mediaInfo.genres').replace(/[:：]\s*$/, '')}
          </h4>
          <div className="flex flex-wrap gap-2">
            {genreList.map((genre) => (
              <Link
                key={`g-${genre}`}
                to={`/search?q=${encodeURIComponent(genre)}`}
                className="rounded-xl px-4 py-1.5 text-sm transition-all duration-300 hover:scale-[1.04]"
                style={{
                  background: 'var(--neon-blue-4)',
                  border: '1px solid var(--neon-blue-8)',
                  color: 'var(--text-secondary)',
                }}
              >
                {genre}
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* 标签 (tags) — 仅当与 genres 不同时单独展示 */}
      {tagList.length > 0 && tagList.join(',') !== genreList.join(',') && (
        <section>
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
            {t('mediaInfo.tags').replace(/[:：]\s*$/, '')}
          </h4>
          <div className="flex flex-wrap gap-2">
            {tagList.map((tg) => (
              <Link
                key={`t-${tg}`}
                to={`/search?q=${encodeURIComponent(tg)}`}
                className="rounded-lg px-3 py-1 text-xs transition-all duration-300 hover:scale-[1.04]"
                style={{
                  background: 'rgba(255,255,255,0.04)',
                  border: '1px solid rgba(255,255,255,0.08)',
                  color: 'var(--text-muted)',
                }}
              >
                #{tg}
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* 影片详情表 */}
      {hasMetaTable && (
        <section>
          <div className="grid gap-x-8 gap-y-3 text-sm sm:grid-cols-2">
            {extractedNum && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.num')}</span>
                <span className="font-mono tracking-wide" style={{ color: 'var(--text-primary)' }}>{extractedNum}</span>
              </div>
            )}
            {directors.length > 0 && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.director')}</span>
                <span style={{ color: 'var(--text-primary)' }}>
                  {directors.map(d => d.person?.name || '').filter(Boolean).join(' / ')}
                </span>
              </div>
            )}
            {actors.length > 0 && (
              <div className="flex gap-2 sm:col-span-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.actors')}</span>
                <span className="line-clamp-2" style={{ color: 'var(--text-primary)' }}>
                  {actors.slice(0, 8).map(a => {
                    const name = a.person?.name || ''
                    return a.character ? `${name}${t('mediaInfo.asCharacter', { character: a.character })}` : name
                  }).filter(Boolean).join(' / ')}
                </span>
              </div>
            )}
            {media.runtime > 0 && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.runtime')}</span>
                <span style={{ color: 'var(--text-primary)' }}>
                  {formatRuntime(media.runtime)}
                  <span className="ml-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                    ({t('mediaInfo.runtimeMinutes', { minutes: media.runtime })})
                  </span>
                </span>
              </div>
            )}
            {(media.release_date || media.premiered) && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.releaseDate')}</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.release_date || media.premiered}</span>
              </div>
            )}
            {media.mpaa && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.mpaa')}</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.mpaa}</span>
              </div>
            )}
            {media.country && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.country')}</span>
                <span style={{ color: 'var(--text-primary)' }}>
                  {media.country}
                  {media.country_code && <span className="ml-1 text-xs" style={{ color: 'var(--text-muted)' }}>({media.country_code})</span>}
                </span>
              </div>
            )}
            {media.language && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.language')}</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.language}</span>
              </div>
            )}
            {media.studio && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.studio')}</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.studio}</span>
              </div>
            )}
            {media.maker && media.maker !== media.studio && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.maker')}</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.maker}</span>
              </div>
            )}
            {media.publisher && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.publisher')}</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.publisher}</span>
              </div>
            )}
            {media.label && media.label !== media.publisher && (
              <div className="flex gap-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.label')}</span>
                <span style={{ color: 'var(--text-primary)' }}>{media.label}</span>
              </div>
            )}
            {media.website && (
              <div className="flex gap-2 sm:col-span-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.website')}</span>
                <a
                  href={media.website}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 transition-colors hover:text-neon-blue"
                  style={{ color: 'var(--text-primary)' }}
                >
                  <span className="truncate">{media.website}</span>
                  <ExternalLink size={12} className="shrink-0" />
                </a>
              </div>
            )}
            {/* 数据来源标识 */}
            {(media.tmdb_id > 0 || media.douban_id || media.bangumi_id > 0) && (
              <div className="flex gap-2 sm:col-span-2">
                <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>{t('mediaInfo.dataSource')}</span>
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
    </>
  )
}
