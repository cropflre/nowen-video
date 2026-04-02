import { useState, useEffect, useCallback } from 'react'
import type { Tag } from '@/types'
import { tagApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Tags,
  Plus,
  Trash2,
  Edit3,
  Check,
  X,
  Loader2,
  Palette,
  Hash,
} from 'lucide-react'
import clsx from 'clsx'

// 预设颜色
const PRESET_COLORS = [
  '#3b82f6', '#ef4444', '#22c55e', '#f59e0b', '#8b5cf6',
  '#ec4899', '#06b6d4', '#f97316', '#14b8a6', '#6366f1',
  '#a855f7', '#84cc16',
]

// 预设分类
const PRESET_CATEGORIES = ['类型', '心情', '场景', '评价', '自定义']

interface TagManagerProps {
  onTagSelect?: (tag: Tag) => void
  selectedMediaIds?: string[]
}

export default function TagManager({ onTagSelect, selectedMediaIds }: TagManagerProps) {
  const toast = useToast()
  const [tags, setTags] = useState<Tag[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [filterCategory, setFilterCategory] = useState('')

  // 创建表单
  const [formName, setFormName] = useState('')
  const [formColor, setFormColor] = useState('#3b82f6')
  const [formCategory, setFormCategory] = useState('')
  const [creating, setCreating] = useState(false)

  // 编辑表单
  const [editName, setEditName] = useState('')
  const [editColor, setEditColor] = useState('')
  const [editCategory, setEditCategory] = useState('')

  const fetchTags = useCallback(async () => {
    try {
      const res = await tagApi.list(filterCategory || undefined)
      setTags(res.data.data || [])
    } catch {
      toast.error('加载标签失败')
    } finally {
      setLoading(false)
    }
  }, [filterCategory])

  useEffect(() => {
    fetchTags()
  }, [fetchTags])

  const handleCreate = async () => {
    if (!formName.trim()) { toast.error('请输入标签名称'); return }
    setCreating(true)
    try {
      await tagApi.create({ name: formName.trim(), color: formColor, category: formCategory })
      toast.success('标签已创建')
      setShowCreate(false)
      setFormName('')
      setFormColor('#3b82f6')
      setFormCategory('')
      fetchTags()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '创建失败')
    } finally {
      setCreating(false)
    }
  }

  const handleUpdate = async (id: string) => {
    try {
      await tagApi.update(id, { name: editName, color: editColor, category: editCategory })
      toast.success('标签已更新')
      setEditingId(null)
      fetchTags()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '更新失败')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除该标签？关联的媒体标签也会被移除。')) return
    try {
      await tagApi.delete(id)
      toast.success('标签已删除')
      fetchTags()
    } catch {
      toast.error('删除失败')
    }
  }

  const handleBatchAdd = async (tagId: string) => {
    if (!selectedMediaIds?.length) return
    try {
      const res = await tagApi.batchAdd({ media_ids: selectedMediaIds, tag_ids: [tagId] })
      toast.success(`已为 ${selectedMediaIds.length} 个媒体添加标签（新增 ${res.data.data.added} 条）`)
    } catch {
      toast.error('批量添加失败')
    }
  }

  const startEdit = (tag: Tag) => {
    setEditingId(tag.id)
    setEditName(tag.name)
    setEditColor(tag.color)
    setEditCategory(tag.category)
  }

  // 获取唯一分类列表
  const categories = [...new Set(tags.map(t => t.category).filter(Boolean))]

  return (
    <section>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Tags size={20} className="text-neon/60" />
          标签管理
        </h2>
        <button
          onClick={() => setShowCreate(!showCreate)}
          className="btn-primary gap-1.5 px-3.5 py-2 text-xs"
        >
          <Plus size={14} />
          新建标签
        </button>
      </div>

      {/* 分类筛选 */}
      {categories.length > 0 && (
        <div className="mb-4 flex flex-wrap gap-2">
          <button
            onClick={() => setFilterCategory('')}
            className={clsx(
              'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
              !filterCategory ? 'bg-neon/10 text-neon' : 'text-surface-400 hover:text-surface-200'
            )}
            style={!filterCategory ? undefined : { background: 'var(--nav-hover-bg)' }}
          >
            全部
          </button>
          {categories.map(cat => (
            <button
              key={cat}
              onClick={() => setFilterCategory(cat)}
              className={clsx(
                'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                filterCategory === cat ? 'bg-neon/10 text-neon' : 'text-surface-400 hover:text-surface-200'
              )}
              style={filterCategory === cat ? undefined : { background: 'var(--nav-hover-bg)' }}
            >
              {cat}
            </button>
          ))}
        </div>
      )}

      {/* 创建表单 */}
      {showCreate && (
        <div className="glass-panel mb-4 animate-slide-up rounded-xl p-5 space-y-4">
          <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>创建标签</h3>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>标签名称</label>
              <input
                type="text"
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                className="input w-full"
                placeholder="如：科幻、温馨、经典"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>分类</label>
              <select
                value={formCategory}
                onChange={(e) => setFormCategory(e.target.value)}
                className="input w-full"
              >
                <option value="">无分类</option>
                {PRESET_CATEGORIES.map(c => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>颜色</label>
              <div className="flex flex-wrap gap-1.5">
                {PRESET_COLORS.map(color => (
                  <button
                    key={color}
                    onClick={() => setFormColor(color)}
                    className={clsx(
                      'h-6 w-6 rounded-full transition-all',
                      formColor === color && 'ring-2 ring-white ring-offset-2 ring-offset-[var(--bg-primary)]'
                    )}
                    style={{ background: color }}
                  />
                ))}
              </div>
            </div>
          </div>
          <div className="flex items-center justify-end gap-2 pt-2">
            <button onClick={() => setShowCreate(false)} className="btn-ghost px-4 py-2 text-sm">取消</button>
            <button onClick={handleCreate} disabled={creating} className="btn-primary gap-1.5 px-4 py-2 text-sm">
              {creating ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
              创建
            </button>
          </div>
        </div>
      )}

      {/* 标签列表 */}
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 size={24} className="animate-spin text-neon/40" />
        </div>
      ) : tags.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {tags.map(tag => (
            <div
              key={tag.id}
              className="group relative flex items-center gap-2 rounded-xl px-3 py-2 transition-all hover:scale-105"
              style={{
                background: `${tag.color}15`,
                border: `1px solid ${tag.color}30`,
              }}
            >
              {editingId === tag.id ? (
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    className="input w-24 text-xs py-1"
                    autoFocus
                  />
                  <div className="flex gap-1">
                    {PRESET_COLORS.slice(0, 6).map(c => (
                      <button
                        key={c}
                        onClick={() => setEditColor(c)}
                        className={clsx('h-4 w-4 rounded-full', editColor === c && 'ring-1 ring-white')}
                        style={{ background: c }}
                      />
                    ))}
                  </div>
                  <button onClick={() => handleUpdate(tag.id)} className="text-green-400 hover:text-green-300">
                    <Check size={14} />
                  </button>
                  <button onClick={() => setEditingId(null)} className="text-surface-400 hover:text-surface-200">
                    <X size={14} />
                  </button>
                </div>
              ) : (
                <>
                  <span
                    className="h-2.5 w-2.5 rounded-full flex-shrink-0"
                    style={{ background: tag.color }}
                  />
                  <span
                    className="text-sm font-medium cursor-pointer"
                    style={{ color: tag.color }}
                    onClick={() => {
                      if (selectedMediaIds?.length) {
                        handleBatchAdd(tag.id)
                      } else {
                        onTagSelect?.(tag)
                      }
                    }}
                  >
                    {tag.name}
                  </span>
                  {tag.category && (
                    <span className="text-[10px] text-surface-500">{tag.category}</span>
                  )}
                  <span className="text-[10px] text-surface-600">{tag.usage_count}</span>
                  <div className="hidden group-hover:flex items-center gap-0.5 ml-1">
                    <button onClick={() => startEdit(tag)} className="text-surface-400 hover:text-neon p-0.5">
                      <Edit3 size={12} />
                    </button>
                    <button onClick={() => handleDelete(tag.id)} className="text-surface-400 hover:text-red-400 p-0.5">
                      <Trash2 size={12} />
                    </button>
                  </div>
                </>
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-12 text-center">
          <div>
            <Hash size={32} className="mx-auto mb-2 text-surface-600" />
            <p className="text-sm text-surface-500">暂无标签</p>
            <p className="mt-1 text-xs text-surface-600">点击「新建标签」开始创建</p>
          </div>
        </div>
      )}
    </section>
  )
}
