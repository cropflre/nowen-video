// ============================================================
// 滚动触发入场动画 Hook
// ============================================================

import { useInView } from 'framer-motion'
import { useRef } from 'react'

/**
 * 滚动进入视口时触发动画
 * @param threshold 可见比例阈值 (0-1)
 * @param once 是否只触发一次
 */
export function useScrollReveal(threshold = 0.2, once = true) {
  const ref = useRef(null)
  const isInView = useInView(ref, {
    amount: threshold,
    once,
  })

  return { ref, isInView }
}
