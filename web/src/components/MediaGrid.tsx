import type { Media, MixedItem } from '@/types'
import MediaCard from './MediaCard'
import { motion } from 'framer-motion'
import { useStaggerVariants } from '@/hooks/useMotion'
import { Section } from '@/components/design-system'

interface MediaGridProps {
  items?: Media[]
  mixedItems?: MixedItem[]
  title?: string
  loading?: boolean
}

export default function MediaGrid({ items, mixedItems, title, loading }: MediaGridProps) {
  const { container, item: itemVariant } = useStaggerVariants()

  if (loading) {
    return (
      <Section title={title}>
        <motion.div className="nv-media-grid" variants={container} initial="hidden" animate="visible">
          {Array.from({ length: 12 }).map((_, i) => (
            <motion.div key={i} variants={itemVariant}>
              <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
              <div className="skeleton mt-2 h-4 w-3/4" />
              <div className="skeleton mt-1 h-3 w-1/2" />
            </motion.div>
          ))}
        </motion.div>
      </Section>
    )
  }

  if (mixedItems) {
    if (mixedItems.length === 0) return null
    return (
      <Section title={title}>
        <motion.div className="nv-media-grid" variants={container} initial="hidden" animate="visible">
          {mixedItems.map((item) => {
            if (item.type === 'series' && item.series) {
              return (
                <motion.div key={`s-${item.series.id}`} variants={itemVariant}>
                  <MediaCard series={item.series} />
                </motion.div>
              )
            }
            if (item.media) {
              return (
                <motion.div key={`m-${item.media.id}`} variants={itemVariant}>
                  <MediaCard media={item.media} />
                </motion.div>
              )
            }
            return null
          })}
        </motion.div>
      </Section>
    )
  }

  if (!items || items.length === 0) return null

  return (
    <Section title={title}>
      <motion.div className="nv-media-grid" variants={container} initial="hidden" animate="visible">
        {items.map((media) => (
          <motion.div key={media.id} variants={itemVariant}>
            <MediaCard media={media} />
          </motion.div>
        ))}
      </motion.div>
    </Section>
  )
}
