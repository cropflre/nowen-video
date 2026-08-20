import type { ButtonHTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'
import { Button } from '@/components/design-system'

export interface FilterChipProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  selected?: boolean
  children: ReactNode
  className?: string
}

export function FilterChip({ selected = false, children, className, ...props }: FilterChipProps) {
  return (
    <Button
      {...props}
      type={props.type || 'button'}
      variant={selected ? 'secondary' : 'ghost'}
      size="sm"
      aria-pressed={selected}
      className={clsx('nv-filter-chip', className)}
    >
      {children}
    </Button>
  )
}

export default FilterChip
