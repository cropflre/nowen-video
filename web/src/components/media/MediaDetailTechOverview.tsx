import type { ReactNode } from 'react'
import type { FileDetail, Media, TechSpecs } from '@/types'
import { formatSize } from '@/utils/format'
import { AudioLines, Gauge, HardDrive, Monitor, Palette, PanelsTopLeft, ScanLine, Video } from 'lucide-react'

interface MediaDetailTechOverviewProps {
  media: Media
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
}

interface StreamShape {
  codec_type?: string
  codec_name?: string
  profile?: string
  width?: number
  height?: number
  bit_rate?: string
  avg_frame_rate?: string
  r_frame_rate?: string
  channels?: number
  channel_layout?: string
  color_transfer?: string
  color_space?: string
  pix_fmt?: string
}

interface FormatShape {
  format_name?: string
  bit_rate?: string
}

function parseNumber(value?: string): number | null {
  if (!value) return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

function parseFrameRate(value?: string): string {
  if (!value) return '--'
  if (!value.includes('/')) {
    const numeric = parseNumber(value)
    return numeric ? `${numeric.toFixed(numeric % 1 === 0 ? 0 : 2)} fps` : value
  }
  const [first, second] = value.split('/').map(Number)
  if (!Number.isFinite(first) || !Number.isFinite(second) || second === 0) return value
  const fps = first / second
  return `${fps.toFixed(Math.abs(fps - Math.round(fps)) < 0.01 ? 0 : 3)} fps`
}

function formatBitrate(value?: string): string {
  const bitRate = parseNumber(value)
  if (!bitRate) return '--'
  if (bitRate >= 1_000_000) return `${(bitRate / 1_000_000).toFixed(1)} Mbps`
  if (bitRate >= 1_000) return `${Math.round(bitRate / 1_000)} Kbps`
  return `${bitRate} bps`
}

function codecLabel(codec?: string, profile?: string): string {
  if (!codec) return '--'
  const normalized = codec.toLowerCase()
  const name = normalized === 'hevc' || normalized === 'h265'
    ? 'HEVC'
    : normalized === 'h264'
      ? 'H.264'
      : normalized === 'av1'
        ? 'AV1'
        : codec.toUpperCase()
  return profile ? `${name} ${profile}` : name
}

function hdrLabel(stream?: StreamShape): string {
  if (!stream) return 'SDR'
  if (stream.color_transfer === 'smpte2084') return 'HDR10'
  if (stream.color_transfer === 'arib-std-b67') return 'HLG'
  if (stream.color_space?.includes('2020')) return 'HDR'
  if (stream.pix_fmt?.includes('10')) return '10-bit'
  return 'SDR'
}

function audioLabel(stream?: StreamShape, fallback?: string): string {
  const codec = codecLabel(stream?.codec_name || fallback)
  if (stream?.channel_layout) return `${codec} · ${stream.channel_layout}`
  if (stream?.channels === 8) return `${codec} · 7.1`
  if (stream?.channels === 6) return `${codec} · 5.1`
  if (stream?.channels === 2) return `${codec} · 2.0`
  return codec
}

function DetailMetric({
  icon,
  label,
  value,
}: {
  icon: ReactNode
  label: string
  value: string
}) {
  return (
    <div className="nv-detail-tech-metric">
      <div className="nv-detail-tech-metric-icon" aria-hidden="true">{icon}</div>
      <div className="min-w-0">
        <span>{label}</span>
        <strong title={value}>{value}</strong>
      </div>
    </div>
  )
}

export default function MediaDetailTechOverview({ media, techSpecs, fileInfo }: MediaDetailTechOverviewProps) {
  const streams = (techSpecs?.streams || []) as unknown as StreamShape[]
  const mainVideo = streams.find((stream) => stream.codec_type === 'video')
  const mainAudio = streams.find((stream) => stream.codec_type === 'audio')
  const format = techSpecs?.format as unknown as FormatShape | null

  const resolution = mainVideo?.width && mainVideo?.height
    ? `${mainVideo.width}×${mainVideo.height}`
    : media.resolution || '--'
  const sourceSize = fileInfo?.file_size || media.file_size || 0
  const container = fileInfo?.file_ext
    ? fileInfo.file_ext.replace(/^\./, '').toUpperCase()
    : format?.format_name?.split(',')[0]?.toUpperCase() || '--'

  return (
    <section className="nv-detail-tech-overview" aria-labelledby="detail-tech-overview-title">
      <div className="nv-detail-section-heading">
        <div>
          <span className="nv-detail-section-eyebrow">Technical</span>
          <h2 id="detail-tech-overview-title">技术规格</h2>
        </div>
        <span className="nv-detail-section-hint">主播放文件</span>
      </div>

      <div className="nv-detail-tech-grid">
        <DetailMetric icon={<Monitor size={15} />} label="分辨率" value={resolution} />
        <DetailMetric icon={<Video size={15} />} label="视频编码" value={codecLabel(mainVideo?.codec_name || media.video_codec, mainVideo?.profile)} />
        <DetailMetric icon={<Palette size={15} />} label="动态范围" value={hdrLabel(mainVideo)} />
        <DetailMetric icon={<ScanLine size={15} />} label="帧率" value={parseFrameRate(mainVideo?.avg_frame_rate || mainVideo?.r_frame_rate)} />
        <DetailMetric icon={<AudioLines size={15} />} label="音频" value={audioLabel(mainAudio, media.audio_codec)} />
        <DetailMetric icon={<Gauge size={15} />} label="总码率" value={formatBitrate(format?.bit_rate || mainVideo?.bit_rate)} />
        <DetailMetric icon={<PanelsTopLeft size={15} />} label="容器" value={container} />
        <DetailMetric icon={<HardDrive size={15} />} label="大小" value={sourceSize > 0 ? formatSize(sourceSize) : '--'} />
      </div>
    </section>
  )
}
