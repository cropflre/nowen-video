/**
 * 家庭内容分级：前端展示用的中文年龄档。
 *
 * 后端字段值只能是 '' | 'G' | 'PG' | 'PG-13' | 'R' | 'NC-17'
 * （见 types/ContentRating 与 adminApi.setFamilyRating），这里不改后端枚举，
 * 只做展示层映射：
 *   ''     -> 未确认
 *   G      -> 全年龄
 *   PG     -> 7+
 *   PG-13  -> 13+
 *   R      -> 16+
 *   NC-17  -> 成人
 */
export type FamilyRatingLevel = '' | 'G' | 'PG' | 'PG-13' | 'R' | 'NC-17'

export const FAMILY_RATING_LEVELS: Exclude<FamilyRatingLevel, ''>[] = ['G', 'PG', 'PG-13', 'R', 'NC-17']

/** 中文年龄档文案（i18n 键，见 locales 的 family.ratingAge*）。 */
export const FAMILY_RATING_AGE_KEY: Record<Exclude<FamilyRatingLevel, ''>, string> = {
  G: 'family.ratingAgeG',
  PG: 'family.ratingAgePG',
  'PG-13': 'family.ratingAgePG13',
  R: 'family.ratingAgeR',
  'NC-17': 'family.ratingAgeNC17',
}

/** 每档一句话说明（i18n 键，见 locales 的 family.ratingDesc*）。 */
export const FAMILY_RATING_DESC_KEY: Record<Exclude<FamilyRatingLevel, ''>, string> = {
  G: 'family.ratingDescG',
  PG: 'family.ratingDescPG',
  'PG-13': 'family.ratingDescPG13',
  R: 'family.ratingDescR',
  'NC-17': 'family.ratingDescNC17',
}

/** 分级严厉程度排序，用于比较“某内容是否超出儿童上限”。 */
const FAMILY_RATING_RANK: Record<FamilyRatingLevel, number> = {
  '': -1,
  G: 0,
  PG: 1,
  'PG-13': 2,
  R: 3,
  'NC-17': 4,
}

export function familyRatingRank(level: string | undefined | null): number {
  if (!level) return -1
  return FAMILY_RATING_RANK[level as FamilyRatingLevel] ?? -1
}

/**
 * 把刮削得到的 MPAA / TV 分级字符串归一化到家庭分级。
 * 未知或空值返回 ''，表示“无法从元数据判断，以服务端家长确认为准”。
 */
export function mpaaToRatingLevel(mpaa: string | undefined | null): FamilyRatingLevel {
  if (!mpaa) return ''
  const text = mpaa.toUpperCase().replace(/[\s_-]/g, '')
  if (text.includes('NC17')) return 'NC-17'
  if (text.includes('PG13') || text.includes('TV14')) return 'PG-13'
  if (text.includes('TVMA') || text === 'R' || text.startsWith('R')) return 'R'
  if (text.includes('TVY7') || text.includes('PG')) return 'PG'
  if (text.includes('TVY') || text.includes('TVG') || text === 'G' || text.startsWith('G')) return 'G'
  return ''
}

export interface RatingSuggestion {
  level: Exclude<FamilyRatingLevel, ''>
  source: string
  original: string
}

const US_RATINGS: Record<string, Exclude<FamilyRatingLevel, ''>> = {
  G: 'G', PG: 'PG', 'PG-13': 'PG-13', R: 'R', 'NC-17': 'NC-17',
}

// Only exact US film classifications can be mapped to these US-based controls.
export function suggestFamilyRating(mpaa?: string | null, tmdbData?: string | null, series = false): RatingSuggestion | null {
  const local = (mpaa || '').trim().toUpperCase()
  const localMatch = /^(?:US\s*:\s*)?(G|PG|PG-13|R|NC-17)$/.exec(local)
  const localLevel = localMatch ? US_RATINGS[localMatch[1]] : undefined
  let records: { country: string; rating: string }[] = []
  if (tmdbData) {
    try {
      const parsed: unknown = JSON.parse(tmdbData)
      if (!parsed || typeof parsed !== 'object' || !Array.isArray((parsed as { results?: unknown }).results)) return null
      for (const item of (parsed as { results: unknown[] }).results) {
        if (!item || typeof item !== 'object') return null
        const entry = item as { iso_3166_1?: unknown; release_dates?: unknown }
        if (typeof entry.iso_3166_1 !== 'string') return null
        if (series) {
          const rating = (item as { rating?: unknown }).rating
          if (typeof rating !== 'string') return null
          if (rating.trim()) records.push({ country: entry.iso_3166_1.toUpperCase(), rating: rating.trim().toUpperCase() })
          continue
        }
        if (!Array.isArray(entry.release_dates)) return null
        for (const release of entry.release_dates) {
          if (!release || typeof release !== 'object') return null
          const rating = (release as { certification?: unknown }).certification
          if (typeof rating !== 'string') return null
          if (rating.trim()) records.push({ country: entry.iso_3166_1.toUpperCase(), rating: rating.trim().toUpperCase() })
        }
      }
    } catch { return null }
  }
  const usRatings = [...new Set(records.filter((item) => item.country === 'US').map((item) => item.rating))]
  // Mixed regional systems cannot be compared safely without a verified mapping.
  if (records.some((item) => item.country !== 'US') || usRatings.length > 1) return null
  const tvRatings: Record<string, Exclude<FamilyRatingLevel, ''>> = {
    'TV-Y': 'G', 'TV-G': 'G', 'TV-Y7': 'PG', 'TV-Y7-FV': 'PG',
    'TV-PG': 'PG', 'TV-14': 'PG-13', 'TV-MA': 'NC-17',
  }
  const tmdbLevel = usRatings.length === 1 ? (series ? tvRatings : US_RATINGS)[usRatings[0]] : undefined
  if (usRatings.length && !tmdbLevel) return null
  if (local && !localLevel) return null
  if (localLevel && tmdbLevel && localLevel !== tmdbLevel) return null
  if (tmdbLevel) return { level: tmdbLevel, source: 'TMDb · 美国', original: usRatings[0] }
  if (localLevel) return { level: localLevel, source: '本地 NFO', original: localMatch![1] }
  return null
}

/**
 * 儿童模式下，某部影片是否可进入“帮我选一部”的候选池。
 * 规则：服务端已经只把家长确认过的影片推给儿童账号，这里是前端第二道防线——
 * 只按元数据分级挡掉明确的成人内容（R / NC-17 及等价 TV-MA）。
 * 分级未知（''）时不挡，交给服务端确认结果，避免误杀无分级信息的影片。
 */
export function isMpaaAllowedForChild(mpaa: string | undefined | null): boolean {
  const level = mpaaToRatingLevel(mpaa)
  if (level === '') return true
  return familyRatingRank(level) <= familyRatingRank('PG-13')
}

/** 剩余秒数换算成展示用的分钟数（向上取整，不足 1 分钟按 1 分钟算）。 */
export function remainingMinutesFromSeconds(seconds: number | null | undefined): number | null {
  if (seconds === null || seconds === undefined || !Number.isFinite(seconds) || seconds <= 0) return null
  return Math.ceil(seconds / 60)
}

/** 是否进入“快用完”区间：还剩 5 分钟以内（含 5 分钟），且还有时间。 */
export function isLowRemaining(seconds: number | null | undefined): boolean {
  if (seconds === null || seconds === undefined || !Number.isFinite(seconds)) return false
  return seconds > 0 && seconds <= 300
}
