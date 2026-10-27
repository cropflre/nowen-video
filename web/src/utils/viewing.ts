/** User-facing watch labels and resume rules shared by all playback engines. */
export function episodeLabel(season: number, episode: number, locale = 'zh-CN'): string {
  const validSeason = Number.isFinite(season) && season >= 0
  const validEpisode = Number.isFinite(episode) && episode > 0
  if (locale === 'en-US') return [validSeason ? season === 0 ? 'Specials' : `Season ${season}` : '', validEpisode ? `Episode ${episode}` : ''].filter(Boolean).join(' · ')
  if (locale === 'ja-JP') return [validSeason ? season === 0 ? '特別編' : `シーズン ${season}` : '', validEpisode ? `第 ${episode} 話` : ''].filter(Boolean).join(' · ')
  return [validSeason ? season === 0 ? '特别篇' : `第 ${season} 季` : '', validEpisode ? `第 ${episode} 集` : ''].filter(Boolean).join(' · ')
}

export function remainingMinutes(position: number, duration: number): number | null {
  if (!Number.isFinite(position) || !Number.isFinite(duration) || duration <= 0) return null
  return Math.ceil(Math.max(0, duration - Math.max(0, position)) / 60)
}

export function resumePosition(history?: { position: number; duration: number; completed?: boolean } | null): number {
  if (!history || history.completed || !Number.isFinite(history.position) || history.position <= 0) return 0
  if (history.duration > 0 && history.position / history.duration >= 0.9) return 0
  return history.position
}

export function subtitleMatchesLanguage(label: string, language: string): boolean {
  const normalized = label.toLowerCase()
  const patterns: Record<string, RegExp> = {
    zh: /(^|[^a-z])(zh(?:-[a-z]+)?|zho|chi|chinese)([^a-z]|$)|中文|简体|繁体|繁體|簡體/,
    en: /(^|[^a-z])(en(?:-[a-z]+)?|eng|english)([^a-z]|$)|英语|英文/,
    ja: /(^|[^a-z])(ja(?:-[a-z]+)?|jpn|japanese)([^a-z]|$)|日语|日文|日本語/,
  }
  return patterns[language]?.test(normalized) ?? false
}
