import { useEffect, useState } from 'react'
import { commentApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import { useDialog } from '@/components/Dialog'
import { useTranslation } from '@/i18n'
import type { Comment } from '@/types'
import { ChevronDown, ChevronUp, MessageSquare, Send, Star, Trash2 } from 'lucide-react'
import { Button, EmptyState, Input, Surface, Tag } from '@/components/design-system'

interface CommentSectionProps {
  mediaId: string
}

export default function CommentSection({ mediaId }: CommentSectionProps) {
  const user = useAuthStore((state) => state.user)
  const { t } = useTranslation()
  const [comments, setComments] = useState<Comment[]>([])
  const [total, setTotal] = useState(0)
  const [avgRating, setAvgRating] = useState(0)
  const [ratingCount, setRatingCount] = useState(0)
  const [content, setContent] = useState('')
  const [rating, setRating] = useState(0)
  const [hoverRating, setHoverRating] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const toast = useToast()
  const dialog = useDialog()

  useEffect(() => {
    void loadComments()
  }, [mediaId, page])

  useEffect(() => {
    setExpanded(false)
    setPage(1)
  }, [mediaId])

  const loadComments = async () => {
    setLoading(true)
    try {
      const res = await commentApi.listByMedia(mediaId, page, 10)
      setComments(res.data.data || [])
      setTotal(res.data.total)
      setAvgRating(res.data.avg_rating)
      setRatingCount(res.data.rating_count)
    } catch {
      toast.error(t('comment.loadFailed'))
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
      await loadComments()
    } catch {
      toast.error(t('comment.submitFailed'))
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await dialog.confirm({
      title: t('comment.deleteConfirm'),
      confirmText: t('common.delete') ?? '删除',
      cancelText: t('common.cancel') ?? '取消',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await commentApi.delete(id)
      await loadComments()
    } catch {
      toast.error(t('comment.deleteFailed'))
    }
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    const locale = t('common.confirm') !== 'Confirm' ? 'zh-CN' : undefined
    return date.toLocaleDateString(locale, { year: 'numeric', month: 'short', day: 'numeric' })
  }

  const totalPages = Math.ceil(total / 10)
  const activeRating = hoverRating || rating
  const summaryText = loading
    ? '正在加载评价…'
    : total > 0
      ? `${total} 条评价${ratingCount > 0 ? ` · ${avgRating.toFixed(1)} 分` : ''}`
      : '暂无评价 · 可展开评分或留言'

  return (
    <section
      className="nv-comment-section"
      aria-labelledby="comment-section-title"
      data-expanded={expanded ? 'true' : 'false'}
    >
      <button
        type="button"
        className="nv-comment-summary"
        aria-expanded={expanded}
        aria-controls="comment-expanded-content"
        onClick={() => setExpanded((value) => !value)}
      >
        <span className="nv-comment-summary-main">
          <span className="nv-comment-summary-icon" aria-hidden="true">
            <MessageSquare size={17} />
          </span>
          <span className="nv-comment-summary-copy">
            <span id="comment-section-title" className="nv-comment-summary-title">评价</span>
            <span className="nv-comment-summary-meta">{summaryText}</span>
          </span>
        </span>

        <span className="nv-comment-summary-actions">
          {ratingCount > 0 && (
            <Tag tone="quality">
              <Star size={11} fill="currentColor" aria-hidden="true" />
              {avgRating.toFixed(1)}
            </Tag>
          )}
          <span className="nv-comment-summary-toggle-label">{expanded ? '收起' : '查看评价'}</span>
          {expanded ? <ChevronUp size={16} aria-hidden="true" /> : <ChevronDown size={16} aria-hidden="true" />}
        </span>
      </button>

      {expanded && (
        <div id="comment-expanded-content" className="nv-comment-expanded">
          <Surface className="nv-comment-composer space-y-4 p-4 sm:p-5">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium text-[var(--nv-text-secondary)]">{t('media.rating')}：</span>
              <div className="flex flex-wrap gap-0.5" role="radiogroup" aria-label={t('media.rating')}>
                {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((value) => {
                  const active = value <= activeRating
                  return (
                    <button
                      key={value}
                      type="button"
                      role="radio"
                      aria-checked={rating === value}
                      aria-label={`${value}/10`}
                      onClick={() => setRating(value === rating ? 0 : value)}
                      onMouseEnter={() => setHoverRating(value)}
                      onMouseLeave={() => setHoverRating(0)}
                      className="rounded p-0.5 transition-[background-color,color,transform] hover:scale-[1.025] hover:bg-[var(--nv-bg-hover)] focus-visible:outline-none focus-visible:shadow-[var(--nv-shadow-focus)]"
                      style={{ color: active ? 'var(--nv-status-rating)' : 'var(--nv-text-tertiary)' }}
                    >
                      <Star size={16} fill={active ? 'currentColor' : 'none'} aria-hidden="true" />
                    </button>
                  )
                })}
              </div>
              {rating > 0 && <Tag tone="quality">{rating}/10</Tag>}
            </div>

            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                type="text"
                value={content}
                onChange={(event) => setContent(event.target.value)}
                placeholder={t('comment.placeholder')}
                className="flex-1"
                onKeyDown={(event) => {
                  if (event.key === 'Enter') void handleSubmit()
                }}
              />
              <Button type="button" variant="primary" onClick={() => void handleSubmit()} disabled={!content.trim()}>
                <Send size={15} aria-hidden="true" />
                {t('comment.submit') || '发表'}
              </Button>
            </div>
          </Surface>

          {loading ? (
            <div className="nv-comment-loading space-y-3" aria-label="正在加载评论">
              {[1, 2, 3].map((index) => (
                <div key={index} className="skeleton h-24 rounded-[var(--nv-radius-card)]" />
              ))}
            </div>
          ) : comments.length === 0 ? (
            <EmptyState
              className="nv-comment-empty"
              icon={<MessageSquare size={22} aria-hidden="true" />}
              title={t('comment.noComments')}
              description="成为第一个留下评分或评论的人。"
            />
          ) : (
            <div className="nv-comment-list space-y-3">
              {comments.map((comment) => (
                <Surface key={comment.id} as="article" className="group p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex min-w-0 items-center gap-3">
                      <div className="nv-comment-avatar flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-sm font-bold text-[var(--nv-text-on-brand)]">
                        {comment.user?.username?.charAt(0).toUpperCase() || '?'}
                      </div>
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                          <span className="truncate text-sm font-medium text-[var(--nv-text-primary)]">
                            {comment.user?.username || '未知用户'}
                          </span>
                          <time className="text-xs text-[var(--nv-text-tertiary)]" dateTime={comment.created_at}>
                            {formatDate(comment.created_at)}
                          </time>
                          {comment.rating > 0 && (
                            <Tag tone="quality">
                              <Star size={11} fill="currentColor" aria-hidden="true" />
                              {comment.rating}
                            </Tag>
                          )}
                        </div>
                      </div>
                    </div>

                    {(comment.user_id === user?.id || user?.role === 'admin') && (
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        iconOnly
                        onClick={() => void handleDelete(comment.id)}
                        className="opacity-70 sm:opacity-0 sm:group-hover:opacity-100 sm:focus:opacity-100"
                        aria-label={t('common.delete') ?? '删除'}
                        title={t('common.delete') ?? '删除'}
                      >
                        <Trash2 size={14} aria-hidden="true" />
                      </Button>
                    )}
                  </div>
                  <p className="mt-3 whitespace-pre-wrap break-words text-sm leading-6 text-[var(--nv-text-secondary)]">
                    {comment.content}
                  </p>
                </Surface>
              ))}
            </div>
          )}

          {totalPages > 1 && (
            <nav className="nv-comment-pagination flex flex-wrap justify-center gap-1.5 pt-2" aria-label="评论分页">
              {Array.from({ length: totalPages }, (_, index) => index + 1).map((pageNumber) => (
                <Button
                  key={pageNumber}
                  type="button"
                  variant={pageNumber === page ? 'secondary' : 'ghost'}
                  size="sm"
                  onClick={() => setPage(pageNumber)}
                  className={pageNumber === page ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : undefined}
                  aria-current={pageNumber === page ? 'page' : undefined}
                >
                  {pageNumber}
                </Button>
              ))}
            </nav>
          )}
        </div>
      )}
    </section>
  )
}
