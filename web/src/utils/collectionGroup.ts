import type { CollectionMediaItem } from '@/types'

/**
 * 折叠后的影片条目：代表"同一部电影"（含可选的多个版本）
 */
export interface GroupedMovieItem {
  /** 主版本（展示用的代表条目） */
  primary: CollectionMediaItem
  /** 该电影包含的全部版本（含 primary，按质量从高到低排序） */
  versions: CollectionMediaItem[]
}

/** 归一化标题：去除首尾空白、全角→半角、统一大小写，用于跨版本匹配 */
function normalizeTitle(s: string): string {
  if (!s) return ''
  return s
    .trim()
    .toLowerCase()
    // 全角 → 半角（Unicode 区间 U+FF01..U+FF5E → U+0021..U+007E）
    .replace(/[\uFF01-\uFF5E]/g, (ch) =>
      String.fromCharCode(ch.charCodeAt(0) - 0xFEE0),
    )
    // 去掉常见标点/空白
    .replace(/[\s·・\-_.,:：，。!！?？'"“”‘’()（）\[\]【】]+/g, '')
}

/**
 * 粗略估算一条 media 的"质量分"，用于在版本组内挑选代表项。
 *  - 有海报加分
 *  - 有 TMDB ID 加分
 *  - 分辨率越高加分越多
 *  - 文件越大加分（通常质量更高）
 *  - 有评分加分
 */
function qualityScore(m: CollectionMediaItem): number {
  let score = 0
  if (m.poster_path) score += 10
  if (m.tmdb_id && m.tmdb_id > 0) score += 20
  const res = (m.resolution || '').toLowerCase()
  if (res.includes('2160') || res.includes('4k') || res.includes('uhd')) score += 40
  else if (res.includes('1080')) score += 30
  else if (res.includes('720')) score += 20
  else if (res.includes('480')) score += 10
  if (m.file_size) score += Math.min(30, Math.floor(m.file_size / (1024 * 1024 * 1024))) // 每 GB 加 1，最多 30
  if (m.rating && m.rating > 0) score += 5
  return score
}

/**
 * 将合集的电影列表按"同片多版本"折叠，返回每部电影一个条目。
 *
 * 折叠判定优先级：
 *   1. 相同 `version_group`（后端已识别并写入）
 *   2. 相同 `tmdb_id`（非 0）
 *   3. 归一化后的 title + year 完全相同（年份任一为 0 时，仅比较 title）
 */
export function groupByMovie(items: CollectionMediaItem[]): GroupedMovieItem[] {
  if (!items || items.length === 0) return []

  // 用统一的 key 函数，按优先级返回第一个可用的 key
  const buckets = new Map<string, CollectionMediaItem[]>()
  // 第二轮（标题+年份）用的 key → bucket key 映射，确保 bucket 合并
  const titleKeyToBucket = new Map<string, string>()
  const tmdbKeyToBucket = new Map<string, string>()
  const versionGroupToBucket = new Map<string, string>()

  let autoKey = 0
  for (const item of items) {
    // 1) version_group
    if (item.version_group) {
      const existing = versionGroupToBucket.get(item.version_group)
      if (existing) {
        buckets.get(existing)!.push(item)
        continue
      }
    }
    // 2) tmdb_id
    if (item.tmdb_id && item.tmdb_id > 0) {
      const tk = `tmdb:${item.tmdb_id}`
      const existing = tmdbKeyToBucket.get(tk)
      if (existing) {
        buckets.get(existing)!.push(item)
        continue
      }
    }
    // 3) normalized title + year
    const nt = normalizeTitle(item.title)
    let titleKey: string | null = null
    if (nt) {
      if (item.year > 0) {
        titleKey = `title:${nt}|${item.year}`
        const existing = titleKeyToBucket.get(titleKey) || titleKeyToBucket.get(`title:${nt}|0`)
        if (existing) {
          buckets.get(existing)!.push(item)
          continue
        }
      } else {
        titleKey = `title:${nt}|0`
        const existing = titleKeyToBucket.get(titleKey)
        if (existing) {
          buckets.get(existing)!.push(item)
          continue
        }
      }
    }

    // 都未命中 → 新建 bucket
    const bucketKey = `g${++autoKey}`
    buckets.set(bucketKey, [item])
    if (item.version_group) versionGroupToBucket.set(item.version_group, bucketKey)
    if (item.tmdb_id && item.tmdb_id > 0) tmdbKeyToBucket.set(`tmdb:${item.tmdb_id}`, bucketKey)
    if (titleKey) titleKeyToBucket.set(titleKey, bucketKey)
  }

  // 输出：每个 bucket 选一个代表，并按质量分对 versions 排序
  const result: GroupedMovieItem[] = []
  for (const versions of buckets.values()) {
    // 对版本按质量从高到低排序
    const sorted = [...versions].sort((a, b) => qualityScore(b) - qualityScore(a))
    // 优先把"当前正在查看"的那条放到第 0 位作为代表，否则用质量最高的
    const currentIdx = sorted.findIndex((v) => v.is_current)
    const primary = currentIdx >= 0 ? sorted[currentIdx] : sorted[0]
    result.push({ primary, versions: sorted })
  }
  return result
}

/** 人类友好的文件大小 */
export function formatFileSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return ''
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return `${gb.toFixed(1)}GB`
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(0)}MB`
}

/** 生成单个版本的简洁标签（"2160p · HEVC · 8.3GB"） */
export function versionLabel(m: CollectionMediaItem): string {
  const parts: string[] = []
  if (m.resolution) parts.push(m.resolution)
  if (m.video_codec) parts.push(m.video_codec.toUpperCase())
  const size = formatFileSize(m.file_size)
  if (size) parts.push(size)
  if (m.version_tag) parts.push(m.version_tag)
  return parts.join(' · ')
}
