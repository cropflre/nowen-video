// Dialogs and text/interactive controls own their keys. Checking the active
// modal also protects listeners registered before the dialog's Escape handler.
export function shouldIgnorePlaybackShortcut(event: KeyboardEvent): boolean {
  if (event.defaultPrevented || event.isComposing || document.querySelector('[aria-modal="true"]')) return true
  return event.target instanceof Element && Boolean(event.target.closest('input, textarea, select, button, a[href], [role="button"], [role="textbox"], [contenteditable]:not([contenteditable="false"])'))
}

export function timelineKeyPosition(key: string, position: number, duration: number): number | null {
  if (!Number.isFinite(duration) || duration <= 0) return null
  switch (key) {
    case 'Home': return 0
    case 'End': return duration
    case 'ArrowLeft':
    case 'ArrowDown': return Math.max(0, position - 10)
    case 'ArrowRight':
    case 'ArrowUp': return Math.min(duration, position + 10)
    default: return null
  }
}
