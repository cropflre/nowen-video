import { useEffect, useRef, useState } from 'react'
import { Globe } from 'lucide-react'
import clsx from 'clsx'
import { useI18nStore, SUPPORTED_LOCALES } from '@/i18n'
import { Button } from '@/components/design-system'

export default function LanguageSwitcher() {
  const { locale, setLocale } = useI18nStore()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleClick = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const currentLang = SUPPORTED_LOCALES.find((lang) => lang.code === locale)

  return (
    <div ref={ref} className="relative">
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={() => setOpen((value) => !value)}
        className="w-full justify-start gap-3"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <Globe size={18} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
        <span className="truncate">{currentLang?.flag} {currentLang?.name}</span>
      </Button>

      {open && (
        <div
          className="nv-surface absolute bottom-full left-0 z-[var(--nv-z-dropdown)] mb-2 w-48 overflow-hidden py-1 shadow-[var(--nv-shadow-elevated)]"
          role="menu"
          aria-label="选择语言"
        >
          {SUPPORTED_LOCALES.map((lang) => {
            const active = locale === lang.code
            return (
              <button
                key={lang.code}
                type="button"
                role="menuitemradio"
                aria-checked={active}
                onClick={() => {
                  setLocale(lang.code)
                  setOpen(false)
                }}
                className={clsx(
                  'flex w-full items-center gap-3 px-4 py-2.5 text-sm transition-colors',
                  active
                    ? 'bg-[var(--nv-bg-active)] font-medium text-[var(--nv-action-primary)]'
                    : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]',
                )}
              >
                <span className="text-base" aria-hidden="true">{lang.flag}</span>
                <span className="min-w-0 flex-1 truncate text-left">{lang.name}</span>
                {active && <span className="text-xs text-[var(--nv-action-primary)]" aria-hidden="true">✓</span>}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
