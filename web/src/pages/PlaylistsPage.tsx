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
import { Button, EmptyState, Input, Surface, Tag } from '@/components/design-system'

export default function PlaylistsPage() {
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const toast = useToast()
  const { t } = useTranslation()
  const dialog = useDialog()

  // 分页（前端分页：后端返回用户全部列表，一般数量不大）
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
      <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold text-[var(--nv-text-primary)]">
            <ListVideo size={23} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            {t('playlists.title')}
          </h1>
          <p className="mt-1 text-sm leading-6 text-[var(--nv-text-tertiary)]">
            整理常看内容，快速进入播放，并管理列表中的媒体项目。
          </p>
        </div>
        <Button
          type="button"
          variant="primary"
          onClick={() => setShowCreate((current) => !current)}
          aria-expanded={showCreate}
        >
          <Plus size={16} aria-hidden="true" />
          {t('playlists.create')}
        </Button>
      </header>

      {showCreate && (
        <Surface className="p-4 sm:p-5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
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
            <div className="flex flex-wrap gap-2 sm:self-end">
              <Button type="button" variant="primary" onClick={() => void handleCreate()} disabled={!newName.trim()}>
                {t('playlists.createBtn')}
              </Button>
              <Button type="button" variant="ghost" onClick={() => setShowCreate(false)}>
                {t('playlists.cancelBtn')}
              </Button>
            </div>
          </div>
        </Surface>
      )}

      {loading ? (
        <div className="space-y-3" aria-label="正在加载播放列表">
          {Array.from({ length: 3 }).map((_, index) => (
            <Surface key={index} className="p-4">
              <div className="skeleton h-5 w-1/4 rounded-[var(--nv-radius-control)]" />
              <div className="skeleton mt-2 h-4 w-1/3 rounded-[var(--nv-radius-control)]" />
            </Surface>
          ))}
        </div>
      ) : playlists.length === 0 ? (
        <Surface className="overflow-hidden p-0">
          <EmptyState
            icon={<ListVideo size={26} />}
            title={t('playlists.empty')}
            description={t('playlists.emptyHint')}
            action={(
              <Button type="button" variant="primary" onClick={() => setShowCreate(true)}>
                <Plus size={15} aria-hidden="true" />
                {t('playlists.create')}
              </Button>
            )}
          />
        </Surface>
      ) : (
        <div className="space-y-4">
          {pagedPlaylists.map((playlist) => {
            const expanded = expandedId === playlist.id
            const itemCount = playlist.items?.length || 0

            return (
              <Surface key={playlist.id} className="overflow-hidden p-0">
                <div className="flex items-center gap-2 px-4 py-3 sm:px-5 sm:py-4">
                  <button
                    type="button"
                    onClick={() => setExpandedId(expanded ? null : playlist.id)}
                    className="flex min-w-0 flex-1 items-center gap-3 rounded-[var(--nv-radius-control)] text-left outline-none transition-colors focus-visible:shadow-[var(--nv-shadow-focus)]"
                    aria-expanded={expanded}
                  >
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
                      <ListVideo size={19} aria-hidden="true" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="truncate text-sm font-semibold text-[var(--nv-text-primary)] sm:text-base">
                          {playlist.name}
                        </h3>
                        <Tag tone="neutral">{itemCount}</Tag>
                      </div>
                      <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">
                        {t('playlists.itemCount', { count: String(itemCount) })}
                      </p>
                    </div>
                    {expanded ? (
                      <ChevronUp size={18} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                    ) : (
                      <ChevronDown size={18} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                    )}
                  </button>

                  <Button
                    type="button"
                    size="sm"
                    variant="danger"
                    iconOnly
                    onClick={() => void handleDelete(playlist.id)}
                    aria-label={t('playlists.deletePlaylist')}
                    title={t('playlists.deletePlaylist')}
                  >
                    <Trash2 size={15} aria-hidden="true" />
                  </Button>
                </div>

                {expanded && (
                  <div className="border-t border-[var(--nv-border-subtle)]">
                    {!playlist.items || playlist.items.length === 0 ? (
                      <div className="px-5 py-8 text-center text-sm text-[var(--nv-text-tertiary)]">
                        {t('playlists.emptyList')}
                      </div>
                    ) : (
                      <div className="divide-y divide-[var(--nv-border-subtle)]">
                        {playlist.items.map((item) => (
                          <div
                            key={item.id}
                            className="group flex items-center gap-3 px-4 py-3 transition-colors hover:bg-[var(--nv-bg-hover)] sm:px-5"
                          >
                            <Link
                              to={`/play/${item.media_id}`}
                              className="relative h-14 w-24 shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]"
                              aria-label={`播放 ${item.media?.title || t('history.unknownMedia')}`}
                            >
                              <img
                                src={streamApi.getPosterUrl(item.media_id)}
                                alt={item.media?.title || ''}
                                className="h-full w-full object-cover"
                                onError={(event) => {
                                  (event.currentTarget as HTMLImageElement).style.display = 'none'
                                }}
                              />
                              <div className="absolute inset-0 flex items-center justify-center bg-black/45 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                                <Play size={16} className="text-white" fill="currentColor" aria-hidden="true" />
                              </div>
                            </Link>

                            <Link
                              to={`/media/${item.media_id}`}
                              className="min-w-0 flex-1 truncate text-sm font-medium text-[var(--nv-text-primary)] transition-colors hover:text-[var(--nv-action-primary)] focus-visible:text-[var(--nv-action-primary)]"
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
              </Surface>
            )
          })}

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
