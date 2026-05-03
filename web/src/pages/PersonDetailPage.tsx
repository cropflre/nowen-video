import { useEffect, useMemo, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { personApi, streamApi } from '@/api'
import type { Person, Media, Series } from '@/types'
import { User, Film, Tv, Star, ExternalLink, ArrowLeft, Calendar } from 'lucide-react'
import { useTranslation } from '@/i18n'
import { motion, AnimatePresence } from 'framer-motion'
import { easeSmooth, durations } from '@/lib/motion'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'

export default function PersonDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const [person, setPerson] = useState<Person | null>(null)
  const [mediaList, setMediaList] = useState<Media[]>([])
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [loading, setLoading] = useState(true)
  const [worksLoading, setWorksLoading] = useState(true)
  const [imgError, setImgError] = useState(false)

  // 电影分页（URL key：mp / ms）
  const moviePagination = usePagination({ initialSize: 18, syncToUrl: true, pageKey: 'mp', sizeKey: 'mps' })
  // 剧集分页
  const seriesPagination = usePagination({ initialSize: 18, syncToUrl: true, pageKey: 'sp', sizeKey: 'sps' })

  // 作品切换时重置分页
  useEffect(() => {
    moviePagination.setPage(1)
    seriesPagination.setPage(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  const pagedMovies = useMemo(() => {
    const start = (moviePagination.page - 1) * moviePagination.size
    return mediaList.slice(start, start + moviePagination.size)
  }, [mediaList, moviePagination.page, moviePagination.size])

  const pagedSeries = useMemo(() => {
    const start = (seriesPagination.page - 1) * seriesPagination.size
    return seriesList.slice(start, start + seriesPagination.size)
  }, [seriesList, seriesPagination.page, seriesPagination.size])

  // 加载演员详情
  useEffect(() => {
    if (!id) return
    const abortController = new AbortController()

    setLoading(true)
    setWorksLoading(true)

    // 并行加载演员信息和作品列表
    personApi.getDetail(id)
      .then((res) => {
        if (!abortController.signal.aborted) {
          setPerson(res.data.data)
        }
      })
      .catch(() => {
        if (!abortController.signal.aborted) {
          setPerson(null)
        }
      })
      .finally(() => {
        if (!abortController.signal.aborted) setLoading(false)
      })

    personApi.getMedia(id)
      .then((res) => {
        if (!abortController.signal.aborted) {
          setMediaList(res.data.media || [])
          setSeriesList(res.data.series || [])
        }
      })
      .catch(() => {
        if (!abortController.signal.aborted) {
          setMediaList([])
          setSeriesList([])
        }
      })
      .finally(() => {
        if (!abortController.signal.aborted) setWorksLoading(false)
      })

    return () => abortController.abort()
  }, [id])

  const totalWorks = mediaList.length + seriesList.length

  // ==================== 骨架屏 ====================
  if (loading) {
    return (
      <AnimatePresence mode="wait">
        <motion.div
          key="skeleton"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: durations.fast }}
          className="space-y-6"
        >
          <div className="flex items-center gap-6">
            <div className="skeleton h-40 w-40 shrink-0 rounded-2xl" />
            <div className="flex-1 space-y-3">
              <div className="skeleton h-8 w-1/3 rounded-lg" />
              <div className="skeleton h-5 w-1/4 rounded-lg" />
              <div className="skeleton h-4 w-1/5 rounded-lg" />
            </div>
          </div>
          <div className="skeleton h-6 w-32 rounded-lg" />
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="skeleton aspect-[2/3] rounded-xl" />
            ))}
          </div>
        </motion.div>
      </AnimatePresence>
    )
  }

  // 演员不存在
  if (!person) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <User size={64} style={{ color: 'var(--text-muted)' }} />
        <p className="mt-4 text-lg font-medium" style={{ color: 'var(--text-secondary)' }}>
          {t('personDetail.notFound')}
        </p>
        <button
          onClick={() => navigate(-1)}
          className="mt-4 rounded-xl px-6 py-2.5 text-sm font-medium transition-all hover:opacity-80"
          style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
        >
          {t('personDetail.goBack')}
        </button>
      </div>
    )
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: durations.page, ease: easeSmooth as unknown as [number, number, number, number] }}
      className="-mx-4 -mt-6 sm:-mx-6 lg:-mx-8"
    >
      {/* ==================== 英雄区 ==================== */}
      <div
        className="relative overflow-hidden"
        style={{
          background: 'linear-gradient(180deg, var(--bg-elevated) 0%, var(--bg-base) 100%)',
        }}
      >
        {/* 装饰性背景 */}
        <div
          className="absolute inset-0 opacity-30"
          style={{
            background: 'radial-gradient(ellipse at 20% 50%, var(--neon-blue-10) 0%, transparent 60%)',
          }}
        />

        <div className="relative mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          {/* 返回按钮 */}
          <button
            onClick={() => navigate(-1)}
            className="mb-6 flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium transition-all hover:opacity-80"
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
            }}
          >
            <ArrowLeft size={16} />
            {t('personDetail.goBack')}
          </button>

          <div className="flex flex-col items-center gap-6 sm:flex-row sm:items-start">
            {/* 头像 */}
            <div
              className="h-40 w-40 shrink-0 overflow-hidden rounded-2xl shadow-xl sm:h-48 sm:w-48"
              style={{
                background: 'var(--bg-surface)',
                border: '2px solid var(--border-default)',
              }}
            >
              {id && !imgError ? (
                <img
                  src={streamApi.getPersonProfileUrl(id)}
                  alt={person.name}
                  className="h-full w-full object-cover"
                  onError={() => setImgError(true)}
                />
              ) : (
                <div
                  className="flex h-full w-full items-center justify-center"
                  style={{
                    background: 'linear-gradient(135deg, var(--neon-blue-4), var(--neon-blue-8))',
                    color: 'var(--text-muted)',
                  }}
                >
                  <User size={64} strokeWidth={1.5} />
                </div>
              )}
            </div>

            {/* 信息区 */}
            <div className="flex-1 text-center sm:text-left">
              <h1
                className="text-2xl font-bold sm:text-3xl"
                style={{ color: 'var(--text-primary)' }}
              >
                {person.name}
              </h1>

              {person.orig_name && person.orig_name !== person.name && (
                <p className="mt-1 text-base" style={{ color: 'var(--text-secondary)' }}>
                  {person.orig_name}
                </p>
              )}

              {/* 统计信息 */}
              <div className="mt-4 flex flex-wrap items-center justify-center gap-4 sm:justify-start">
                {!worksLoading && totalWorks > 0 && (
                  <div
                    className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium"
                    style={{
                      background: 'var(--neon-blue-4)',
                      border: '1px solid var(--border-default)',
                      color: 'var(--text-secondary)',
                    }}
                  >
                    <Film size={14} />
                    {t('personDetail.worksCount', { count: totalWorks })}
                  </div>
                )}

                {mediaList.length > 0 && (
                  <div
                    className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm"
                    style={{ color: 'var(--text-muted)' }}
                  >
                    <Film size={14} />
                    {t('personDetail.movieCount', { count: mediaList.length })}
                  </div>
                )}

                {seriesList.length > 0 && (
                  <div
                    className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm"
                    style={{ color: 'var(--text-muted)' }}
                  >
                    <Tv size={14} />
                    {t('personDetail.seriesCount', { count: seriesList.length })}
                  </div>
                )}
              </div>

              {/* 外部链接 */}
              <div className="mt-4 flex flex-wrap items-center justify-center gap-2 sm:justify-start">
                {person.tmdb_id > 0 && (
                  <a
                    href={`https://www.themoviedb.org/person/${person.tmdb_id}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-all hover:opacity-80"
                    style={{ background: 'rgba(1,180,228,0.12)', color: '#01b4e4' }}
                  >
                    <ExternalLink size={12} />
                    {t('personDetail.viewOnTMDb')}
                  </a>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ==================== 作品列表 ==================== */}
      <div className="mx-auto max-w-7xl space-y-8 px-4 py-8 sm:px-6 lg:px-8">
        {worksLoading ? (
          <div className="space-y-8 animate-fade-in">
            {/* 电影作品骨架 */}
            <section>
              <div className="mb-4 flex items-center gap-2">
                <div className="skeleton h-5 w-5 rounded" />
                <div className="skeleton h-6 w-20 rounded-lg" />
                <div className="skeleton h-4 w-8 rounded" />
              </div>
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className="overflow-hidden rounded-xl" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-default)' }}>
                    <div className="skeleton aspect-[2/3] w-full rounded-none" />
                    <div className="p-2.5 space-y-1.5">
                      <div className="skeleton h-4 w-3/4 rounded" />
                      <div className="skeleton h-3 w-1/2 rounded" />
                    </div>
                  </div>
                ))}
              </div>
            </section>
            {/* 剧集作品骨架 */}
            <section>
              <div className="mb-4 flex items-center gap-2">
                <div className="skeleton h-5 w-5 rounded" />
                <div className="skeleton h-6 w-20 rounded-lg" />
                <div className="skeleton h-4 w-8 rounded" />
              </div>
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className="overflow-hidden rounded-xl" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-default)' }}>
                    <div className="skeleton aspect-[2/3] w-full rounded-none" />
                    <div className="p-2.5 space-y-1.5">
                      <div className="skeleton h-4 w-3/4 rounded" />
                      <div className="skeleton h-3 w-1/2 rounded" />
                    </div>
                  </div>
                ))}
              </div>
            </section>
          </div>
        ) : totalWorks === 0 ? (
          <div className="py-16 text-center">
            <Film size={48} className="mx-auto mb-3" style={{ color: 'var(--text-muted)' }} />
            <p className="text-base" style={{ color: 'var(--text-muted)' }}>
              {t('personDetail.noWorks')}
            </p>
          </div>
        ) : (
          <>
            {/* 电影作品 */}
            {mediaList.length > 0 && (
              <section>
                <h2
                  className="mb-4 flex items-center gap-2 text-lg font-bold"
                  style={{ color: 'var(--text-primary)' }}
                >
                  <Film size={20} className="text-blue-400" />
                  {t('personDetail.movies')}
                  <span className="text-sm font-normal" style={{ color: 'var(--text-muted)' }}>
                    ({mediaList.length})
                  </span>
                </h2>
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
                  {pagedMovies.map((media) => (
                    <WorkCard
                      key={media.id}
                      id={media.id}
                      title={media.title}
                      year={media.year}
                      rating={media.rating}
                      genres={media.genres}
                      hasPoster={!!media.poster_path}
                      posterUrl={streamApi.getPosterUrl(media.id)}
                      type="movie"
                      linkTo={`/media/${media.id}`}
                    />
                  ))}
                </div>
                <Pagination
                  page={moviePagination.page}
                  totalPages={moviePagination.totalPages(mediaList.length)}
                  total={mediaList.length}
                  pageSize={moviePagination.size}
                  pageSizeOptions={[12, 18, 24, 48]}
                  onPageChange={moviePagination.setPage}
                  onPageSizeChange={moviePagination.setSize}
                />
              </section>
            )}

            {/* 剧集作品 */}
            {seriesList.length > 0 && (
              <section>
                <h2
                  className="mb-4 flex items-center gap-2 text-lg font-bold"
                  style={{ color: 'var(--text-primary)' }}
                >
                  <Tv size={20} className="text-purple-400" />
                  {t('personDetail.tvShows')}
                  <span className="text-sm font-normal" style={{ color: 'var(--text-muted)' }}>
                    ({seriesList.length})
                  </span>
                </h2>
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
                  {pagedSeries.map((s) => (
                    <WorkCard
                      key={s.id}
                      id={s.id}
                      title={s.title}
                      year={s.year}
                      rating={s.rating}
                      genres={s.genres}
                      hasPoster={!!s.poster_path}
                      posterUrl={streamApi.getSeriesPosterUrl(s.id)}
                      type="series"
                      linkTo={`/series/${s.id}`}
                    />
                  ))}
                </div>
                <Pagination
                  page={seriesPagination.page}
                  totalPages={seriesPagination.totalPages(seriesList.length)}
                  total={seriesList.length}
                  pageSize={seriesPagination.size}
                  pageSizeOptions={[12, 18, 24, 48]}
                  onPageChange={seriesPagination.setPage}
                  onPageSizeChange={seriesPagination.setSize}
                />
              </section>
            )}
          </>
        )}
      </div>
    </motion.div>
  )
}

/** 作品卡片 — 参考 Emby 风格 */
function WorkCard({
  id: _id,
  title,
  year,
  rating,
  genres,
  hasPoster,
  posterUrl,
  type,
  linkTo,
}: {
  id: string
  title: string
  year: number
  rating: number
  genres: string
  hasPoster: boolean
  posterUrl: string
  type: 'movie' | 'series'
  linkTo: string
}) {
  const [imgError, setImgError] = useState(false)
  const genreList = genres ? genres.split(',').slice(0, 2).map((g) => g.trim()).filter(Boolean) : []

  return (
    <Link
      to={linkTo}
      className="group flex flex-col overflow-hidden rounded-xl text-left transition-all duration-300 hover:scale-[1.03] hover:shadow-xl"
      style={{
        background: 'var(--bg-card)',
        border: '1px solid var(--border-default)',
      }}
    >
      {/* 海报 */}
      <div className="relative aspect-[2/3] w-full overflow-hidden" style={{ background: 'var(--bg-surface)' }}>
        {hasPoster && !imgError ? (
          <img
            src={posterUrl}
            alt={title}
            className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-110"
            loading="lazy"
            onError={() => setImgError(true)}
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center" style={{ color: 'var(--text-muted)' }}>
            {type === 'movie' ? <Film size={32} /> : <Tv size={32} />}
          </div>
        )}

        {/* 悬浮遮罩 */}
        <div
          className="absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
          style={{
            background: 'linear-gradient(to top, rgba(0,0,0,0.8) 0%, transparent 50%)',
          }}
        />

        {/* 评分角标 */}
        {rating > 0 && (
          <div
            className="absolute right-2 top-2 flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-[10px] font-bold"
            style={{
              background: 'rgba(0,0,0,0.75)',
              backdropFilter: 'blur(4px)',
              color: rating >= 8 ? '#FBBF24' : rating >= 6 ? '#34D399' : 'var(--text-secondary)',
            }}
          >
            <Star size={10} fill="currentColor" />
            {rating.toFixed(1)}
          </div>
        )}

        {/* 类型标识 */}
        <div
          className="absolute bottom-2 left-2 rounded-md px-1.5 py-0.5 text-[9px] font-bold uppercase"
          style={{
            background: type === 'movie' ? 'rgba(59,130,246,0.85)' : 'rgba(168,85,247,0.85)',
            color: '#fff',
            backdropFilter: 'blur(4px)',
          }}
        >
          {type === 'movie' ? 'MOVIE' : 'TV'}
        </div>
      </div>

      {/* 信息区 */}
      <div className="flex flex-1 flex-col p-2.5">
        <p
          className="truncate text-sm font-medium transition-colors group-hover:text-neon"
          style={{ color: 'var(--text-primary)' }}
          title={title}
        >
          {title}
        </p>

        <div className="mt-1 flex items-center gap-2">
          {year > 0 && (
            <span className="flex items-center gap-0.5 text-[11px]" style={{ color: 'var(--text-muted)' }}>
              <Calendar size={10} />
              {year}
            </span>
          )}
        </div>

        {/* 类型标签 */}
        {genreList.length > 0 && (
          <div className="mt-1.5 flex flex-wrap gap-1">
            {genreList.map((genre) => (
              <span
                key={genre}
                className="rounded px-1.5 py-0.5 text-[9px]"
                style={{ background: 'var(--neon-blue-6)', color: 'var(--text-muted)' }}
              >
                {genre}
              </span>
            ))}
          </div>
        )}
      </div>
    </Link>
  )
}
