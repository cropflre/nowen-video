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

    return () => {
      active = false
    }
  }, [])

  const handleTargetLangChange = (lang: string) => {
    setTargetLang(lang)
    localStorage.setItem('rename_target_lang', lang)
    if (renamePreviews.length > 0) setRenamePreviews([])
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

  const handleExecute = async () => {
    setRenaming(true)
    try {
      const res = await fileManagerApi.executeRename(Array.from(selectedIds), renameTemplate)
      toast.success(`已重命名 ${res.data.renamed} 个文件`)
      onClose()
      onSuccess()
    } catch {
      toast.error('重命名失败')
    } finally {
      setRenaming(false)
    }
  }

  return (
    <Modal open onClose={onClose} size="lg" ariaLabel="批量重命名">
      <div className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title="批量重命名"
          description={`为已选择的 ${selectedCount} 个文件生成新名称，确认预览后再执行。`}
          icon={<Wand2 size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody className="space-y-5">
          <section>
            <div className="mb-2 text-xs font-medium text-[var(--nv-text-tertiary)]">重命名方式</div>
            <div className="grid gap-2 sm:grid-cols-2" role="group" aria-label="重命名方式">
              <button
                type="button"
                onClick={() => setUseAIRename(false)}
                aria-pressed={!useAIRename}
                disabled={renaming}
                className="rounded-[var(--nv-radius-control)] border px-4 py-3 text-left outline-none transition-[background-color,border-color,box-shadow] duration-200 hover:bg-[var(--nv-bg-hover)] focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50"
                style={{
                  borderColor: !useAIRename ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
                  background: !useAIRename ? 'var(--nv-bg-surface-soft)' : 'var(--nv-bg-control)',
                }}
              >
                <div className="flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">
                  <Wand2 size={15} className={!useAIRename ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-tertiary)]'} aria-hidden="true" />
                  模板重命名
                </div>
                <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  使用变量模板生成稳定、可预测的文件名称。
                </div>
              </button>

              <button
                type="button"
                onClick={() => setUseAIRename(true)}
                aria-pressed={useAIRename}
                disabled={renaming}
                className="rounded-[var(--nv-radius-control)] border px-4 py-3 text-left outline-none transition-[background-color,border-color,box-shadow] duration-200 hover:bg-[var(--nv-bg-hover)] focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50"
                style={{
                  borderColor: useAIRename ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
                  background: useAIRename ? 'var(--nv-bg-surface-soft)' : 'var(--nv-bg-control)',
                }}
              >
                <div className="flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">
                  <Sparkles size={15} className={useAIRename ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-tertiary)]'} aria-hidden="true" />
                  AI 智能重命名
                </div>
                <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  由 AI 规范化标题，并可按目标语言生成名称。
                </div>
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
                  onChange={(event) => setRenameTemplate(event.target.value)}
                  className="font-mono"
                  disabled={renaming}
                  aria-label="命名模板"
                />
              </label>

              {renameTemplates.length > 0 && (
                <div>
                  <div className="mb-2 text-xs font-medium text-[var(--nv-text-tertiary)]">常用模板</div>
                  <div className="flex flex-wrap gap-2">
                    {renameTemplates.map((template, index) => {
                      const selected = renameTemplate === template.pattern
                      return (
                        <button
                          key={`${template.pattern}-${index}`}
                          type="button"
                          onClick={() => setRenameTemplate(template.pattern)}
                          disabled={renaming}
                          aria-pressed={selected}
                          title={`示例: ${template.example}`}
                          className="rounded-[var(--nv-radius-control)] border bg-[var(--nv-bg-control)] px-2.5 py-1.5 font-mono text-xs text-[var(--nv-text-secondary)] outline-none transition-[background-color,border-color,color,box-shadow] duration-200 hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50"
                          style={{
                            borderColor: selected ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
                            color: selected ? 'var(--nv-action-primary)' : undefined,
                          }}
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
              <div className="flex flex-wrap gap-2">
                {LANGUAGE_OPTIONS.map((language) => {
                  const selected = targetLang === language.value
                  return (
                    <button
                      key={language.value}
                      type="button"
                      onClick={() => handleTargetLangChange(language.value)}
                      disabled={renaming}
                      aria-pressed={selected}
                      className="flex items-center gap-1.5 rounded-[var(--nv-radius-control)] border bg-[var(--nv-bg-control)] px-2.5 py-1.5 text-xs text-[var(--nv-text-secondary)] outline-none transition-[background-color,border-color,color,box-shadow] duration-200 hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-50"
                      style={{
                        borderColor: selected ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
                        color: selected ? 'var(--nv-action-primary)' : undefined,
                      }}
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

          <section className="rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]">
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--nv-border-subtle)] px-3 py-2.5 sm:px-4">
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  loading={renaming}
                  onClick={() => void handlePreview()}
                >
                  {renaming
                    ? <Loader2 size={14} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
                    : <Eye size={14} aria-hidden="true" />}
                  {renaming ? '生成中...' : '生成预览'}
                </Button>
                {renamePreviews.length > 0 && (
                  <span className="text-xs text-[var(--nv-text-tertiary)]">共 {renamePreviews.length} 条结果</span>
                )}
              </div>

              {renamePreviews.length > 0 && (
                <Tag tone="success">{renamePreviews.length} 项待重命名</Tag>
              )}
            </div>

            {renamePreviews.length > 0 ? (
              <div>
                <div className="flex items-center justify-between gap-2 px-3 py-2 sm:px-4">
                  <button
                    type="button"
                    onClick={() => setPreviewsExpanded((expanded) => !expanded)}
                    className="flex items-center gap-1.5 rounded-[var(--nv-radius-control)] px-1 py-1 text-xs text-[var(--nv-text-tertiary)] outline-none transition-colors hover:text-[var(--nv-text-primary)] focus-visible:shadow-[var(--nv-shadow-focus)]"
                    aria-expanded={previewsExpanded}
                  >
                    <ChevronsUpDown size={13} aria-hidden="true" />
                    {previewsExpanded ? '折叠预览' : '展开预览'}
                  </button>
                </div>

                {previewsExpanded && (
                  <div className="border-t border-[var(--nv-border-subtle)] p-2 sm:p-3">
                    <div className="space-y-2">
                      {renamePreviews.map((preview, index) => (
                        <div
                          key={`${preview.media_id}-${index}`}
                          className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] p-3 transition-colors hover:bg-[var(--nv-bg-hover)]"
                        >
                          <div className="flex items-start gap-3">
                            <span className="w-6 shrink-0 pt-0.5 text-right font-mono text-xs text-[var(--nv-text-tertiary)]">
                              {index + 1}
                            </span>
                            <div className="min-w-0 flex-1 space-y-2">
                              <div className="break-all text-sm leading-5 text-[var(--nv-status-danger)] line-through decoration-current/60">
                                {preview.old_title}
                              </div>
                              <div className="flex items-start gap-2">
                                <span className="pt-0.5 text-xs text-[var(--nv-text-tertiary)]" aria-hidden="true">↓</span>
                                <span className="min-w-0 break-all text-sm font-medium leading-5 text-[var(--nv-status-success)]">
                                  {preview.new_title}
                                </span>
                              </div>
                              {preview.reason && (
                                <div className="flex items-start gap-1.5 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                                  <Sparkles size={12} className="mt-0.5 shrink-0" aria-hidden="true" />
                                  <span>{preview.reason}</span>
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <div className="flex min-h-44 flex-col items-center justify-center px-6 py-8 text-center">
                <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] text-[var(--nv-text-tertiary)]">
                  <Wand2 size={20} aria-hidden="true" />
                </div>
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
          <div className="mr-auto hidden text-xs text-[var(--nv-text-tertiary)] sm:block">
            已选择 {selectedCount} 个文件
          </div>
          <Button type="button" variant="ghost" onClick={onClose} disabled={renaming}>
            取消
          </Button>
          <Button
            type="button"
            variant="primary"
            loading={renaming}
            onClick={() => void handleExecute()}
            disabled={renamePreviews.length === 0}
          >
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
