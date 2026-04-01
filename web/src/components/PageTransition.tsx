import { motion, useReducedMotion } from 'framer-motion'
import { pageVariants } from '@/lib/motion'

/**
 * 页面过渡动画包裹器
 * 使用 framer-motion 实现入场 + 退出动画
 */
export default function PageTransition({ children }: { children: React.ReactNode }) {
  const prefersReducedMotion = useReducedMotion()

  if (prefersReducedMotion) {
    return <>{children}</>
  }

  return (
    <motion.div
      initial="initial"
      animate="enter"
      exit="exit"
      variants={pageVariants}
      style={{ minHeight: '100%' }}
    >
      {children}
    </motion.div>
  )
}
