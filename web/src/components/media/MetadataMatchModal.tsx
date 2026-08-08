import type { Dispatch, SetStateAction } from 'react'
import { Database, Loader2, Search } from 'lucide-react'
import { Button, EmptyState, Input, Tag } from '@/components/design-system'
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@/components/design-system/Modal'
import { useTranslation } from '@/i18n'

export type MetadataMatchSource = 'tmdb' | 'bangumi' | 'douban' | 'thetvdb'

interface MetadataMatchModalProps {
  source: MetadataMatchSource
  onSourceChange: (source: MetadataMatchSource) => void
  query: string
  setQuery: Dispatch<SetStateAction<string>>
  results: any[]
  searching: boolean
  selectedId?: number | string | null
  applying: boolean
  onSearch: () => void
  onSelect: (id: number | string) => void
  onApply?: () => void
  onClose: () => void
  mode?: 'confirm' | 'immediate'
  title?: string
  description?: string
  searchPlaceholder?: string
  applyingLabel?: string
  applyingDescription?: string
  sourceDescriptions?: Partial<Record<MetadataMatchSource, string>>
  emptyDescription?: string
}

const sourceLabels: Record<MetadataMatchSource, string> = {
  tmdb: 'TMDb',
  douban: '豆瓣',
  bangumi: 'Bangumi',
  thetvdb: 'TheTVDB',
}

export default function MetadataMatchModal({
  source,
  onSourceChange,
  query,
  setQuery,
  results,
  searching,
  selectedId = null,
  applying,
  onSearch,
  onSelect,
  onApply,
  onClose,
  mode = 'confirm',
  title,
  description,
  searchPlaceholder,
  applyingLabel = '应用中...',
  applyingDescription = '正在获取并同步元数据信息',
  sourceDescriptions,
  emptyDescription,
}: MetadataMatchModalProps) {
  const { t } = useTranslation()
  const dialogTitle = title || t('mediaDetail.manualMatch')

  const defaultSourceDescriptions: Record<MetadataMatchSource, string> = {
    tmdb: t('mediaDetail.tmdbDesc'),
    douban: t('mediaDetail.doubanDesc'),
    bangumi: t('mediaDetail.bangumiDesc'),
    thetvdb: t('mediaDetail.thetvdbDesc'),
  }

  return (
    <Modal
      onClose={onClose}
      size="md"
      ariaLabel={dialogTitle}
      closeOnBackdrop={!applying}
      closeOnEscape={!applying}
      panelClassName="relative"
    >
      <ModalHeader
        title={dialogTitle}
        description={description || '选择元数据来源并搜索目标条目。数据源只影响内容来源，不改变主操作视觉层级。'}
        icon={<Database size={18} aria-hidden="true" />}
        onClose={applying ? undefined : onClose}
      />

      <ModalBody className="space-y-5">
        <div className="space-y-2">
          <div className="flex flex-wrap gap-2" role="tablist" aria-label="元数据来源">
            {(Object.keys(sourceLabels) as MetadataMatchSource[]).map((item) => (
              <button
                key={item}
                type="button"
                role="tab"
                aria-selected={source === item}
                disabled={applying}
                onClick={() => onSourceChange(item)}
                className={`rounded-[var(--nv-radius-control)] border px-3 py-2 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${source === item ? 'border-[var(--nv-action-primary)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] text-[var(--nv-text-secondary)] hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)]'}`}
              >
                {sourceLabels[item]}
              </button>
            ))}
          </div>
          <p className="text-xs leading-5 text-[var(--nv-text-tertiary)]">
            {sourceDescriptions?.[source] || defaultSourceDescriptions[source]}
          </p>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row">
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => event.key === 'Enter' && onSearch()}
            placeholder={searchPlaceholder || t('mediaDetail.searchPlaceholder')}
            autoFocus
            className="flex-1"
            disabled={applying}
          />
          <Button type="button" variant="primary" onClick={onSearch} loading={searching} disabled={!query.trim() || applying}>
            <Search size={15} aria-hidden="true" />
            {searching ? t('mediaDetail.searching') : t('mediaDetail.searchBtn')}
          </Button>
        </div>

        <div className="max-h-[22rem] space-y-2 overflow-y-auto pr-1">
          {results.map((result) => (
            <MatchResultCard
              key={`${source}-${String(result.id)}`}
              source={source}
              result={result}
              selected={mode === 'confirm' && selectedId === result.id}
              disabled={applying}
              onSelect={() => onSelect(result.id)}
            />
          ))}

          {results.length === 0 && !searching && (
            <EmptyState
              icon={<Search size={22} />}
              title="搜索元数据"
              description={emptyDescription || t('mediaDetail.searchHint', { source: ` ${sourceLabels[source]}` })}
              className="min-h-44"
            />
          )}
        </div>
      </ModalBody>

      <ModalFooter className={mode === 'confirm' ? 'justify-between' : undefined}>
        {mode === 'confirm' ? (
          <>
            <p className="max-w-sm text-xs leading-5 text-[var(--nv-text-tertiary)]">
              {selectedId !== null ? '已选中 1 项，确认后将替换当前元数据。' : '先选择搜索结果，再应用匹配。'}
            </p>
            <div className="flex gap-2">
              <Button type="button" variant="secondary" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button>
              <Button type="button" variant="primary" onClick={onApply} loading={applying} disabled={selectedId === null || !onApply}>
                {applying ? applyingLabel : '应用'}
              </Button>
            </div>
          </>
        ) : (
          <Button type="button" variant="secondary" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button>
        )}
      </ModalFooter>

      {applying && mode === 'immediate' && (
        <div className="absolute inset-0 z-10 flex flex-col items-center justify-center bg-[color-mix(in_srgb,var(--nv-bg-elevated)_82%,transparent)] backdrop-blur-sm">
          <Loader2 size={30} className="animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" />
          <p className="mt-3 text-sm font-medium text-[var(--nv-text-primary)]">{applyingLabel}</p>
          <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{applyingDescription}</p>
        </div>
      )}
    </Modal>
  )
}

function MatchResultCard({
  source,
  result,
  selected,
  disabled,
  onSelect,
}: {
  source: MetadataMatchSource
  result: any
  selected: boolean
  disabled: boolean
  onSelect: () => void
}) {
  const { t } = useTranslation()
  let title = ''
  let originalTitle = ''
  let year = ''
  let overview = ''
  let posterUrl: string | null = null
  let rating = 0

  if (source === 'tmdb') {
    title = result.title || result.name
    originalTitle = result.original_title || result.original_name
    year = (result.release_date || result.first_air_date)?.split('-')[0] || ''
    rating = result.vote_average || 0
    overview = result.overview || ''
    posterUrl = result.poster_path ? `https://image.tmdb.org/t/p/w92${result.poster_path}` : null
  } else if (source === 'douban') {
    title = result.title
    year = result.year > 0 ? String(result.year) : ''
    rating = result.rating || 0
    overview = result.overview || ''
    posterUrl = result.cover || null
  } else if (source === 'thetvdb') {
    title = result.name || result.seriesName
    originalTitle = result.originalName || ''
    year = result.year || result.firstAired?.split('-')[0] || ''
    overview = result.overview || ''
    posterUrl = result.image || result.poster || null
    if (posterUrl && !posterUrl.startsWith('http')) posterUrl = `https://artworks.thetvdb.com${posterUrl}`
  } else {
    title = result.name_cn || result.name
    originalTitle = result.name
    year = result.air_date?.split('-')[0] || ''
    rating = result.rating?.score || 0
    overview = result.summary || ''
    posterUrl = result.images?.common || result.images?.medium || null
  }

  return (
    <button
      type="button"
      onClick={onSelect}
      disabled={disabled}
      className={`flex w-full items-center gap-3 rounded-[var(--nv-radius-card)] border p-3 text-left transition-[background-color,border-color,box-shadow] disabled:cursor-not-allowed disabled:opacity-60 ${selected ? 'border-[var(--nv-action-primary)] bg-[var(--nv-bg-active)] shadow-[var(--nv-shadow-focus)]' : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)]'}`}
      aria-pressed={selected || undefined}
    >
      {posterUrl ? (
        <img src={posterUrl} alt="" className="h-16 w-11 shrink-0 rounded-[var(--nv-radius-sm)] object-cover" />
      ) : (
        <div className="flex h-16 w-11 shrink-0 items-center justify-center rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)] text-[10px] text-[var(--nv-text-tertiary)]">N/A</div>
      )}

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{title}</span>
          {source === 'bangumi' && <Tag>{result.type === 2 ? '动画' : result.type === 6 ? '三次元' : 'BGM'}</Tag>}
          {source === 'douban' && result.genres && <Tag>{result.genres.split(',')[0]}</Tag>}
        </div>
        {originalTitle && originalTitle !== title && <div className="truncate text-xs text-[var(--nv-text-tertiary)]">{originalTitle}</div>}
        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {year && <span>{year}</span>}
          {rating > 0 && <Tag tone="rating">★ {rating.toFixed(1)}</Tag>}
          {source === 'bangumi' && result.eps > 0 && <span>{result.eps}{t('mediaDetail.episodes')}</span>}
        </div>
        {overview && <p className="mt-1 line-clamp-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{overview}</p>}
      </div>
    </button>
  )
}
