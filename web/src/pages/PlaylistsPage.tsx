import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { playlistApi, streamApi } from '@/api'
import type { Playlist } from '@/types'
import {
  ListVideo,
  Plus,
  Trash2,
  Play,
  ChevronDown,
  ChevronUp,
  X,
} from 'lucide-react'

export default function PlaylistsPage() {
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const fetchPlaylists = async () => {
    try {
      const res = await playlistApi.list()
      setPlaylists(res.data.data || [])
    } catch {
      // 静默处理
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchPlaylists()
  }, [])

  const handleCreate = async () => {
    if (!newName.trim()) return
    try {
      await playlistApi.create({ name: newName.trim() })
      setNewName('')
      setShowCreate(false)
      fetchPlaylists()
    } catch {
      alert('创建播放列表失败')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此播放列表？')) return
    try {
      await playlistApi.delete(id)
      setPlaylists((p) => p.filter((pl) => pl.id !== id))
    } catch {
      alert('删除失败')
    }
  }

  const handleRemoveItem = async (playlistId: string, mediaId: string) => {
    try {
      await playlistApi.removeItem(playlistId, mediaId)
      fetchPlaylists()
    } catch {
      alert('移除失败')
    }
  }

  return (
    <div>
      {/* 标题栏 */}
      <div className="mb-6 flex items-center justify-between">
        <h1 className="flex items-center gap-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <ListVideo size={24} className="text-neon" />
          播放列表
        </h1>
        <button
          onClick={() => setShowCreate(!showCreate)}
          className="btn-primary gap-1.5 px-3 py-2 text-sm"
        >
          <Plus size={16} />
          新建列表
        </button>
      </div>

      {/* 创建表单 */}
      {showCreate && (
        <div className="glass-panel mb-6 animate-slide-up rounded-xl p-4">
          <div className="flex gap-3">
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              className="input flex-1"
              placeholder="播放列表名称"
              autoFocus
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <button onClick={handleCreate} className="btn-primary px-4 py-2 text-sm">
              创建
            </button>
            <button
              onClick={() => setShowCreate(false)}
              className="btn-ghost px-4 py-2 text-sm"
            >
              取消
            </button>
          </div>
        </div>
      )}

      {/* 加载状态 */}
      {loading && (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="rounded-xl p-4" style={{
              background: 'var(--bg-card)',
              border: '1px solid var(--border-default)',
            }}>
              <div className="skeleton h-6 w-1/4 rounded" />
              <div className="skeleton mt-2 h-4 w-1/3 rounded" />
            </div>
          ))}
        </div>
      )}

      {/* 播放列表 */}
      {!loading && (
        <div className="space-y-4">
          {playlists.map((playlist) => (
            <div
              key={playlist.id}
              className="glass-panel overflow-hidden rounded-xl"
            >
              {/* 列表头 */}
              <div className="flex items-center justify-between p-4">
                <button
                  onClick={() =>
                    setExpandedId(expandedId === playlist.id ? null : playlist.id)
                  }
                  className="flex flex-1 items-center gap-3 text-left"
                >
                  <ListVideo size={20} className="text-neon" />
                  <div>
                    <h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>{playlist.name}</h3>
                    <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                      {playlist.items?.length || 0} 个项目
                    </p>
                  </div>
                  {expandedId === playlist.id ? (
                    <ChevronUp size={18} className="text-surface-400" />
                  ) : (
                    <ChevronDown size={18} className="text-surface-400" />
                  )}
                </button>
                <button
                  onClick={() => handleDelete(playlist.id)}
                  className="rounded-lg p-2 text-surface-500 transition-colors hover:text-red-400 hover:bg-red-400/5"
                  title="删除播放列表"
                >
                  <Trash2 size={16} />
                </button>
              </div>

              {/* 展开的项目列表 */}
              {expandedId === playlist.id && (
                <div style={{ borderTop: '1px solid var(--border-default)' }}>
                  {(!playlist.items || playlist.items.length === 0) && (
                    <div className="px-4 py-8 text-center text-sm" style={{ color: 'var(--text-tertiary)' }}>
                      此列表暂无内容，在媒体详情页将内容添加到此列表
                    </div>
                  )}
                  {playlist.items?.map((item) => (
                    <div
                      key={item.id}
                      className="group flex items-center gap-3 px-4 py-3 last:border-b-0 transition-colors hover:bg-neon-blue/3"
                      style={{ borderBottom: '1px solid var(--border-default)' }}
                    >
                      {/* 缩略图 */}
                      <Link
                        to={`/play/${item.media_id}`}
                        className="relative h-14 w-24 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-surface)' }}
                      >
                        <img
                          src={streamApi.getPosterUrl(item.media_id)}
                          alt={item.media?.title}
                          className="h-full w-full object-cover"
                          onError={(e) => {
                            (e.target as HTMLImageElement).style.display = 'none'
                          }}
                        />
                        <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
                          <Play size={16} className="text-white" fill="white" />
                        </div>
                      </Link>

                      {/* 信息 */}
                      <Link
                        to={`/media/${item.media_id}`}
                        className="flex-1 text-sm font-medium transition-colors hover:text-neon"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {item.media?.title || '未知媒体'}
                      </Link>

                      {/* 移除 */}
                      <button
                        onClick={() => handleRemoveItem(playlist.id, item.media_id)}
                        className="rounded-lg p-1.5 text-surface-500 opacity-0 transition-all hover:text-red-400 group-hover:opacity-100"
                        title="从列表移除"
                      >
                        <X size={14} />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}

          {/* 空状态 */}
          {playlists.length === 0 && (
            <div className="flex flex-col items-center justify-center py-24 text-center">
              <div
                className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl animate-float"
                style={{
                  background: 'rgba(0, 240, 255, 0.05)',
                  border: '1px solid rgba(0, 240, 255, 0.08)',
                }}
              >
                <ListVideo size={36} className="text-surface-600" />
              </div>
              <p className="font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>还没有播放列表</p>
              <p className="mt-1 text-sm" style={{ color: 'var(--text-muted)' }}>
                点击上方按钮创建自定义播放列表
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
