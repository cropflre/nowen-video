import { useState, useRef, useEffect } from 'react'
import type { CreateLibraryRequest, LibraryAdvancedSettings } from '@/types'
import {
  X,
  Film,
  Tv,
  Layers,
  Video,
  FolderPlus,
  ChevronDown,
  ChevronUp,
  AlertCircle,
  Loader2,
  Globe,
  Eye,
  Search,
} from 'lucide-react'
import FileBrowser from './FileBrowser'

// 内容类型配置
const LIBRARY_TYPES = [
  {
    value: 'movie' as const,
    label: '电影',
    desc: '各种类型电影',
    icon: Film,
    color: 'var(--neon-blue)',
    bg: 'var(--neon-blue-8)',
    border: 'var(--neon-blue-20)',
  },
  {
    value: 'tvshow' as const,
    label: '电视节目',
    desc: '电视剧、综艺等',
    icon: Tv,
    color: 'var(--neon-purple)',
    bg: 'var(--neon-purple-8)',
    border: 'var(--neon-purple-20)',
  },
  {
    value: 'mixed' as const,
    label: '混合影片',
    desc: '电影和电视节目',
    icon: Layers,
    color: '#F59E0B',
    bg: 'rgba(245, 158, 11, 0.08)',
    border: 'rgba(245, 158, 11, 0.2)',
  },
  {
    value: 'other' as const,
    label: '其他视频',
    desc: '个人视频、课程等',
    icon: Video,
    color: '#10B981',
    bg: 'rgba(16, 185, 129, 0.08)',
    border: 'rgba(16, 185, 129, 0.2)',
  },
]

// 元数据语言选项
const METADATA_LANG_OPTIONS = [
  { value: 'zh-CN', label: '中文简体' },
  { value: 'zh-TW', label: '中文繁體' },
  { value: 'en-US', label: 'English' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
  { value: 'fr', label: 'Français' },
  { value: 'de', label: 'Deutsch' },
  { value: 'es', label: 'Español' },
]

// 默认高级设置（媒体库级别）
const DEFAULT_ADVANCED: LibraryAdvancedSettings = {
  prefer_local_nfo: true,
  enable_file_filter: true,
  min_file_size: 3,
  metadata_lang: 'zh-CN',
  allow_adult_content: false,
  auto_download_sub: false,
  auto_scrape_metadata: true,
  enable_file_watch: false,
}

interface CreateLibraryModalProps {
  open: boolean
  onClose: () => void
  onCreate: (data: CreateLibraryRequest) => Promise<void>
}

// ===== 可复用的 Toggle 开关组件 =====
function ToggleSwitch({
  checked,
  onChange,
  disabled,
}: {
  checked: boolean
  onChange: (val: boolean) => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full transition-colors duration-300 focus:outline-none disabled:opacity-40 disabled:cursor-not-allowed"
      style={{
        background: checked
          ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))'
          : 'var(--border-default)',
        boxShadow: checked ? 'var(--neon-glow-shadow-md)' : 'none',
      }}
    >
      <span
        className="pointer-events-none inline-block h-5 w-5 rounded-full shadow-lg transition-transform duration-300"
        style={{
          transform: checked ? 'translateX(20px) translateY(2px)' : 'translateX(2px) translateY(2px)',
          background: checked ? '#fff' : 'var(--text-muted)',
        }}
      />
    </button>
  )
}

export default function CreateLibraryModal({ open, onClose, onCreate }: CreateLibraryModalProps) {
  const [selectedType, setSelectedType] = useState<CreateLibraryRequest['type']>('movie')
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [advanced, setAdvanced] = useState<LibraryAdvancedSettings>({ ...DEFAULT_ADVANCED })
  const [showLangDropdown, setShowLangDropdown] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [showFileBrowser, setShowFileBrowser] = useState(false)
  const nameInputRef = useRef<HTMLInputElement>(null)
  const overlayRef = useRef<HTMLDivElement>(null)

  // 打开时重置状态
  useEffect(() => {
    if (open) {
      setSelectedType('movie')
      setName('')
      setPath('')
      setShowAdvanced(false)
      setAdvanced({ ...DEFAULT_ADVANCED })
      setShowLangDropdown(false)
      setError('')
      setTimeout(() => nameInputRef.current?.focus(), 100)
    }
  }, [open])

  // 点击遮罩关闭
  const handleOverlayClick = (e: React.MouseEvent) => {
    if (e.target === overlayRef.current) {
      onClose()
    }
  }

  // ESC关闭
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && open) onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, onClose])

  // 更新高级设置的辅助函数
  const updateAdvanced = <K extends keyof LibraryAdvancedSettings>(
    key: K,
    value: LibraryAdvancedSettings[K]
  ) => {
    setAdvanced((prev) => ({ ...prev, [key]: value }))
  }

  const handleSubmit = async () => {
    if (!name.trim()) {
      setError('请输入媒体库名称')
      nameInputRef.current?.focus()
      return
    }
    if (!path.trim()) {
      setError('请输入媒体文件夹路径')
      return
    }
    setError('')
    setSubmitting(true)
    try {
      await onCreate({
        name: name.trim(),
        path: path.trim(),
        type: selectedType,
        // 高级设置
        ...advanced,
      })
      onClose()
    } catch {
      setError('创建媒体库失败，请检查路径是否正确')
    } finally {
      setSubmitting(false)
    }
  }

  if (!open) return null

  const selectedLangLabel =
    METADATA_LANG_OPTIONS.find((l) => l.value === advanced.metadata_lang)?.label || advanced.metadata_lang

  return (
    <div
      ref={overlayRef}
      className="modal-overlay flex items-center justify-center animate-fade-in"
      onClick={handleOverlayClick}
    >
      <div
        className="relative w-full max-w-xl mx-4 rounded-2xl overflow-hidden animate-slide-up"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-strong)',
          boxShadow: 'var(--shadow-elevated), var(--modal-panel-glow)',
          backdropFilter: 'blur(30px)',
          maxHeight: '90vh',
        }}
      >
        {/* 顶部霓虹光条 */}
        <div
          className="absolute top-0 left-0 right-0 h-[2px] z-10"
          style={{
            background: 'linear-gradient(90deg, transparent, var(--neon-blue), var(--neon-purple), transparent)',
          }}
        />

        {/* 可滚动内容容器 */}
        <div className="overflow-y-auto" style={{ maxHeight: '90vh' }}>
          {/* 头部 */}
          <div className="flex items-center justify-between px-6 pt-6 pb-4 sticky top-0 z-10" style={{ background: 'var(--bg-elevated)' }}>
            <h2
              className="font-display text-lg font-bold tracking-wide"
              style={{ color: 'var(--text-primary)' }}
            >
              创建媒体库
            </h2>
            <button
              onClick={onClose}
              className="rounded-lg p-1.5 transition-all hover:bg-[var(--nav-hover-bg)]"
              style={{ color: 'var(--text-tertiary)' }}
            >
              <X size={20} />
            </button>
          </div>

          {/* 内容区域 */}
          <div className="px-6 pb-6 space-y-5">
            {/* ===== 内容类型选择 ===== */}
            <div>
              <label
                className="mb-3 block text-sm font-semibold"
                style={{ color: 'var(--text-secondary)' }}
              >
                内容类型
              </label>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                {LIBRARY_TYPES.map((type) => {
                  const Icon = type.icon
                  const isSelected = selectedType === type.value
                  return (
                    <button
                      key={type.value}
                      onClick={() => setSelectedType(type.value)}
                      className="group relative flex flex-col items-center gap-2 rounded-xl p-3.5 transition-all duration-300"
                      style={{
                        background: isSelected ? type.bg : 'transparent',
                        border: `1.5px solid ${isSelected ? type.border : 'var(--border-default)'}`,
                        boxShadow: isSelected ? `0 0 20px ${type.bg}` : 'none',
                      }}
                    >
                      {isSelected && (
                        <div
                          className="absolute -top-px left-1/4 right-1/4 h-[2px] rounded-b"
                          style={{ background: type.color, boxShadow: `0 0 8px ${type.color}` }}
                        />
                      )}
                      <div
                        className="flex h-10 w-10 items-center justify-center rounded-lg transition-all duration-300"
                        style={{
                          background: isSelected ? `${type.bg}` : 'var(--nav-hover-bg)',
                          color: isSelected ? type.color : 'var(--text-tertiary)',
                        }}
                      >
                        <Icon size={22} />
                      </div>
                      <div className="text-center">
                        <p
                          className="text-sm font-semibold transition-colors"
                          style={{ color: isSelected ? type.color : 'var(--text-primary)' }}
                        >
                          {type.label}
                        </p>
                        <p
                          className="mt-0.5 text-[11px] leading-tight"
                          style={{ color: 'var(--text-tertiary)' }}
                        >
                          {type.desc}
                        </p>
                      </div>
                    </button>
                  )
                })}
              </div>
            </div>

            {/* ===== 媒体库名称 ===== */}
            <div>
              <label
                className="mb-2 block text-sm font-semibold"
                style={{ color: 'var(--text-secondary)' }}
              >
                媒体库名称
              </label>
              <div className="relative">
                <input
                  ref={nameInputRef}
                  type="text"
                  value={name}
                  onChange={(e) => {
                    if (e.target.value.length <= 32) {
                      setName(e.target.value)
                      setError('')
                    }
                  }}
                  className="input pr-16"
                  placeholder="请输入媒体库名称"
                  onKeyDown={(e) => e.key === 'Enter' && handleSubmit()}
                />
                <span
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-xs tabular-nums"
                  style={{ color: 'var(--text-muted)' }}
                >
                  {name.length} / 32
                </span>
              </div>
            </div>

            {/* ===== 媒体文件夹 ===== */}
            <div>
              <div className="mb-2 flex items-center justify-between">
                <label
                  className="text-sm font-semibold"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  媒体文件夹
                </label>
              </div>
              <p
                className="mb-2.5 text-xs leading-relaxed"
                style={{ color: 'var(--text-tertiary)' }}
              >
                请输入媒体文件所在的文件夹路径，如 <code className="rounded px-1 py-0.5 text-neon" style={{ background: 'var(--nav-hover-bg)' }}>/media/movies</code>
              </p>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setShowFileBrowser(true)}
                  className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl transition-colors cursor-pointer hover:bg-[var(--nav-hover-bg)]"
                  style={{
                    border: '1.5px dashed var(--border-hover)',
                    color: 'var(--text-tertiary)',
                  }}
                  title="浏览服务器目录"
                >
                  <FolderPlus size={18} />
                </button>
                <input
                  type="text"
                  value={path}
                  onChange={(e) => {
                    setPath(e.target.value)
                    setError('')
                  }}
                  className="input flex-1"
                  placeholder="如: /media/movies 或 D:\Videos"
                  onKeyDown={(e) => e.key === 'Enter' && handleSubmit()}
                />
              </div>
            </div>

            {/* ===== 高级设置（可展开） — 飞牛影视风格 ===== */}
            <div>
              <button
                onClick={() => setShowAdvanced(!showAdvanced)}
                className="flex items-center gap-1.5 text-sm font-semibold transition-colors"
                style={{ color: 'var(--text-secondary)' }}
              >
                {showAdvanced ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                高级设置
              </button>

              {showAdvanced && (
                <div className="mt-4 space-y-6 animate-slide-up">

                  {/* ———— 1. 优先读取本地 NFO 和图片 ———— */}
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1">
                      <h4
                        className="text-sm font-semibold"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        优先读取本地 NFO 和图片
                      </h4>
                      <p
                        className="mt-1 text-xs leading-relaxed"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        优先读取本地 NFO 文件中的信息和本地图片，仅从互联网上获取缺失的信息。
                      </p>
                    </div>
                    <ToggleSwitch
                      checked={advanced.prefer_local_nfo}
                      onChange={(v) => updateAdvanced('prefer_local_nfo', v)}
                    />
                  </div>

                  {/* 分割线 */}
                  <div style={{ borderTop: '1px solid var(--border-default)' }} />

                  {/* ———— 2. 文件过滤 ———— */}
                  <div>
                    <h4
                      className="text-sm font-semibold mb-2.5"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      文件过滤
                    </h4>
                    <div className="flex items-center gap-3">
                      {/* Checkbox */}
                      <button
                        type="button"
                        onClick={() => updateAdvanced('enable_file_filter', !advanced.enable_file_filter)}
                        className="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded transition-all"
                        style={{
                          background: advanced.enable_file_filter
                            ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))'
                            : 'transparent',
                          border: advanced.enable_file_filter
                            ? 'none'
                            : '2px solid var(--border-hover)',
                          boxShadow: advanced.enable_file_filter
                            ? '0 0 8px var(--neon-blue-25)'
                            : 'none',
                        }}
                      >
                        {advanced.enable_file_filter && (
                          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                            <path d="M2 6L5 9L10 3" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                          </svg>
                        )}
                      </button>
                      <span
                        className="text-sm"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        排除小于
                      </span>
                      <input
                        type="number"
                        value={advanced.min_file_size}
                        onChange={(e) => {
                          const v = Math.max(0, Math.min(999, parseInt(e.target.value) || 0))
                          updateAdvanced('min_file_size', v)
                        }}
                        disabled={!advanced.enable_file_filter}
                        className="input w-20 text-center tabular-nums disabled:opacity-40"
                        min={0}
                        max={999}
                      />
                      <span
                        className="text-sm"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        MB 的视频文件
                      </span>
                    </div>
                  </div>

                  {/* 分割线 */}
                  <div style={{ borderTop: '1px solid var(--border-default)' }} />

                  {/* ———— 3. 媒体元数据下载语言 ———— */}
                  <div>
                    <h4
                      className="text-sm font-semibold"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      媒体元数据下载语言
                    </h4>
                    <p
                      className="mt-1 mb-3 text-xs leading-relaxed"
                      style={{ color: 'var(--text-tertiary)' }}
                    >
                      优先使用首选语言下载元数据，如影片信息、演员信息、海报等。
                    </p>
                    {/* 下拉选择框 */}
                    <div className="relative">
                      <button
                        type="button"
                        onClick={() => setShowLangDropdown(!showLangDropdown)}
                        className="input flex w-full items-center justify-between gap-2 text-left"
                      >
                        <div className="flex items-center gap-2">
                          <Globe size={16} style={{ color: 'var(--text-muted)' }} />
                          <span style={{ color: 'var(--text-primary)' }}>{selectedLangLabel}</span>
                        </div>
                        <ChevronDown
                          size={16}
                          style={{ color: 'var(--text-muted)' }}
                          className={`transition-transform ${showLangDropdown ? 'rotate-180' : ''}`}
                        />
                      </button>
                      {showLangDropdown && (
                        <>
                          <div className="fixed inset-0 z-30" onClick={() => setShowLangDropdown(false)} />
                          <div
                            className="absolute left-0 right-0 top-full z-40 mt-1 overflow-hidden rounded-xl py-1 animate-slide-up"
                            style={{
                              background: 'var(--bg-elevated)',
                              border: '1px solid var(--border-strong)',
                              boxShadow: 'var(--shadow-elevated)',
                            }}
                          >
                            {METADATA_LANG_OPTIONS.map((lang) => (
                              <button
                                key={lang.value}
                                onClick={() => {
                                  updateAdvanced('metadata_lang', lang.value)
                                  setShowLangDropdown(false)
                                }}
                                className="w-full px-4 py-2.5 text-left text-sm transition-colors hover:bg-[var(--nav-hover-bg)]"
                                style={{
                                  color: advanced.metadata_lang === lang.value
                                    ? 'var(--neon-blue)'
                                    : 'var(--text-secondary)',
                                  background: advanced.metadata_lang === lang.value
                                    ? 'var(--nav-active-bg)'
                                    : undefined,
                                }}
                              >
                                {lang.label}
                              </button>
                            ))}
                          </div>
                        </>
                      )}
                    </div>
                  </div>

                  {/* 分割线 */}
                  <div style={{ borderTop: '1px solid var(--border-default)' }} />

                  {/* ———— 4. 媒体元数据允许成人内容 ———— */}
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1">
                      <h4
                        className="text-sm font-semibold"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        媒体元数据允许成人内容
                      </h4>
                      <p
                        className="mt-1 text-xs leading-relaxed"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        从第三方公开数据库搜索、下载元数据时，允许成人内容。成人内容判断依据为 TMDB 的成人标签。
                      </p>
                    </div>
                    <ToggleSwitch
                      checked={advanced.allow_adult_content}
                      onChange={(v) => updateAdvanced('allow_adult_content', v)}
                    />
                  </div>

                  {/* 分割线 */}
                  <div style={{ borderTop: '1px solid var(--border-default)' }} />

                  {/* ———— 5. 自动下载字幕 ———— */}
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1">
                      <h4
                        className="text-sm font-semibold"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        自动下载字幕
                      </h4>
                      <p
                        className="mt-1 text-xs leading-relaxed"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        对未内嵌字幕的媒体文件，自动从互联网上下载字幕。
                      </p>
                    </div>
                    <ToggleSwitch
                      checked={advanced.auto_download_sub}
                      onChange={(v) => updateAdvanced('auto_download_sub', v)}
                    />
                  </div>

                  {/* 分割线 */}
                  <div style={{ borderTop: '1px solid var(--border-default)' }} />

                  {/* ———— 6. 扫描后自动刮削元数据 ———— */}
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Search size={16} style={{ color: '#06B6D4' }} />
                        <h4
                          className="text-sm font-semibold"
                          style={{ color: 'var(--text-primary)' }}
                        >
                          扫描后自动刮削元数据
                        </h4>
                      </div>
                      <p
                        className="mt-1 text-xs leading-relaxed"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        扫描媒体库后自动从 TMDb、豆瓣等数据源识别并下载影片信息（海报、简介、评分等）。关闭后需在媒体详情页手动触发刮削。
                      </p>
                    </div>
                    <ToggleSwitch
                      checked={advanced.auto_scrape_metadata}
                      onChange={(v) => updateAdvanced('auto_scrape_metadata', v)}
                    />
                  </div>

                  {/* 分割线 */}
                  <div style={{ borderTop: '1px solid var(--border-default)' }} />

                  {/* ———— 7. 实时文件监控 ———— */}
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Eye size={16} style={{ color: 'var(--neon-purple)' }} />
                        <h4
                          className="text-sm font-semibold"
                          style={{ color: 'var(--text-primary)' }}
                        >
                          实时文件监控
                        </h4>
                      </div>
                      <p
                        className="mt-1 text-xs leading-relaxed"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        实时监控媒体文件夹的变化，自动检测并同步新增、修改或删除的媒体文件，无需手动扫描。
                      </p>
                    </div>
                    <ToggleSwitch
                      checked={advanced.enable_file_watch}
                      onChange={(v) => updateAdvanced('enable_file_watch', v)}
                    />
                  </div>
                </div>
              )}
            </div>

            {/* 错误提示 */}
            {error && (
              <div
                className="flex items-center gap-2 rounded-xl px-4 py-3 text-sm"
                style={{
                  background: 'rgba(239, 68, 68, 0.08)',
                  border: '1px solid rgba(239, 68, 68, 0.15)',
                  color: '#EF4444',
                }}
              >
                <AlertCircle size={16} />
                {error}
              </div>
            )}
          </div>

          {/* 底部按钮 */}
          <div
            className="flex items-center justify-end gap-3 px-6 py-4 sticky bottom-0"
            style={{
              borderTop: '1px solid var(--border-default)',
              background: 'var(--bg-elevated)',
            }}
          >
            <button
              onClick={onClose}
              className="rounded-xl px-6 py-2.5 text-sm font-medium transition-all"
              style={{
                color: 'var(--text-secondary)',
                border: '1px solid var(--border-default)',
                background: 'transparent',
              }}
            >
              取消
            </button>
            <button
              onClick={handleSubmit}
              disabled={submitting}
              className="btn-primary gap-2 px-6 py-2.5 text-sm"
            >
              {submitting ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  创建中...
                </>
              ) : (
                '确认创建'
              )}
            </button>
          </div>
        </div>
      </div>

      {/* 服务端文件浏览器 */}
      <FileBrowser
        open={showFileBrowser}
        onClose={() => setShowFileBrowser(false)}
        onSelect={(selectedPath) => {
          setPath(selectedPath)
          setError('')
        }}
        initialPath={path || '/'}
      />
    </div>
  )
}
