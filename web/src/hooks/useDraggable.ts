import { useRef, useCallback, useEffect, useState } from 'react'

// ==================== 可复用拖拽 Hook ====================

/** 拖拽配置选项 */
export interface DraggableOptions {
  /** 初始位置，null 表示使用 CSS 默认定位 */
  initialPosition?: { x: number; y: number } | null
  /** 边缘吸附阈值（px），拖拽到视口边缘此距离内自动吸附 */
  snapThreshold?: number
  /** 是否启用惯性滑动 */
  enableMomentum?: boolean
  /** 惯性摩擦系数（0~1，越小越快停止） */
  friction?: number
  /** 判定为拖拽的最小移动距离（px），低于此值视为点击 */
  dragThreshold?: number
  /** 边界内边距（px），防止完全贴边 */
  boundaryPadding?: number
  /** 是否允许整个元素作为拖拽手柄（为 true 时不过滤 button 等交互元素） */
  selfAsDragHandle?: boolean
  /** 拖拽开始回调 */
  onDragStart?: () => void
  /** 拖拽中回调 */
  onDragMove?: (x: number, y: number) => void
  /** 拖拽结束回调（返回最终位置） */
  onDragEnd?: (x: number, y: number) => void
}

/** 拖拽状态 */
export interface DraggableState {
  /** 当前位置，null 表示使用默认位置 */
  position: { x: number; y: number } | null
  /** 是否正在拖拽中 */
  isDragging: boolean
  /** 本次交互是否产生了拖拽移动（用于区分点击） */
  wasDragged: boolean
}

/** Hook 返回值 */
export interface DraggableReturn {
  /** 绑定到可拖拽元素的 ref */
  elementRef: React.RefObject<HTMLElement | null>
  /** 拖拽状态 */
  state: DraggableState
  /** 绑定到拖拽手柄的 mousedown 处理器 */
  handleMouseDown: (e: React.MouseEvent) => void
  /** 手动设置位置 */
  setPosition: (pos: { x: number; y: number } | null) => void
  /** 获取用于定位的 style 对象 */
  getPositionStyle: (defaultStyle: React.CSSProperties) => React.CSSProperties
  /** 实时读取 wasDragged 状态的 ref（用于事件处理器中区分点击和拖拽） */
  wasDraggedRef: React.RefObject<boolean>
}

export function useDraggable(options: DraggableOptions = {}): DraggableReturn {
  const {
    initialPosition = null,
    snapThreshold = 20,
    enableMomentum = true,
    friction = 0.92,
    dragThreshold = 3,
    boundaryPadding = 0,
    selfAsDragHandle = false,
    onDragStart,
    onDragMove,
    onDragEnd,
  } = options

  // ---- 状态 ----
  const [position, setPosition] = useState<{ x: number; y: number } | null>(initialPosition)
  const [isDragging, setIsDragging] = useState(false)

  // ---- Refs（高频操作避免触发渲染） ----
  const elementRef = useRef<HTMLElement | null>(null)
  const draggingRef = useRef(false)
  const wasDraggedRef = useRef(false)
  const startMouseRef = useRef({ x: 0, y: 0 })
  const offsetRef = useRef({ x: 0, y: 0 })
  const currentPosRef = useRef({ x: 0, y: 0 })
  const rafIdRef = useRef<number>(0)

  // 速度追踪（用于惯性）
  const velocityRef = useRef({ x: 0, y: 0 })
  const lastMoveTimeRef = useRef(0)
  const lastMovePos = useRef({ x: 0, y: 0 })
  const momentumRafRef = useRef<number>(0)

  // 回调 refs（避免闭包陷阱）
  const onDragStartRef = useRef(onDragStart)
  const onDragMoveRef = useRef(onDragMove)
  const onDragEndRef = useRef(onDragEnd)
  useEffect(() => {
    onDragStartRef.current = onDragStart
    onDragMoveRef.current = onDragMove
    onDragEndRef.current = onDragEnd
  }, [onDragStart, onDragMove, onDragEnd])

  // ---- 边界约束 ----
  const clampPosition = useCallback((x: number, y: number): { x: number; y: number } => {
    const el = elementRef.current
    const w = el?.offsetWidth || 100
    const h = el?.offsetHeight || 48
    const vw = window.innerWidth
    const vh = window.innerHeight
    return {
      x: Math.max(boundaryPadding, Math.min(x, vw - w - boundaryPadding)),
      y: Math.max(boundaryPadding, Math.min(y, vh - h - boundaryPadding)),
    }
  }, [boundaryPadding])

  // ---- 边缘吸附 ----
  const applySnap = useCallback((x: number, y: number): { x: number; y: number } => {
    const el = elementRef.current
    const w = el?.offsetWidth || 100
    const h = el?.offsetHeight || 48
    const vw = window.innerWidth
    const vh = window.innerHeight

    let snappedX = x
    let snappedY = y

    // 左边缘吸附
    if (x <= snapThreshold + boundaryPadding) snappedX = boundaryPadding
    // 右边缘吸附
    if (x >= vw - w - snapThreshold - boundaryPadding) snappedX = vw - w - boundaryPadding
    // 上边缘吸附
    if (y <= snapThreshold + boundaryPadding) snappedY = boundaryPadding
    // 下边缘吸附
    if (y >= vh - h - snapThreshold - boundaryPadding) snappedY = vh - h - boundaryPadding

    return { x: snappedX, y: snappedY }
  }, [snapThreshold, boundaryPadding])

  // ---- 直接操作 DOM（高性能，绕过 React 渲染） ----
  const applyTransform = useCallback((x: number, y: number) => {
    const el = elementRef.current
    if (!el) return
    el.style.left = `${x}px`
    el.style.top = `${y}px`
    el.style.right = 'auto'
    el.style.bottom = 'auto'
  }, [])

  // ---- 惯性动画 ----
  const startMomentum = useCallback(() => {
    if (!enableMomentum) return

    const vx = velocityRef.current.x
    const vy = velocityRef.current.y

    // 速度太小则不启动惯性
    if (Math.abs(vx) < 0.5 && Math.abs(vy) < 0.5) return

    const animate = () => {
      velocityRef.current.x *= friction
      velocityRef.current.y *= friction

      // 速度衰减到极小值时停止
      if (Math.abs(velocityRef.current.x) < 0.3 && Math.abs(velocityRef.current.y) < 0.3) {
        // 最终位置应用吸附并同步到 React state
        const snapped = applySnap(currentPosRef.current.x, currentPosRef.current.y)
        currentPosRef.current = snapped
        applyTransform(snapped.x, snapped.y)
        setPosition({ ...snapped })
        return
      }

      const newX = currentPosRef.current.x + velocityRef.current.x
      const newY = currentPosRef.current.y + velocityRef.current.y
      const clamped = clampPosition(newX, newY)
      currentPosRef.current = clamped
      applyTransform(clamped.x, clamped.y)

      // 碰到边界时速度反弹衰减
      if (clamped.x !== newX) velocityRef.current.x *= -0.3
      if (clamped.y !== newY) velocityRef.current.y *= -0.3

      momentumRafRef.current = requestAnimationFrame(animate)
    }

    momentumRafRef.current = requestAnimationFrame(animate)
  }, [enableMomentum, friction, clampPosition, applySnap, applyTransform])

  // ---- 拖拽事件处理 ----
  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!draggingRef.current) return

    // 计算移动距离，判断是否超过拖拽阈值
    const dx = e.clientX - startMouseRef.current.x
    const dy = e.clientY - startMouseRef.current.y

    if (!wasDraggedRef.current) {
      if (Math.abs(dx) < dragThreshold && Math.abs(dy) < dragThreshold) return
      // 首次超过阈值，标记为拖拽并触发回调
      wasDraggedRef.current = true
      setIsDragging(true)
      onDragStartRef.current?.()

      // 添加拖拽中的视觉效果
      const el = elementRef.current
      if (el) {
        el.style.transition = 'none'
        el.style.opacity = '0.9'
        el.style.filter = 'drop-shadow(0 8px 32px rgba(0, 170, 255, 0.4))'
        el.style.transform = 'scale(1.01)'
      }
    }

    // 使用 rAF 节流，确保每帧只更新一次
    if (rafIdRef.current) cancelAnimationFrame(rafIdRef.current)

    rafIdRef.current = requestAnimationFrame(() => {
      const rawX = e.clientX - offsetRef.current.x
      const rawY = e.clientY - offsetRef.current.y
      const clamped = clampPosition(rawX, rawY)

      // 计算速度（用于惯性）
      const now = performance.now()
      const dt = now - lastMoveTimeRef.current
      if (dt > 0 && dt < 100) {
        // 使用指数移动平均平滑速度
        const alpha = 0.4
        velocityRef.current.x = alpha * ((clamped.x - lastMovePos.current.x) / (dt / 16)) + (1 - alpha) * velocityRef.current.x
        velocityRef.current.y = alpha * ((clamped.y - lastMovePos.current.y) / (dt / 16)) + (1 - alpha) * velocityRef.current.y
      }
      lastMoveTimeRef.current = now
      lastMovePos.current = { ...clamped }

      currentPosRef.current = clamped
      applyTransform(clamped.x, clamped.y)
      onDragMoveRef.current?.(clamped.x, clamped.y)
    })
  }, [dragThreshold, clampPosition, applyTransform])

  const handleMouseUp = useCallback(() => {
    if (!draggingRef.current) return
    draggingRef.current = false

    // 清理 rAF
    if (rafIdRef.current) {
      cancelAnimationFrame(rafIdRef.current)
      rafIdRef.current = 0
    }

    // 移除拖拽视觉效果
    const el = elementRef.current
    if (el) {
      el.style.opacity = ''
      el.style.filter = ''
      el.style.transform = ''
      // 恢复过渡动画（用于吸附/惯性结束时的平滑效果）
      el.style.transition = 'opacity 0.2s, filter 0.2s, transform 0.2s, box-shadow 0.2s'
    }

    setIsDragging(false)

    if (wasDraggedRef.current) {
      // 启动惯性动画
      startMomentum()
      // 如果没有惯性或惯性很小，直接应用吸附
      if (!enableMomentum || (Math.abs(velocityRef.current.x) < 0.5 && Math.abs(velocityRef.current.y) < 0.5)) {
        const snapped = applySnap(currentPosRef.current.x, currentPosRef.current.y)
        currentPosRef.current = snapped
        // 使用 CSS transition 实现平滑吸附
        if (el) {
          el.style.transition = 'left 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94), top 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94), opacity 0.2s, filter 0.2s, transform 0.2s'
          applyTransform(snapped.x, snapped.y)
        }
        setPosition({ ...snapped })
      }

      onDragEndRef.current?.(currentPosRef.current.x, currentPosRef.current.y)
    }

    document.removeEventListener('mousemove', handleMouseMove)
    document.removeEventListener('mouseup', handleMouseUp)
  }, [handleMouseMove, startMomentum, enableMomentum, applySnap, applyTransform])

  // ---- 拖拽入口 ----
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    // 如果点击的是交互元素内部（非拖拽手柄本身），不触发拖拽
    const target = e.target as HTMLElement
    const currentEl = elementRef.current

    if (!selfAsDragHandle) {
      // 标准模式：过滤掉内部的按钮、输入框等交互元素
      if (target.closest('button') || target.closest('input') || target.closest('textarea') || target.closest('a')) return
    } else {
      // 自身作为拖拽手柄模式：只过滤掉内部子元素中的交互元素（不过滤自身）
      const interactiveEl = target.closest('button, input, textarea, a')
      if (interactiveEl && interactiveEl !== currentEl) return
    }

    e.preventDefault()
    e.stopPropagation()

    // 停止正在进行的惯性动画
    if (momentumRafRef.current) {
      cancelAnimationFrame(momentumRafRef.current)
      momentumRafRef.current = 0
    }

    draggingRef.current = true
    wasDraggedRef.current = false
    velocityRef.current = { x: 0, y: 0 }
    startMouseRef.current = { x: e.clientX, y: e.clientY }

    const el = elementRef.current
    if (el) {
      const rect = el.getBoundingClientRect()
      offsetRef.current = { x: e.clientX - rect.left, y: e.clientY - rect.top }
      currentPosRef.current = { x: rect.left, y: rect.top }
    }

    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
  }, [selfAsDragHandle, handleMouseMove, handleMouseUp])

  // ---- 窗口 resize 时约束位置 ----
  useEffect(() => {
    const handleResize = () => {
      setPosition(prev => {
        if (!prev) return null
        return clampPosition(prev.x, prev.y)
      })
      // 同步 DOM
      if (position) {
        const clamped = clampPosition(position.x, position.y)
        currentPosRef.current = clamped
        applyTransform(clamped.x, clamped.y)
      }
    }
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [clampPosition, applyTransform, position])

  // ---- 清理 ----
  useEffect(() => {
    return () => {
      if (rafIdRef.current) cancelAnimationFrame(rafIdRef.current)
      if (momentumRafRef.current) cancelAnimationFrame(momentumRafRef.current)
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }
  }, [handleMouseMove, handleMouseUp])

  // ---- 获取定位样式 ----
  const getPositionStyle = useCallback((defaultStyle: React.CSSProperties): React.CSSProperties => {
    if (position) {
      return {
        left: position.x,
        top: position.y,
        right: 'auto',
        bottom: 'auto',
      }
    }
    return defaultStyle
  }, [position])

  return {
    elementRef,
    state: {
      position,
      isDragging,
      wasDragged: wasDraggedRef.current,
    },
    handleMouseDown,
    setPosition,
    getPositionStyle,
    wasDraggedRef,
  }
}
