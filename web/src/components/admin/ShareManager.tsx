import { useState, useEffect, useCallback } from 'react'
import type { ShareLink, Library } from '@/types'
import { shareApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Share2,
  Plus,
  Trash2,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  Lock,
  Unlock,
  Loader2,
  ToggleLeft,
  ToggleRight,
  Link2,
  Clock,
  Download,
} from 'lucide-react'
import clsx from 'clsx'

export default function ShareManager() {
  const toast = useToast()
  const [shares, setShares] = useState<ShareLink[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)

  const fetchShares = useCallback(async () => {
    try {
      const res = await shareApi.list(page, 20)
      setShares(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error('加载分享列表失败')
    } finally {
      setLoading(false)
    }
  }, [page])

  useEffect(() => {
    fetchShares()
  }, [fetchShares])

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除该分享链接？')) return
    try {
      await shareApi.delete(id)
      toast.success('分享链接已删除')
      fetchShares()
    } catch {
      toast.error('删除失败')
    }
  }

  const handleToggle = async (id: string) => {
    try {
      await shareApi.toggle(id)
      toast.success('状态已切换')
      fetchShares()
    } catch {
      toast.error('操作失败')
    }
  }

  const copyLink = (code: string) => {
    const url = `${window.location.origin}/share/${code}`
    navigator.clipboard.writeText(url)
    toast.success('链接已复制到剪贴板')
  }

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return '永不过期'
    const d = new Date(dateStr)
    if (d.getTime() < Date.now()) return '已过期'
    return d.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  return (
    <section>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Share2 size={20} className="text-neon/60" />
          分享链接管理
        </h2>
        <span className="text-xs text-surface-500">共 {total} 个分享</span>
      </div>

      <p className="mb-4 text-xs text-surface-500">
        💡 在媒体详情页点击「分享」按钮可创建分享链接。此处管理所有已创建的分享。
      </p>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 size={24} className="animate-spin text-neon/40" />
        </div>
      ) : shares.length > 0 ? (
        <div className="space-y-3">
          {shares.map(share => (
            <div
              key={share.id}
              className={clsx(
                'glass-panel-subtle group rounded-xl p-4 transition-all',
                !share.is_active && 'opacity-50'
              )}
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <Link2 size={14} className="text-neon flex-shrink-0" />
                    <span className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                      {share.title || `分享 ${share.code}`}
                    </span>
                    {share.password && (
                      <Lock size={12} className="text-amber-400 flex-shrink-0" title="需要密码" />
                    )}
                    {share.allow_download && (
                      <Download size={12} className="text-green-400 flex-shrink-0" title="允许下载" />
                    )}
                  </div>
                  <div className="flex flex-wrap items-center gap-3 text-xs text-surface-500">
                    <span className="font-mono text-neon/70">{share.code}</span>
                    <span className="flex items-center gap-1">
                      <Eye size={10} /> {share.view_count}{share.max_views > 0 ? `/${share.max_views}` : ''}
                    </span>
                    <span className="flex items-center gap-1">
                      <Clock size={10} /> {formatDate(share.expires_at)}
                    </span>
                    <span>
                      {share.media_id ? '📽️ 媒体' : '📺 剧集'}
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-1 flex-shrink-0">
                  <button
                    onClick={() => copyLink(share.code)}
                    className="rounded-lg p-1.5 text-surface-400 hover:text-neon hover:bg-neon-blue/5"
                    title="复制链接"
                  >
                    <Copy size={14} />
                  </button>
                  <button
                    onClick={() => window.open(`/share/${share.code}`, '_blank')}
                    className="rounded-lg p-1.5 text-surface-400 hover:text-neon hover:bg-neon-blue/5"
                    title="预览"
                  >
                    <ExternalLink size={14} />
                  </button>
                  <button
                    onClick={() => handleToggle(share.id)}
                    className="rounded-lg p-1.5 text-surface-400 hover:text-yellow-400 hover:bg-yellow-400/5"
                    title={share.is_active ? '禁用' : '启用'}
                  >
                    {share.is_active ? <ToggleRight size={14} className="text-green-400" /> : <ToggleLeft size={14} />}
                  </button>
                  <button
                    onClick={() => handleDelete(share.id)}
                    className="rounded-lg p-1.5 text-surface-400 hover:text-red-400 hover:bg-red-400/5"
                    title="删除"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            </div>
          ))}

          {/* 分页 */}
          {total > 20 && (
            <div className="flex items-center justify-center gap-2 pt-4">
              <button
                onClick={() => setPage(p => Math.max(1, p - 1))}
                disabled={page === 1}
                className="btn-ghost px-3 py-1.5 text-xs disabled:opacity-30"
              >
                上一页
              </button>
              <span className="text-xs text-surface-500">第 {page} 页</span>
              <button
                onClick={() => setPage(p => p + 1)}
                disabled={shares.length < 20}
                className="btn-ghost px-3 py-1.5 text-xs disabled:opacity-30"
              >
                下一页
              </button>
            </div>
          )}
        </div>
      ) : (
        <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-12 text-center">
          <div>
            <Share2 size={32} className="mx-auto mb-2 text-surface-600" />
            <p className="text-sm text-surface-500">暂无分享链接</p>
            <p className="mt-1 text-xs text-surface-600">在媒体详情页点击「分享」按钮创建</p>
          </div>
        </div>
      )}
    </section>
  )
}
