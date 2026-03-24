import { useState, useEffect } from 'react'
import { commentApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import type { Comment } from '@/types'
import { MessageSquare, Star, Send, Trash2 } from 'lucide-react'

interface CommentSectionProps {
  mediaId: string
}

export default function CommentSection({ mediaId }: CommentSectionProps) {
  const user = useAuthStore((s) => s.user)
  const [comments, setComments] = useState<Comment[]>([])
  const [total, setTotal] = useState(0)
  const [avgRating, setAvgRating] = useState(0)
  const [ratingCount, setRatingCount] = useState(0)
  const [content, setContent] = useState('')
  const [rating, setRating] = useState(0)
  const [hoverRating, setHoverRating] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const toast = useToast()

  useEffect(() => {
    loadComments()
  }, [mediaId, page])

  const loadComments = async () => {
    setLoading(true)
    try {
      const res = await commentApi.listByMedia(mediaId, page, 10)
      setComments(res.data.data || [])
      setTotal(res.data.total)
      setAvgRating(res.data.avg_rating)
      setRatingCount(res.data.rating_count)
    } catch {
      toast.error('加载评论失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = async () => {
    if (!content.trim()) return
    try {
      await commentApi.create(mediaId, {
        content: content.trim(),
        rating: rating > 0 ? rating : undefined,
      })
      setContent('')
      setRating(0)
      loadComments()
    } catch {
      toast.error('发表评论失败')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除这条评论？')) return
    try {
      await commentApi.delete(id)
      loadComments()
    } catch {
      toast.error('删除评论失败')
    }
  }

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr)
    return d.toLocaleDateString('zh-CN', { year: 'numeric', month: 'short', day: 'numeric' })
  }

  const totalPages = Math.ceil(total / 10)

  return (
    <section className="space-y-4">
      <h3 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <MessageSquare size={20} className="text-neon" />
        评论与评分
        {ratingCount > 0 && (
          <span className="ml-2 flex items-center gap-1 text-sm font-normal text-yellow-400">
            <Star size={14} fill="currentColor" />
            {avgRating.toFixed(1)}
            <span className="text-surface-500">({ratingCount}人评价)</span>
          </span>
        )}
      </h3>

      {/* 发表评论 */}
      <div className="glass-panel rounded-xl p-4 space-y-3">
        {/* 评分 */}
        <div className="flex items-center gap-2">
          <span className="text-sm text-surface-400">我的评分：</span>
          <div className="flex gap-0.5">
            {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((v) => (
              <button
                key={v}
                onClick={() => setRating(v === rating ? 0 : v)}
                onMouseEnter={() => setHoverRating(v)}
                onMouseLeave={() => setHoverRating(0)}
                className="p-0.5"
              >
                <Star
                  size={16}
                  className={
                    v <= (hoverRating || rating)
                      ? 'text-yellow-400'
                      : 'text-surface-600'
                  }
                  fill={v <= (hoverRating || rating) ? 'currentColor' : 'none'}
                />
              </button>
            ))}
          </div>
          {rating > 0 && <span className="text-sm text-yellow-400">{rating}/10</span>}
        </div>

        {/* 评论输入 */}
        <div className="flex gap-2">
          <input
            type="text"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="分享你的观影感受..."
            className="flex-1 rounded-xl px-4 py-2.5 text-sm text-white placeholder-surface-500 outline-none"
            style={{ background: 'var(--bg-input)', border: '1px solid var(--glass-border)', color: 'var(--text-primary)' }}
            onKeyDown={(e) => e.key === 'Enter' && handleSubmit()}
          />
          <button
            onClick={handleSubmit}
            disabled={!content.trim()}
            className="rounded-xl px-4 py-2.5 text-sm font-medium text-white disabled:opacity-50 transition-all"
            style={{ background: 'linear-gradient(135deg, rgba(0,240,255,0.9), rgba(0,180,220,0.9))', boxShadow: '0 0 10px rgba(0,240,255,0.2)' }}
          >
            <Send size={16} />
          </button>
        </div>
      </div>

      {/* 评论列表 */}
      {loading ? (
        <div className="animate-pulse space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
      ) : comments.length === 0 ? (
        <div className="py-8 text-center text-sm" style={{ color: 'var(--text-tertiary)' }}>
          还没有评论，来做第一个评论者吧
        </div>
      ) : (
        <div className="space-y-3">
          {comments.map((comment) => (
            <div key={comment.id} className="glass-panel-subtle group rounded-xl p-4">
              <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold" style={{ background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))', boxShadow: 'var(--shadow-neon)', color: 'var(--text-on-neon)' }}>
                    {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                  </div>
                  <div>
                    <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                      {comment.user?.username || '未知用户'}
                    </span>
                    <span className="ml-2 text-xs text-surface-500">{formatDate(comment.created_at)}</span>
                  </div>
                  {comment.rating > 0 && (
                    <span className="flex items-center gap-0.5 text-xs text-yellow-400">
                      <Star size={12} fill="currentColor" />
                      {comment.rating}
                    </span>
                  )}
                </div>

                {/* 删除按钮（本人或管理员可见） */}
                {(comment.user_id === user?.id || user?.role === 'admin') && (
                  <button
                    onClick={() => handleDelete(comment.id)}
                    className="rounded p-1 text-surface-600 opacity-0 group-hover:opacity-100 hover:text-red-400"
                  >
                    <Trash2 size={14} />
                  </button>
                )}
              </div>
              <p className="mt-2 text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>{comment.content}</p>
            </div>
          ))}
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="flex justify-center gap-2 pt-2">
          {Array.from({ length: totalPages }, (_, i) => i + 1).map((p) => (
            <button
              key={p}
              onClick={() => setPage(p)}
              className={`rounded-lg px-3 py-1 text-sm transition-all ${
                p === page
                  ? ''
                  : 'hover:opacity-80'
              }`}
              style={p === page ? {
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: 'var(--shadow-neon)',
                color: 'var(--text-on-neon)',
              } : {
                background: 'var(--bg-card)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
              }}
            >
              {p}
            </button>
          ))}
        </div>
      )}
    </section>
  )
}
