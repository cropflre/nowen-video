import { useRef, useState, type ChangeEvent } from 'react'
import { adminApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { TMDbImageInfo } from '@/types'
import { Check, Image as ImageIcon, Link2, Loader2, Search, Upload } from 'lucide-react'
import clsx from 'clsx'
import { Button, Input, Surface, Tag } from '@/components/design-system'
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@/components/design-system/Modal'

type ImageSourceMode = 'upload' | 'url' | 'tmdb'
type ImageEditType = 'poster' | 'backdrop'

interface EditMetadataModalProps {
  type: 'media' | 'series'
  id: string
  tmdbId?: number
  mediaType?: string
  editForm: Record<string, any>
  setEditForm: (form: any) => void
  currentPoster: string
  hasPoster: boolean
  hasBackdrop: boolean
  onSave: () => Promise<void> | void
  onClose: () => void
  hasTagline?: boolean
}

export default function EditMetadataModal({
  type,
  id,
  tmdbId,
  mediaType,
  editForm,
  setEditForm,
  currentPoster,
  hasPoster,
  hasBackdrop,
  onSave,
  onClose,
  hasTagline = false,
}: EditMetadataModalProps) {
  const toast = useToast()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [imageTab, setImageTab] = useState<ImageEditType>('poster')
  const [imageMode, setImageMode] = useState<ImageSourceMode | null>(null)
  const [imageUrl, setImageUrl] = useState('')
  const [imageUploading, setImageUploading] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [tmdbImages, setTmdbImages] = useState<{ posters: TMDbImageInfo[]; backdrops: TMDbImageInfo[] } | null>(null)
  const [tmdbImagesLoading, setTmdbImagesLoading] = useState(false)
  const [selectedTmdbPath, setSelectedTmdbPath] = useState<string | null>(null)

  const resetImageMode = () => {
    setImageMode(null)
    setPreviewUrl(null)
    setSelectedFile(null)
    setImageUrl('')
    setSelectedTmdbPath(null)
  }

  const selectImageTab = (tab: ImageEditType) => {
    setImageTab(tab)
    resetImageMode()
  }

  const handleLoadTmdbImages = async () => {
    if (!tmdbId || tmdbId <= 0) {
      toast.error('当前条目未关联 TMDb，请先手动匹配')
      return
    }
    setTmdbImagesLoading(true)
    try {
      const tmdbType = type === 'series' ? 'tv' : (mediaType || 'movie')
      const res = await adminApi.searchTMDbImages(tmdbId, tmdbType)
      setTmdbImages(res.data.data)
      const images = imageTab === 'poster' ? res.data.data.posters : res.data.data.backdrops
      if (images.length === 0) toast.info('TMDb 上暂无可用图片')
    } catch {
      toast.error('获取 TMDb 图片列表失败')
    } finally {
      setTmdbImagesLoading(false)
    }
  }

  const handleFileSelect = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      toast.error('仅支持 JPG、PNG、WebP 格式')
      return
    }
    if (file.size > 10 * 1024 * 1024) {
      toast.error('图片文件过大，最大支持 10MB')
      return
    }
    setSelectedFile(file)
    const reader = new FileReader()
    reader.onload = () => setPreviewUrl(reader.result as string)
    reader.readAsDataURL(file)
  }

  const handleApplyImage = async () => {
    setImageUploading(true)
    try {
      if (imageMode === 'upload' && selectedFile) {
        if (type === 'media') await adminApi.uploadMediaImage(id, selectedFile, imageTab)
        else await adminApi.uploadSeriesImage(id, selectedFile, imageTab)
      } else if (imageMode === 'url' && imageUrl.trim()) {
        if (type === 'media') await adminApi.setMediaImageByURL(id, imageUrl.trim(), imageTab)
        else await adminApi.setSeriesImageByURL(id, imageUrl.trim(), imageTab)
      } else if (imageMode === 'tmdb' && selectedTmdbPath) {
        if (type === 'media') await adminApi.setMediaImageFromTMDb(id, selectedTmdbPath, imageTab)
        else await adminApi.setSeriesImageFromTMDb(id, selectedTmdbPath, imageTab)
      } else {
        return
      }
      toast.success(`${imageTab === 'poster' ? '海报' : '背景图'}已更新`)
      resetImageMode()
    } catch (error: any) {
      toast.error(error?.response?.data?.error || '图片更新失败')
    } finally {
      setImageUploading(false)
    }
  }

  const tmdbImageList = tmdbImages
    ? (imageTab === 'poster' ? tmdbImages.posters : tmdbImages.backdrops)
    : []

  const setField = (key: string, value: unknown) => setEditForm({ ...editForm, [key]: value })

  return (
    <Modal onClose={onClose} size="lg" ariaLabel="编辑元数据" closeOnBackdrop={!imageUploading}>
      <ModalHeader
        title="编辑元数据"
        description="修改展示信息与海报、背景图。图片更新会立即写入当前条目。"
        icon={<ImageIcon size={18} aria-hidden="true" />}
        onClose={onClose}
      />

      <ModalBody className="space-y-6">
        <Surface className="p-4 sm:p-5">
          <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
                <ImageIcon size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                图片管理
              </div>
              <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">支持 JPG、PNG、WebP，单文件最大 10MB</p>
            </div>
            <div className="flex rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] p-1">
              {(['poster', 'backdrop'] as ImageEditType[]).map((tab) => (
                <button
                  key={tab}
                  type="button"
                  onClick={() => selectImageTab(tab)}
                  className={clsx(
                    'rounded-[calc(var(--nv-radius-control)-2px)] px-3 py-1.5 text-xs font-medium transition-colors',
                    imageTab === tab
                      ? 'bg-[var(--nv-bg-elevated)] text-[var(--nv-action-primary)] shadow-sm'
                      : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)]',
                  )}
                  aria-pressed={imageTab === tab}
                >
                  {tab === 'poster' ? '海报' : '背景图'}
                </button>
              ))}
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-[auto_minmax(0,1fr)]">
            <div
              className="relative overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)]"
              style={{ width: imageTab === 'poster' ? 96 : 160, height: imageTab === 'poster' ? 144 : 90 }}
            >
              {previewUrl ? (
                <img src={previewUrl} alt="新图片预览" className="h-full w-full object-cover" />
              ) : selectedTmdbPath ? (
                <img
                  src={`https://image.tmdb.org/t/p/${imageTab === 'poster' ? 'w185' : 'w300'}${selectedTmdbPath}`}
                  alt="TMDb 图片预览"
                  className="h-full w-full object-cover"
                />
              ) : (imageTab === 'poster' ? hasPoster : hasBackdrop) ? (
                <img src={currentPoster} alt="当前图片" className="h-full w-full object-cover" />
              ) : (
                <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
                  <ImageIcon size={24} aria-hidden="true" />
                </div>
              )}
            </div>

            <div className="min-w-0 space-y-3">
              {!imageMode && (
                <div className="flex flex-wrap gap-2">
                  <Button type="button" variant="secondary" size="sm" onClick={() => { resetImageMode(); setImageMode('upload') }}>
                    <Upload size={13} aria-hidden="true" /> 本地上传
                  </Button>
                  <Button type="button" variant="secondary" size="sm" onClick={() => { resetImageMode(); setImageMode('url') }}>
                    <Link2 size={13} aria-hidden="true" /> 输入 URL
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={() => { resetImageMode(); setImageMode('tmdb'); void handleLoadTmdbImages() }}
                  >
                    <Search size={13} aria-hidden="true" /> TMDb 图片
                  </Button>
                </div>
              )}

              {imageMode === 'upload' && (
                <div className="space-y-3">
                  <input ref={fileInputRef} type="file" accept=".jpg,.jpeg,.png,.webp" onChange={handleFileSelect} className="hidden" />
                  <button
                    type="button"
                    onClick={() => fileInputRef.current?.click()}
                    className="w-full rounded-[var(--nv-radius-control)] border border-dashed border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] px-4 py-5 text-center text-xs text-[var(--nv-text-secondary)] transition-colors hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)]"
                  >
                    {selectedFile ? `已选择：${selectedFile.name}` : '点击选择图片文件'}
                  </button>
                  <div className="flex gap-2">
                    <Button type="button" variant="ghost" size="sm" onClick={resetImageMode}>取消</Button>
                    {selectedFile && (
                      <Button type="button" variant="primary" size="sm" onClick={handleApplyImage} loading={imageUploading}>
                        {imageUploading ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
                        确认上传
                      </Button>
                    )}
                  </div>
                </div>
              )}

              {imageMode === 'url' && (
                <div className="space-y-3">
                  <Input
                    value={imageUrl}
                    onChange={(event) => setImageUrl(event.target.value)}
                    placeholder="输入图片 URL 地址..."
                    autoFocus
                  />
                  <div className="flex gap-2">
                    <Button type="button" variant="ghost" size="sm" onClick={resetImageMode}>取消</Button>
                    {imageUrl.trim() && (
                      <Button type="button" variant="primary" size="sm" onClick={handleApplyImage} loading={imageUploading}>
                        {imageUploading ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
                        确认下载
                      </Button>
                    )}
                  </div>
                </div>
              )}

              {imageMode === 'tmdb' && (
                <div className="space-y-3">
                  {tmdbImagesLoading ? (
                    <div className="flex items-center gap-2 py-4 text-xs text-[var(--nv-text-tertiary)]">
                      <Loader2 size={14} className="animate-spin" /> 加载 TMDb 图片列表...
                    </div>
                  ) : tmdbImageList.length > 0 ? (
                    <>
                      <div className="grid max-h-48 grid-cols-4 gap-2 overflow-y-auto pr-1 sm:grid-cols-5">
                        {tmdbImageList.slice(0, 20).map((image) => {
                          const selected = selectedTmdbPath === image.file_path
                          return (
                            <button
                              key={image.file_path}
                              type="button"
                              onClick={() => { setSelectedTmdbPath(image.file_path); setPreviewUrl(null) }}
                              className={clsx(
                                'group relative overflow-hidden rounded-[var(--nv-radius-sm)] border bg-[var(--nv-bg-surface-soft)] transition-[border-color,box-shadow,transform]',
                                selected
                                  ? 'border-[var(--nv-action-primary)] shadow-[var(--nv-shadow-focus)]'
                                  : 'border-[var(--nv-border-default)] hover:border-[var(--nv-border-hover)]',
                              )}
                              aria-pressed={selected}
                            >
                              <img
                                src={`https://image.tmdb.org/t/p/${imageTab === 'poster' ? 'w92' : 'w185'}${image.file_path}`}
                                alt=""
                                className="w-full object-cover"
                                style={{ aspectRatio: imageTab === 'poster' ? '2 / 3' : '16 / 9' }}
                              />
                              {selected && (
                                <div className="absolute inset-0 flex items-center justify-center bg-black/35 text-white">
                                  <Check size={18} aria-hidden="true" />
                                </div>
                              )}
                              <div className="absolute inset-x-0 bottom-0 bg-black/65 px-1 py-0.5 text-center text-[9px] text-white/80">
                                {image.width}×{image.height}
                              </div>
                            </button>
                          )
                        })}
                      </div>
                      <div className="flex gap-2">
                        <Button type="button" variant="ghost" size="sm" onClick={resetImageMode}>取消</Button>
                        {selectedTmdbPath && (
                          <Button type="button" variant="primary" size="sm" onClick={handleApplyImage} loading={imageUploading}>
                            {imageUploading ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
                            应用选中图片
                          </Button>
                        )}
                      </div>
                    </>
                  ) : (
                    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4 text-xs text-[var(--nv-text-tertiary)]">
                      <p>{!tmdbId ? '当前条目未关联 TMDb，请先手动匹配后再搜索图片' : '暂无可用图片'}</p>
                      <Button type="button" variant="ghost" size="sm" onClick={resetImageMode} className="mt-2">返回</Button>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </Surface>

        <section aria-labelledby="metadata-fields-title" className="space-y-4">
          <div className="flex items-center gap-2">
            <h3 id="metadata-fields-title" className="text-sm font-semibold text-[var(--nv-text-primary)]">基础信息</h3>
            <Tag tone="neutral">Metadata</Tag>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="标题"><Input value={editForm.title || ''} onChange={(event) => setField('title', event.target.value)} /></Field>
            <Field label="原始标题"><Input value={editForm.orig_title || ''} onChange={(event) => setField('orig_title', event.target.value)} /></Field>
          </div>

          <div className="grid gap-4 sm:grid-cols-3">
            <Field label="年份">
              <Input type="number" value={editForm.year || ''} onChange={(event) => setField('year', parseInt(event.target.value) || 0)} />
            </Field>
            <Field label="评分">
              <Input type="number" step="0.1" min="0" max="10" value={editForm.rating || ''} onChange={(event) => setField('rating', parseFloat(event.target.value) || 0)} />
            </Field>
            <Field label="类型">
              <Input value={editForm.genres || ''} onChange={(event) => setField('genres', event.target.value)} placeholder="动作,科幻" />
            </Field>
          </div>

          <Field label="简介">
            <textarea
              value={editForm.overview || ''}
              onChange={(event) => setField('overview', event.target.value)}
              rows={5}
              className="w-full resize-y rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] px-3 py-2.5 text-sm leading-6 text-[var(--nv-text-primary)] outline-none transition-[border-color,box-shadow] placeholder:text-[var(--nv-text-tertiary)] hover:border-[var(--nv-border-hover)] focus:border-[var(--nv-action-primary)] focus:shadow-[var(--nv-shadow-focus)]"
            />
          </Field>

          {hasTagline && (
            <Field label="宣传语"><Input value={editForm.tagline || ''} onChange={(event) => setField('tagline', event.target.value)} /></Field>
          )}

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="国家 / 地区"><Input value={editForm.country || ''} onChange={(event) => setField('country', event.target.value)} /></Field>
            <Field label="语言"><Input value={editForm.language || ''} onChange={(event) => setField('language', event.target.value)} /></Field>
          </div>

          <Field label="出品公司"><Input value={editForm.studio || ''} onChange={(event) => setField('studio', event.target.value)} /></Field>
        </section>
      </ModalBody>

      <ModalFooter>
        <Button type="button" variant="secondary" onClick={onClose}>取消</Button>
        <Button type="button" variant="primary" onClick={onSave}>保存</Button>
      </ModalFooter>
    </Modal>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="block text-xs font-medium text-[var(--nv-text-secondary)]">{label}</span>
      {children}
    </label>
  )
}
