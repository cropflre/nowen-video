import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'

/**
 * 页面过渡动画包裹器
 * 路由切换时自动播放淡入 + 上浮动画
 */
export default function PageTransition({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const [displayChildren, setDisplayChildren] = useState(children)
  const [transitionStage, setTransitionStage] = useState<'enter' | 'idle'>('enter')

  useEffect(() => {
    setTransitionStage('enter')
    setDisplayChildren(children)
    const timer = setTimeout(() => setTransitionStage('idle'), 500)
    return () => clearTimeout(timer)
  }, [location.pathname]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div
      className={transitionStage === 'enter' ? 'animate-page-enter' : ''}
      style={{ minHeight: '100%' }}
    >
      {displayChildren}
    </div>
  )
}
