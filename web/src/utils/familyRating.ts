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
