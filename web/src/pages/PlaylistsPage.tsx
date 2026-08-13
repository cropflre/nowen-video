import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronUp, ListVideo, Play, Plus, Trash2, X } from 'lucide-react'
import { playlistApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useDialog } from '@/components/Dialog'
import { useTranslation } from '@/i18n'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import type { Playlist } from '@/types'
import { Button, EmptyState, Input, Tag } from '@/components/design-system'

export default function PlaylistsPage() {
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const toast = useToast()
  const { t } = useTranslation()
  const dialog = useDialog()

  const { page, size, setPage, setSize, totalPages } = usePagination({ initialSize: 10 })

  const fetchPlaylists = async () => {
    try {
      const res = await playlistApi.list()
      setPlaylists(res.data.data || [])
    } catch {
      toast.error(t('playlists.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void fetchPlaylists()
  }, [])

  const handleCreate = async () => {
    if (!newName.trim()) return
    try {
      await playlistApi.create({ name: newName.trim() })
      setNewName('')
      setShowCreate(false)
      void fetchPlaylists()
    } catch {
      toast.error(t('playlists.createFailed'))
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await dialog.confirm({
      title: t('playlists.deleteConfirmTitle') || '删除播放列表',
      message: t('playlists.deleteConfirm'),
      confirmText: t('playlists.delete') || '删除',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await playlistApi.delete(id)
      setPlaylists((previous) => previous.filter((playlist) => playlist.id !== id))
    } catch {
      toast.error(t('playlists.deleteFailed'))
    }
  }

  const handleRemoveItem = async (playlistId: string, mediaId: string) => {
    try {
      await playlistApi.removeItem(playlistId, mediaId)
      void fetchPlaylists()
    } catch {
      toast.error(t('playlists.removeFailed'))
    }
  }

  const pagedPlaylists = useMemo(() => {
    const start = (page - 1) * size
    return playlists.slice(start, start + size)
  }, [playlists, page, size])

  const total = playlists.length
  const pages = totalPages(total)

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-4 border-b border-[var(--nv-border-subtle)] pb-5 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-[-0.02em] text-[var(--nv-text-primary)]">
            <ListVideo size={20} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
            {t('playlists.title')}
          </h1>
          <p className="mt-1.5 text-sm leading-6 text-[var(--nv-text-tertiary)]">
            整理常看内容，快速进入播放，并管理列表中的媒体项目。
          </p>
        </div>
        <Button type="button" variant="secondary" onClick={() => setShowCreate((current) => !current)} aria-expanded={showCreate}>
          <Plus size={15} aria-hidden="true" />
          {t('playlists.create')}
        </Button>
      </header>

      {showCreate && (
        <section className="border-y border-[var(--nv-border-subtle)] py-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="min-w-0 flex-1">
              <label htmlFor="playlist-name" className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">
                {t('playlists.namePlaceholder')}
              </label>
              <Input
                id="playlist-name"
                type="text"
                value={newName}
                onChange={(event) => setNewName(event.target.value)}
                placeholder={t('playlists.namePlaceholder')}
                autoFocus
                onKeyDown={(event) => {
                  if (event.key === 'Enter') void handleCreate()
                }}
              />
            </div>
            <div className="flex flex-wrap gap-1.5">
              <Button type="button" variant="secondary" onClick={() => void handleCreate()} disabled={!newName.trim()}>
                {t('playlists.createBtn')}
              </Button>
              <Button type="button" variant="ghost" onClick={() => setShowCreate(false)}>
                {t('playlists.cancelBtn')}
              </Button>
            </div>
          </div>
        </section>
      )}

      {loading ? (
        <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]" aria-label="正在加载播放列表">
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="py-4">
              <div className="skeleton h-5 w-1/4 rounded-[var(--nv-radius-control)]" />
              <div className="skeleton mt-2 h-4 w-1/3 rounded-[var(--nv-radius-control)]" />
            </div>
          ))}
        </div>
      ) : playlists.length === 0 ? (
        <EmptyState
          icon={<ListVideo size={26} />}
          title={t('playlists.empty')}
          description={t('playlists.emptyHint')}
          action={(
            <Button type="button" variant="secondary" onClick={() => setShowCreate(true)}>
              <Plus size={15} aria-hidden="true" />
              {t('playlists.create')}
            </Button>
          )}
        />
      ) : (
        <div className="space-y-5">
          <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
            {pagedPlaylists.map((playlist) => {
              const expanded = expandedId === playlist.id
              const itemCount = playlist.items?.length || 0

              return (
                <article key={playlist.id}>
                  <div className="flex items-center gap-2 px-1 py-3">
                    <button
                      type="button"
                      onClick={() => setExpandedId(expanded ? null : playlist.id)}
                      className="group flex min-w-0 flex-1 items-center gap-3 rounded-[var(--nv-radius-control)] px-1 py-1 text-left outline-none transition-colors duration-150 hover:bg-[var(--nv-fill-hover)] focus-visible:shadow-[var(--nv-shadow-focus)]"
                      aria-expanded={expanded}
                    >
                      <div className="grid h-8 w-8 shrink-0 place-items-center rounded-[var(--nv-radius-control)] bg-[var(--nv-fill-hover)] text-[var(--nv-text-tertiary)]">
                        <ListVideo size={16} aria-hidden="true" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{playlist.name}</h3>
                          <Tag>{itemCount}</Tag>
                        </div>
                        <p className="mt-0.5 text-xs text-[var(--nv-text-tertiary)]">
                          {t('playlists.itemCount', { count: String(itemCount) })}
                        </p>
                      </div>
                      {expanded
                        ? <ChevronUp size={16} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                        : <ChevronDown size={16} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />}
                    </button>

                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      iconOnly
                      onClick={() => void handleDelete(playlist.id)}
                      className="text-[var(--nv-text-tertiary)] hover:text-[var(--nv-status-danger)]"
                      aria-label={t('playlists.deletePlaylist')}
                      title={t('playlists.deletePlaylist')}
                    >
                      <Trash2 size={15} aria-hidden="true" />
                    </Button>
                  </div>

                  {expanded && (
                    <div className="ml-12 border-t border-[var(--nv-border-subtle)]">
                      {!playlist.items || playlist.items.length === 0 ? (
                        <div className="py-6 text-sm text-[var(--nv-text-tertiary)]">{t('playlists.emptyList')}</div>
                      ) : (
                        <div className="divide-y divide-[var(--nv-border-subtle)]">
                          {playlist.items.map((item) => (
                            <div key={item.id} className="group flex items-center gap-3 py-3 transition-colors duration-150 hover:bg-[var(--nv-fill-hover)]">
                              <Link
                                to={`/play/${item.media_id}`}
                                className="relative h-14 w-24 shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]"
                                aria-label={`播放 ${item.media?.title || t('history.unknownMedia')}`}
                              >
                                <img
                                  src={streamApi.getPosterUrl(item.media_id)}
                                  alt={item.media?.title || ''}
                                  className="h-full w-full object-cover"
                                  onError={(event) => { event.currentTarget.style.display = 'none' }}
                                />
                                <div className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100">
                                  <span className="grid h-7 w-7 place-items-center rounded-full bg-white/90 text-black">
                                    <Play size={12} className="ml-0.5" fill="currentColor" aria-hidden="true" />
                                  </span>
                                </div>
                              </Link>

                              <Link
                                to={`/media/${item.media_id}`}
                                className="min-w-0 flex-1 truncate text-sm font-medium text-[var(--nv-text-primary)] outline-none transition-colors duration-150 hover:text-[var(--nv-text-secondary)] focus-visible:text-[var(--nv-text-secondary)]"
                              >
                                {item.media?.title || t('history.unknownMedia')}
                              </Link>

                              <Button
                                type="button"
                                size="sm"
                                variant="ghost"
                                iconOnly
                                onClick={() => void handleRemoveItem(playlist.id, item.media_id)}
                                className="shrink-0 text-[var(--nv-text-tertiary)] sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
                                aria-label={t('playlists.removeFromList')}
                                title={t('playlists.removeFromList')}
                              >
                                <X size={14} aria-hidden="true" />
                              </Button>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </article>
              )
            })}
          </div>

          <Pagination
            page={page}
            totalPages={pages}
            total={total}
            pageSize={size}
            pageSizeOptions={[10, 20, 50]}
            onPageChange={setPage}
            onPageSizeChange={setSize}
          />
        </div>
      )}
    </div>
  )
}
