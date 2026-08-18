import type { ReactNode } from 'react'
import clsx from 'clsx'
import { Button } from '@/components/design-system'

export interface SegmentedControlItem<T extends string> {
  value: T
  label: ReactNode
  icon?: ReactNode
  title?: string
  disabled?: boolean
}

export interface SegmentedControlProps<T extends string> {
  value: T
  items: SegmentedControlItem<T>[]
  onChange: (value: T) => void
  ariaLabel: string
  iconOnly?: boolean
  className?: string
}

export function SegmentedControl<T extends string>({
  value,
  items,
  onChange,
  ariaLabel,
  iconOnly = false,
  className,
}: SegmentedControlProps<T>) {
  return (
    <div className={clsx('nv-segmented-control', className)} role="group" aria-label={ariaLabel}>
      {items.map((item) => {
        const active = item.value === value
        return (
          <Button
            key={item.value}
            type="button"
            variant="ghost"
            size="sm"
            iconOnly={iconOnly}
            disabled={item.disabled}
            onClick={() => onChange(item.value)}
            aria-pressed={active}
            aria-label={iconOnly && typeof item.label === 'string' ? item.label : undefined}
            title={item.title || (typeof item.label === 'string' ? item.label : undefined)}
            className={active ? 'is-active' : undefined}
          >
            {item.icon}
            {!iconOnly && item.label}
          </Button>
        )
      })}
    </div>
  )
}

export default SegmentedControl
