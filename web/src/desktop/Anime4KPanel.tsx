/**
 * Anime4KPanel
 *
 * Anime4K 超分档位选择：off / low / medium / high。
 */

import { Check, Zap } from 'lucide-react'
import type { Anime4KLevel } from './bridge'

export type { Anime4KLevel }

interface Props {
  value: Anime4KLevel
  onChange: (level: Anime4KLevel) => void
}

interface LevelMeta {
  key: Anime4KLevel
  label: string
  desc: string
  badge?: string
}

const LEVELS: LevelMeta[] = [
  { key: 'off', label: '关闭', desc: '原始画质，零开销' },
  { key: 'low', label: '低', desc: 'CNN-M · 约 +10% GPU', badge: '节能' },
  { key: 'medium', label: '中', desc: 'CNN-VL · 约 +25% GPU', badge: '推荐' },
  { key: 'high', label: '高', desc: 'CNN-UL · 约 +50% GPU', badge: '极致' },
]

export default function Anime4KPanel({ value, onChange }: Props) {
  return (
    <div className="min-w-[280px] rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] p-3 shadow-[var(--nv-player-shadow)]">
      <div className="mb-2 flex items-center gap-2 px-1">
        <Zap className="h-4 w-4 text-[var(--nv-player-text-tertiary)]" aria-hidden="true" />
        <span className="text-sm font-semibold text-[var(--nv-player-text-primary)]">Anime4K 超分</span>
        <span className="ml-auto text-[10px] uppercase tracking-wider text-[var(--nv-player-text-faint)]">GPU Shader</span>
      </div>

      <div className="space-y-0.5">
        {LEVELS.map((level) => {
          const active = value === level.key
          return (
            <button
              key={level.key}
              type="button"
              onClick={() => onChange(level.key)}
              className={`flex w-full items-center gap-3 rounded-[var(--nv-player-radius-control)] px-3 py-2 text-left transition-[background-color,color] duration-150 ${active
                ? 'bg-[var(--nv-player-surface-hover)] text-[var(--nv-player-text-primary)]'
                : 'text-[var(--nv-player-text-secondary)] hover:bg-[var(--nv-player-surface-subtle)]'}`}
              aria-pressed={active}
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{level.label}</span>
                  {level.badge && <span className="text-[10px] text-[var(--nv-player-text-tertiary)]">{level.badge}</span>}
                </div>
                <div className="mt-0.5 truncate text-[11px] text-[var(--nv-player-text-tertiary)]">{level.desc}</div>
              </div>
              {active && <Check className="h-4 w-4 shrink-0 text-[var(--nv-player-text-primary)]" aria-hidden="true" />}
            </button>
          )
        })}
      </div>

      <div className="mt-2 border-t border-[var(--nv-player-border-subtle)] px-1 pt-2 text-[10px] leading-relaxed text-[var(--nv-player-text-faint)]">
        适合动漫和 2D 插画。3D 真人影片建议“关闭”，实时超分对人脸会产生伪影。
      </div>
    </div>
  )
}
