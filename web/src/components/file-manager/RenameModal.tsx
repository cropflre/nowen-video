import { useEffect, useState } from 'react'
import {
  Check,
  ChevronsUpDown,
  Eye,
  Languages,
  Loader2,
  Sparkles,
  Wand2,
} from 'lucide-react'
import type { RenamePreview, RenameTemplate } from '@/types'
import { fileManagerApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Tag,
} from '@/components/design-system'
import { LANGUAGE_OPTIONS } from './constants'

interface RenameModalProps {
  selectedCount: number
  selectedIds: Set<string>
  onClose: () => void
  onSuccess: () => void
}

const AI_APPLY_CONCURRENCY = 5

export default function RenameModal({ selectedCount, selectedIds, onClose, onSuccess }: RenameModalProps) {
  const toast = useToast()
  const [useAIRename, setUseAIRename] = useState(false)
  const [renameTemplate, setRenameTemplate] = useState('{title} ({year}) [{resolution}]')
  const [renamePreviews, setRenamePreviews] = useState<RenamePreview[]>([])
  const [renameTemplates, setRenameTemplates] = useState<RenameTemplate[]>([])
  const [renaming, setRenaming] = useState(false)
  const [targetLang, setTargetLang] = useState(() => localStorage.getItem('rename_target_lang') || '')
  const [previewsExpanded, setPreviewsExpanded] = useState(true)

  useEffect(() => {
    let active = true
    fileManagerApi.getRenameTemplates()
      .then((res) => {
        if (active) setRenameTemplates(res.data.data || [])
      })
      .catch(() => {})
    return () => { active = false }
  }, [])

  const invalidatePreview = () => {
    if (renamePreviews.length > 0) setRenamePreviews([])
  }

  const handleModeChange = (nextUseAI: boolean) => {
    if (nextUseAI === useAIRename) return
    setUseAIRename(nextUseAI)
    invalidatePreview()
  }

  const handleTemplateChange = (template: string) => {
    setRenameTemplate(template)
    invalidatePreview()
  }

  const handleTargetLangChange = (lang: string) => {
    setTargetLang(lang)
    localStorage.setItem('rename_target_lang', lang)
    invalidatePreview()
  }

  const handlePreview = async () => {
    setRenaming(true)
    try {
      const ids = Array.from(selectedIds)
      const res = useAIRename
        ? await fileManagerApi.aiGenerateRenames(ids, targetLang || undefined)
        : await fileManagerApi.previewRename(ids, renameTemplate)
      setRenamePreviews(res.data.data || [])
      setPreviewsExpanded(true)
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '生成预览失败')
    } finally {
      setRenaming(false)
    }
  }

  const executeAIPreview = async () => {
    let renamed = 0
    let failed = 0

    for (let index = 0; index < renamePreviews.length; index += AI_APPLY_CONCURRENCY) {
      const batch = renamePreviews.slice(index, index + AI_APPLY_CONCURRENCY)
      const results = await Promise.allSettled(
        batch.map((preview) => fileManagerApi.updateFile(preview.media_id, { title: preview.new_title })),
      )
      for (const result of results) {
        if (result.status === 'fulfilled') renamed += 1
        else failed += 1
      }
    }

    if (renamed > 0) toast.success(`已应用 ${renamed} 个 AI 重命名结果`)
    if (failed > 0) toast.error(`${failed} 个文件重命名失败，请刷新后重试`)
    return { renamed, failed }
  }

  const handleExecute = async () => {
    if (renamePreviews.length === 0) return

    setRenaming(true)
    try {
      if (useAIRename) {
        // AI preview contains concrete titles. The legacy executeRename endpoint
        // only accepts a template and would regenerate a different result, so
        // apply the exact titles the user just reviewed via the existing update API.
        const result = await executeAIPreview()
        if (result.renamed > 0) {
          onClose()
          onSuccess()
        }
        return
      }

      const res = await fileManagerApi.executeRename(Array.from(selectedIds), renameTemplate)
      const renamed = res.data.renamed || 0
      const errors = res.data.errors || []
      if (renamed > 0) toast.success(`已重命名 ${renamed} 个文件`)
      else if (errors.length === 0) toast.info('当前预览没有需要执行的标题变更')
      if (errors.length > 0) toast.error(`${errors.length} 个文件重命名失败`)
      if (renamed > 0 || errors.length === 0) {
        onClose()
        onSuccess()
      }
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '重命名失败')
    } finally {
      setRenaming(false)
    }
  }

  return (
    <Modal open onClose={onClose} size="lg" ariaLabel="批量重命名">
      <div className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title="批量重命名"
          description={`为已选择的 ${selectedCount} 个文件生成新标题，确认预览后再执行。此操作只更新媒体记录标题，不移动磁盘文件。`}
          icon={<Wand2 size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody className="space-y-5">
          <section>
            <div className="mb-2 text-xs font-medium text-[var(--nv-text-tertiary)]">重命名方式</div>
            <div className="grid border-y border-[var(--nv-border-subtle)] sm:grid-cols-2 sm:divide-x sm:divide-[var(--nv-border-subtle)]" role="group" aria-label="重命名方式">
              <button
                type="button"
                onClick={() => handleModeChange(false)}
                aria-pressed={!useAIRename}
                disabled={renaming}
                className={`px-3 py-3 text-left outline-none transition-colors duration-150 focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50 ${!useAIRename ? 'bg-[var(--nv-fill-active)]' : 'hover:bg-[var(--nv-fill-hover)]'}`}
              >
                <div className="flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">
                  <Wand2 size={15} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                  模板重命名
                </div>
                <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">使用变量模板生成稳定、可预测的媒体标题。</div>
              </button>

              <button
                type="button"
                onClick={() => handleModeChange(true)}
                aria-pressed={useAIRename}
                disabled={renaming}
                className={`border-t border-[var(--nv-border-subtle)] px-3 py-3 text-left outline-none transition-colors duration-150 focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50 sm:border-t-0 ${useAIRename ? 'bg-[var(--nv-fill-active)]' : 'hover:bg-[var(--nv-fill-hover)]'}`}
              >
                <div className="flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">
                  <Sparkles size={15} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                  AI 智能重命名
                </div>
                <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">由 AI 规范化标题，并可按目标语言生成名称。</div>
              </button>
            </div>
          </section>

          {!useAIRename ? (
            <section className="space-y-3">
              <label className="block space-y-1.5">
                <span className="text-xs font-medium text-[var(--nv-text-tertiary)]">命名模板</span>
                <Input
                  type="text"
                  value={renameTemplate}
                  onChange={(event) => handleTemplateChange(event.target.value)}
                  className="font-mono"
                  disabled={renaming}
                  aria-label="命名模板"
                />
              </label>

              {renameTemplates.length > 0 && (
                <div>
                  <div className="mb-2 text-xs font-medium text-[var(--nv-text-tertiary)]">常用模板</div>
                  <div className="flex flex-wrap gap-1.5">
                    {renameTemplates.map((template, index) => {
                      const selected = renameTemplate === template.pattern
                      return (
                        <button
                          key={`${template.pattern}-${index}`}
                          type="button"
                          onClick={() => handleTemplateChange(template.pattern)}
                          disabled={renaming}
                          aria-pressed={selected}
                          title={`示例: ${template.example}`}
                          className={`rounded-[var(--nv-radius-control)] px-2.5 py-1.5 font-mono text-xs outline-none transition-colors duration-150 focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50 ${selected ? 'bg-[var(--nv-fill-active)] text-[var(--nv-text-primary)]' : 'text-[var(--nv-text-tertiary)] hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-secondary)]'}`}
                        >
                          {template.pattern}
                        </button>
                      )
                    })}
                  </div>
                </div>
              )}

              <div className="text-xs leading-5 text-[var(--nv-text-tertiary)]">
                可用变量：{'{title}'}、{'{orig_title}'}、{'{year}'}、{'{resolution}'}、{'{media_type}'}
              </div>
            </section>
          ) : (
            <section className="space-y-3">
              <div className="flex items-center gap-2 text-xs font-medium text-[var(--nv-text-tertiary)]">
                <Languages size={14} aria-hidden="true" />
                目标翻译语言
              </div>
              <div className="flex flex-wrap gap-1.5">
                {LANGUAGE_OPTIONS.map((language) => {
                  const selected = targetLang === language.value
                  return (
                    <button
                      key={language.value}
                      type="button"
                      onClick={() => handleTargetLangChange(language.value)}
                      disabled={renaming}
                      aria-pressed={selected}
                      className={`flex items-center gap-1.5 rounded-[var(--nv-radius-control)] px-2.5 py-1.5 text-xs outline-none transition-colors duration-150 focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50 ${selected ? 'bg-[var(--nv-fill-active)] text-[var(--nv-text-primary)]' : 'text-[var(--nv-text-tertiary)] hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-secondary)]'}`}
                    >
                      <span aria-hidden="true">{language.flag}</span>
                      <span>{language.label}</span>
                    </button>
                  )
                })}
              </div>
              {targetLang && (
                <div className="flex items-center gap-1.5 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  <Sparkles size={12} aria-hidden="true" />
                  AI 将生成规范化标题并翻译为 {LANGUAGE_OPTIONS.find((language) => language.value === targetLang)?.label}
                </div>
              )}
            </section>
          )}

          <section className="border-y border-[var(--nv-border-subtle)]">
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--nv-border-subtle)] py-2.5">
              <div className="flex flex-wrap items-center gap-2">
                <Button type="button" variant="secondary" size="sm" loading={renaming} onClick={() => void handlePreview()}>
                  {renaming
                    ? <Loader2 size={14} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
                    : <Eye size={14} aria-hidden="true" />}
                  {renaming ? '生成中...' : '生成预览'}
                </Button>
                {renamePreviews.length > 0 && <span className="text-xs text-[var(--nv-text-tertiary)]">共 {renamePreviews.length} 条结果</span>}
              </div>
              {renamePreviews.length > 0 && <Tag tone="success">{renamePreviews.length} 项待更新</Tag>}
            </div>

            {renamePreviews.length > 0 ? (
              <div>
                <div className="flex items-center justify-between gap-2 py-2">
                  <button
                    type="button"
                    onClick={() => setPreviewsExpanded((expanded) => !expanded)}
                    className="flex items-center gap-1.5 rounded-[var(--nv-radius-control)] px-1 py-1 text-xs text-[var(--nv-text-tertiary)] outline-none transition-colors duration-150 hover:text-[var(--nv-text-primary)] focus-visible:shadow-[var(--nv-shadow-focus)]"
                    aria-expanded={previewsExpanded}
                  >
                    <ChevronsUpDown size={13} aria-hidden="true" />
                    {previewsExpanded ? '折叠预览' : '展开预览'}
                  </button>
                </div>

                {previewsExpanded && (
                  <div className="divide-y divide-[var(--nv-border-subtle)] border-t border-[var(--nv-border-subtle)]">
                    {renamePreviews.map((preview, index) => (
                      <div key={`${preview.media_id}-${index}`} className="flex items-start gap-3 px-1 py-3 transition-colors duration-150 hover:bg-[var(--nv-fill-hover)]">
                        <span className="w-6 shrink-0 pt-0.5 text-right font-mono text-xs text-[var(--nv-text-tertiary)]">{index + 1}</span>
                        <div className="min-w-0 flex-1 space-y-1.5">
                          <div className="break-all text-sm leading-5 text-[var(--nv-text-tertiary)] line-through decoration-current/50">{preview.old_title}</div>
                          <div className="flex items-start gap-2">
                            <span className="pt-0.5 text-xs text-[var(--nv-text-tertiary)]" aria-hidden="true">↓</span>
                            <span className="min-w-0 break-all text-sm font-medium leading-5 text-[var(--nv-text-primary)]">{preview.new_title}</span>
                          </div>
                          {preview.reason && (
                            <div className="flex items-start gap-1.5 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                              <Sparkles size={12} className="mt-0.5 shrink-0" aria-hidden="true" />
                              <span>{preview.reason}</span>
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              <div className="flex min-h-44 flex-col items-center justify-center px-6 py-8 text-center">
                <Wand2 size={22} className="mb-3 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                <div className="text-sm font-medium text-[var(--nv-text-secondary)]">尚未生成重命名预览</div>
                <div className="mt-1 max-w-md text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  {useAIRename
                    ? '选择目标语言后生成预览，确认 AI 命名结果再执行。'
                    : '设置命名模板后生成预览，确认新旧标题变化再执行。'}
                </div>
              </div>
            )}
          </section>
        </ModalBody>

        <ModalFooter>
          <div className="mr-auto hidden text-xs text-[var(--nv-text-tertiary)] sm:block">已选择 {selectedCount} 个文件</div>
          <Button type="button" variant="ghost" onClick={onClose} disabled={renaming}>取消</Button>
          <Button type="button" variant="primary" loading={renaming} onClick={() => void handleExecute()} disabled={renamePreviews.length === 0}>
            {renaming
              ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
              : <Check size={15} aria-hidden="true" />}
            执行重命名 {renamePreviews.length > 0 && `(${renamePreviews.length})`}
          </Button>
        </ModalFooter>
      </div>
    </Modal>
  )
}
