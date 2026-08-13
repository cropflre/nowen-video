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
    <div className="min-w-0 py-2.5">
      <dt className="text-[11px] font-medium text-[var(--nv-text-tertiary)]">{label}</dt>
      <dd className="mt-1 break-words text-[13px] leading-5 text-[var(--nv-text-secondary)]">{children}</dd>
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
        description="媒体元数据、来源文件与刮削标识。"
        icon={<Eye size={18} aria-hidden="true" />}
        onClose={onClose}
      />

      <ModalBody>
        <div className="grid gap-6 sm:grid-cols-[9rem_minmax(0,1fr)]">
          <div className="mx-auto w-32 sm:mx-0 sm:w-36">
            <div className="aspect-[2/3] overflow-hidden rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-surface-soft)]">
              {!posterFailed ? (
                <img
                  src={streamApi.getPosterUrl(media.id)}
                  alt={`${media.title || '媒体'}海报`}
                  className="h-full w-full object-cover"
                  onError={() => setPosterFailed(true)}
                />
              ) : (
                <div className="flex h-full flex-col items-center justify-center gap-2 px-4 text-center text-[var(--nv-text-tertiary)]">
                  <ImageOff size={24} aria-hidden="true" />
                  <span className="text-xs leading-5">暂无可用海报</span>
                </div>
              )}
            </div>
          </div>

          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-1.5">
              <Tag>{media.media_type === 'movie' ? '电影' : '剧集'}</Tag>
              {media.resolution && <Tag tone="quality">{media.resolution}</Tag>}
              {rating > 0 && <Tag tone="rating">★ {rating.toFixed(1)}</Tag>}
            </div>

            <div className="mt-3">
              <h3 className="break-words text-lg font-semibold leading-7 text-[var(--nv-text-primary)]">
                {media.title || '未命名媒体'}
              </h3>
              {media.orig_title && (
                <p className="mt-0.5 break-words text-xs leading-5 text-[var(--nv-text-tertiary)]">{media.orig_title}</p>
              )}
            </div>

            <dl className="mt-4 grid border-y border-[var(--nv-border-subtle)] sm:grid-cols-2 sm:divide-x sm:divide-[var(--nv-border-subtle)]">
              <div className="divide-y divide-[var(--nv-border-subtle)] sm:pr-4">
                <DetailItem label="年份">{media.year || '-'}</DetailItem>
                <DetailItem label="类型">{media.genres || '-'}</DetailItem>
                <DetailItem label="语言">{media.language || '-'}</DetailItem>
              </div>
              <div className="divide-y divide-[var(--nv-border-subtle)] sm:pl-4">
                <DetailItem label="文件大小">{media.file_size > 0 ? formatFileSize(media.file_size) : '-'}</DetailItem>
                <DetailItem label="国家 / 地区">{media.country || '-'}</DetailItem>
                <DetailItem label="TMDb ID">{media.tmdb_id || '-'}</DetailItem>
                {media.bangumi_id > 0 && <DetailItem label="Bangumi ID">{media.bangumi_id}</DetailItem>}
              </div>
            </dl>
          </div>
        </div>

        {media.overview && (
          <section className="mt-6 border-t border-[var(--nv-border-subtle)] pt-5">
            <h3 className="text-xs font-semibold uppercase tracking-[0.08em] text-[var(--nv-text-tertiary)]">简介</h3>
            <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-[var(--nv-text-secondary)]">{media.overview}</p>
          </section>
        )}

        <section className="mt-6 border-t border-[var(--nv-border-subtle)] pt-5">
          <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-[0.08em] text-[var(--nv-text-tertiary)]">
            <HardDrive size={13} aria-hidden="true" />
            文件路径
          </div>
          <div className="mt-2 break-all font-mono text-xs leading-5 text-[var(--nv-text-secondary)]">{media.file_path || '-'}</div>
        </section>
      </ModalBody>

      <ModalFooter>
        <Button type="button" variant="ghost" onClick={onEdit}>
          <Edit3 size={15} aria-hidden="true" />编辑
        </Button>
        <Button type="button" variant="primary" onClick={onScrape}>
          <Sparkles size={15} aria-hidden="true" />刮削元数据
        </Button>
      </ModalFooter>
    </Modal>
  )
}
