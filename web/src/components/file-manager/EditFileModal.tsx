import { useState, type FormEvent } from 'react'
import { Edit3, Loader2, Save } from 'lucide-react'
import type { Media } from '@/types'
import { fileManagerApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Select,
  Textarea,
} from '@/components/design-system'

interface EditFileModalProps {
  media: Media
  onClose: () => void
  onSuccess: () => void
}

type EditForm = Record<string, unknown>

const TEXT_FIELDS = [
  { key: 'title', label: '标题', type: 'text' },
  { key: 'orig_title', label: '原始标题', type: 'text' },
  { key: 'year', label: '年份', type: 'number' },
  { key: 'rating', label: '评分', type: 'number' },
  { key: 'genres', label: '类型', type: 'text', placeholder: '动作,科幻,冒险' },
  { key: 'country', label: '国家/地区', type: 'text' },
  { key: 'language', label: '语言', type: 'text' },
] as const

export default function EditFileModal({ media, onClose, onSuccess }: EditFileModalProps) {
  const toast = useToast()
  const [saving, setSaving] = useState(false)
  const [editForm, setEditForm] = useState<EditForm>({
    title: media.title,
    orig_title: media.orig_title,
    year: media.year,
    overview: media.overview,
    genres: media.genres,
    rating: media.rating,
    media_type: media.media_type,
    country: media.country,
    language: media.language,
  })

  const handleFieldChange = (key: string, value: string, type: 'text' | 'number' = 'text') => {
    setEditForm((previous) => ({
      ...previous,
      [key]: type === 'number' ? Number(value) : value,
    }))
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await fileManagerApi.updateFile(media.id, editForm)
      toast.success('更新成功')
      onClose()
      onSuccess()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '更新失败')
    } finally {
      setSaving(false)
    }
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!saving) void handleSave()
  }

  return (
    <Modal
      open
      onClose={onClose}
      size="md"
      ariaLabel="编辑文件信息"
      panelClassName="max-w-2xl"
    >
      <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title="编辑文件信息"
          description={`修改「${media.title || '未命名媒体'}」的展示元数据。保存后不会移动或重命名源文件。`}
          icon={<Edit3 size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody>
          <div className="grid gap-4 sm:grid-cols-2">
            {TEXT_FIELDS.map((field) => (
              <label
                key={field.key}
                className={field.key === 'genres' ? 'block space-y-1.5 sm:col-span-2' : 'block space-y-1.5'}
              >
                <span className="text-sm font-medium text-[var(--nv-text-secondary)]">{field.label}</span>
                <Input
                  type={field.type}
                  value={editForm[field.key] ? String(editForm[field.key]) : ''}
                  onChange={(event) => handleFieldChange(field.key, event.target.value, field.type)}
                  placeholder={'placeholder' in field ? field.placeholder : undefined}
                  disabled={saving}
                  step={field.key === 'rating' ? '0.1' : undefined}
                />
              </label>
            ))}

            <label className="block space-y-1.5">
              <span className="text-sm font-medium text-[var(--nv-text-secondary)]">媒体类型</span>
              <Select
                value={String(editForm.media_type || 'movie')}
                onChange={(event) => handleFieldChange('media_type', event.target.value)}
                className="w-full"
                disabled={saving}
              >
                <option value="movie">电影</option>
                <option value="episode">剧集</option>
              </Select>
            </label>

            <div className="hidden sm:block" aria-hidden="true" />

            <label className="block space-y-1.5 sm:col-span-2">
              <span className="text-sm font-medium text-[var(--nv-text-secondary)]">简介</span>
              <Textarea
                value={String(editForm.overview || '')}
                onChange={(event) => handleFieldChange('overview', event.target.value)}
                rows={5}
                className="resize-y"
                disabled={saving}
              />
            </label>

            <div className="sm:col-span-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-2.5 text-xs leading-5 text-[var(--nv-text-tertiary)]">
              此处只更新 Nowen Video 中保存的媒体元数据，不修改磁盘上的视频文件内容。
            </div>
          </div>
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose} disabled={saving}>
            取消
          </Button>
          <Button type="submit" variant="primary" loading={saving}>
            {saving
              ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
              : <Save size={15} aria-hidden="true" />}
            {saving ? '保存中...' : '保存'}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
