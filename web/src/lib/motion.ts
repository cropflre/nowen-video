// ============================================================
// 深空流体 · 动画设计令牌系统
// 统一管理所有动画参数，确保全局一致�?
// ============================================================

import type { Variants, Transition } from 'framer-motion'

// ==================== 缓动函数 ====================
/** 深空流体标准缓动 �?快速启动，优雅减�?*/
export const easeSmooth = [0.22, 1, 0.36, 1] as const
/** 退出缓�?�?快速加速离开 */
export const easeExit = [0.36, 0, 0.66, -0.56] as const

export const springDefault: Transition = { type: 'spring', stiffness: 300, damping: 30 }
export const springBouncy: Transition = { type: 'spring', stiffness: 400, damping: 25 }
export const springSnappy: Transition = { type: 'spring', stiffness: 500, damping: 35 }

// ==================== 时长令牌 ====================
export const durations = {
  instant: 0.1,
  fast: 0.2,
  normal: 0.3,
  slow: 0.5,
  slower: 0.7,
  page: 0.4,
}

// ==================== 页面过渡变体 ====================
export const pageVariants: Variants = {
  initial: {
    opacity: 0,
    y: 12,
    filter: 'blur(4px)',
  },
  enter: {
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: {
      duration: durations.page,
      ease: easeSmooth as unknown as [number, number, number, number],
    },
  },
  exit: {
    opacity: 0,
    y: -8,
    filter: 'blur(2px)',
    transition: {
      duration: durations.normal,
      ease: easeExit as unknown as [number, number, number, number],
    },
  },
}

// ==================== 淡入变体 ====================
export const fadeInVariants: Variants = {
  hidden: { opacity: 0, filter: 'blur(4px)' },
  visible: {
    opacity: 1,
    filter: 'blur(0px)',
    transition: { duration: durations.normal, ease: easeSmooth as unknown as [number, number, number, number] },
  },
}

// ==================== 上浮入场变体 ====================
export const slideUpVariants: Variants = {
  hidden: { opacity: 0, y: 20, filter: 'blur(4px)' },
  visible: {
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: { duration: durations.slow, ease: easeSmooth as unknown as [number, number, number, number] },
  },
}

// ==================== 缩放入场变体 ====================
export const scaleInVariants: Variants = {
  hidden: { opacity: 0, scale: 0.92, filter: 'blur(4px)' },
  visible: {
    opacity: 1,
    scale: 1,
    filter: 'blur(0px)',
    transition: { duration: durations.normal, ease: easeSmooth as unknown as [number, number, number, number] },
  },
}

// ==================== 交错子元素容�?====================
export const staggerContainerVariants: Variants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.04,
      delayChildren: 0.08,
    },
  },
}

// ==================== 交错子元素项 ====================
export const staggerItemVariants: Variants = {
  hidden: { opacity: 0, y: 16, scale: 0.96 },
  visible: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: {
      duration: durations.normal,
      ease: easeSmooth as unknown as [number, number, number, number],
    },
  },
}

// ==================== 模态框变体 ====================
export const modalOverlayVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { duration: durations.fast } },
  exit: { opacity: 0, transition: { duration: durations.fast, delay: 0.1 } },
}

export const modalContentVariants: Variants = {
  hidden: { opacity: 0, scale: 0.92, y: 20, filter: 'blur(8px)' },
  visible: {
    opacity: 1,
    scale: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: { duration: durations.normal, ease: easeSmooth as unknown as [number, number, number, number] },
  },
  exit: {
    opacity: 0,
    scale: 0.95,
    y: 10,
    filter: 'blur(4px)',
    transition: { duration: durations.fast, ease: easeExit as unknown as [number, number, number, number] },
  },
}

// ==================== Toast 通知变体 ====================
export const toastVariants: Variants = {
  initial: { opacity: 0, x: 80, scale: 0.9 },
  animate: {
    opacity: 1,
    x: 0,
    scale: 1,
    transition: springDefault,
  },
  exit: {
    opacity: 0,
    x: 80,
    scale: 0.9,
    transition: { duration: durations.fast, ease: easeExit as unknown as [number, number, number, number] },
  },
}

// ==================== 侧边栏变�?====================
export const sidebarVariants: Variants = {
  collapsed: {
    width: 68,
    minWidth: 68,
    transition: { duration: durations.normal, ease: easeSmooth as unknown as [number, number, number, number] },
  },
  expanded: {
    width: 240,
    minWidth: 240,
    transition: { duration: durations.normal, ease: easeSmooth as unknown as [number, number, number, number] },
  },
}

export const sidebarMobileVariants: Variants = {
  hidden: { x: '-100%' },
  visible: {
    x: 0,
    transition: springDefault,
  },
  exit: {
    x: '-100%',
    transition: { duration: durations.normal, ease: easeExit as unknown as [number, number, number, number] },
  },
}

// ==================== 轮播切换变体 ====================
export const carouselVariants: Variants = {
  enter: (direction: number) => ({
    opacity: 0,
    x: direction > 0 ? 60 : -60,
    scale: 0.98,
  }),
  center: {
    opacity: 1,
    x: 0,
    scale: 1,
    transition: { duration: durations.slow, ease: easeSmooth as unknown as [number, number, number, number] },
  },
  exit: (direction: number) => ({
    opacity: 0,
    x: direction > 0 ? -60 : 60,
    scale: 0.98,
    transition: { duration: durations.normal, ease: easeExit as unknown as [number, number, number, number] },
  }),
}

// ==================== 下拉菜单变体 ====================
export const dropdownVariants: Variants = {
  hidden: {
    opacity: 0,
    scale: 0.95,
    y: -4,
    transformOrigin: 'top left',
  },
  visible: {
    opacity: 1,
    scale: 1,
    y: 0,
    transition: springDefault,
  },
  exit: {
    opacity: 0,
    scale: 0.95,
    y: -4,
    transition: { duration: durations.fast },
  },
}

// ==================== 悬停交互预设 ====================
export const hoverScale = {
  whileHover: { scale: 1.03 },
  whileTap: { scale: 0.97 },
  transition: springDefault,
}

export const hoverLift = {
  whileHover: { y: -4 },
  whileTap: { y: 0 },
  transition: springDefault,
}

export const hoverGlow = {
  whileHover: {
    boxShadow: '0 0 20px var(--neon-blue-30), 0 8px 32px rgba(0, 0, 0, 0.3)',
  },
  transition: { duration: durations.normal },
}

// ==================== reduce-motion 兼容 ====================
export const reducedMotionVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { duration: 0.01 } },
}

// ==================== 骨架屏 → 内容过渡变体 ====================
/** 骨架屏退出动画 */
export const skeletonExitVariants: Variants = {
  initial: { opacity: 0 },
  animate: { opacity: 1, transition: { duration: durations.fast } },
  exit: { opacity: 0, transition: { duration: durations.fast } },
}

/** 内容入场动画（骨架屏消失后） */
export const contentEnterVariants: Variants = {
  initial: { opacity: 0, y: 8 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: durations.page,
      ease: easeSmooth as unknown as [number, number, number, number],
    },
  },
}
