import { useEffect, useRef, useState } from 'react'
import { Check, Globe } from 'lucide-react'
import clsx from 'clsx'
import { useI18nStore, SUPPORTED_LOCALES } from '@/i18n'
import { Button } from '@/components/design-system'

interface LanguageSwitcherProps {
  className?: string
  buttonClassName?: string
  compact?: boolean
}

export default function LanguageSwitcher({ className, buttonClassName, compact = false }: LanguageSwitcherProps) {
  const { locale, setLocale } = useI18nStore()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleClick = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const currentLang = SUPPORTED_LOCALES.find((lang) => lang.code === locale)

  if (compact) {
    return (
      <div ref={ref} className={clsx('relative', className)}>
        <button
          type="button"
          className={clsx('nv-rail-item', buttonClassName)}
          onClick={() => setOpen((value) => !value)}
          aria-label={`语言：${currentLang?.name || locale}`}
          title={`语言：${currentLang?.name || locale}`}
          aria-haspopup="menu"
          aria-expanded={open}
        >
          <span className="nv-rail-icon"><Globe size={17} aria-hidden="true" /></span>
          <span className="nv-rail-label">语言</span>
        </button>

        {open && (
          <div className="nv-menu absolute bottom-0 left-[calc(100%+10px)] z-[var(--nv-z-dropdown)] w-44" role="menu" aria-label="选择语言">
            {SUPPORTED_LOCALES.map((lang) => {
              const active = locale === lang.code
              return (
                <button
                  key={lang.code}
                  type="button"
                  className="nv-menu-item"
                  role="menuitemradio"
                  aria-checked={active}
                  onClick={() => {
                    setLocale(lang.code)
                    setOpen(false)
                  }}
                >
                  <span className="min-w-0 flex-1 truncate">{lang.flag} {lang.name}</span>
                  {active && <Check size={14} aria-hidden="true" />}
                </button>
              )
            })}
          </div>
        )}
      </div>
    )
  }

  return (
    <div ref={ref} className={clsx('relative', className)}>
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={() => setOpen((value) => !value)}
        className={clsx('w-full !justify-start gap-3 text-left', buttonClassName)}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <Globe size={17} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
        <span className="truncate">{currentLang?.flag} {currentLang?.name}</span>
      </Button>

      {open && (
        <div className="nv-menu absolute bottom-full left-0 z-[var(--nv-z-dropdown)] mb-2 w-48" role="menu" aria-label="选择语言">
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
                className="nv-menu-item"
              >
                <span className="min-w-0 flex-1 truncate">{lang.flag} {lang.name}</span>
                {active && <Check size={14} aria-hidden="true" />}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
