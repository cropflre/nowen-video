// Dialogs and text/interactive controls own their keys. Checking the active
// modal also protects listeners registered before the dialog's Escape handler.
export function shouldIgnorePlaybackShortcut(event: KeyboardEvent): boolean {
  if (event.defaultPrevented || event.isComposing || document.querySelector('[aria-modal="true"]')) return true
  return event.target instanceof Element && Boolean(event.target.closest('input, textarea, select, button, a[href], [role="button"], [role="textbox"], [contenteditable]:not([contenteditable="false"])'))
}
