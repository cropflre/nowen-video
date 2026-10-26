import type { FileDetail, Media, StreamDetail, TechSpecs } from '@/types'
import { formatSize } from './format'

export function realMediaCodec(codec?: string): string | undefined {
  return codec && !codec.toLowerCase().startsWith('strm_') ? codec : undefined
}

export function mediaSourceInfo(media: Media, specs: TechSpecs | null, file: FileDetail | null) {
  const isSTRM = /\.strm$/i.test(media.file_path) || file?.file_ext?.toLowerCase() === '.strm' || Boolean(media.stream_url)
  const probedSize = Number(specs?.format?.size)
  const size = isSTRM
    ? (Number.isFinite(probedSize) && probedSize > 0 ? probedSize : 0)
    : file?.file_size || media.file_size || 0
  const container = specs?.format?.format_name?.split(',')[0]?.toUpperCase()
    || (isSTRM ? '--' : file?.file_ext?.replace(/^\./, '').toUpperCase() || '--')
  return { isSTRM, size, container }
}

export function mediaDynamicRange(stream?: StreamDetail): string {
  if (!stream) return '--'
  if (stream.color_transfer === 'smpte2084') return 'HDR10'
  if (stream.color_transfer === 'arib-std-b67') return 'HLG'
  if (stream.color_space?.includes('2020')) return 'HDR'
  if (stream.pix_fmt?.includes('10')) return '10-bit'
  return stream.pix_fmt || stream.color_transfer ? 'SDR' : '--'
}

export function formatIndexSize(size: number): string {
  if (!Number.isFinite(size) || size <= 0) return '--'
  return size < 1024 * 1024 ? `${size.toLocaleString()} B` : formatSize(size)
}
