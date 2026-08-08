import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { formatDate, formatDuration, formatSize } from '@/utils/format'
import type {
  FileDetail,
  LibraryInfo,
  Media,
  PlaybackStatsInfo,
  StreamDetail,
  TechSpecs,
} from '@/types'
import { Button, EmptyState, Surface, Tag } from '@/components/design-system'
import {
  BarChart3,
  Check,
  ChevronDown,
  ChevronUp,
  Clock,
  Copy,
  Cpu,
  FileCode,
  FileJson,
  FileText,
  FolderOpen,
  HardDrive,
  Hash,
  HelpCircle,
  Info,
  Layers,
  Monitor,
  Music,
  Play,
  Shield,
  Subtitles,
  User,
  Users,
} from 'lucide-react'

function formatBitRate(bitRate?: string): string {
  if (!bitRate) return '-'
  const num = parseInt(bitRate)
  if (Number.isNaN(num)) return bitRate
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(2)} Mbps`
  if (num >= 1_000) return `${(num / 1_000).toFixed(0)} Kbps`
  return `${num} bps`
}

function formatSampleRate(rate?: string): string {
  if (!rate) return '-'
  const num = parseInt(rate)
  if (Number.isNaN(num)) return rate
  return `${num} Hz`
}

function formatChannels(channels?: number, layout?: string): string {
  if (layout) {
    const layoutMap: Record<string, string> = {
      mono: '单声道',
      stereo: '立体声',
      '5.1': '5.1 环绕声',
      '5.1(side)': '5.1 环绕声',
      '7.1': '7.1 环绕声',
      '7.1(wide)': '7.1 环绕声',
    }
    return layoutMap[layout] || layout
  }
  if (!channels) return '-'
  if (channels === 1) return '单声道'
  if (channels === 2) return '立体声'
  if (channels === 6) return '5.1 环绕声'
  if (channels === 8) return '7.1 环绕声'
  return `${channels} 声道`
}

function formatCodecName(name: string, longName?: string): string {
  const codecMap: Record<string, string> = {
    h264: 'H.264 / AVC',
    hevc: 'H.265 / HEVC',
    h265: 'H.265 / HEVC',
    vp9: 'VP9',
    av1: 'AV1',
    mpeg4: 'MPEG-4',
    aac: 'AAC',
    ac3: 'AC-3 / Dolby Digital',
    eac3: 'E-AC-3 / Dolby Digital Plus',
    dts: 'DTS',
    flac: 'FLAC',
    opus: 'Opus',
    vorbis: 'Vorbis',
    mp3: 'MP3',
    truehd: 'Dolby TrueHD',
    pcm_s16le: 'PCM 16-bit',
    pcm_s24le: 'PCM 24-bit',
    srt: 'SRT',
    ass: 'ASS/SSA',
    subrip: 'SRT',
    hdmv_pgs_subtitle: 'PGS (蓝光)',
    dvd_subtitle: 'VobSub',
    webvtt: 'WebVTT',
    mov_text: 'MOV Text',
  }
  return codecMap[name] || longName || name.toUpperCase()
}

function formatContainerName(name: string): string {
  const containerMap: Record<string, string> = {
    'matroska,webm': 'Matroska (MKV)',
    'mov,mp4,m4a,3gp,3g2,mj2': 'MP4 / MOV',
    avi: 'AVI',
    mpegts: 'MPEG-TS',
    flv: 'FLV',
    ogg: 'OGG',
    webm: 'WebM',
  }
  return containerMap[name] || name
}

function formatLanguage(lang?: string): string {
  if (!lang || lang === 'und') return '未知'
  const langMap: Record<string, string> = {
    chi: '中文', zho: '中文', zh: '中文',
    eng: '英语', en: '英语',
    jpn: '日语', ja: '日语',
    kor: '韩语', ko: '韩语',
    fre: '法语', fra: '法语', fr: '法语',
    ger: '德语', deu: '德语', de: '德语',
    spa: '西班牙语', es: '西班牙语',
    ita: '意大利语', it: '意大利语',
    por: '葡萄牙语', pt: '葡萄牙语',
    rus: '俄语', ru: '俄语',
    tha: '泰语', th: '泰语',
    vie: '越南语', vi: '越南语',
    ara: '阿拉伯语', ar: '阿拉伯语',
  }
  return langMap[lang] || lang
}

function formatPixFmt(fmt?: string): string {
  if (!fmt) return '-'
  const fmtMap: Record<string, string> = {
    yuv420p: 'YUV 4:2:0 8-bit',
    yuv420p10le: 'YUV 4:2:0 10-bit',
    yuv420p10be: 'YUV 4:2:0 10-bit',
    yuv422p: 'YUV 4:2:2 8-bit',
    yuv444p: 'YUV 4:4:4 8-bit',
    yuv444p10le: 'YUV 4:4:4 10-bit',
    rgb24: 'RGB 24-bit',
    nv12: 'NV12',
  }
  return fmtMap[fmt] || fmt
}

function isHDR(stream: StreamDetail): boolean {
  const hdrTransfers = ['smpte2084', 'arib-std-b67', 'smpte428']
  const hdrSpaces = ['bt2020nc', 'bt2020c']
  return Boolean(
    (stream.color_transfer && hdrTransfers.includes(stream.color_transfer)) ||
    (stream.color_space && hdrSpaces.includes(stream.color_space)) ||
    stream.pix_fmt?.includes('10'),
  )
}

function getHDRLabel(stream: StreamDetail): string {
  if (stream.color_transfer === 'smpte2084') return 'HDR10'
  if (stream.color_transfer === 'arib-std-b67') return 'HLG'
  if (stream.color_space === 'bt2020nc' || stream.color_space === 'bt2020c') return 'HDR'
  return 'SDR'
}

function formatColorPrimaries(primaries?: string): string {
  if (!primaries) return '--'
  const map: Record<string, string> = {
    bt709: 'BT.709',
    bt2020: 'BT.2020',
    smpte170m: 'SMPTE 170M',
    smpte240m: 'SMPTE 240M',
    bt470bg: 'BT.470 BG',
  }
  return map[primaries] || primaries
}

const paramHints: Record<string, string> = {
  编码器: '视频/音频数据的压缩编码格式',
  配置: '编码器使用的预设配置档次',
  等级: '编码器的复杂度等级',
  分辨率: '视频画面的像素宽度×高度',
  帧率: '每秒显示的画面帧数',
  码率: '每秒传输的数据量，越高画质越好',
  位深度: '每个像素的色彩精度，10-bit 色彩更丰富',
  像素格式: '像素数据的存储格式和色度采样方式',
  视频动态范围: 'SDR 为标准动态范围，HDR 提供更高亮度和对比度',
  宽高比: '画面的宽度与高度之比',
  色彩空间: '定义颜色的数学模型',
  色彩转换: '亮度信号的传输特性曲线',
  色彩原色: '定义红绿蓝三原色的色度坐标',
  色彩范围: 'TV 为有限范围(16-235)，PC 为完整范围(0-255)',
  隔行扫描: '是否使用隔行扫描（交错显示奇偶行）',
  参考帧: '编码时参考的前后帧数量',
  总帧数: '视频流中的总帧数',
  语言: '音频/字幕轨道的语言',
  布局: '音频声道的空间布局方式',
  声道: '音频的声道数量',
  采样率: '每秒采集的音频样本数，越高音质越好',
  位深: '每个音频样本的精度',
  'MIME 类型': '文件的互联网媒体类型标识',
  MD5: '文件内容的 MD5 哈希校验值',
  精确时长: '基于容器元数据的精确播放时长',
  总码率: '所有流（视频+音频+字幕）的总数据传输速率',
}

type TabKey = 'overview' | 'video' | 'audio' | 'subtitle' | 'file' | 'stats'

interface TabDef {
  key: TabKey
  label: string
  icon: ReactNode
  badge?: string | number
}

interface MediaTechSpecsProps {
  media: Media
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
  library: LibraryInfo | null
  playbackStats: PlaybackStatsInfo | null
  loading: boolean
  isAdmin: boolean
}

export default function MediaTechSpecs({
  media,
  techSpecs,
  fileInfo,
  library,
  playbackStats,
  loading,
  isAdmin,
}: MediaTechSpecsProps) {
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const [expanded, setExpanded] = useState(false)

  const exportJSON = useCallback(() => {
    const data = { techSpecs, fileInfo, library, playbackStats }
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `tech-specs-${fileInfo?.file_name || media.title || 'media'}.json`
    anchor.click()
    URL.revokeObjectURL(url)
  }, [techSpecs, fileInfo, library, playbackStats, media.title])

  const exportXML = useCallback(() => {
    const toXML = (obj: unknown, rootName: string): string => {
      const escape = (value: string) => String(value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
      const convert = (value: unknown, name: string, indent: string): string => {
        if (value === null || value === undefined) return `${indent}<${name}/>\n`
        if (typeof value !== 'object') return `${indent}<${name}>${escape(String(value))}</${name}>\n`
        if (Array.isArray(value)) return value.map((item) => convert(item, name.replace(/s$/, ''), indent)).join('')
        let xml = `${indent}<${name}>\n`
        for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
          xml += convert(child, key, `${indent}  `)
        }
        return `${xml}${indent}</${name}>\n`
      }
      return `<?xml version="1.0" encoding="UTF-8"?>\n${convert(obj, rootName, '')}`
    }

    const data = { techSpecs, fileInfo, library, playbackStats }
    const blob = new Blob([toXML(data, 'MediaTechSpecs')], { type: 'application/xml' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `tech-specs-${fileInfo?.file_name || media.title || 'media'}.xml`
    anchor.click()
    URL.revokeObjectURL(url)
  }, [techSpecs, fileInfo, library, playbackStats, media.title])

  const videoStreams = useMemo(() => techSpecs?.streams?.filter((stream) => stream.codec_type === 'video') || [], [techSpecs])
  const audioStreams = useMemo(() => techSpecs?.streams?.filter((stream) => stream.codec_type === 'audio') || [], [techSpecs])
  const subtitleStreams = useMemo(() => techSpecs?.streams?.filter((stream) => stream.codec_type === 'subtitle') || [], [techSpecs])
  const mainVideo = videoStreams[0]
  const mainAudio = audioStreams[0]

  const tabs = useMemo<TabDef[]>(() => {
    const values: TabDef[] = [
      { key: 'overview', label: '概览', icon: <Cpu size={14} /> },
      { key: 'video', label: '视频', icon: <Monitor size={14} />, badge: videoStreams.length || undefined },
      { key: 'audio', label: '音频', icon: <Music size={14} />, badge: audioStreams.length || undefined },
      { key: 'subtitle', label: '字幕', icon: <Subtitles size={14} />, badge: subtitleStreams.length || undefined },
      { key: 'file', label: '文件', icon: <Layers size={14} /> },
    ]
    if (isAdmin && playbackStats && (playbackStats.total_play_count > 0 || playbackStats.unique_viewers > 0)) {
      values.push({ key: 'stats', label: '统计', icon: <BarChart3 size={14} /> })
    }
    return values
  }, [audioStreams.length, isAdmin, playbackStats, subtitleStreams.length, videoStreams.length])

  if (loading) {
    return (
      <section aria-label="文件信息与技术规格加载中">
        <div className="mb-3 skeleton h-5 w-36 rounded-lg" />
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[1, 2, 3, 4].map((item) => <div key={item} className="skeleton h-24 rounded-[var(--nv-radius-card)]" />)}
        </div>
      </section>
    )
  }

  return (
    <section className="space-y-3" aria-labelledby="media-tech-specs-title">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-[var(--nv-text-primary)]">
            <Cpu size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            <h3 id="media-tech-specs-title" className="text-sm font-semibold">文件信息与技术规格</h3>
          </div>
          <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">容器、音视频流、字幕与播放统计</p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {isAdmin && (
            <>
              <Button type="button" variant="ghost" size="sm" onClick={exportJSON} title="导出为 JSON">
                <FileJson size={13} aria-hidden="true" />
                JSON
              </Button>
              <Button type="button" variant="ghost" size="sm" onClick={exportXML} title="导出为 XML">
                <FileCode size={13} aria-hidden="true" />
                XML
              </Button>
            </>
          )}
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => setExpanded((value) => !value)}
            aria-expanded={expanded}
            aria-controls="media-tech-specs-details"
          >
            {expanded ? <ChevronUp size={13} aria-hidden="true" /> : <ChevronDown size={13} aria-hidden="true" />}
            {expanded ? '收起' : '查看详情'}
          </Button>
        </div>
      </div>

      <div className="grid overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] sm:grid-cols-2 lg:grid-cols-4">
        <SummaryCell
          icon={<Monitor size={16} />}
          label="视频"
          value={mainVideo
            ? `${mainVideo.width && mainVideo.height ? `${mainVideo.height}p ` : ''}${formatCodecName(mainVideo.codec_name)}`
            : (media.resolution || media.video_codec || '-')}
          detail={mainVideo
            ? [
                mainVideo.width && mainVideo.height ? `${mainVideo.width}×${mainVideo.height}` : null,
                mainVideo.frame_rate ? `${parseFloat(mainVideo.frame_rate).toFixed(0)}fps` : null,
                mainVideo.bit_rate ? formatBitRate(mainVideo.bit_rate) : null,
              ].filter(Boolean).join(' · ')
            : '无详细视频流'}
          tags={mainVideo ? (
            <>
              {isHDR(mainVideo) && <Tag tone="rating">{getHDRLabel(mainVideo)}</Tag>}
              {mainVideo.is_interlaced && <Tag tone="danger">隔行</Tag>}
            </>
          ) : undefined}
        />
        <SummaryCell
          icon={<Music size={16} />}
          label="音频"
          value={mainAudio
            ? `${formatCodecName(mainAudio.codec_name)} · ${formatChannels(mainAudio.channels, mainAudio.channel_layout)}`
            : (media.audio_codec || '-')}
          detail={mainAudio
            ? [
                mainAudio.sample_rate ? formatSampleRate(mainAudio.sample_rate) : null,
                mainAudio.bit_rate ? formatBitRate(mainAudio.bit_rate) : null,
                mainAudio.language ? formatLanguage(mainAudio.language) : null,
              ].filter(Boolean).join(' · ')
            : '无详细音频流'}
          tags={audioStreams.length > 1 ? <Tag>{audioStreams.length} 轨</Tag> : undefined}
        />
        <SummaryCell
          icon={<Subtitles size={16} />}
          label="字幕"
          value={subtitleStreams.length > 0 ? `内嵌 ${subtitleStreams.length} 条` : '无内嵌字幕'}
          detail={subtitleStreams.length > 0
            ? subtitleStreams.map((stream) => formatLanguage(stream.language)).filter((value, index, array) => array.indexOf(value) === index).join(' / ')
            : '-'}
        />
        <SummaryCell
          icon={<HardDrive size={16} />}
          label="容器"
          value={techSpecs?.format ? formatContainerName(techSpecs.format.format_name) : fileInfo?.file_ext?.replace('.', '').toUpperCase() || '-'}
          detail={techSpecs?.format
            ? [
                techSpecs.format.bit_rate ? formatBitRate(techSpecs.format.bit_rate) : null,
                techSpecs.format.stream_count ? `${techSpecs.format.stream_count} 流` : null,
                formatSize(media.file_size),
              ].filter(Boolean).join(' · ')
            : formatSize(media.file_size)}
        />
      </div>

      {expanded && (
        <div id="media-tech-specs-details" className="space-y-4 animate-fade-in">
          <div
            className="flex gap-1 overflow-x-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-1"
            role="tablist"
            aria-label="技术规格分类"
          >
            {tabs.map((tab) => {
              const selected = activeTab === tab.key
              return (
                <button
                  key={tab.key}
                  type="button"
                  role="tab"
                  aria-selected={selected}
                  onClick={() => setActiveTab(tab.key)}
                  className="inline-flex min-h-9 items-center gap-1.5 whitespace-nowrap rounded-[var(--nv-radius-sm)] border px-3 text-xs font-medium transition-[background-color,border-color,color] duration-200"
                  style={{
                    background: selected ? 'var(--nv-bg-active)' : 'transparent',
                    borderColor: selected ? 'var(--nv-border-hover)' : 'transparent',
                    color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
                  }}
                >
                  {tab.icon}
                  {tab.label}
                  {tab.badge !== undefined && <Tag tone={selected ? 'brand' : 'neutral'}>{tab.badge}</Tag>}
                </button>
              )
            })}
          </div>

          <div role="tabpanel">
            {activeTab === 'overview' && (
              <OverviewPanel media={media} techSpecs={techSpecs} fileInfo={fileInfo} library={library} isAdmin={isAdmin} />
            )}
            {activeTab === 'video' && <VideoPanel media={media} streams={videoStreams} />}
            {activeTab === 'audio' && <AudioPanel media={media} streams={audioStreams} />}
            {activeTab === 'subtitle' && <SubtitlePanel streams={subtitleStreams} />}
            {activeTab === 'file' && (
              <FilePanel media={media} techSpecs={techSpecs} fileInfo={fileInfo} library={library} isAdmin={isAdmin} />
            )}
            {activeTab === 'stats' && playbackStats && <StatsPanel stats={playbackStats} />}
          </div>
        </div>
      )}
    </section>
  )
}

function SummaryCell({
  icon,
  label,
  value,
  detail,
  tags,
}: {
  icon: ReactNode
  label: string
  value: string
  detail: string
  tags?: ReactNode
}) {
  return (
    <div className="min-w-0 border-b border-[var(--nv-border-subtle)] p-3 last:border-b-0 sm:[&:nth-child(odd)]:border-r lg:border-b-0 lg:border-r lg:last:border-r-0">
      <div className="flex items-start gap-2.5">
        <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
          {icon}
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-[10px] font-medium uppercase tracking-[var(--nv-tracking-wide)] text-[var(--nv-text-tertiary)]">{label}</div>
          <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-1.5">
            <span className="min-w-0 truncate text-xs font-semibold text-[var(--nv-text-primary)]">{value}</span>
            {tags}
          </div>
          <div className="mt-1 truncate text-[10px] text-[var(--nv-text-tertiary)]">{detail}</div>
        </div>
      </div>
    </div>
  )
}

function PanelSurface({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <Surface className="p-4 sm:p-5">
      <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
        <span className="text-[var(--nv-action-primary)]" aria-hidden="true">{icon}</span>
        <h4>{title}</h4>
      </div>
      {children}
    </Surface>
  )
}

function OverviewPanel({
  media,
  techSpecs,
  fileInfo,
  library,
  isAdmin,
}: {
  media: Media
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
  library: LibraryInfo | null
  isAdmin: boolean
}) {
  return (
    <div className="space-y-4">
      <PanelSurface icon={<FileText size={15} />} title="文件基本信息">
        {media.file_path && isAdmin && <PathRow label="路径" value={media.file_path} />}
        <InfoGrid>
          <CopyableInfo label="文件大小" value={formatSize(media.file_size)} highlight />
          {fileInfo?.file_ext && <CopyableInfo label="文件格式" value={fileInfo.file_ext.replace('.', '').toUpperCase()} />}
          {media.duration > 0 && <CopyableInfo label="时长" value={formatDuration(media.duration)} highlight />}
          {techSpecs?.format?.duration && <CopyableInfo label="精确时长" value={formatDuration(parseFloat(techSpecs.format.duration))} hint={paramHints.精确时长} />}
          {techSpecs?.format?.bit_rate && <CopyableInfo label="总码率" value={formatBitRate(techSpecs.format.bit_rate)} hint={paramHints.总码率} />}
          {fileInfo?.mime_type && <CopyableInfo label="MIME 类型" value={fileInfo.mime_type} hint={paramHints['MIME 类型']} />}
          <CopyableInfo label="创建时间" value={fileInfo?.created_at ? formatDate(fileInfo.created_at) : formatDate(media.created_at)} />
          {fileInfo?.modified_at && <CopyableInfo label="修改时间" value={formatDate(fileInfo.modified_at)} />}
          {fileInfo?.permissions && fileInfo.permissions !== '-' && <CopyableInfo label="权限" value={fileInfo.permissions} icon={<Shield size={11} />} />}
          {fileInfo?.owner && fileInfo.owner !== '-' && <CopyableInfo label="所有者" value={fileInfo.owner} icon={<User size={11} />} />}
        </InfoGrid>
        {fileInfo?.md5 && (
          <div className="mt-4 border-t border-[var(--nv-border-subtle)] pt-4">
            <CopyableInfo label="MD5" value={fileInfo.md5} icon={<Hash size={11} />} mono hint={paramHints.MD5} />
          </div>
        )}
      </PanelSurface>

      {library && isAdmin && (
        <PanelSurface icon={<FolderOpen size={15} />} title="所属媒体库">
          <InfoGrid>
            <CopyableInfo label="名称" value={library.name} />
            <CopyableInfo label="类型" value={{ movie: '电影', tvshow: '电视剧', mixed: '混合', other: '其他' }[library.type] || library.type} />
            {library.path && <CopyableInfo label="路径" value={library.path} mono />}
          </InfoGrid>
        </PanelSurface>
      )}

      {techSpecs?.format?.tags && Object.keys(techSpecs.format.tags).length > 0 && (
        <PanelSurface icon={<Info size={15} />} title="元数据标签">
          <InfoGrid>
            {Object.entries(techSpecs.format.tags).map(([key, value]) => <CopyableInfo key={key} label={key} value={String(value)} />)}
          </InfoGrid>
        </PanelSurface>
      )}
    </div>
  )
}

function VideoPanel({ media, streams }: { media: Media; streams: StreamDetail[] }) {
  if (streams.length === 0) {
    return (
      <EmptyState
        icon={<Monitor size={26} />}
        title="无详细视频流信息"
        description={[media.video_codec, media.resolution].filter(Boolean).join(' · ') || '当前媒体没有可展示的视频流元数据。'}
      />
    )
  }

  return (
    <div className="space-y-3">
      {streams.map((stream) => (
        <PanelSurface key={stream.index} icon={<Monitor size={15} />} title={`视频流 #${stream.index}`}>
          <div className="mb-4 flex flex-wrap items-center gap-2">
            <Tag tone="brand">{formatCodecName(stream.codec_name)}</Tag>
            {stream.width && stream.height && <Tag>{stream.width}×{stream.height}</Tag>}
            {stream.is_default && <Tag tone="brand">默认</Tag>}
            {isHDR(stream) && <Tag tone="rating">{getHDRLabel(stream)}</Tag>}
            {stream.is_interlaced && <Tag tone="danger">隔行扫描</Tag>}
          </div>
          <InfoGrid>
            <CopyableInfo label="编码器" value={formatCodecName(stream.codec_name, stream.codec_long_name)} hint={paramHints.编码器} />
            {stream.profile && <CopyableInfo label="配置" value={stream.profile} hint={paramHints.配置} />}
            {stream.level ? <CopyableInfo label="等级" value={String(stream.level)} hint={paramHints.等级} /> : null}
            <CopyableInfo label="分辨率" value={stream.width && stream.height ? `${stream.width} × ${stream.height}` : '-'} highlight hint={paramHints.分辨率} />
            <CopyableInfo label="帧率" value={stream.frame_rate ? `${parseFloat(stream.frame_rate).toFixed(2)} fps` : '-'} hint={paramHints.帧率} />
            <CopyableInfo label="码率" value={formatBitRate(stream.bit_rate)} hint={paramHints.码率} />
            {stream.bit_depth ? <CopyableInfo label="位深度" value={`${stream.bit_depth} bit`} hint={paramHints.位深度} /> : null}
            <CopyableInfo label="像素格式" value={formatPixFmt(stream.pix_fmt)} hint={paramHints.像素格式} />
            <CopyableInfo label="视频动态范围" value={getHDRLabel(stream)} hint={paramHints.视频动态范围} />
            {stream.aspect_ratio && <CopyableInfo label="宽高比" value={stream.aspect_ratio} hint={paramHints.宽高比} />}
            {stream.color_space && <CopyableInfo label="色彩空间" value={stream.color_space} hint={paramHints.色彩空间} />}
            {stream.color_transfer && <CopyableInfo label="色彩转换" value={stream.color_transfer} hint={paramHints.色彩转换} />}
            {stream.color_primaries && <CopyableInfo label="色彩原色" value={formatColorPrimaries(stream.color_primaries)} hint={paramHints.色彩原色} />}
            {stream.color_range && <CopyableInfo label="色彩范围" value={stream.color_range === 'tv' ? 'TV (Limited)' : stream.color_range === 'pc' ? 'PC (Full)' : stream.color_range} hint={paramHints.色彩范围} />}
            <CopyableInfo label="隔行扫描" value={stream.is_interlaced ? '是' : '否'} hint={paramHints.隔行扫描} />
            {stream.ref_frames ? <CopyableInfo label="参考帧" value={String(stream.ref_frames)} hint={paramHints.参考帧} /> : null}
            {stream.nb_frames && <CopyableInfo label="总帧数" value={stream.nb_frames} hint={paramHints.总帧数} />}
          </InfoGrid>
        </PanelSurface>
      ))}
    </div>
  )
}

function AudioPanel({ media, streams }: { media: Media; streams: StreamDetail[] }) {
  if (streams.length === 0) {
    return (
      <EmptyState
        icon={<Music size={26} />}
        title="无详细音频流信息"
        description={media.audio_codec ? `音频编码：${media.audio_codec}` : '当前媒体没有可展示的音频流元数据。'}
      />
    )
  }

  return (
    <div className="space-y-3">
      {streams.map((stream) => (
        <PanelSurface key={stream.index} icon={<Music size={15} />} title={`音频轨道 #${stream.index}`}>
          <div className="mb-4 flex flex-wrap items-center gap-2">
            <Tag tone="brand">{formatCodecName(stream.codec_name)}</Tag>
            <Tag>{formatChannels(stream.channels, stream.channel_layout)}</Tag>
            {stream.language && <Tag>{formatLanguage(stream.language)}</Tag>}
            {stream.is_default && <Tag tone="brand">默认</Tag>}
            {stream.is_forced && <Tag tone="warning">强制</Tag>}
          </div>
          {stream.title && <p className="mb-4 text-xs text-[var(--nv-text-tertiary)]">{stream.title}</p>}
          <InfoGrid>
            <CopyableInfo label="语言" value={formatLanguage(stream.language)} hint={paramHints.语言} />
            <CopyableInfo label="编码器" value={formatCodecName(stream.codec_name, stream.codec_long_name)} hint={paramHints.编码器} />
            {stream.profile && <CopyableInfo label="配置" value={stream.profile} hint={paramHints.配置} />}
            <CopyableInfo label="布局" value={stream.channel_layout || '-'} hint={paramHints.布局} />
            <CopyableInfo label="声道" value={stream.channels ? `${stream.channels} ch` : '-'} hint={paramHints.声道} />
            <CopyableInfo label="采样率" value={formatSampleRate(stream.sample_rate)} hint={paramHints.采样率} />
            <CopyableInfo label="码率" value={formatBitRate(stream.bit_rate)} hint={paramHints.码率} />
            {stream.bits_per_sample && stream.bits_per_sample > 0 && <CopyableInfo label="位深" value={`${stream.bits_per_sample}-bit`} hint={paramHints.位深} />}
            <CopyableInfo label="默认" value={stream.is_default ? '是' : '否'} />
          </InfoGrid>
        </PanelSurface>
      ))}
    </div>
  )
}

function SubtitlePanel({ streams }: { streams: StreamDetail[] }) {
  if (streams.length === 0) {
    return <EmptyState icon={<Subtitles size={26} />} title="无内嵌字幕" description="当前文件没有检测到可展示的内嵌字幕轨道。" />
  }

  return (
    <PanelSurface icon={<Subtitles size={15} />} title={`字幕轨道 (${streams.length})`}>
      <div className="divide-y divide-[var(--nv-border-subtle)]">
        {streams.map((stream) => (
          <div key={stream.index} className="flex min-w-0 flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-center">
            <div className="flex min-w-0 items-center gap-2">
              <span className="shrink-0 font-mono text-[11px] text-[var(--nv-text-tertiary)]">#{stream.index}</span>
              <span className="font-medium text-xs text-[var(--nv-text-primary)]">{formatCodecName(stream.codec_name)}</span>
              <span className="text-xs text-[var(--nv-text-secondary)]">{formatLanguage(stream.language)}</span>
              {stream.title && <span className="truncate text-xs text-[var(--nv-text-tertiary)]">{stream.title}</span>}
            </div>
            <div className="flex gap-1.5 sm:ml-auto">
              {stream.is_default && <Tag tone="brand">默认</Tag>}
              {stream.is_forced && <Tag tone="warning">强制</Tag>}
            </div>
          </div>
        ))}
      </div>
    </PanelSurface>
  )
}

function FilePanel({
  media,
  techSpecs,
  fileInfo,
  library,
  isAdmin,
}: {
  media: Media
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
  library: LibraryInfo | null
  isAdmin: boolean
}) {
  return (
    <div className="space-y-4">
      <PanelSurface icon={<Layers size={15} />} title="文件详情">
        {media.file_path && isAdmin && <PathRow label="完整路径" value={media.file_path} />}
        <InfoGrid>
          {fileInfo?.file_name && <CopyableInfo label="文件名" value={fileInfo.file_name} />}
          <CopyableInfo label="文件格式" value={fileInfo?.file_ext?.replace('.', '').toUpperCase() || '-'} />
          <CopyableInfo label="文件大小" value={formatSize(media.file_size)} highlight />
          {fileInfo?.mime_type && <CopyableInfo label="MIME 类型" value={fileInfo.mime_type} hint={paramHints['MIME 类型']} />}
          {techSpecs?.format?.duration && <CopyableInfo label="精确时长" value={formatDuration(parseFloat(techSpecs.format.duration))} hint={paramHints.精确时长} />}
          {techSpecs?.format?.bit_rate && <CopyableInfo label="总码率" value={formatBitRate(techSpecs.format.bit_rate)} hint={paramHints.总码率} />}
          <CopyableInfo label="创建时间" value={fileInfo?.created_at ? formatDate(fileInfo.created_at) : formatDate(media.created_at)} />
          {fileInfo?.modified_at && <CopyableInfo label="修改时间" value={formatDate(fileInfo.modified_at)} />}
          {fileInfo?.permissions && fileInfo.permissions !== '-' && <CopyableInfo label="权限" value={fileInfo.permissions} icon={<Shield size={11} />} />}
          {fileInfo?.owner && fileInfo.owner !== '-' && <CopyableInfo label="所有者" value={fileInfo.owner} icon={<User size={11} />} />}
        </InfoGrid>
        {fileInfo?.md5 && (
          <div className="mt-4 border-t border-[var(--nv-border-subtle)] pt-4">
            <CopyableInfo label="MD5" value={fileInfo.md5} icon={<Hash size={11} />} mono hint={paramHints.MD5} />
          </div>
        )}
      </PanelSurface>

      {techSpecs?.format && (
        <PanelSurface icon={<HardDrive size={15} />} title="容器格式">
          <InfoGrid>
            <CopyableInfo label="格式名称" value={formatContainerName(techSpecs.format.format_name)} />
            {techSpecs.format.format_long_name && <CopyableInfo label="完整名称" value={techSpecs.format.format_long_name} />}
            <CopyableInfo label="流数量" value={`${techSpecs.format.stream_count} 个`} />
            {techSpecs.format.size && <CopyableInfo label="容器大小" value={formatSize(parseInt(techSpecs.format.size))} />}
            {techSpecs.format.start_time && <CopyableInfo label="起始时间" value={`${parseFloat(techSpecs.format.start_time).toFixed(3)}s`} />}
          </InfoGrid>
        </PanelSurface>
      )}

      {techSpecs?.format?.tags && Object.keys(techSpecs.format.tags).length > 0 && (
        <PanelSurface icon={<Info size={15} />} title="容器元数据">
          <InfoGrid>
            {Object.entries(techSpecs.format.tags).map(([key, value]) => <CopyableInfo key={key} label={key} value={String(value)} />)}
          </InfoGrid>
        </PanelSurface>
      )}

      {library && isAdmin && (
        <PanelSurface icon={<FolderOpen size={15} />} title="媒体库">
          <InfoGrid>
            <CopyableInfo label="名称" value={library.name} />
            <CopyableInfo label="类型" value={library.type} />
            {library.path && <CopyableInfo label="路径" value={library.path} mono />}
          </InfoGrid>
        </PanelSurface>
      )}
    </div>
  )
}

function StatsPanel({ stats }: { stats: PlaybackStatsInfo }) {
  const metrics = [
    { label: '总播放次数', value: String(stats.total_play_count), icon: <Play size={16} /> },
    { label: '观看人数', value: String(stats.unique_viewers), icon: <Users size={16} /> },
    {
      label: '总观看时长',
      value: stats.total_watch_minutes > 60 ? `${(stats.total_watch_minutes / 60).toFixed(1)}h` : `${stats.total_watch_minutes.toFixed(0)}m`,
      icon: <Clock size={16} />,
    },
    ...(stats.last_played_at
      ? [{ label: '最后播放', value: formatDate(stats.last_played_at), icon: <BarChart3 size={16} /> }]
      : []),
  ]

  return (
    <PanelSurface icon={<BarChart3 size={15} />} title="播放统计">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {metrics.map((metric) => (
          <div key={metric.label} className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
            <div className="flex items-center gap-2 text-[var(--nv-action-primary)]">
              {metric.icon}
              <span className="text-lg font-semibold text-[var(--nv-text-primary)]">{metric.value}</span>
            </div>
            <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{metric.label}</div>
          </div>
        ))}
      </div>
    </PanelSurface>
  )
}

function InfoGrid({ children }: { children: ReactNode }) {
  return <div className="grid gap-x-6 gap-y-3 text-xs sm:grid-cols-2 lg:grid-cols-3">{children}</div>
}

function PathRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center">
      <span className="shrink-0 text-xs font-medium text-[var(--nv-text-tertiary)]">{label}</span>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <code className="min-w-0 flex-1 truncate rounded-[var(--nv-radius-sm)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] px-3 py-2 text-xs text-[var(--nv-text-secondary)]">
          {value}
        </code>
        <CopyButton value={value} />
      </div>
    </div>
  )
}

function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(value).then(() => {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    }).catch(() => {})
  }, [value])

  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      iconOnly
      onClick={handleCopy}
      title={copied ? '已复制' : '复制'}
      aria-label={copied ? '已复制' : '复制'}
      className={copied ? 'text-[var(--nv-status-success)]' : undefined}
    >
      {copied ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />}
    </Button>
  )
}

function CopyableInfo({
  label,
  value,
  highlight = false,
  icon,
  mono = false,
  hint,
}: {
  label: string
  value: string
  highlight?: boolean
  icon?: ReactNode
  mono?: boolean
  hint?: string
}) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(value).then(() => {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    }).catch(() => {})
  }, [value])

  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center gap-1 text-[11px] text-[var(--nv-text-tertiary)]">
        {icon}
        <span>{label}</span>
        {hint && (
          <span title={hint} className="inline-flex cursor-help" aria-label={hint}>
            <HelpCircle size={11} className="opacity-60" aria-hidden="true" />
          </span>
        )}
      </div>
      <button
        type="button"
        onClick={handleCopy}
        title="点击复制"
        className={`block max-w-full text-left transition-colors hover:text-[var(--nv-action-primary)] ${mono ? 'break-all font-mono text-[11px]' : 'truncate text-xs'} ${highlight ? 'font-semibold' : 'font-medium'}`}
        style={{ color: copied ? 'var(--nv-status-success)' : highlight ? 'var(--nv-text-primary)' : 'var(--nv-text-secondary)' }}
      >
        {copied ? '✓ 已复制' : value}
      </button>
    </div>
  )
}
