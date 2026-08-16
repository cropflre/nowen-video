import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { CreateLibraryRequest, LibraryAdvancedSettings } from '@/types'
import { adminApi, adultScraperApi, aiApi } from '@/api'
import {
  AlertCircle,
  Eye,
  Film,
  FolderPlus,
  Info,
  Layers,
  Plus,
  Search,
  Sparkles,
  Trash2,
  Tv,
  Video,
} from 'lucide-react'
import FileBrowser from './FileBrowser'
import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Select,
  Surface,
  Tag,
} from './design-system'

const LIBRARY_TYPES = [
  { value: 'movie' as const, label: '电影', desc: '各种类型电影', icon: Film },
  { value: 'tvshow' as const, label: '电视节目', desc: '电视剧、综艺等', icon: Tv },
  { value: 'mixed' as const, label: '混合影片', desc: '电影和电视节目', icon: Layers },
  { value: 'other' as const, label: '其他视频', desc: '个人视频、课程等', icon: Video },
]

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

const DEFAULT_ADVANCED: LibraryAdvancedSettings = {
  prefer_local_nfo: true,
  enable_file_filter: true,
  min_file_size: 3,
  metadata_lang: 'zh-CN',
  allow_adult_content: false,
  auto_download_sub: false,
  auto_scrape_metadata: true,
  auto_organize_mode: 'rule_only',
  organize_output_dir: '',
  enable_file_watch: false,
}

const DEFAULT_METADATA_SOURCE_TOOLTIP = [
  '当前元数据来源链接：',
  '• TMDb API：https://api.themoviedb.org',
  '• TMDb 图片：https://image.tmdb.org',
  '• 成人番号数据源：以「系统设置 → 番号刮削配置」中启用的链接为准',
  '说明：此开关只决定搜索/下载元数据时是否允许成人结果。',
].join('\n')

function buildMetadataSourceTooltip(
  tmdbApiURL: string,
  tmdbImageURL: string,
  adultSourceLines: string[],
  adultEnabled: boolean,
) {
  const lines = [
    '当前元数据来源链接：',
    `• TMDb API：${tmdbApiURL}`,
    `• TMDb 图片：${tmdbImageURL}`,
  ]
  if (adultEnabled && adultSourceLines.length > 0) {
    lines.push('• 成人番号数据源：')
    adultSourceLines.forEach((line) => lines.push(`  - ${line}`))
  } else {
    lines.push('• 成人番号数据源：未启用或未配置')
  }
  lines.push('说明：此开关只决定搜索/下载元数据时是否允许成人结果。')
  return lines.join('\n')
}

interface CreateLibraryModalProps {
  open: boolean
  onClose: () => void
  onCreate: (data: CreateLibraryRequest) => Promise<void>
}

function ToggleSwitch({
  checked,
  onChange,
  disabled,
}: {
  checked: boolean
  onChange: (value: boolean) => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className="relative inline-flex h-6 w-11 shrink-0 rounded-full border transition-colors duration-200 focus:outline-none focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50"
      style={{
        background: checked ? 'var(--nv-action-primary)' : 'var(--nv-bg-control)',
        borderColor: checked ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
      }}
    >
      <span
        className="absolute top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform duration-200"
        style={{ transform: checked ? 'translateX(20px)' : 'translateX(2px)' }}
      />
    </button>
  )
}

function FieldBlock({ label, description, children }: { label: string; description?: ReactNode; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <div>
        <div className="text-sm font-medium text-[var(--nv-text-primary)]">{label}</div>
        {description && (
          <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">{description}</div>
        )}
      </div>
      {children}
    </div>
  )
}

function SettingRow({
  title,
  description,
  icon,
  checked,
  onChange,
}: {
  title: string
  description: ReactNode
  icon?: ReactNode
  checked: boolean
  onChange: (value: boolean) => void
}) {
  return (
    <div className="flex items-start justify-between gap-4 py-1">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">
          {icon && <span className="text-[var(--nv-action-primary)]">{icon}</span>}
          {title}
        </div>
        <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">{description}</div>
      </div>
      <ToggleSwitch checked={checked} onChange={onChange} />
    </div>
  )
}

export default function CreateLibraryModal({ open, onClose, onCreate }: CreateLibraryModalProps) {
  const [selectedType, setSelectedType] = useState<CreateLibraryRequest['type']>('movie')
  const [name, setName] = useState('')
  const [paths, setPaths] = useState<string[]>([''])
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [advanced, setAdvanced] = useState<LibraryAdvancedSettings>({ ...DEFAULT_ADVANCED })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [browsingIndex, setBrowsingIndex] = useState<number | null>(null)
  const [metadataSourceTooltip, setMetadataSourceTooltip] = useState(DEFAULT_METADATA_SOURCE_TOOLTIP)
  const [aiConfigured, setAIConfigured] = useState(false)
  const [aiStatusLoading, setAIStatusLoading] = useState(false)
  const [aiStatusUnavailable, setAIStatusUnavailable] = useState(false)
  const nameInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    setSelectedType('movie')
    setName('')
    setPaths([''])
    setShowAdvanced(false)
    setAdvanced({ ...DEFAULT_ADVANCED })
    setError('')
    setAIConfigured(false)
    setAIStatusUnavailable(false)
    setTimeout(() => nameInputRef.current?.focus(), 100)
  }, [open])

  useEffect(() => {
    if (!open) return
    let cancelled = false

    setAIStatusLoading(true)
    setAIStatusUnavailable(false)
    aiApi.getStatus({ allowCachedOnError: false })
      .then((response) => {
        if (cancelled) return
        const status = response.data.data
        const configured = Boolean(
          status?.enabled
          && status?.api_configured
          && status?.provider
          && status?.api_base
          && status?.model,
        )
        setAIConfigured(configured)
        setAIStatusUnavailable(false)
        if (!configured) {
          setAdvanced((current) => (
            current.auto_organize_mode === 'ai_assisted'
              ? { ...current, auto_organize_mode: 'rule_only' }
              : current
          ))
        }
      })
      .catch(() => {
        if (cancelled) return
        setAIConfigured(false)
        setAIStatusUnavailable(true)
        setAdvanced((current) => (
          current.auto_organize_mode === 'ai_assisted'
            ? { ...current, auto_organize_mode: 'rule_only' }
            : current
        ))
      })
      .finally(() => {
        if (!cancelled) setAIStatusLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    let cancelled = false

    const loadMetadataSourceTooltip = async () => {
      const [tmdbResult, adultResult] = await Promise.allSettled([
        adminApi.getTMDbConfig(),
        adultScraperApi.getConfig(),
      ])
      if (cancelled) return

      const tmdb = tmdbResult.status === 'fulfilled' ? tmdbResult.value.data.data : null
      const adult = adultResult.status === 'fulfilled' ? adultResult.value.data.data : null
      const adultSourceLines = adult?.enabled
        ? [...(adult.sources || [])]
            .filter((source) => source.enabled && source.url)
            .sort((a, b) => a.priority - b.priority)
            .map((source) => `${source.name || source.id}：${source.url}`)
        : []

      setMetadataSourceTooltip(
        buildMetadataSourceTooltip(
          tmdb?.api_proxy || 'https://api.themoviedb.org',
          tmdb?.image_proxy || 'https://image.tmdb.org',
          adultSourceLines,
          Boolean(adult?.enabled),
        ),
      )
    }

    loadMetadataSourceTooltip().catch(() => {
      if (!cancelled) setMetadataSourceTooltip(DEFAULT_METADATA_SOURCE_TOOLTIP)
    })
    return () => {
      cancelled = true
    }
  }, [open])

  const updateAdvanced = <K extends keyof LibraryAdvancedSettings>(
    key: K,
    value: LibraryAdvancedSettings[K],
  ) => {
    setAdvanced((prev) => ({ ...prev, [key]: value }))
  }

  const handleSubmit = async () => {
    if (!name.trim()) {
      setError('请输入媒体库名称')
      nameInputRef.current?.focus()
      return
    }

    const cleanedPaths = paths.map((path) => path.trim()).filter(Boolean)
    if (cleanedPaths.length === 0) {
      setError('请至少添加一个媒体文件夹路径')
      return
    }

    const dedupedPaths = Array.from(new Set(cleanedPaths))
    if (dedupedPaths.length !== cleanedPaths.length) {
      setError('存在重复的路径，请删除重复项')
      return
    }

    if (advanced.auto_organize_mode === 'ai_assisted' && !aiConfigured) {
      setError('AI 尚未配置或启用，请先配置 AI，或改用「仅规则」整理模式')
      return
    }

    setError('')
    setSubmitting(true)
    try {
      await onCreate({
        name: name.trim(),
        paths: dedupedPaths,
        type: selectedType,
        ...advanced,
      })
      onClose()
    } catch (err: any) {
      setError(err?.response?.data?.error || '创建媒体库失败，请检查路径是否正确')
    } finally {
      setSubmitting(false)
    }
  }

  const organizeOptions = [
    {
      value: 'ai_assisted' as const,
      title: 'AI 辅助',
      tag: '推荐',
      description: '规则识别 + 低置信度时调用 AI 兜底',
      requiresAI: true,
    },
    {
      value: 'rule_only' as const,
      title: '仅规则',
      tag: '半自动',
      description: '只做规则识别与命名建议，不调用 AI',
      requiresAI: false,
    },
    {
      value: 'off' as const,
      title: '关闭',
      tag: '手动',
      description: '扫描后不做任何自动整理',
      requiresAI: false,
    },
  ]

  return (
    <>
      <Modal open={open} onClose={onClose} size="lg" ariaLabel="创建媒体库">
        <ModalHeader
          title="创建媒体库"
          description="选择内容类型、添加一个或多个媒体目录，并按需配置扫描与整理策略。"
          icon={<FolderPlus size={18} />}
          onClose={onClose}
        />

        <ModalBody className="space-y-6">
          <FieldBlock label="内容类型" description="类型只影响媒体识别策略，选中状态统一使用主品牌色。">
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              {LIBRARY_TYPES.map((type) => {
                const Icon = type.icon
                const selected = selectedType === type.value
                return (
                  <button
                    key={type.value}
                    type="button"
                    onClick={() => setSelectedType(type.value)}
                    className="rounded-[var(--nv-radius-card)] border p-3 text-left transition-[background-color,border-color,transform] duration-200 hover:-translate-y-0.5"
                    style={{
                      background: selected ? 'var(--nv-bg-active)' : 'var(--nv-bg-surface-soft)',
                      borderColor: selected ? 'var(--nv-action-primary)' : 'var(--nv-border-subtle)',
                    }}
                  >
                    <div
                      className="mb-3 flex h-9 w-9 items-center justify-center rounded-[var(--nv-radius-control)]"
                      style={{
                        background: selected ? 'var(--nv-ambient-cyan)' : 'var(--nv-bg-control)',
                        color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-tertiary)',
                      }}
                    >
                      <Icon size={18} />
                    </div>
                    <div className="text-sm font-semibold text-[var(--nv-text-primary)]">{type.label}</div>
                    <div className="mt-1 text-[11px] leading-4 text-[var(--nv-text-tertiary)]">{type.desc}</div>
                  </button>
                )
              })}
            </div>
          </FieldBlock>

          <FieldBlock label="媒体库名称">
            <div className="relative">
              <Input
                ref={nameInputRef}
                value={name}
                invalid={Boolean(error && !name.trim())}
                onChange={(event) => {
                  if (event.target.value.length <= 32) {
                    setName(event.target.value)
                    setError('')
                  }
                }}
                placeholder="请输入媒体库名称"
                className="pr-16"
                onKeyDown={(event) => event.key === 'Enter' && handleSubmit()}
              />
              <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs tabular-nums text-[var(--nv-text-tertiary)]">
                {name.length} / 32
              </span>
            </div>
          </FieldBlock>

          <FieldBlock
            label="媒体文件夹"
            description={
              <>
                支持多个目录，第一个路径作为主路径。例如{' '}
                <code className="rounded bg-[var(--nv-bg-control)] px-1.5 py-0.5 font-mono text-[11px] text-[var(--nv-action-primary)]">
                  /media/movies
                </code>
                。
              </>
            }
          >
            <div className="space-y-2">
              {paths.map((path, index) => (
                <div key={index} className="flex items-center gap-2">
                  <Button
                    type="button"
                    variant="ghost"
                    iconOnly
                    onClick={() => setBrowsingIndex(index)}
                    title="浏览服务器目录"
                  >
                    <FolderPlus size={17} />
                  </Button>
                  <Input
                    value={path}
                    invalid={Boolean(error && paths.every((item) => !item.trim()))}
                    onChange={(event) => {
                      const value = event.target.value
                      setPaths((prev) => prev.map((item, itemIndex) => (itemIndex === index ? value : item)))
                      setError('')
                    }}
                    className="flex-1"
                    placeholder={index === 0 ? '主路径，如 /media/movies 或 D:\\Videos' : '额外路径'}
                    onKeyDown={(event) => event.key === 'Enter' && handleSubmit()}
                  />
                  {paths.length > 1 && (
                    <Button
                      type="button"
                      variant="ghost"
                      iconOnly
                      onClick={() => {
                        setPaths((prev) => prev.filter((_, itemIndex) => itemIndex !== index))
                        setError('')
                      }}
                      title="移除该路径"
                      className="text-[var(--nv-status-danger)]"
                    >
                      <Trash2 size={16} />
                    </Button>
                  )}
                </div>
              ))}
            </div>
            <Button type="button" variant="ghost" size="sm" onClick={() => setPaths((prev) => [...prev, ''])}>
              <Plus size={14} />
              添加文件夹
            </Button>
          </FieldBlock>

          <Surface className="overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]">
            <button
              type="button"
              onClick={() => setShowAdvanced((value) => !value)}
              className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left"
            >
              <div>
                <div className="text-sm font-semibold text-[var(--nv-text-primary)]">高级设置</div>
                <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">扫描、元数据、自动整理与文件监控策略</div>
              </div>
              <Tag tone={showAdvanced ? 'brand' : 'neutral'}>{showAdvanced ? '已展开' : '展开'}</Tag>
            </button>

            {showAdvanced && (
              <div className="space-y-5 border-t border-[var(--nv-border-subtle)] px-4 py-4">
                <SettingRow
                  title="优先读取本地 NFO 和图片"
                  description="优先读取本地 NFO 与图片，仅从互联网补充缺失信息。"
                  checked={advanced.prefer_local_nfo}
                  onChange={(value) => updateAdvanced('prefer_local_nfo', value)}
                />

                <div className="border-t border-[var(--nv-border-subtle)] pt-5">
                  <FieldBlock label="文件过滤" description="扫描时排除过小的视频文件。">
                    <div className="flex flex-wrap items-center gap-3">
                      <ToggleSwitch
                        checked={advanced.enable_file_filter}
                        onChange={(value) => updateAdvanced('enable_file_filter', value)}
                      />
                      <span className="text-sm text-[var(--nv-text-secondary)]">排除小于</span>
                      <Input
                        type="number"
                        min={0}
                        max={999}
                        value={advanced.min_file_size}
                        disabled={!advanced.enable_file_filter}
                        onChange={(event) => {
                          const value = Math.max(0, Math.min(999, Number.parseInt(event.target.value) || 0))
                          updateAdvanced('min_file_size', value)
                        }}
                        className="w-24 text-center tabular-nums"
                      />
                      <span className="text-sm text-[var(--nv-text-secondary)]">MB 的视频文件</span>
                    </div>
                  </FieldBlock>
                </div>

                <div className="border-t border-[var(--nv-border-subtle)] pt-5">
                  <FieldBlock label="媒体元数据下载语言" description="优先使用所选语言下载影片、演员与海报信息。">
                    <Select
                      value={advanced.metadata_lang}
                      onChange={(event) => updateAdvanced('metadata_lang', event.target.value)}
                      className="w-full"
                    >
                      {METADATA_LANG_OPTIONS.map((option) => (
                        <option key={option.value} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </Select>
                  </FieldBlock>
                </div>

                <div className="border-t border-[var(--nv-border-subtle)] pt-5">
                  <SettingRow
                    title="媒体元数据允许成人内容"
                    description={
                      <span className="inline-flex items-start gap-1.5">
                        <span>允许第三方公开数据库返回成人内容。</span>
                        <button
                          type="button"
                          title={metadataSourceTooltip}
                          aria-label="查看元数据来源链接"
                          className="mt-0.5 inline-flex text-[var(--nv-text-tertiary)] hover:text-[var(--nv-text-primary)]"
                        >
                          <Info size={13} />
                        </button>
                      </span>
                    }
                    checked={advanced.allow_adult_content}
                    onChange={(value) => updateAdvanced('allow_adult_content', value)}
                  />
                </div>

                <div className="border-t border-[var(--nv-border-subtle)] pt-5">
                  <SettingRow
                    title="自动下载字幕"
                    description="对未内嵌字幕的媒体文件，自动从互联网下载字幕。"
                    checked={advanced.auto_download_sub}
                    onChange={(value) => updateAdvanced('auto_download_sub', value)}
                  />
                </div>

                <div className="border-t border-[var(--nv-border-subtle)] pt-5">
                  <SettingRow
                    title="扫描后自动刮削元数据"
                    description="扫描后自动从 TMDb、豆瓣等数据源识别并下载海报、简介和评分。"
                    icon={<Search size={15} />}
                    checked={advanced.auto_scrape_metadata}
                    onChange={(value) => updateAdvanced('auto_scrape_metadata', value)}
                  />
                </div>

                <div className="space-y-3 border-t border-[var(--nv-border-subtle)] pt-5">
                  <div className="flex items-start gap-2">
                    <Sparkles size={16} className="mt-0.5 text-[var(--nv-action-primary)]" />
                    <div>
                      <div className="text-sm font-medium text-[var(--nv-text-primary)]">AI 自动整理</div>
                      <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                        扫描入库后执行智能识别、虚拟归类和命名建议。所有结果默认仅写入数据库，不修改源文件。
                      </div>
                    </div>
                  </div>

                  <div className="grid gap-2 sm:grid-cols-3">
                    {organizeOptions.map((option) => {
                      const active = advanced.auto_organize_mode === option.value
                      const disabled = option.requiresAI && (!aiConfigured || aiStatusLoading)
                      const aiTag = option.requiresAI && !aiConfigured
                        ? (aiStatusLoading ? '检测中' : '未配置')
                        : option.tag
                      return (
                        <button
                          key={option.value}
                          type="button"
                          disabled={disabled}
                          aria-disabled={disabled}
                          title={disabled ? (aiStatusLoading ? '正在检测 AI 配置状态' : '请先在管理中心配置并启用 AI') : undefined}
                          onClick={() => {
                            if (!disabled) updateAdvanced('auto_organize_mode', option.value)
                          }}
                          className="rounded-[var(--nv-radius-control)] border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-45"
                          style={{
                            borderColor: active ? 'var(--nv-action-primary)' : 'var(--nv-border-subtle)',
                            background: active ? 'var(--nv-bg-active)' : 'var(--nv-bg-control)',
                          }}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-sm font-semibold text-[var(--nv-text-primary)]">{option.title}</span>
                            <Tag tone={active ? 'brand' : 'neutral'}>{aiTag}</Tag>
                          </div>
                          <div className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{option.description}</div>
                        </button>
                      )
                    })}
                  </div>

                  {!aiStatusLoading && !aiConfigured && (
                    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-3 py-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                      {aiStatusUnavailable
                        ? '无法确认 AI 配置状态，已安全禁用「AI 辅助」。请检查服务状态后重试。'
                        : 'AI 尚未配置或启用，「AI 辅助」暂不可用。请先在管理中心 → AI 设置中完成配置。'}
                    </div>
                  )}

                  {advanced.auto_organize_mode === 'off' && (
                    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-3 py-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                      关闭后扫描和重建索引都不会触发 AI 整理。后续可在媒体库编辑中重新开启。
                    </div>
                  )}
                  {advanced.auto_organize_mode === 'rule_only' && (
                    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-3 py-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                      仅使用规则解析文件名与数据库字段，不会产生 AI 调用费用。
                    </div>
                  )}

                  {advanced.auto_organize_mode !== 'off' && (
                    <FieldBlock label="虚拟化目录（硬链接输出）" description="留空则只写数据库；配置后按分类结果创建硬链接目录树，源文件不变。">
                      <Input
                        value={advanced.organize_output_dir}
                        onChange={(event) => updateAdvanced('organize_output_dir', event.target.value)}
                        placeholder="如 /media/organized 或 D:\\Media\\Organized"
                      />
                      {advanced.organize_output_dir && (
                        <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-status-success)] bg-[var(--nv-bg-surface)] px-3 py-2 text-xs text-[var(--nv-status-success)]">
                          已配置输出目录，整理完成后会在此创建硬链接。
                        </div>
                      )}
                    </FieldBlock>
                  )}
                </div>

                <div className="border-t border-[var(--nv-border-subtle)] pt-5">
                  <SettingRow
                    title="实时文件监控"
                    description="实时监控媒体目录变化，自动同步新增、修改或删除的文件。"
                    icon={<Eye size={15} />}
                    checked={advanced.enable_file_watch}
                    onChange={(value) => updateAdvanced('enable_file_watch', value)}
                  />
                </div>
              </div>
            )}
          </Surface>

          {error && (
            <div className="flex items-start gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-status-danger)] bg-[var(--nv-bg-surface-soft)] px-4 py-3 text-sm text-[var(--nv-status-danger)]">
              <AlertCircle size={16} className="mt-0.5 shrink-0" />
              <span>{error}</span>
            </div>
          )}
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            取消
          </Button>
          <Button type="button" variant="primary" onClick={handleSubmit} loading={submitting}>
            {submitting ? '创建中...' : '确认创建'}
          </Button>
        </ModalFooter>
      </Modal>

      <FileBrowser
        open={browsingIndex !== null}
        onClose={() => setBrowsingIndex(null)}
        onSelect={(selectedPath) => {
          if (browsingIndex !== null) {
            setPaths((prev) => prev.map((path, index) => (index === browsingIndex ? selectedPath : path)))
            setError('')
          }
        }}
        initialPath={(browsingIndex !== null ? paths[browsingIndex] : '') || '/'}
      />
    </>
  )
}
