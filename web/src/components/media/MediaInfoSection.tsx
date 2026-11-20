import { useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import type { Media, MediaPerson } from '@/types'
import { Button, EmptyState, Tag } from '@/components/design-system'
import { ChevronDown, ChevronUp, ExternalLink, FileText, Info, Shapes, Tags } from 'lucide-react'
import clsx from 'clsx'
import { useTranslation } from '@/i18n'
import { formatDuration, getDisplayDurationSeconds } from '@/utils/format'
import DetailSectionTitle from './DetailSectionTitle'

function DetailItem({ label, children, wide = false }: { label: ReactNode; children: ReactNode; wide?: boolean }) {
  return (
    <div className={clsx('min-w-0 border-b border-[var(--nv-border-subtle)] py-2.5', wide && 'sm:col-span-2')}>
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

/**
 * 影片简介补充段。简介全文只在 Hero 出现一次，这里只留 Hero 没有的补充文字
 * （标语/梗概/原文剧情）；无补充且 Hero 已有简介时，本段不渲染。
 */
export function MediaSynopsisSection({ media }: { media: Media }) {
  const { t } = useTranslation()
  const [origPlotExpanded, setOrigPlotExpanded] = useState(false)
  const isLongOrigPlot = (media.original_plot?.length || 0) > 120
  const hasIntroExtras = !!(media.tagline || media.outline || media.original_plot)

  if (!hasIntroExtras) {
    if (media.overview) return null
    return (
      <section id="synopsis" className="nv-detail-content-section">
        <EmptyState
          className="nv-detail-tab-empty-state"
          icon={<FileText size={23} aria-hidden="true" />}
          title="暂无简介"
          description="当前媒体暂未提供剧情简介或相关文字信息。"
        />
      </section>
    )
  }

  return (
    <section id="synopsis" className="nv-detail-content-section">
      <section className="nv-media-info-intro py-5 sm:py-6">
        <div className="mb-4">
          <DetailSectionTitle icon={<FileText size={17} />}>影片简介</DetailSectionTitle>
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
    </section>
  )
}

/**
 * 影片详情段：分类 chips 与紧凑定义列表。分类只在这里出现一次（Hero 已去掉类型 chips）；
 * 演员卡片只在演职人员段出现一次，这里不再重复演员 rail。
 */
export function MediaDetailsSection({ media, persons }: { media: Media; persons: MediaPerson[] }) {
  const { t } = useTranslation()
  // 展示用片长与 Hero 共用同一值：文件实际时长优先，缺失时用刮削片长换算。
  const displaySeconds = getDisplayDurationSeconds(media.duration, media.runtime)

  const directors = persons.filter((person) => person.role === 'director')

  const extractedNum = (() => {
    if (media.num) return media.num
    const match = media.title?.match(/\b([A-Z]{2,6})-?(\d{2,5})\b/i)
    return match ? `${match[1].toUpperCase()}-${match[2]}` : ''
  })()

  const tagList = (media.tags || '').split(',').map((value) => value.trim()).filter(Boolean)
  const genreList = (media.genres || '').split(',').map((value) => value.trim()).filter(Boolean)

  const hasMetaTable = !!(
    extractedNum || media.maker || media.publisher || media.label || media.studio || media.release_date ||
    media.premiered || media.mpaa || media.country || media.language || displaySeconds > 0 || media.website ||
    directors.length > 0
  )
  const hasClassifications = genreList.length > 0 || (tagList.length > 0 && tagList.join(',') !== genreList.join(','))

  if (!(extractedNum || media.mpaa) && !hasClassifications && !hasMetaTable) return null

  return (
    <section id="details" className="nv-detail-content-section">
      <div className="nv-media-info divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
        {(extractedNum || media.mpaa) && (
          <div className="nv-media-info-badges flex flex-wrap items-center gap-1.5 py-3" aria-label="媒体标识">
            {extractedNum && <Tag>{extractedNum}</Tag>}
            {media.mpaa && <Tag>{media.mpaa}</Tag>}
          </div>
        )}

        {hasClassifications && (
          <section className="nv-media-info-classifications grid gap-5 py-5 sm:grid-cols-2 sm:py-6">
            {genreList.length > 0 && (
              <div>
                <DetailSectionTitle as="h3" icon={<Shapes size={17} />} className="mb-2">
                  {t('mediaInfo.genres').replace(/[:：]\s*$/, '')}
                </DetailSectionTitle>
                <div className="flex flex-wrap gap-1.5">
                  {genreList.map((genre) => (
                    <MetadataLink key={`g-${genre}`} to={`/search?q=${encodeURIComponent(genre)}`}>{genre}</MetadataLink>
                  ))}
                </div>
              </div>
            )}

            {tagList.length > 0 && tagList.join(',') !== genreList.join(',') && (
              <div>
                <DetailSectionTitle as="h3" icon={<Tags size={17} />} className="mb-2">
                  {t('mediaInfo.tags').replace(/[:：]\s*$/, '')}
                </DetailSectionTitle>
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
              <DetailSectionTitle icon={<Info size={17} />}>影片详情</DetailSectionTitle>
            </div>

            {/* 扁平流式网格：不再分固定左右两栏，某项缺失时后面的项自动补位，不留空栏。 */}
            <div className="nv-media-info-grid grid gap-x-8 text-sm sm:grid-cols-2">
              {extractedNum && <DetailItem label={t('mediaInfo.num')}><span className="font-mono tracking-wide">{extractedNum}</span></DetailItem>}
              {directors.length > 0 && <DetailItem label={t('mediaInfo.director')}>{directors.map((director) => director.person?.name || '').filter(Boolean).join(' / ')}</DetailItem>}
              {displaySeconds > 0 && <DetailItem label={t('mediaInfo.runtime')}>{formatDuration(displaySeconds)}</DetailItem>}
              {(media.release_date || media.premiered) && <DetailItem label={t('mediaInfo.releaseDate')}>{media.release_date || media.premiered}</DetailItem>}
              {media.mpaa && <DetailItem label={t('mediaInfo.mpaa')}>{media.mpaa}</DetailItem>}
              {media.country && <DetailItem label={t('mediaInfo.country')}>{media.country}{media.country_code && <span className="ml-1 text-xs text-[var(--nv-text-tertiary)]">({media.country_code})</span>}</DetailItem>}
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

              {(media.tmdb_id > 0 || media.douban_id || media.bangumi_id > 0) && (
                <div className="py-3 sm:col-span-2">
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
    </section>
  )
}
