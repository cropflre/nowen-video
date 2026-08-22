export interface ParsedSubtitleCue {
  startTime: number
  endTime: number
  text: string
}

function parseTimestamp(value: string): number | null {
  const parts = value.trim().replace(',', '.').split(':')
  if (parts.length !== 2 && parts.length !== 3) return null

  const seconds = Number(parts.pop())
  const minutes = Number(parts.pop())
  const hours = parts.length ? Number(parts.pop()) : 0
  if (![hours, minutes, seconds].every(Number.isFinite)) return null
  return hours * 3600 + minutes * 60 + seconds
}

function decodeCueText(value: string): string {
  return value
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<[^>]*>/g, '')
    .replace(/&#(\d+);/g, (_match, code: string) => String.fromCodePoint(Number(code)))
    .replace(/&#x([\da-f]+);/gi, (_match, code: string) => String.fromCodePoint(parseInt(code, 16)))
    .replace(/&nbsp;/gi, '\u00a0')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&amp;/gi, '&')
    .trim()
}

/** Parse the subset of WebVTT used by Nowen's generated subtitle endpoints. */
export function parseWebVtt(value: string): ParsedSubtitleCue[] {
  const lines = value.replace(/^\uFEFF/, '').replace(/\r\n?/g, '\n').split('\n')
  const cues: ParsedSubtitleCue[] = []

  for (let index = 0; index < lines.length;) {
    const line = lines[index].trim()
    if (!line || line === 'WEBVTT') {
      index += 1
      continue
    }
    if (/^(NOTE|STYLE|REGION)(?:\s|$)/.test(line)) {
      while (index < lines.length && lines[index].trim()) index += 1
      continue
    }

    const timingIndex = line.includes('-->') ? index : index + 1
    const timingLine = lines[timingIndex]?.trim() || ''
    const timing = timingLine.match(/^(\S+)\s+-->\s+(\S+)/)
    if (!timing) {
      index += 1
      continue
    }

    const startTime = parseTimestamp(timing[1])
    const endTime = parseTimestamp(timing[2])
    index = timingIndex + 1
    const textLines: string[] = []
    while (index < lines.length && lines[index].trim()) {
      textLines.push(lines[index])
      index += 1
    }

    const text = decodeCueText(textLines.join('\n'))
    if (startTime !== null && endTime !== null && endTime > startTime && text) {
      cues.push({ startTime, endTime, text })
    }
  }

  return cues.sort((left, right) => left.startTime - right.startTime)
}
