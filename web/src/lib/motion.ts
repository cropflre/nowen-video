import type { Transition, Variants } from 'framer-motion'

export const easeSmooth = [0.2, 0.72, 0.2, 1] as const
export const easeExit = [0.4, 0, 1, 1] as const

export const springDefault: Transition = { duration: 0.18, ease: easeSmooth as unknown as [number, number, number, number] }
export const springBouncy: Transition = springDefault
export const springSnappy: Transition = { duration: 0.12, ease: easeSmooth as unknown as [number, number, number, number] }

export const durations = {
  instant: 0.1,
  fast: 0.12,
  normal: 0.18,
  slow: 0.2,
  slower: 0.2,
  page: 0.18,
}

export const pageVariants: Variants = {
  initial: { opacity: 0, y: 3 },
  enter: { opacity: 1, y: 0, transition: springDefault },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

export const fadeInVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: springDefault },
}

export const slideUpVariants: Variants = {
  hidden: { opacity: 0, y: 4 },
  visible: { opacity: 1, y: 0, transition: springDefault },
}

export const scaleInVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: springDefault },
}

export const staggerContainerVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { duration: durations.fast } },
}

export const staggerItemVariants: Variants = {
  hidden: { opacity: 0, y: 2 },
  visible: { opacity: 1, y: 0, transition: springDefault },
}

export const modalOverlayVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { duration: durations.fast } },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

export const modalContentVariants: Variants = {
  hidden: { opacity: 0, y: 4 },
  visible: { opacity: 1, y: 0, transition: springDefault },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

export const toastVariants: Variants = {
  initial: { opacity: 0, y: 4 },
  animate: { opacity: 1, y: 0, transition: springDefault },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

export const sidebarVariants: Variants = {
  collapsed: { width: 72, minWidth: 72, transition: springDefault },
  expanded: { width: 72, minWidth: 72, transition: springDefault },
}

export const sidebarMobileVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: springDefault },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

export const carouselVariants: Variants = {
  enter: (direction: number) => ({ opacity: 0, x: direction > 0 ? 4 : -4 }),
  center: { opacity: 1, x: 0, transition: springDefault },
  exit: (direction: number) => ({ opacity: 0, x: direction > 0 ? -4 : 4, transition: { duration: durations.fast } }),
}

export const dropdownVariants: Variants = {
  hidden: { opacity: 0, y: -2 },
  visible: { opacity: 1, y: 0, transition: springDefault },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

export const hoverScale = {
  whileHover: { y: -2 },
  whileTap: { y: 0 },
  transition: springDefault,
}

export const hoverLift = {
  whileHover: { y: -3 },
  whileTap: { y: 0 },
  transition: springDefault,
}

export const hoverGlow = {
  whileHover: { boxShadow: 'var(--nv-shadow-card-hover)' },
  transition: springDefault,
}

export const reducedMotionVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { duration: 0.01 } },
}

export const skeletonExitVariants: Variants = {
  initial: { opacity: 0 },
  animate: { opacity: 1, transition: { duration: durations.fast } },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

export const contentEnterVariants: Variants = {
  initial: { opacity: 0, y: 3 },
  animate: { opacity: 1, y: 0, transition: springDefault },
}
