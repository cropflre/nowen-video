import { useState, useCallback } from 'react'
import { formatSize, formatDuration, formatDate } from '@/utils/format'
import type { Media, TechSpecs, FileDetail, LibraryInfo, PlaybackStatsInfo, StreamDetail } from '@/types'
import {
  Monitor,
  Music,
  Subtitles,
  HardDrive,
  ChevronDown,
  ChevronUp,
  Cpu,
  Layers,
  BarChart3,
  Users,
  Clock,
  Play,
  FolderOpen,
  Download,
  FileJson,
  FileCode,
  Shield,
  User,
  Hash,
  Info,
  Copy,
  Check,
  FileText,
  HelpCircle,
} from 'lucide-react'

// ==================== 工具函数 ====================

/** 格式化码率为可读格式 */
function formatBitRate(bitRate?: string): string {
  if (!bitRate) return '-'
  const num = parseInt(bitRate)
  if (isNaN(num)) return bitRate
  if (num >= 1000000) return `${(num / 1000000).toFixed(2)} Mbps`
  if (num >= 1000) return `${(num / 1000).toFixed(0)} Kbps`
  return `${num} bps`
}

/** 格式化采样率 */
function formatSampleRate(rate?: string): string {
  if (!rate) return '-'
  const num = parseInt(rate)
  if (isNaN(num)) return rate
  return `${num} Hz`
}

/** 格式化声道数 */
function formatChannels(channels?: number, layout?: string): string {
  if (layout) {
    const layoutMap: Record<string, string> = {
      'mono': '单声道',
      'stereo': '立体声',
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

/** 格式化编码器名称 */
function formatCodecName(name: string, longName?: string): string {
  const codecMap: Record<string, string> = {
    'h264': 'H.264 / AVC',
    'hevc': 'H.265 / HEVC',
    'h265': 'H.265 / HEVC',
    'vp9': 'VP9',
    'av1': 'AV1',
    'mpeg4': 'MPEG-4',
    'aac': 'AAC',
    'ac3': 'AC-3 / Dolby Digital',
    'eac3': 'E-AC-3 / Dolby Digital Plus',
    'dts': 'DTS',
    'flac': 'FLAC',
    'opus': 'Opus',
    'vorbis': 'Vorbis',
    'mp3': 'MP3',
    'truehd': 'Dolby TrueHD',
    'pcm_s16le': 'PCM 16-bit',
    'pcm_s24le': 'PCM 24-bit',
    'srt': 'SRT',
    'ass': 'ASS/SSA',
    'subrip': 'SRT',
    'hdmv_pgs_subtitle': 'PGS (蓝光)',
    'dvd_subtitle': 'VobSub',
    'webvtt': 'WebVTT',
    'mov_text': 'MOV Text',
  }
  return codecMap[name] || longName || name.toUpperCase()
}

/** 格式化容器格式名称 */
function formatContainerName(name: string): string {
  const containerMap: Record<string, string> = {
    'matroska,webm': 'Matroska (MKV)',
    'mov,mp4,m4a,3gp,3g2,mj2': 'MP4 / MOV',
    'avi': 'AVI',
    'mpegts': 'MPEG-TS',
    'flv': 'FLV',
    'ogg': 'OGG',
    'webm': 'WebM',
  }
  return containerMap[name] || name
}

/** 格式化语言代码 */
function formatLanguage(lang?: string): string {
  if (!lang || lang === 'und') return '未知'
  const langMap: Record<string, string> = {
    'chi': '中文', 'zho': '中文', 'zh': '中文',
    'eng': '英语', 'en': '英语',
    'jpn': '日语', 'ja': '日语',
    'kor': '韩语', 'ko': '韩语',
    'fre': '法语', 'fra': '法语', 'fr': '法语',
    'ger': '德语', 'deu': '德语', 'de': '德语',
    'spa': '西班牙语', 'es': '西班牙语',
    'ita': '意大利语', 'it': '意大利语',
    'por': '葡萄牙语', 'pt': '葡萄牙语',
    'rus': '俄语', 'ru': '俄语',
    'tha': '泰语', 'th': '泰语',
    'vie': '越南语', 'vi': '越南语',
    'ara': '阿拉伯语', 'ar': '阿拉伯语',
  }
  return langMap[lang] || lang
}

/** 格式化像素格式 */
function formatPixFmt(fmt?: string): string {
  if (!fmt) return '-'
  const fmtMap: Record<string, string> = {
    'yuv420p': 'YUV 4:2:0 8-bit',
    'yuv420p10le': 'YUV 4:2:0 10-bit',
    'yuv420p10be': 'YUV 4:2:0 10-bit',
    'yuv422p': 'YUV 4:2:2 8-bit',
    'yuv444p': 'YUV 4:4:4 8-bit',
    'yuv444p10le': 'YUV 4:4:4 10-bit',
    'rgb24': 'RGB 24-bit',
    'nv12': 'NV12',
  }
  return fmtMap[fmt] || fmt
}

/** 判断是否为HDR */
function isHDR(stream: StreamDetail): boolean {
  const hdrTransfers = ['smpte2084', 'arib-std-b67', 'smpte428']
  const hdrSpaces = ['bt2020nc', 'bt2020c']
  return (
    (stream.color_transfer ? hdrTransfers.includes(stream.color_transfer) : false) ||
    (stream.color_space ? hdrSpaces.includes(stream.color_space) : false) ||
    (stream.pix_fmt?.includes('10') ?? false)
  )
}

/** 获取HDR类型标签 */
function getHDRLabel(stream: StreamDetail): string {
  if (stream.color_transfer === 'smpte2084') return 'HDR10'
  if (stream.color_transfer === 'arib-std-b67') return 'HLG'
  if (stream.color_space === 'bt2020nc' || stream.color_space === 'bt2020c') return 'HDR'
  return 'SDR'
}

/** 格式化色彩原色 */
function formatColorPrimaries(primaries?: string): string {
  if (!primaries) return '--'
  const map: Record<string, string> = {
    'bt709': 'BT.709',
    'bt2020': 'BT.2020',
    'smpte170m': 'SMPTE 170M',
    'smpte240m': 'SMPTE 240M',
    'bt470bg': 'BT.470 BG',
  }
  return map[primaries] || primaries
}

/** 参数说明映射 */
const paramHints: Record<string, string> = {
  '编码器': '视频/音频数据的压缩编码格式',
  '配置': '编码器使用的预设配置档次',
  '等级': '编码器的复杂度等级',
  '分辨率': '视频画面的像素宽度×高度',
  '帧率': '每秒显示的画面帧数',
  '码率': '每秒传输的数据量，越高画质越好',
  '位深度': '每个像素的色彩精度，10-bit 色彩更丰富',
  '像素格式': '像素数据的存储格式和色度采样方式',
  '视频动态范围': 'SDR 为标准动态范围，HDR 提供更高亮度和对比度',
  '宽高比': '画面的宽度与高度之比',
  '色彩空间': '定义颜色的数学模型',
  '色彩转换': '亮度信号的传输特性曲线',
  '色彩原色': '定义红绿蓝三原色的色度坐标',
  '色彩范围': 'TV 为有限范围(16-235)，PC 为完整范围(0-255)',
  '隔行扫描': '是否使用隔行扫描（交错显示奇偶行）',
  '参考帧': '编码时参考的前后帧数量',
  '总帧数': '视频流中的总帧数',
  '语言': '音频/字幕轨道的语言',
  '布局': '音频声道的空间布局方式',
  '声道': '音频的声道数量',
  '采样率': '每秒采集的音频样本数，越高音质越好',
  '位深': '每个音频样本的精度',
  'MIME 类型': '文件的互联网媒体类型标识',
  'MD5': '文件内容的 MD5 哈希校验值',
  '精确时长': '基于容器元数据的精确播放时长',
  '总码率': '所有流（视频+音频+字幕）的总数据传输速率',
}

// ==================== 标签页定义 ====================
type TabKey = 'overview' | 'video' | 'audio' | 'file' | 'stats'

interface TabDef {
  key: TabKey
  label: string
  icon: React.ReactNode
  badge?: string | number
}

// ==================== 组件 Props ====================
interface MediaTechSpecsProps {
  media: Media
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
  library: LibraryInfo | null
  playbackStats: PlaybackStatsInfo | null
  loading: boolean
  isAdmin: boolean
}

export default function MediaTechSpecs({ media, techSpecs, fileInfo, library, playbackStats, loading, isAdmin }: MediaTechSpecsProps) {
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const [expanded, setExpanded] = useState(false)

  /** 导出技术规格为JSON */
  const exportJSON = useCallback(() => {
    const data = { techSpecs, fileInfo, library, playbackStats }
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `tech-specs-${fileInfo?.file_name || media.title || 'media'}.json`
    a.click()
    URL.revokeObjectURL(url)
  }, [techSpecs, fileInfo, library, playbackStats, media.title])

  /** 导出技术规格为XML */
  const exportXML = useCallback(() => {
    const toXML = (obj: any, rootName: string): string => {
      const escape = (s: string) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      const convert = (o: any, name: string, indent: string): string => {
        if (o === null || o === undefined) return `${indent}<${name}/>\n`
        if (typeof o !== 'object') return `${indent}<${name}>${escape(String(o))}</${name}>\n`
        if (Array.isArray(o)) return o.map((item) => convert(item, name.replace(/s$/, ''), indent)).join('')
        let xml = `${indent}<${name}>\n`
        for (const [k, v] of Object.entries(o)) {
          xml += convert(v, k, indent + '  ')
        }
        xml += `${indent}</${name}>\n`
        return xml
      }
      return `<?xml version="1.0" encoding="UTF-8"?>\n${convert(obj, rootName, '')}`
    }
    const data = { techSpecs, fileInfo, library, playbackStats }
    const xml = toXML(data, 'MediaTechSpecs')
    const blob = new Blob([xml], { type: 'application/xml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `tech-specs-${fileInfo?.file_name || media.title || 'media'}.xml`
    a.click()
    URL.revokeObjectURL(url)
  }, [techSpecs, fileInfo, library, playbackStats, media.title])

  if (loading) {
    return (
      <section>
        <div className="flex items-center gap-2 mb-3">
          <div className="skeleton h-5 w-32 rounded-lg" />
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {[1, 2, 3, 4].map(i => (
            <div key={i} className="skeleton h-24 rounded-xl" />
          ))}
        </div>
      </section>
    )
  }

  const videoStreams = techSpecs?.streams?.filter(s => s.codec_type === 'video') || []
  const audioStreams = techSpecs?.streams?.filter(s => s.codec_type === 'audio') || []
  const subtitleStreams = techSpecs?.streams?.filter(s => s.codec_type === 'subtitle') || []
  const mainVideo = videoStreams[0]
  const mainAudio = audioStreams[0]

  // 构建标签页列表
  const tabs: TabDef[] = [
    { key: 'overview', label: '概览', icon: <Cpu size={14} /> },
    { key: 'video', label: '视频', icon: <Monitor size={14} />, badge: videoStreams.length > 0 ? undefined : undefined },
    { key: 'audio', label: '音频', icon: <Music size={14} />, badge: audioStreams.length > 1 ? `${audioStreams.length}轨` : undefined },
    { key: 'file', label: '文件', icon: <Layers size={14} /> },
  ]
  // 仅管理员可见播放统计标签
  if (isAdmin && playbackStats && (playbackStats.total_play_count > 0 || playbackStats.unique_viewers > 0)) {
    tabs.push({ key: 'stats', label: '统计', icon: <BarChart3 size={14} /> })
  }

  return (
    <section>
      {/* ==================== 标题栏 ==================== */}
      <div className="mb-4 flex items-center justify-between">
        <h3 className="flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Cpu size={16} className="text-neon/60" />
          文件信息与技术规格
        </h3>
        <div className="flex items-center gap-2">
          {/* 导出按钮 */}
          {isAdmin && (
            <div className="flex items-center gap-1">
              <button
                onClick={exportJSON}
                className="flex items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-medium transition-colors hover:text-neon"
                style={{ color: 'var(--text-muted)', background: 'var(--nav-hover-bg)' }}
                title="导出为 JSON"
              >
                <FileJson size={12} /> JSON
              </button>
              <button
                onClick={exportXML}
                className="flex items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-medium transition-colors hover:text-neon"
                style={{ color: 'var(--text-muted)', background: 'var(--nav-hover-bg)' }}
                title="导出为 XML"
              >
                <FileCode size={12} /> XML
              </button>
            </div>
          )}
        </div>
      </div>

      {/* ==================== 四宫格概览卡片（始终可见） ==================== */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {/* 视频概览 */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="mb-2 flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: 'var(--neon-blue-8)' }}>
              <Monitor size={14} className="text-neon" />
            </div>
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>视频</span>
            {mainVideo && isHDR(mainVideo) && (
              <span className="rounded px-1.5 py-0.5 text-[10px] font-bold" style={{ background: 'rgba(234, 179, 8, 0.15)', color: '#FBBF24' }}>{getHDRLabel(mainVideo)}</span>
            )}
            {mainVideo?.is_interlaced && (
              <span className="rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(239,68,68,0.1)', color: '#ef4444' }}>隔行</span>
            )}
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              {mainVideo ? `${mainVideo.width && mainVideo.height ? `${mainVideo.height}p` : ''} ${formatCodecName(mainVideo.codec_name)} ${getHDRLabel(mainVideo)}` : (media.resolution || '-')}
            </p>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {mainVideo ? [
                mainVideo.width && mainVideo.height ? `${mainVideo.width}×${mainVideo.height}` : null,
                mainVideo.frame_rate ? `${parseFloat(mainVideo.frame_rate).toFixed(2)} fps` : null,
                mainVideo.bit_rate ? formatBitRate(mainVideo.bit_rate) : null,
              ].filter(Boolean).join(' · ') : (media.video_codec || '无视频流')}
            </p>
          </div>
        </div>

        {/* 音频概览 */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="mb-2 flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: 'var(--neon-purple-8)' }}>
              <Music size={14} className="text-purple-400" />
            </div>
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>音频</span>
            {audioStreams.length > 1 && (
              <span className="rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'var(--neon-purple-8)', color: 'var(--text-secondary)' }}>
                {audioStreams.length} 轨
              </span>
            )}
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              {mainAudio ? `${formatCodecName(mainAudio.codec_name)} ${formatChannels(mainAudio.channels, mainAudio.channel_layout)}` : (media.audio_codec || '-')}
            </p>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {mainAudio ? [
                mainAudio.sample_rate ? formatSampleRate(mainAudio.sample_rate) : null,
                mainAudio.bit_rate ? formatBitRate(mainAudio.bit_rate) : null,
                mainAudio.language ? formatLanguage(mainAudio.language) : null,
              ].filter(Boolean).join(' · ') : '无音频流'}
            </p>
          </div>
        </div>

        {/* 字幕概览 */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="mb-2 flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: 'rgba(0, 255, 136, 0.08)' }}>
              <Subtitles size={14} style={{ color: 'var(--neon-green)' }} />
            </div>
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>字幕</span>
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              {subtitleStreams.length > 0 ? `${subtitleStreams.length} 条内嵌字幕` : '无内嵌字幕'}
            </p>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {subtitleStreams.length > 0
                ? subtitleStreams.map(s => formatLanguage(s.language)).filter((v, i, a) => a.indexOf(v) === i).join(' / ')
                : '-'
              }
            </p>
          </div>
        </div>

        {/* 容器/文件概览 */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="mb-2 flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: 'rgba(255, 165, 0, 0.08)' }}>
              <HardDrive size={14} style={{ color: '#FFA500' }} />
            </div>
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>容器</span>
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              {techSpecs?.format ? formatContainerName(techSpecs.format.format_name) : fileInfo?.file_ext?.toUpperCase() || '-'}
            </p>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {techSpecs?.format ? [
                techSpecs.format.bit_rate ? `总码率 ${formatBitRate(techSpecs.format.bit_rate)}` : null,
                techSpecs.format.stream_count ? `${techSpecs.format.stream_count} 个流` : null,
              ].filter(Boolean).join(' · ') : formatSize(media.file_size)}
            </p>
          </div>
        </div>
      </div>

      {/* ==================== 展开/收起按钮 ==================== */}
      <div className="mt-3 flex justify-center">
        <button
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-1.5 rounded-full px-4 py-1.5 text-xs font-medium transition-all hover:text-neon"
          style={{ color: 'var(--text-muted)', background: 'var(--nav-hover-bg)' }}
        >
          {expanded ? <><ChevronUp size={14} />收起详细信息</> : <><ChevronDown size={14} />展开详细信息</>}
        </button>
      </div>

      {/* ==================== 展开的详细信息（标签页形式） ==================== */}
      {expanded && (
        <div className="mt-4 animate-fade-in">
          {/* 标签页导航 */}
          <div className="mb-4 flex items-center gap-1 overflow-x-auto rounded-xl p-1" style={{ background: 'var(--bg-subtle)' }}>
            {tabs.map(tab => (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className="flex items-center gap-1.5 whitespace-nowrap rounded-lg px-3.5 py-2 text-xs font-medium transition-all"
                style={{
                  background: activeTab === tab.key ? 'var(--bg-elevated)' : 'transparent',
                  color: activeTab === tab.key ? 'var(--neon-blue)' : 'var(--text-muted)',
                  boxShadow: activeTab === tab.key ? '0 1px 3px rgba(0,0,0,0.1)' : 'none',
                }}
              >
                {tab.icon}
                {tab.label}
                {tab.badge && (
                  <span className="rounded px-1.5 py-0.5 text-[10px]" style={{ background: 'var(--neon-purple-8)', color: 'var(--text-secondary)' }}>
                    {tab.badge}
                  </span>
                )}
              </button>
            ))}
          </div>

          {/* ==================== 概览标签页 ==================== */}
          {activeTab === 'overview' && (
            <div className="space-y-4">
              {/* 文件基本信息 — 整合路径、大小、时长等 */}
              <div className="glass-panel rounded-xl p-4">
                <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  <FileText size={14} className="text-neon/60" />
                  文件基本信息
                </h4>

                {/* 文件路径（突出显示） */}
                {media.file_path && isAdmin && (
                  <div className="mb-3 flex items-start gap-3">
                    <span className="shrink-0 text-xs font-medium mt-1.5" style={{ color: 'var(--text-muted)' }}>路径</span>
                    <div className="flex min-w-0 flex-1 items-center gap-2">
                      <code className="flex-1 truncate rounded-lg px-3 py-1.5 text-xs"
                        style={{ background: 'var(--bg-subtle)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                      >
                        {media.file_path}
                      </code>
                      <CopyButton value={media.file_path} />
                    </div>
                  </div>
                )}

                {/* 关键信息网格 */}
                <div className="grid grid-cols-2 gap-x-6 gap-y-2.5 text-xs sm:grid-cols-3 lg:grid-cols-4">
                  <InfoItemWithCopy label="文件大小" value={formatSize(media.file_size)} highlight />
                  {fileInfo?.file_ext && <InfoItemWithCopy label="文件格式" value={fileInfo.file_ext.replace('.', '').toUpperCase()} />}
                  {media.duration > 0 && <InfoItemWithCopy label="时长" value={formatDuration(media.duration)} highlight />}
                  {techSpecs?.format?.duration && (
                    <InfoItemWithCopy label="精确时长" value={formatDuration(parseFloat(techSpecs.format.duration))} hint={paramHints['精确时长']} />
                  )}
                  {techSpecs?.format?.bit_rate && (
                    <InfoItemWithCopy label="总码率" value={formatBitRate(techSpecs.format.bit_rate)} hint={paramHints['总码率']} />
                  )}
                  {fileInfo?.mime_type && <InfoItemWithCopy label="MIME 类型" value={fileInfo.mime_type} hint={paramHints['MIME 类型']} />}
                  <InfoItemWithCopy label="创建时间" value={fileInfo?.created_at ? formatDate(fileInfo.created_at) : formatDate(media.created_at)} />
                  {fileInfo?.modified_at && <InfoItemWithCopy label="修改时间" value={formatDate(fileInfo.modified_at)} />}
                  {fileInfo?.permissions && fileInfo.permissions !== '-' && (
                    <InfoItemWithCopy label="权限" value={fileInfo.permissions} icon={<Shield size={10} />} />
                  )}
                  {fileInfo?.owner && fileInfo.owner !== '-' && (
                    <InfoItemWithCopy label="所有者" value={fileInfo.owner} icon={<User size={10} />} />
                  )}
                </div>

                {/* 哈希值（单独一行，突出显示） */}
                {fileInfo?.md5 && (
                  <div className="mt-3 pt-3" style={{ borderTop: '1px solid var(--border-default)' }}>
                    <InfoItemWithCopy label="MD5" value={fileInfo.md5} icon={<Hash size={10} />} mono hint={paramHints['MD5']} />
                  </div>
                )}
              </div>

              {/* 媒体库信息 */}
              {library && isAdmin && (
                <div className="glass-panel rounded-xl p-4">
                  <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                    <FolderOpen size={14} className="text-neon/60" />
                    所属媒体库
                  </h4>
                  <div className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
                    <div>
                      <span style={{ color: 'var(--text-muted)' }}>名称：</span>
                      <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{library.name}</span>
                    </div>
                    <div>
                      <span style={{ color: 'var(--text-muted)' }}>类型：</span>
                      <span className="font-medium" style={{ color: 'var(--text-primary)' }}>
                        {{ movie: '电影', tvshow: '电视剧', mixed: '混合', other: '其他' }[library.type] || library.type}
                      </span>
                    </div>
                  </div>
                </div>
              )}

              {/* 容器元数据标签 */}
              {techSpecs?.format?.tags && Object.keys(techSpecs.format.tags).length > 0 && (
                <div className="glass-panel rounded-xl p-4">
                  <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                    <Info size={14} className="text-neon/60" />
                    元数据标签
                  </h4>
                  <div className="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-2 lg:grid-cols-3">
                    {Object.entries(techSpecs.format.tags).map(([key, value]) => (
                      <InfoItemWithCopy key={key} label={key} value={String(value)} />
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* ==================== 视频标签页 ==================== */}
          {activeTab === 'video' && (
            <div className="space-y-4">
              {videoStreams.length > 0 ? videoStreams.map((stream, idx) => (
                <div key={idx} className="glass-panel rounded-xl p-4">
                  {/* 视频流标题行 */}
                  <div className="mb-3 flex items-center gap-2 flex-wrap">
                    <Monitor size={14} className="text-neon/60" />
                    <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      视频流 #{stream.index}
                    </span>
                    <span className="text-xs font-bold" style={{ color: 'var(--text-secondary)' }}>
                      {stream.width && stream.height ? `${stream.height}p` : ''} {formatCodecName(stream.codec_name)} {getHDRLabel(stream)}
                    </span>
                    {stream.is_default && (
                      <span className="rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'var(--neon-blue-8)', color: 'var(--neon-blue)' }}>默认</span>
                    )}
                    {isHDR(stream) && (
                      <span className="rounded px-1.5 py-0.5 text-[10px] font-bold" style={{ background: 'rgba(234, 179, 8, 0.15)', color: '#FBBF24' }}>{getHDRLabel(stream)}</span>
                    )}
                  </div>
                  {/* 详细参数表格 */}
                  <div className="grid gap-x-4 gap-y-2 text-xs sm:grid-cols-2 lg:grid-cols-3">
                    <InfoItemWithCopy label="编码器" value={formatCodecName(stream.codec_name, stream.codec_long_name)} hint={paramHints['编码器']} />
                    {stream.profile && <InfoItemWithCopy label="配置" value={stream.profile} hint={paramHints['配置']} />}
                    {stream.level ? <InfoItemWithCopy label="等级" value={String(stream.level)} hint={paramHints['等级']} /> : null}
                    <InfoItemWithCopy label="分辨率" value={stream.width && stream.height ? `${stream.width} × ${stream.height}` : '-'} highlight hint={paramHints['分辨率']} />
                    <InfoItemWithCopy label="帧率" value={stream.frame_rate ? `${parseFloat(stream.frame_rate).toFixed(2)} fps` : '-'} hint={paramHints['帧率']} />
                    <InfoItemWithCopy label="码率" value={formatBitRate(stream.bit_rate)} hint={paramHints['码率']} />
                    {stream.bit_depth ? <InfoItemWithCopy label="位深度" value={`${stream.bit_depth} bit`} hint={paramHints['位深度']} /> : null}
                    <InfoItemWithCopy label="像素格式" value={formatPixFmt(stream.pix_fmt)} hint={paramHints['像素格式']} />
                    <InfoItemWithCopy label="视频动态范围" value={getHDRLabel(stream)} hint={paramHints['视频动态范围']} />
                    {stream.aspect_ratio && <InfoItemWithCopy label="宽高比" value={stream.aspect_ratio} hint={paramHints['宽高比']} />}
                    {stream.color_space && <InfoItemWithCopy label="色彩空间" value={stream.color_space} hint={paramHints['色彩空间']} />}
                    {stream.color_transfer && <InfoItemWithCopy label="色彩转换" value={stream.color_transfer} hint={paramHints['色彩转换']} />}
                    {stream.color_primaries && <InfoItemWithCopy label="色彩原色" value={formatColorPrimaries(stream.color_primaries)} hint={paramHints['色彩原色']} />}
                    {stream.color_range && <InfoItemWithCopy label="色彩范围" value={stream.color_range === 'tv' ? 'TV (Limited)' : stream.color_range === 'pc' ? 'PC (Full)' : stream.color_range} hint={paramHints['色彩范围']} />}
                    <InfoItemWithCopy label="隔行扫描" value={stream.is_interlaced ? '是' : '否'} hint={paramHints['隔行扫描']} />
                    {stream.ref_frames ? <InfoItemWithCopy label="参考帧" value={String(stream.ref_frames)} hint={paramHints['参考帧']} /> : null}
                    {stream.nb_frames && <InfoItemWithCopy label="总帧数" value={stream.nb_frames} hint={paramHints['总帧数']} />}
                  </div>
                </div>
              )) : (
                <div className="glass-panel rounded-xl p-8 text-center">
                  <Monitor size={32} className="mx-auto mb-2 opacity-30" style={{ color: 'var(--text-muted)' }} />
                  <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
                    {media.video_codec ? `视频编码: ${media.video_codec}` : '无详细视频流信息'}
                  </p>
                  {media.resolution && (
                    <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>分辨率: {media.resolution}</p>
                  )}
                </div>
              )}

              {/* 字幕流信息也放在视频标签页下 */}
              {subtitleStreams.length > 0 && (
                <div className="glass-panel rounded-xl p-4">
                  <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                    <Subtitles size={14} style={{ color: 'var(--neon-green)', opacity: 0.6 }} />
                    字幕轨道 ({subtitleStreams.length})
                  </h4>
                  <div className="space-y-1.5">
                    {subtitleStreams.map((stream, idx) => (
                      <div key={idx} className="flex items-center gap-3 rounded-lg px-3 py-2 text-xs" style={{ background: 'var(--bg-subtle)' }}>
                        <span className="shrink-0 font-mono" style={{ color: 'var(--text-muted)' }}>#{stream.index}</span>
                        <span className="font-medium" style={{ color: 'var(--text-primary)' }}>
                          {formatCodecName(stream.codec_name)}
                        </span>
                        <span style={{ color: 'var(--text-secondary)' }}>
                          {formatLanguage(stream.language)}
                        </span>
                        {stream.title && (
                          <span className="truncate" style={{ color: 'var(--text-muted)' }}>{stream.title}</span>
                        )}
                        <div className="ml-auto flex gap-1.5">
                          {stream.is_default && (
                            <span className="rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'var(--neon-blue-8)', color: 'var(--neon-blue)' }}>默认</span>
                          )}
                          {stream.is_forced && (
                            <span className="rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(255,165,0,0.1)', color: '#FFA500' }}>强制</span>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* ==================== 音频标签页 ==================== */}
          {activeTab === 'audio' && (
            <div className="space-y-4">
              {audioStreams.length > 0 ? audioStreams.map((stream, idx) => (
                <div key={idx} className="glass-panel rounded-xl p-4">
                  <div className="mb-3 flex items-center gap-2 flex-wrap">
                    <Music size={14} className="text-purple-400/60" />
                    <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      音频轨道 #{stream.index}
                    </span>
                    <span className="text-xs font-bold" style={{ color: 'var(--text-secondary)' }}>
                      {formatCodecName(stream.codec_name)} {formatChannels(stream.channels, stream.channel_layout)}
                    </span>
                    {stream.title && (
                      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>— {stream.title}</span>
                    )}
                    {stream.is_default && (
                      <span className="rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'var(--neon-blue-8)', color: 'var(--neon-blue)' }}>默认</span>
                    )}
                    {stream.is_forced && (
                      <span className="rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(255,165,0,0.1)', color: '#FFA500' }}>强制</span>
                    )}
                  </div>
                  <div className="grid gap-x-4 gap-y-2 text-xs sm:grid-cols-2 lg:grid-cols-3">
                    <InfoItemWithCopy label="语言" value={formatLanguage(stream.language)} hint={paramHints['语言']} />
                    <InfoItemWithCopy label="编码器" value={formatCodecName(stream.codec_name, stream.codec_long_name)} hint={paramHints['编码器']} />
                    {stream.profile && <InfoItemWithCopy label="配置" value={stream.profile} hint={paramHints['配置']} />}
                    <InfoItemWithCopy label="布局" value={stream.channel_layout || '-'} hint={paramHints['布局']} />
                    <InfoItemWithCopy label="声道" value={stream.channels ? `${stream.channels} ch` : '-'} hint={paramHints['声道']} />
                    <InfoItemWithCopy label="采样率" value={formatSampleRate(stream.sample_rate)} hint={paramHints['采样率']} />
                    <InfoItemWithCopy label="码率" value={formatBitRate(stream.bit_rate)} hint={paramHints['码率']} />
                    {stream.bits_per_sample && stream.bits_per_sample > 0 && <InfoItemWithCopy label="位深" value={`${stream.bits_per_sample}-bit`} hint={paramHints['位深']} />}
                    <InfoItemWithCopy label="默认" value={stream.is_default ? '是' : '否'} />
                  </div>
                </div>
              )) : (
                <div className="glass-panel rounded-xl p-8 text-center">
                  <Music size={32} className="mx-auto mb-2 opacity-30" style={{ color: 'var(--text-muted)' }} />
                  <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
                    {media.audio_codec ? `音频编码: ${media.audio_codec}` : '无详细音频流信息'}
                  </p>
                </div>
              )}
            </div>
          )}

          {/* ==================== 文件标签页 ==================== */}
          {activeTab === 'file' && (
            <div className="space-y-4">
              {/* 文件详情 */}
              <div className="glass-panel rounded-xl p-4">
                <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  <Layers size={14} className="text-neon/60" />
                  文件详情
                </h4>

                {/* 文件路径（管理员可见） */}
                {media.file_path && isAdmin && (
                  <div className="mb-3 flex items-start gap-3">
                    <span className="shrink-0 text-xs font-medium mt-1.5" style={{ color: 'var(--text-muted)' }}>完整路径</span>
                    <div className="flex min-w-0 flex-1 items-center gap-2">
                      <code className="flex-1 truncate rounded-lg px-3 py-1.5 text-xs"
                        style={{ background: 'var(--bg-subtle)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                      >
                        {media.file_path}
                      </code>
                      <CopyButton value={media.file_path} />
                    </div>
                  </div>
                )}

                <div className="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-2 lg:grid-cols-3">
                  {fileInfo && <InfoItemWithCopy label="文件名" value={fileInfo.file_name} />}
                  <InfoItemWithCopy label="文件格式" value={fileInfo?.file_ext?.replace('.', '').toUpperCase() || '-'} />
                  <InfoItemWithCopy label="文件大小" value={formatSize(media.file_size)} highlight />
                  {fileInfo?.mime_type && <InfoItemWithCopy label="MIME 类型" value={fileInfo.mime_type} hint={paramHints['MIME 类型']} />}
                  {techSpecs?.format?.duration && (
                    <InfoItemWithCopy label="精确时长" value={formatDuration(parseFloat(techSpecs.format.duration))} hint={paramHints['精确时长']} />
                  )}
                  {techSpecs?.format?.bit_rate && (
                    <InfoItemWithCopy label="总码率" value={formatBitRate(techSpecs.format.bit_rate)} hint={paramHints['总码率']} />
                  )}
                  <InfoItemWithCopy label="创建时间" value={fileInfo?.created_at ? formatDate(fileInfo.created_at) : formatDate(media.created_at)} />
                  {fileInfo?.modified_at && <InfoItemWithCopy label="修改时间" value={formatDate(fileInfo.modified_at)} />}
                  {fileInfo?.permissions && fileInfo.permissions !== '-' && (
                    <InfoItemWithCopy label="权限" value={fileInfo.permissions} icon={<Shield size={10} />} />
                  )}
                  {fileInfo?.owner && fileInfo.owner !== '-' && (
                    <InfoItemWithCopy label="所有者" value={fileInfo.owner} icon={<User size={10} />} />
                  )}
                </div>

                {/* 哈希值 */}
                {fileInfo?.md5 && (
                  <div className="mt-3 pt-3" style={{ borderTop: '1px solid var(--border-default)' }}>
                    <div className="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-1">
                      <InfoItemWithCopy label="MD5" value={fileInfo.md5} icon={<Hash size={10} />} mono hint={paramHints['MD5']} />
                    </div>
                  </div>
                )}
              </div>

              {/* 容器格式详情 */}
              {techSpecs?.format && (
                <div className="glass-panel rounded-xl p-4">
                  <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                    <HardDrive size={14} style={{ color: '#FFA500', opacity: 0.6 }} />
                    容器格式
                  </h4>
                  <div className="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-2 lg:grid-cols-3">
                    <InfoItemWithCopy label="格式名称" value={formatContainerName(techSpecs.format.format_name)} />
                    {techSpecs.format.format_long_name && <InfoItemWithCopy label="完整名称" value={techSpecs.format.format_long_name} />}
                    <InfoItemWithCopy label="流数量" value={`${techSpecs.format.stream_count} 个`} />
                    {techSpecs.format.size && <InfoItemWithCopy label="容器大小" value={formatSize(parseInt(techSpecs.format.size))} />}
                    {techSpecs.format.start_time && <InfoItemWithCopy label="起始时间" value={`${parseFloat(techSpecs.format.start_time).toFixed(3)}s`} />}
                  </div>
                </div>
              )}

              {/* 容器元数据标签 */}
              {techSpecs?.format?.tags && Object.keys(techSpecs.format.tags).length > 0 && (
                <div className="glass-panel rounded-xl p-4">
                  <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                    <Info size={14} className="text-neon/60" />
                    元数据标签
                  </h4>
                  <div className="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-2 lg:grid-cols-3">
                    {Object.entries(techSpecs.format.tags).map(([key, value]) => (
                      <InfoItemWithCopy key={key} label={key} value={String(value)} />
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* ==================== 统计标签页 ==================== */}
          {activeTab === 'stats' && playbackStats && (
            <div className="space-y-4">
              <div className="glass-panel rounded-xl p-4">
                <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  <BarChart3 size={14} className="text-neon/60" />
                  播放统计
                </h4>
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                  <div className="text-center">
                    <div className="flex items-center justify-center gap-1 text-lg font-bold" style={{ color: 'var(--neon-blue)' }}>
                      <Play size={16} />
                      {playbackStats.total_play_count}
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-muted)' }}>总播放次数</div>
                  </div>
                  <div className="text-center">
                    <div className="flex items-center justify-center gap-1 text-lg font-bold" style={{ color: 'var(--neon-blue)' }}>
                      <Users size={16} />
                      {playbackStats.unique_viewers}
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-muted)' }}>观看人数</div>
                  </div>
                  <div className="text-center">
                    <div className="flex items-center justify-center gap-1 text-lg font-bold" style={{ color: 'var(--neon-blue)' }}>
                      <Clock size={16} />
                      {playbackStats.total_watch_minutes > 60
                        ? `${(playbackStats.total_watch_minutes / 60).toFixed(1)}h`
                        : `${playbackStats.total_watch_minutes.toFixed(0)}m`
                      }
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-muted)' }}>总观看时长</div>
                  </div>
                  {playbackStats.last_played_at && (
                    <div className="text-center">
                      <div className="text-sm font-bold" style={{ color: 'var(--neon-blue)' }}>
                        {formatDate(playbackStats.last_played_at)}
                      </div>
                      <div className="text-xs" style={{ color: 'var(--text-muted)' }}>最后播放</div>
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </section>
  )
}

// ==================== 子组件 ====================

/** 复制按钮组件 */
function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(value).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }).catch(() => {})
  }, [value])

  return (
    <button
      onClick={handleCopy}
      className="shrink-0 rounded-lg p-1.5 transition-all hover:text-neon hover:bg-neon-blue/5"
      style={{ color: copied ? 'var(--neon-green)' : 'var(--text-muted)' }}
      title={copied ? '已复制' : '复制'}
    >
      {copied ? <Check size={14} /> : <Copy size={14} />}
    </button>
  )
}

/** 带复制和提示功能的信息项组件 */
function InfoItemWithCopy({ label, value, highlight, icon, mono, hint }: {
  label: string
  value: string
  highlight?: boolean
  icon?: React.ReactNode
  mono?: boolean
  hint?: string
}) {
  const [copied, setCopied] = useState(false)
  const [showHint, setShowHint] = useState(false)

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(value).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    }).catch(() => {})
  }, [value])

  return (
    <div className="group flex items-start gap-2 relative">
      <span className="shrink-0 flex items-center gap-1" style={{ color: 'var(--text-muted)' }}>
        {icon}
        {label}
        {hint && (
          <span
            className="relative cursor-help"
            onMouseEnter={() => setShowHint(true)}
            onMouseLeave={() => setShowHint(false)}
          >
            <HelpCircle size={10} className="opacity-40 hover:opacity-80 transition-opacity" />
            {showHint && (
              <span className="absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 w-48 rounded-lg px-2.5 py-1.5 text-[10px] leading-relaxed shadow-lg z-50 pointer-events-none"
                style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
              >
                {hint}
              </span>
            )}
          </span>
        )}
        ：
      </span>
      <span
        className={`${highlight ? 'font-semibold' : 'font-medium'} ${mono ? 'font-mono text-[11px] break-all' : ''} cursor-pointer transition-colors hover:text-neon`}
        style={{ color: copied ? 'var(--neon-green)' : highlight ? '#FBBF24' : 'var(--text-primary)' }}
        onClick={handleCopy}
        title="点击复制"
      >
        {copied ? '✓ 已复制' : value}
      </span>
    </div>
  )
}
