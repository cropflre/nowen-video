import { useState } from 'react'
import { RefreshCw } from 'lucide-react'
import clsx from 'clsx'
import { Button } from '@/components/design-system'
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@/components/design-system/Modal'
import { useTranslation } from '@/i18n'

export type ScrapeMode = 'fill-missing' | 'replace'

interface ScrapeModeDialogProps {
  onConfirm: (mode: ScrapeMode) => void | Promise<void>
  onClose: () => void
  confirming?: boolean
}

const options: Array<{ mode: ScrapeMode; titleKey: string; descKey: string }> = [
  { mode: 'fill-missing', titleKey: 'scrapeMode.fillMissing', descKey: 'scrapeMode.fillMissingDesc' },
  { mode: 'replace', titleKey: 'scrapeMode.replace', descKey: 'scrapeMode.replaceDesc' },
]

export default function ScrapeModeDialog({ onConfirm, onClose, confirming = false }: ScrapeModeDialogProps) {
  const { t } = useTranslation()
  // 默认只补缺失：不覆盖已有数据。
  const [mode, setMode] = useState<ScrapeMode>('fill-missing')

  return (
    <Modal onClose={onClose} size="sm" ariaLabel={t('scrapeMode.title')} closeOnBackdrop={!confirming}>
      <ModalHeader
        title={t('scrapeMode.title')}
        description={t('scrapeMode.hint')}
        icon={<RefreshCw size={18} aria-hidden="true" />}
        onClose={confirming ? undefined : onClose}
      />

      <ModalBody>
        <div role="radiogroup" aria-label={t('scrapeMode.title')} className="space-y-2">
          {options.map((option) => {
            const selected = mode === option.mode
            return (
              <button
                key={option.mode}
                type="button"
                role="radio"
                aria-checked={selected}
                disabled={confirming}
                onClick={() => setMode(option.mode)}
                className={clsx(
                  'flex min-h-[44px] w-full items-start gap-3 rounded-[var(--nv-radius-control)] border px-3 py-2.5 text-left transition-[border-color,background-color,box-shadow] disabled:cursor-not-allowed disabled:opacity-60',
                  selected
                    ? 'border-[var(--nv-action-primary)] bg-[var(--nv-bg-active)] shadow-[var(--nv-shadow-focus)]'
                    : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)]',
                )}
              >
                <span
                  aria-hidden="true"
                  className={clsx(
                    'mt-1 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2',
                    selected ? 'border-[var(--nv-action-primary)]' : 'border-[var(--nv-border-default)]',
                  )}
                >
                  {selected && <span className="h-2 w-2 rounded-full bg-[var(--nv-action-primary)]" />}
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-medium text-[var(--nv-text-primary)]">{t(option.titleKey)}</span>
                  <span className="mt-0.5 block text-xs leading-5 text-[var(--nv-text-tertiary)]">{t(option.descKey)}</span>
                </span>
              </button>
            )
          })}
        </div>
      </ModalBody>

      <ModalFooter>
        <Button type="button" variant="secondary" onClick={onClose} disabled={confirming}>{t('common.cancel')}</Button>
        <Button type="button" variant="primary" onClick={() => void onConfirm(mode)} loading={confirming}>
          {t('scrapeMode.confirm')}
        </Button>
      </ModalFooter>
    </Modal>
  )
}
