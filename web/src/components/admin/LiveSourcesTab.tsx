import { useState, useEffect, useCallback } from 'react'
import { liveApi } from '@/api'
import type { LiveSource, LivePlaylist } from '@/types'
import { useToast } from '@/components/Toast'
import {
  Radio,
  Plus,
  Upload,
  Search,
  Filter,
  Trash2,
  Edit3,
  Check,
  X,
  Loader2,
  Circle,
  RefreshCw,
  Power,
  Link,
  FileText,
  AlertCircle,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'
import clsx from 'clsx'

type ModalType = 'add' | 'edit' | 'import' | null

interface SourceForm {
  name: string
  url: string
  type: 'iptv' | 'custom' | 'rtmp'
  category: string
  logo: string
  quality: string
}

interface ImportForm {
  name: string
  url: string
  file_path: string
}

const EMPTY_SOURCE_FORM: SourceForm = { name: '', url: '', type: 'iptv', category: '', logo: '', quality: '' }
const EMPTY_IMPORT_FORM: ImportForm = { name: '', url: '', file_path: '' }
const QUALITY_OPTIONS = ['', 'SD', 'HD', 'FHD', '4K']
const TYPE_OPTIONS = [
  { value: 'iptv', label: 'IPTV' },
  { value: 'custom', label: '自定义' },
  { value: 'rtmp', label: 'RTMP' },
]

export default function LiveSourcesTab() {
  const toast = useToast()

  // 数据状态
  const [sources, setSources] = useState<LiveSource[]>([])
  const [playlists, setPlaylists] = useState<LivePlaylist[]>([])
  const [categories, setCategories] = useState<string[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  // 筛选状态
  const [keyword, setKeyword] = useState('')
  const [selectedCategory, setSelectedCategory] = useState('')
  const [page, setPage] = useState(1)
  const pageSize = 20

  // 弹窗状态
  const [modal, setModal] = useState<ModalType>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [sourceForm, setSourceForm] = useState<SourceForm>(EMPTY_SOURCE_FORM)
  const [importForm, setImportForm] = useState<ImportForm>(EMPTY_IMPORT_FORM)
  const [saving, setSaving] = useState(false)

  // 操作状态
  const [checkingId, setCheckingId] = useState<string | null>(null)
  const [batchChecking, setBatchChecking] = useState(false)
  const [togglingId, setTogglingId] = useState<string | null>(null)

  // 获取直播源列表
  const fetchSources = useCallback(async () => {
    setLoading(true)
    try {
      const res = await liveApi.listSourcesAdmin({
        category: selectedCategory || undefined,
        keyword: keyword || undefined,
        page,
        size: pageSize,
      })
      setSources(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error('获取直播源列表失败')
    } finally {
      setLoading(false)
    }
  }, [selectedCategory, keyword, page])

  // 获取分类和播放列表
  const fetchMeta = useCallback(async () => {
    try {
      const [catRes, plRes] = await Promise.all([
        liveApi.getCategories(),
        liveApi.listPlaylists(),
      ])
      setCategories(catRes.data.data || [])
      setPlaylists(plRes.data.data || [])
    } catch {
      // 静默
    }
  }, [])

  useEffect(() => { fetchSources() }, [fetchSources])
  useEffect(() => { fetchMeta() }, [fetchMeta])

  // ==================== 操作处理 ====================

  // 添加直播源
  const handleAddSource = async () => {
    if (!sourceForm.name.trim() || !sourceForm.url.trim()) {
      toast.error('名称和URL为必填项')
      return
    }
    setSaving(true)
    try {
      await liveApi.addSource(sourceForm)
      toast.success('直播源添加成功')
      setModal(null)
      setSourceForm(EMPTY_SOURCE_FORM)
      fetchSources()
      fetchMeta()
    } catch {
      toast.error('添加直播源失败')
    } finally {
      setSaving(false)
    }
  }

  // 编辑直播源
  const handleEditSource = async () => {
    if (!editingId || !sourceForm.name.trim() || !sourceForm.url.trim()) {
      toast.error('名称和URL为必填项')
      return
    }
    setSaving(true)
    try {
      await liveApi.updateSource(editingId, sourceForm)
      toast.success('直播源更新成功')
      setModal(null)
      setEditingId(null)
      setSourceForm(EMPTY_SOURCE_FORM)
      fetchSources()
      fetchMeta()
    } catch {
      toast.error('更新直播源失败')
    } finally {
      setSaving(false)
    }
  }

  // 删除直播源
  const handleDeleteSource = async (id: string, name: string) => {
    if (!confirm(`确定要删除直播源「${name}」吗？`)) return
    try {
      await liveApi.deleteSource(id)
      toast.success('已删除')
      setSources(prev => prev.filter(s => s.id !== id))
      setTotal(prev => prev - 1)
    } catch {
      toast.error('删除失败')
    }
  }

  // 切换启用/禁用
  const handleToggleActive = async (id: string) => {
    setTogglingId(id)
    try {
      const res = await liveApi.toggleSourceActive(id)
      const updated = res.data.data
      setSources(prev => prev.map(s => s.id === id ? { ...s, is_active: updated.is_active } : s))
      toast.success(updated.is_active ? '已启用' : '已禁用')
    } catch {
      toast.error('操作失败')
    } finally {
      setTogglingId(null)
    }
  }

  // 检测单个直播源
  const handleCheckSource = async (id: string) => {
    setCheckingId(id)
    try {
      const res = await liveApi.checkSource(id)
      setSources(prev => prev.map(s => s.id === id ? { ...s, check_status: res.data.status, last_check_at: new Date().toISOString() } : s))
      toast.success(`检测结果: ${res.data.status === 'ok' ? '正常' : res.data.status === 'timeout' ? '超时' : '异常'}`)
    } catch {
      toast.error('检测失败')
    } finally {
      setCheckingId(null)
    }
  }

  // 批量检测
  const handleBatchCheck = async () => {
    setBatchChecking(true)
    try {
      const res = await liveApi.batchCheck()
      const results = res.data.data
      setSources(prev => prev.map(s => ({
        ...s,
        check_status: results[s.id] || s.check_status,
        last_check_at: results[s.id] ? new Date().toISOString() : s.last_check_at,
      })))
      const okCount = Object.values(results).filter(v => v === 'ok').length
      toast.success(`批量检测完成: ${okCount}/${Object.keys(results).length} 个正常`)
    } catch {
      toast.error('批量检测失败')
    } finally {
      setBatchChecking(false)
    }
  }

  // 导入M3U
  const handleImportM3U = async () => {
    if (!importForm.name.trim()) {
      toast.error('请输入播放列表名称')
      return
    }
    if (!importForm.url.trim() && !importForm.file_path.trim()) {
      toast.error('请输入M3U文件URL或本地路径')
      return
    }
    setSaving(true)
    try {
      const res = await liveApi.importM3U({
        name: importForm.name,
        url: importForm.url || undefined,
        file_path: importForm.file_path || undefined,
      })
      toast.success(`导入成功，共 ${res.data.count} 个频道`)
      setModal(null)
      setImportForm(EMPTY_IMPORT_FORM)
      fetchSources()
      fetchMeta()
    } catch (err: unknown) {
      toast.error((err as { response?: { data?: { error?: string } } })?.response?.data?.error || '导入失败')
    } finally {
      setSaving(false)
    }
  }

  // 删除播放列表
  const handleDeletePlaylist = async (id: string, name: string) => {
    if (!confirm(`确定要删除播放列表「${name}」吗？（不会删除已导入的直播源）`)) return
    try {
      await liveApi.deletePlaylist(id)
      toast.success('已删除')
      setPlaylists(prev => prev.filter(p => p.id !== id))
    } catch {
      toast.error('删除失败')
    }
  }

  // 打开编辑弹窗
  const openEdit = (source: LiveSource) => {
    setEditingId(source.id)
    setSourceForm({
      name: source.name,
      url: source.url,
      type: source.type,
      category: source.category,
      logo: source.logo,
      quality: source.quality,
    })
    setModal('edit')
  }

  // 状态指示器
  const StatusDot = ({ status }: { status: string }) => {
    if (status === 'ok') return <Circle size={8} className="fill-green-500 text-green-500" />
    if (status === 'error') return <Circle size={8} className="fill-red-500 text-red-500" />
    if (status === 'timeout') return <Circle size={8} className="fill-yellow-500 text-yellow-500" />
    return <Circle size={8} className="fill-surface-400 text-surface-400" />
  }

  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="space-y-6">
      {/* 功能引导说明 */}
      <div className="flex items-start gap-3 rounded-xl p-4" style={{ background: 'linear-gradient(135deg, rgba(0,180,220,0.05), rgba(120,0,255,0.03))', border: '1px solid var(--neon-blue-15)' }}>
        <Radio size={20} className="mt-0.5 flex-shrink-0 text-neon" />
        <div>
          <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>直播源管理</h3>
          <p className="mt-1 text-xs leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>
            在此管理系统中的所有直播频道源。您可以手动添加单个直播源，也可以通过导入 M3U 播放列表文件批量添加频道。
            添加的直播源将在「直播频道」页面中展示给所有用户。
          </p>
        </div>
      </div>

      {/* 操作按钮区 */}
      <div className="flex flex-wrap items-center gap-3">
        <button
          onClick={() => { setSourceForm(EMPTY_SOURCE_FORM); setModal('add') }}
          className="inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-medium transition-all hover:-translate-y-0.5"
          style={{ background: 'linear-gradient(135deg, var(--neon-blue), rgba(0,180,220,0.9))', color: 'var(--text-on-neon)', boxShadow: 'var(--shadow-neon)' }}
        >
          <Plus size={16} />
          添加直播源
        </button>
        <button
          onClick={() => { setImportForm(EMPTY_IMPORT_FORM); setModal('import') }}
          className="inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-medium transition-all hover:-translate-y-0.5"
          style={{ background: 'var(--bg-surface)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}
        >
          <Upload size={16} />
          导入 M3U
        </button>
        <button
          onClick={handleBatchCheck}
          disabled={batchChecking}
          className="inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-medium transition-all hover:-translate-y-0.5"
          style={{ background: 'var(--bg-surface)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}
        >
          {batchChecking ? <Loader2 size={16} className="animate-spin" /> : <RefreshCw size={16} />}
          批量检测
        </button>

        {/* 搜索和筛选 */}
        <div className="ml-auto flex items-center gap-2">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-surface-400" />
            <input
              type="text"
              placeholder="搜索名称或URL..."
              value={keyword}
              onChange={e => { setKeyword(e.target.value); setPage(1) }}
              className="input pl-8 text-sm"
              style={{ width: '200px' }}
            />
          </div>
          <div className="flex items-center gap-1.5">
            <Filter size={14} style={{ color: 'var(--text-tertiary)' }} />
            <select
              value={selectedCategory}
              onChange={e => { setSelectedCategory(e.target.value); setPage(1) }}
              className="input text-sm"
            >
              <option value="">全部分类</option>
              {categories.map(cat => (
                <option key={cat} value={cat}>{cat}</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* 直播源列表 */}
      <div className="glass-panel rounded-2xl overflow-hidden">
        <div className="flex items-center gap-2 px-6 py-4" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
          <Radio size={18} style={{ color: 'var(--neon-blue)' }} />
          <h3 className="font-display text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
            直播源列表
          </h3>
          <span className="ml-auto text-xs" style={{ color: 'var(--text-muted)' }}>
            共 {total} 个
          </span>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 size={24} className="animate-spin" style={{ color: 'var(--neon-blue)' }} />
          </div>
        ) : sources.length === 0 ? (
          <div className="py-12 text-center">
            <Radio size={32} className="mx-auto mb-3 text-surface-600" />
            <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {keyword || selectedCategory ? '没有找到匹配的直播源' : '暂无直播源，点击上方按钮添加或导入'}
            </p>
          </div>
        ) : (
          <div className="divide-y" style={{ borderColor: 'var(--border-subtle)' }}>
            {sources.map(source => (
              <div
                key={source.id}
                className={clsx(
                  'flex items-center gap-4 px-6 py-3.5 transition-colors hover:bg-white/[0.02]',
                  !source.is_active && 'opacity-50'
                )}
              >
                {/* Logo / 图标 */}
                <div className="flex-shrink-0">
                  {source.logo ? (
                    <img src={source.logo} alt={source.name} className="h-9 w-9 rounded-lg object-cover" />
                  ) : (
                    <div className="flex h-9 w-9 items-center justify-center rounded-lg" style={{ background: 'var(--bg-tertiary)' }}>
                      <Radio size={16} className="text-neon" />
                    </div>
                  )}
                </div>

                {/* 信息 */}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                      {source.name}
                    </p>
                    {!source.is_active && (
                      <span className="rounded px-1.5 py-0.5 text-[10px] font-medium bg-surface-500/10 text-surface-400">
                        已禁用
                      </span>
                    )}
                  </div>
                  <div className="mt-0.5 flex items-center gap-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                    <span className="truncate max-w-[300px]" title={source.url}>{source.url}</span>
                    {source.category && (
                      <span className="rounded px-1.5 py-0.5" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-tertiary)' }}>
                        {source.category}
                      </span>
                    )}
                    {source.quality && <span className="text-neon">{source.quality}</span>}
                    <span className="uppercase text-surface-500">{source.type}</span>
                  </div>
                </div>

                {/* 状态 */}
                <div className="flex items-center gap-1.5" title={`状态: ${source.check_status || '未检测'}`}>
                  <StatusDot status={source.check_status} />
                  <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                    {source.check_status === 'ok' ? '正常' : source.check_status === 'error' ? '异常' : source.check_status === 'timeout' ? '超时' : '未检测'}
                  </span>
                </div>

                {/* 操作按钮 */}
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => handleCheckSource(source.id)}
                    disabled={checkingId === source.id}
                    className="rounded-lg p-2 transition-colors hover:bg-white/[0.05]"
                    title="检测"
                  >
                    {checkingId === source.id ? (
                      <Loader2 size={15} className="animate-spin text-surface-400" />
                    ) : (
                      <RefreshCw size={15} style={{ color: 'var(--text-tertiary)' }} />
                    )}
                  </button>
                  <button
                    onClick={() => handleToggleActive(source.id)}
                    disabled={togglingId === source.id}
                    className={clsx('rounded-lg p-2 transition-colors hover:bg-white/[0.05]')}
                    title={source.is_active ? '禁用' : '启用'}
                  >
                    {togglingId === source.id ? (
                      <Loader2 size={15} className="animate-spin text-surface-400" />
                    ) : (
                      <Power size={15} className={source.is_active ? 'text-green-400' : 'text-surface-500'} />
                    )}
                  </button>
                  <button
                    onClick={() => openEdit(source)}
                    className="rounded-lg p-2 transition-colors hover:bg-white/[0.05]"
                    title="编辑"
                  >
                    <Edit3 size={15} style={{ color: 'var(--text-tertiary)' }} />
                  </button>
                  <button
                    onClick={() => handleDeleteSource(source.id, source.name)}
                    className="rounded-lg p-2 transition-colors hover:bg-red-500/10"
                    title="删除"
                  >
                    <Trash2 size={15} className="text-red-400" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* 分页 */}
        {totalPages > 1 && (
          <div className="flex items-center justify-center gap-3 px-6 py-4" style={{ borderTop: '1px solid var(--border-subtle)' }}>
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="rounded-lg p-2 transition-colors hover:bg-white/[0.05] disabled:opacity-30"
            >
              <ChevronLeft size={16} style={{ color: 'var(--text-secondary)' }} />
            </button>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              第 {page} 页 / 共 {totalPages} 页
            </span>
            <button
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="rounded-lg p-2 transition-colors hover:bg-white/[0.05] disabled:opacity-30"
            >
              <ChevronRight size={16} style={{ color: 'var(--text-secondary)' }} />
            </button>
          </div>
        )}
      </div>

      {/* 已导入的播放列表 */}
      {playlists.length > 0 && (
        <div className="glass-panel rounded-2xl overflow-hidden">
          <div className="flex items-center gap-2 px-6 py-4" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
            <FileText size={18} style={{ color: 'var(--neon-blue)' }} />
            <h3 className="font-display text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              已导入的播放列表
            </h3>
            <span className="ml-auto text-xs" style={{ color: 'var(--text-muted)' }}>
              共 {playlists.length} 个
            </span>
          </div>
          <div className="divide-y" style={{ borderColor: 'var(--border-subtle)' }}>
            {playlists.map(pl => (
              <div key={pl.id} className="flex items-center gap-4 px-6 py-3 transition-colors hover:bg-white/[0.02]">
                <div className="flex-shrink-0">
                  <FileText size={18} className="text-cyan-400" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{pl.name}</p>
                  <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                    {pl.source_count} 个频道
                    {pl.url && <> · <span className="truncate">{pl.url}</span></>}
                    {pl.last_update && <> · 更新于 {new Date(pl.last_update).toLocaleString('zh-CN')}</>}
                    {' · '}导入于 {new Date(pl.created_at).toLocaleString('zh-CN')}
                  </p>
                </div>
                <button
                  onClick={() => handleDeletePlaylist(pl.id, pl.name)}
                  className="rounded-lg p-2 transition-colors hover:bg-red-500/10"
                  title="删除播放列表"
                >
                  <Trash2 size={15} className="text-red-400" />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 提示信息 */}
      <div className="flex items-start gap-2 rounded-xl p-3 text-xs text-yellow-400/80" style={{ background: 'rgba(234, 179, 8, 0.03)', border: '1px solid rgba(234, 179, 8, 0.08)' }}>
        <AlertCircle size={14} className="mt-0.5 flex-shrink-0" />
        <span>
          直播源支持 m3u8、RTMP、RTSP 等常见流媒体协议。导入 M3U 播放列表时，系统会自动解析频道名称、分类和 Logo 信息。
          禁用的直播源不会在用户端的「直播频道」页面中显示。
        </span>
      </div>

      {/* ==================== 添加/编辑弹窗 ==================== */}
      {(modal === 'add' || modal === 'edit') && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setModal(null)}>
          <div
            className="w-full max-w-lg rounded-2xl p-6"
            style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-15)', backdropFilter: 'blur(20px)' }}
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-6">
              <h3 className="font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
                {modal === 'add' ? '添加直播源' : '编辑直播源'}
              </h3>
              <button onClick={() => setModal(null)} className="text-surface-400 hover:text-white">
                <X size={20} />
              </button>
            </div>

            <div className="space-y-4">
              {/* 名称 */}
              <div>
                <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  频道名称 <span className="text-red-400">*</span>
                </label>
                <input
                  type="text"
                  value={sourceForm.name}
                  onChange={e => setSourceForm(f => ({ ...f, name: e.target.value }))}
                  className="input w-full"
                  placeholder="例如: CCTV-1 综合"
                  autoFocus
                />
              </div>

              {/* URL */}
              <div>
                <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  直播源地址 <span className="text-red-400">*</span>
                </label>
                <input
                  type="text"
                  value={sourceForm.url}
                  onChange={e => setSourceForm(f => ({ ...f, url: e.target.value }))}
                  className="input w-full font-mono text-xs"
                  placeholder="例如: http://example.com/live/stream.m3u8"
                />
              </div>

              {/* 类型 + 画质 */}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>类型</label>
                  <select
                    value={sourceForm.type}
                    onChange={e => setSourceForm(f => ({ ...f, type: e.target.value as SourceForm['type'] }))}
                    className="input w-full"
                  >
                    {TYPE_OPTIONS.map(opt => (
                      <option key={opt.value} value={opt.value}>{opt.label}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>画质</label>
                  <select
                    value={sourceForm.quality}
                    onChange={e => setSourceForm(f => ({ ...f, quality: e.target.value }))}
                    className="input w-full"
                  >
                    {QUALITY_OPTIONS.map(q => (
                      <option key={q} value={q}>{q || '未指定'}</option>
                    ))}
                  </select>
                </div>
              </div>

              {/* 分类 */}
              <div>
                <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>分类</label>
                <input
                  type="text"
                  value={sourceForm.category}
                  onChange={e => setSourceForm(f => ({ ...f, category: e.target.value }))}
                  className="input w-full"
                  placeholder="例如: 央视、卫视、体育、电影"
                  list="category-suggestions"
                />
                <datalist id="category-suggestions">
                  {categories.map(cat => (
                    <option key={cat} value={cat} />
                  ))}
                </datalist>
              </div>

              {/* Logo */}
              <div>
                <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>Logo URL</label>
                <input
                  type="text"
                  value={sourceForm.logo}
                  onChange={e => setSourceForm(f => ({ ...f, logo: e.target.value }))}
                  className="input w-full text-xs"
                  placeholder="频道Logo图片地址（可选）"
                />
              </div>
            </div>

            {/* 按钮 */}
            <div className="mt-6 flex items-center justify-end gap-2">
              <button
                onClick={() => setModal(null)}
                className="rounded-xl px-4 py-2 text-sm font-medium transition-all"
                style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={modal === 'add' ? handleAddSource : handleEditSource}
                disabled={saving}
                className="btn-primary gap-1.5 px-5 py-2 text-sm"
              >
                {saving ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                {modal === 'add' ? '添加' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ==================== 导入M3U弹窗 ==================== */}
      {modal === 'import' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setModal(null)}>
          <div
            className="w-full max-w-lg rounded-2xl p-6"
            style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-15)', backdropFilter: 'blur(20px)' }}
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-6">
              <h3 className="font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
                导入 M3U 播放列表
              </h3>
              <button onClick={() => setModal(null)} className="text-surface-400 hover:text-white">
                <X size={20} />
              </button>
            </div>

            <div className="space-y-4">
              {/* 说明 */}
              <div className="rounded-lg p-3 text-xs leading-relaxed" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)', color: 'var(--text-tertiary)' }}>
                支持标准 M3U/M3U8 格式的播放列表文件。可以通过远程 URL 下载或指定服务器上的本地文件路径导入。
                系统会自动解析频道名称、分类、Logo 等信息。
              </div>

              {/* 名称 */}
              <div>
                <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  播放列表名称 <span className="text-red-400">*</span>
                </label>
                <input
                  type="text"
                  value={importForm.name}
                  onChange={e => setImportForm(f => ({ ...f, name: e.target.value }))}
                  className="input w-full"
                  placeholder="例如: 国内IPTV频道"
                  autoFocus
                />
              </div>

              {/* URL */}
              <div>
                <label className="mb-1.5 flex items-center gap-1.5 text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  <Link size={14} />
                  远程 URL
                </label>
                <input
                  type="text"
                  value={importForm.url}
                  onChange={e => setImportForm(f => ({ ...f, url: e.target.value, file_path: '' }))}
                  className="input w-full font-mono text-xs"
                  placeholder="https://example.com/iptv.m3u"
                  disabled={!!importForm.file_path}
                />
              </div>

              <div className="flex items-center gap-3">
                <div className="flex-1 border-t" style={{ borderColor: 'var(--border-subtle)' }} />
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>或</span>
                <div className="flex-1 border-t" style={{ borderColor: 'var(--border-subtle)' }} />
              </div>

              {/* 本地路径 */}
              <div>
                <label className="mb-1.5 flex items-center gap-1.5 text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  <FileText size={14} />
                  服务器本地路径
                </label>
                <input
                  type="text"
                  value={importForm.file_path}
                  onChange={e => setImportForm(f => ({ ...f, file_path: e.target.value, url: '' }))}
                  className="input w-full font-mono text-xs"
                  placeholder="/path/to/playlist.m3u"
                  disabled={!!importForm.url}
                />
              </div>
            </div>

            {/* 按钮 */}
            <div className="mt-6 flex items-center justify-end gap-2">
              <button
                onClick={() => setModal(null)}
                className="rounded-xl px-4 py-2 text-sm font-medium transition-all"
                style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={handleImportM3U}
                disabled={saving}
                className="btn-primary gap-1.5 px-5 py-2 text-sm"
              >
                {saving ? <Loader2 size={14} className="animate-spin" /> : <Upload size={14} />}
                导入
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
