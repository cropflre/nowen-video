import { useState, type ReactNode } from 'react'
import { Edit3, Eye, HardDrive, ImageOff, Sparkles } from 'lucide-react'
import type { Media } from '@/types'
import { streamApi } from '@/api/stream'
import {
  Button,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Tag,
} from '@/components/design-system'
import { formatFileSize } from './constants'

interface FileDetailModalProps {
  media: Media
  onClose: () => void
  onEdit: () => void
  onScrape: () => void
}

interface DetailItemProps {
  label: string
  children: ReactNode
}

function DetailItem({ label, children }: DetailItemProps) {
  return (
    <div className="min-w-0">
      <dt className="text-xs font-medium text-[var(--nv-text-tertiary)]">{label}</dt>
      <dd className="mt-1 break-words text-sm leading-5 text-[var(--nv-text-secondary)]">
        {children}
      </dd>
    </div>
  )
}

export default function FileDetailModal({ media, onClose, onEdit, onScrape }: FileDetailModalProps) {
  const [posterFailed, setPosterFailed] = useState(false)
  const rating = Number(media.rating || 0)

  return (
    <Modal open onClose={onClose} size="lg" ariaLabel="文件详情">
      <ModalHeader
        title="文件详情"
        description="查看媒体元数据、来源文件与刮削标识。"
        icon={<Eye size={18} aria-hidden="true" />}
        onClose={onClose}
      />

      <ModalBody>
        <div className="grid gap-6 sm:grid-cols-[10rem_minmax(0,1fr)]">
          <div className="mx-auto w-36 sm:mx-0 sm:w-40">
            <div className="aspect-[2/3] overflow-hidden rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]">
              {!posterFailed ? (
                <img
                  src={streamApi.getPosterUrl(media.id)}
                  alt={`${media.title || '媒体'}海报`}
                  className="h-full w-full object-cover"
                  onError={() => setPosterFailed(true)}
                />
              ) : (
                <div className="flex h-full flex-col items-center justify-center gap-2 px-4 text-center text-[var(--nv-text-tertiary)]">
                  <ImageOff size={28} aria-hidden="true" />
                  <span className="text-xs leading-5">暂无可用海报</span>
                </div>
              )}
            </div>
          </div>

          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <Tag tone="brand">{media.media_type === 'movie' ? '电影' : '剧集'}</Tag>
              {media.resolution && <Tag tone="quality">{media.resolution}</Tag>}
              {rating > 0 && <Tag tone="rating">★ {rating.toFixed(1)}</Tag>}
            </div>

            <div className="mt-4">
              <h3 className="break-words text-lg font-semibold leading-7 text-[var(--nv-text-primary)]">
                {media.title || '未命名媒体'}
              </h3>
              {media.orig_title && (
                <p className="mt-1 break-words text-sm leading-6 text-[var(--nv-text-tertiary)]">
                  {media.orig_title}
                </p>
              )}
            </div>

            <dl className="mt-5 grid gap-x-6 gap-y-4 border-t border-[var(--nv-border-subtle)] pt-5 sm:grid-cols-2">
              <DetailItem label="年份">{media.year || '-'}</DetailItem>
              <DetailItem label="文件大小">{media.file_size > 0 ? formatFileSize(media.file_size) : '-'}</DetailItem>
              <DetailItem label="类型">{media.genres || '-'}</DetailItem>
              <DetailItem label="国家 / 地区">{media.country || '-'}</DetailItem>
              <DetailItem label="语言">{media.language || '-'}</DetailItem>
              <DetailItem label="TMDb ID">{media.tmdb_id || '-'}</DetailItem>
              {media.bangumi_id > 0 && (
                <DetailItem label="Bangumi ID">{media.bangumi_id}</DetailItem>
              )}
            </dl>
          </div>
        </div>

        {media.overview && (
          <section className="mt-6 border-t border-[var(--nv-border-subtle)] pt-5">
            <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">简介</h3>
            <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-[var(--nv-text-secondary)]">
              {media.overview}
            </p>
          </section>
        )}

        <section className="mt-6 rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
          <div className="flex items-center gap-2 text-xs font-medium text-[var(--nv-text-tertiary)]">
            <HardDrive size={14} aria-hidden="true" />
            文件路径
          </div>
          <div className="mt-2 break-all font-mono text-xs leading-5 text-[var(--nv-text-secondary)]">
            {media.file_path || '-'}
          </div>
        </section>
      </ModalBody>

      <ModalFooter>
        <Button type="button" variant="ghost" onClick={onEdit}>
          <Edit3 size={15} aria-hidden="true" />
          编辑
        </Button>
        <Button type="button" variant="primary" onClick={onScrape}>
          <Sparkles size={15} aria-hidden="true" />
          刮削元数据
        </Button>
      </ModalFooter>
    </Modal>
  )
}
