import { Link } from 'react-router-dom'
import { CheckCircle2, Clapperboard, Database, Play, Tv2 } from 'lucide-react'
import type { Media, Series, WatchHistory } from '@/types'
import { Tag, buttonClassName } from '@/components/design-system'

type SeriesDetailSidebarProps = {
  series: Series
  episodes: Media[]
  historyMap: Record<string, WatchHistory>
  playEpisode: Media | null
  playLabel: string
}

function isCompleted(history?: WatchHistory) {
  if (!history) return false
  if (history.completed) return true
  return history.duration > 0 && history.position / history.duration >= 0.9
}

function episodeCode(episode?: Media | null) {
  if (!episode) return '—'
  return `S${String(episode.season_num).padStart(2, '0')}E${String(episode.episode_num).padStart(2, '0')}`
}

export default function SeriesDetailSidebar({ series, episodes, historyMap, playEpisode, playLabel }: SeriesDetailSidebarProps) {
  const watchedCount = episodes.filter((episode) => isCompleted(historyMap[episode.id])).length
  const inProgressCount = episodes.filter((episode) => {
    const history = historyMap[episode.id]
    return !!history && history.position > 0 && !isCompleted(history)
  }).length
  const progress = episodes.length > 0 ? Math.round((watchedCount / episodes.length) * 100) : 0
  const genres = (series.genres || '').split(',').map((item) => item.trim()).filter(Boolean)
  const hasSources = series.tmdb_id > 0 || Boolean(series.douban_id) || series.bangumi_id > 0

  return (
    <aside className="nv-series-detail-sidebar" aria-label="剧集状态与信息">
      <section className="nv-series-sidebar-card nv-series-progress-card">
        <div className="nv-series-sidebar-eyebrow"><Clapperboard size={12} aria-hidden="true" /> WATCHING</div>
        <div className="nv-series-sidebar-heading-row">
          <div>
            <h2>观看进度</h2>
            <p>{episodes.length > 0 ? `已看 ${watchedCount} / ${episodes.length} 集` : '暂无可播放单集'}</p>
          </div>
          <span className="nv-series-progress-value">{progress}%</span>
        </div>
        <div className="nv-series-progress-track" aria-label={`观看进度 ${progress}%`}>
          <span style={{ width: `${progress}%` }} />
        </div>
        <div className="nv-series-progress-meta">
          <span>当前 {episodeCode(playEpisode)}</span>
          {inProgressCount > 0 && <span>{inProgressCount} 集进行中</span>}
        </div>
        {playEpisode && (
          <Link to={`/play/${playEpisode.id}`} className={`${buttonClassName({ variant: 'primary', size: 'sm' })} nv-series-sidebar-play`}>
            <Play size={14} fill="currentColor" aria-hidden="true" />
            <span>{playLabel}</span>
          </Link>
        )}
      </section>

      <section className="nv-series-sidebar-card">
        <div className="nv-series-sidebar-eyebrow"><Tv2 size={12} aria-hidden="true" /> SERIES</div>
        <h2>剧集信息</h2>
        <dl className="nv-series-sidebar-facts">
          <div><dt>季数</dt><dd>{series.season_count || 0} 季</dd></div>
          <div><dt>集数</dt><dd>{series.episode_count || episodes.length || 0} 集</dd></div>
          {series.year > 0 && <div><dt>年份</dt><dd>{series.year}</dd></div>}
          {series.country && <div><dt>地区</dt><dd>{series.country}</dd></div>}
          {series.language && <div><dt>语言</dt><dd>{series.language}</dd></div>}
          {series.studio && <div><dt>制作</dt><dd title={series.studio}>{series.studio}</dd></div>}
        </dl>
        {genres.length > 0 && (
          <div className="nv-series-sidebar-tags">
            {genres.slice(0, 6).map((genre) => <Tag key={genre}>{genre}</Tag>)}
          </div>
        )}
      </section>

      <section className="nv-series-sidebar-card">
        <div className="nv-series-sidebar-eyebrow"><Database size={12} aria-hidden="true" /> METADATA</div>
        <h2>元数据来源</h2>
        {hasSources ? (
          <div className="nv-series-source-list">
            {series.tmdb_id > 0 && (
              <a href={`https://www.themoviedb.org/tv/${series.tmdb_id}`} target="_blank" rel="noopener noreferrer">
                <span>TMDb</span><strong>#{series.tmdb_id}</strong>
              </a>
            )}
            {series.douban_id && (
              <a href={`https://movie.douban.com/subject/${series.douban_id}/`} target="_blank" rel="noopener noreferrer">
                <span>豆瓣</span><strong>#{series.douban_id}</strong>
              </a>
            )}
            {series.bangumi_id > 0 && (
              <a href={`https://bgm.tv/subject/${series.bangumi_id}`} target="_blank" rel="noopener noreferrer">
                <span>Bangumi</span><strong>#{series.bangumi_id}</strong>
              </a>
            )}
          </div>
        ) : (
          <div className="nv-series-source-empty"><CheckCircle2 size={15} aria-hidden="true" /> 暂无外部元数据匹配</div>
        )}
      </section>
    </aside>
  )
}
