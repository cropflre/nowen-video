import type { Media, MixedItem } from '@/types'
import MediaCard from './MediaCard'
import { motion } from 'framer-motion'
import { useStaggerVariants } from '@/hooks/useMotion'

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
      <motion.div variants={container} initial="hidden" animate="visible">
        {title && (
          <motion.h2 variants={itemVariant} className="mb-4 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            {title}
          </motion.h2>
        )}
      <div className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {Array.from({ length: 12 }).map((_, i) => (
            <motion.div key={i} variants={itemVariant}>
              <div className="skeleton aspect-[2/3] rounded-xl" />
              <div className="skeleton mt-2 h-4 w-3/4 rounded" />
              <div className="skeleton mt-1 h-3 w-1/2 rounded" />
            </motion.div>
          ))}
        </div>
      </motion.div>
    )
  }

  // 混合模式
  if (mixedItems) {
    if (mixedItems.length === 0) return null
    return (
      <motion.div variants={container} initial="hidden" animate="visible">
        {title && (
          <motion.h2 variants={itemVariant} className="mb-4 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            {title}
          </motion.h2>
        )}
      <div className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
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
        </div>
      </motion.div>
    )
  }

  // 普通模式
  if (!items || items.length === 0) return null

  return (
    <motion.div variants={container} initial="hidden" animate="visible">
      {title && (
        <motion.h2 variants={itemVariant} className="mb-4 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          {title}
        </motion.h2>
      )}
      <div className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {items.map((media) => (
          <motion.div key={media.id} variants={itemVariant}>
            <MediaCard media={media} />
          </motion.div>
        ))}
      </div>
    </motion.div>
  )
}
