/**
 * 公共格式化工具函数
 * 统一管理整个项目中的格式化逻辑，避免多处重复定义
 */

/** 格式化文件大小（字节 → GB/MB） */
export function formatSize(bytes: number): string {
  if (!bytes) return '-'
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return `${gb.toFixed(2)} GB`
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(0)} MB`
}

/** 格式化时长（秒 → "X 小时 Y 分钟"） */
export function formatDuration(seconds: number): string {
  if (!seconds) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return `${h} 小时 ${m} 分钟`
  return `${m} 分钟`
}

/**
 * 详情页展示用时长（秒）。
 * 文件实际时长（duration，探针秒数）优先；缺失时用刮削片长（runtime，分钟）换算；
 * 两者都没有返回 0，不要编造时长。
 */
export function getDisplayDurationSeconds(durationSeconds: number, runtimeMinutes: number): number {
  if (durationSeconds > 0) return durationSeconds
  if (runtimeMinutes > 0) return runtimeMinutes * 60
  return 0
}

/** 格式化时长（秒 → "Xh Ym" 短格式） */
export function formatDurationShort(seconds: number): string {
  if (!seconds) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return `${h}h${m}m`
  return `${m}min`
}

/** 格式化日期字符串为中文本地化日期 */
export function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return d.toLocaleDateString('zh-CN')
}

/** 格式化观看进度百分比 */
export function formatProgress(position: number, duration: number): number {
  if (!duration) return 0
  return Math.round((position / duration) * 100)
}

/** 格式化时间为 HH:MM:SS 或 MM:SS */
export function formatTime(seconds: number): string {
  if (!seconds || isNaN(seconds)) return '0:00'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  return `${m}:${s.toString().padStart(2, '0')}`
}
