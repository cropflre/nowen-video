import { useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import type { Media, MediaPlayInfo, MediaPerson } from '@/types'
import { Button, EmptyState, Tag } from '@/components/design-system'
import { ChevronDown, ChevronUp, ExternalLink, FileText } from 'lucide-react'
import clsx from 'clsx'
import { useTranslation } from '@/i18n'

interface MediaInfoSectionProps {
  media: Media
  playInfo: MediaPlayInfo | null
  persons: MediaPerson[]
}

function formatRuntime(minutes: number): string {
  if (!minutes || minutes <= 0) return ''
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours === 0) return `${mins}min`
  if (mins === 0) return `${hours}h`
  return `${hours}h ${mins}min`
}

function DetailItem({ label, children, wide = false }: { label: ReactNode; children: ReactNode; wide?: boolean }) {
  return (
    <div className={clsx('min-w-0 py-2.5', wide && 'sm:col-span-2')}>
      <div className="text-[11px] font-medium uppercase tracking-[0.07em] text-[var(--nv-text-tertiary)]">{label}</div>
      <div className="mt-1 min-w-0 text-[13px] leading-5 text-[var(--nv-text-secondary)]">{children}</div>
    </div>
  )
}

function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-[0.09em] text-[var(--nv-text-tertiary)]">
      {children}
    </h3>
  )
}

function MetadataLink({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Link
      to={to}
      className="nv-tag transition-[background-color,border-color,color] duration-150 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-primary)]"
    >
      {children}
    </Link>
  )
}

function SourceLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="nv-tag transition-[background-color,border-color,color] duration-150 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-primary)]"
    >
      {children}
      <ExternalLink size={10} aria-hidden="true" />
    </a>
  )
}

export default function MediaInfoSection({ media, playInfo: _playInfo, persons }: MediaInfoSectionProps) {
  const { t } = useTranslation()
  const [plotExpanded, setPlotExpanded] = useState(false)
  const [origPlotExpanded, setOrigPlotExpanded] = useState(false)
  const isLongPlot = (media.overview?.length || 0) > 200
  const isLongOrigPlot = (media.original_plot?.length || 0) > 120

  const directors = persons.filter((person) => person.role === 'director')
  const actors = persons.filter((person) => person.role === 'actor')

  const extractedNum = (() => {
    if (media.num) return media.num
    const match = media.title?.match(/\b([A-Z]{2,6})-?(\d{2,5})\b/i)
    return match ? `${match[1].toUpperCase()}-${match[2]}` : ''
  })()

  const tagList = (media.tags || '').split(',').map((value) => value.trim()).filter(Boolean)
  const genreList = (media.genres || '').split(',').map((value) => value.trim()).filter(Boolean)

  const hasMetaTable = !!(
    extractedNum || media.maker || media.publisher || media.label || media.studio || media.release_date ||
    media.premiered || media.mpaa || media.country || media.language || media.runtime || media.website ||
    directors.length > 0 || actors.length > 0
  )

  const hasIntro = !!(media.tagline || media.outline || media.overview || media.original_plot)
  const hasClassifications = genreList.length > 0 || (tagList.length > 0 && tagList.join(',') !== genreList.join(','))

  return (
    <div className="nv-media-info divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
      {(extractedNum || media.mpaa) && (
        <div className="nv-media-info-badges flex flex-wrap items-center gap-1.5 py-3" aria-label="媒体标识">
          {extractedNum && <Tag>{extractedNum}</Tag>}
          {media.mpaa && <Tag>{media.mpaa}</Tag>}
        </div>
      )}

      {hasIntro ? (
        <section className="nv-media-info-intro py-5 sm:py-6">
          <div className="mb-4">
            <h2 className="text-sm font-semibold text-[var(--nv-text-primary)]">影片简介</h2>
            {media.orig_title && media.orig_title !== media.title && (
              <p className="mt-1 text-xs italic text-[var(--nv-text-tertiary)]">{media.orig_title}</p>
            )}
          </div>

          <div className="max-w-4xl space-y-4">
            {media.tagline && (
              <blockquote className="border-l border-[var(--nv-border-default)] pl-3 text-sm italic leading-6 text-[var(--nv-text-secondary)]">
                “{media.tagline}”
              </blockquote>
            )}

            {media.outline && media.outline !== media.overview && (
              <div>
                <SectionLabel>{t('mediaInfo.outline')}</SectionLabel>
                <p className="text-sm leading-7 text-[var(--nv-text-secondary)]">{media.outline}</p>
              </div>
            )}

            {media.overview && (
              <div>
                {media.outline && media.outline !== media.overview && <SectionLabel>{t('mediaInfo.plot')}</SectionLabel>}
                <p className={clsx('text-sm leading-7 text-[var(--nv-text-secondary)]', !plotExpanded && isLongPlot && 'line-clamp-3')}>
                  {media.overview}
                </p>
                {isLongPlot && (
                  <Button type="button" variant="ghost" size="sm" className="mt-1.5" onClick={() => setPlotExpanded((value) => !value)}>
                    {plotExpanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronDown size={14} aria-hidden="true" />}
                    {plotExpanded ? t('mediaInfo.collapse') : t('mediaInfo.expandAll')}
                  </Button>
                )}
              </div>
            )}

            {media.original_plot && (
              <div className="border-t border-[var(--nv-border-subtle)] pt-4">
                <SectionLabel>{t('mediaInfo.originalPlot')}</SectionLabel>
                <p className={clsx('text-sm italic leading-7 text-[var(--nv-text-tertiary)]', !origPlotExpanded && isLongOrigPlot && 'line-clamp-2')}>
                  {media.original_plot}
                </p>
                {isLongOrigPlot && (
                  <Button type="button" variant="ghost" size="sm" className="mt-1.5" onClick={() => setOrigPlotExpanded((value) => !value)}>
                    {origPlotExpanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronDown size={14} aria-hidden="true" />}
                    {origPlotExpanded ? t('mediaInfo.collapse') : t('mediaInfo.expandAll')}
                  </Button>
                )}
              </div>
            )}
          </div>
        </section>
      ) : (
        <EmptyState
          className="nv-detail-tab-empty-state"
          icon={<FileText size={23} aria-hidden="true" />}
          title="暂无简介"
          description="当前媒体暂未提供剧情简介或相关文字信息。"
        />
      )}

      {hasClassifications && (
        <section className="nv-media-info-classifications grid gap-5 py-5 sm:grid-cols-2 sm:py-6">
          {genreList.length > 0 && (
            <div>
              <SectionLabel>{t('mediaInfo.genres').replace(/[:：]\s*$/, '')}</SectionLabel>
              <div className="flex flex-wrap gap-1.5">
                {genreList.map((genre) => (
                  <MetadataLink key={`g-${genre}`} to={`/search?q=${encodeURIComponent(genre)}`}>{genre}</MetadataLink>
                ))}
              </div>
            </div>
          )}

          {tagList.length > 0 && tagList.join(',') !== genreList.join(',') && (
            <div>
              <SectionLabel>{t('mediaInfo.tags').replace(/[:：]\s*$/, '')}</SectionLabel>
              <div className="flex flex-wrap gap-1.5">
                {tagList.map((tag) => (
                  <MetadataLink key={`t-${tag}`} to={`/search?q=${encodeURIComponent(tag)}`}>#{tag}</MetadataLink>
                ))}
              </div>
            </div>
          )}
        </section>
      )}

      {hasMetaTable && (
        <section className="nv-media-info-metadata py-5 sm:py-6">
          <div className="mb-2 flex items-center justify-between gap-3">
            <h2 className="text-sm font-semibold text-[var(--nv-text-primary)]">影片详情</h2>
            <span className="text-[10px] uppercase tracking-[0.1em] text-[var(--nv-text-tertiary)]">Metadata</span>
          </div>

          <div className="nv-media-info-grid grid divide-y divide-[var(--nv-border-subtle)] text-sm sm:grid-cols-2 sm:divide-y-0">
            <div className="nv-media-info-column divide-y divide-[var(--nv-border-subtle)] sm:pr-8">
              {extractedNum && <DetailItem label={t('mediaInfo.num')}><span className="font-mono tracking-wide">{extractedNum}</span></DetailItem>}
              {directors.length > 0 && <DetailItem label={t('mediaInfo.director')}>{directors.map((director) => director.person?.name || '').filter(Boolean).join(' / ')}</DetailItem>}
              {media.runtime > 0 && <DetailItem label={t('mediaInfo.runtime')}>{formatRuntime(media.runtime)} <span className="text-xs text-[var(--nv-text-tertiary)]">({t('mediaInfo.runtimeMinutes', { minutes: media.runtime })})</span></DetailItem>}
              {(media.release_date || media.premiered) && <DetailItem label={t('mediaInfo.releaseDate')}>{media.release_date || media.premiered}</DetailItem>}
              {media.mpaa && <DetailItem label={t('mediaInfo.mpaa')}>{media.mpaa}</DetailItem>}
              {media.country && <DetailItem label={t('mediaInfo.country')}>{media.country}{media.country_code && <span className="ml-1 text-xs text-[var(--nv-text-tertiary)]">({media.country_code})</span>}</DetailItem>}
            </div>

            <div className="nv-media-info-column divide-y divide-[var(--nv-border-subtle)] sm:border-l sm:border-[var(--nv-border-subtle)] sm:pl-8">
              {media.language && <DetailItem label={t('mediaInfo.language')}>{media.language}</DetailItem>}
              {media.studio && <DetailItem label={t('mediaInfo.studio')}>{media.studio}</DetailItem>}
              {media.maker && media.maker !== media.studio && <DetailItem label={t('mediaInfo.maker')}>{media.maker}</DetailItem>}
              {media.publisher && <DetailItem label={t('mediaInfo.publisher')}>{media.publisher}</DetailItem>}
              {media.label && media.label !== media.publisher && <DetailItem label={t('mediaInfo.label')}>{media.label}</DetailItem>}
              {media.website && (
                <DetailItem label={t('mediaInfo.website')}>
                  <a href={media.website} target="_blank" rel="noopener noreferrer" className="inline-flex max-w-full items-center gap-1 text-[var(--nv-text-secondary)] hover:text-[var(--nv-text-primary)]">
                    <span className="truncate">{media.website}</span><ExternalLink size={12} className="shrink-0" aria-hidden="true" />
                  </a>
                </DetailItem>
              )}
            </div>

            {actors.length > 0 && (
              <div className="border-t border-[var(--nv-border-subtle)] sm:col-span-2">
                <DetailItem label={t('mediaInfo.actors')} wide>
                  <span className="line-clamp-2">
                    {actors.slice(0, 8).map((actor) => {
                      const name = actor.person?.name || ''
                      return actor.character ? `${name}${t('mediaInfo.asCharacter', { character: actor.character })}` : name
                    }).filter(Boolean).join(' / ')}
                  </span>
                </DetailItem>
              </div>
            )}

            {(media.tmdb_id > 0 || media.douban_id || media.bangumi_id > 0) && (
              <div className="border-t border-[var(--nv-border-subtle)] py-3 sm:col-span-2">
                <div className="mb-2 text-[11px] font-medium uppercase tracking-[0.07em] text-[var(--nv-text-tertiary)]">{t('mediaInfo.dataSource')}</div>
                <div className="flex flex-wrap gap-1.5">
                  {media.tmdb_id > 0 && <SourceLink href={`https://www.themoviedb.org/${media.media_type === 'episode' ? 'tv' : 'movie'}/${media.tmdb_id}`}>TMDb #{media.tmdb_id}</SourceLink>}
                  {media.douban_id && <SourceLink href={`https://movie.douban.com/subject/${media.douban_id}/`}>豆瓣 #{media.douban_id}</SourceLink>}
                  {media.bangumi_id > 0 && <SourceLink href={`https://bgm.tv/subject/${media.bangumi_id}`}>Bangumi #{media.bangumi_id}</SourceLink>}
                </div>
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}
