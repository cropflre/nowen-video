import { useState, useRef, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Search, X } from 'lucide-react'
import { mobileTokens } from '@/styles/mobile-tokens'

interface MobileSearchBarProps {
  value?: string
  onChange?: (value: string) => void
  onSubmit?: (value: string) => void
  placeholder?: string
  autoFocus?: boolean
  className?: string
}

/**
 * 移动端搜索框
 * Hills Pro 风格：大圆角 + 半透明背景 + 搜索图标
 */
export default function MobileSearchBar({
  value: controlledValue,
  onChange,
  onSubmit,
  placeholder = '输入搜索内容',
  autoFocus = false,
  className = '',
}: MobileSearchBarProps) {
  const [internalValue, setInternalValue] = useState(controlledValue || '')
  const inputRef = useRef<HTMLInputElement>(null)

  const value = controlledValue !== undefined ? controlledValue : internalValue

  useEffect(() => {
    if (autoFocus && inputRef.current) {
      inputRef.current.focus()
    }
  }, [autoFocus])

  const handleChange = (newValue: string) => {
    if (controlledValue === undefined) {
      setInternalValue(newValue)
    }
    onChange?.(newValue)
  }

  const handleSubmit = () => {
    onSubmit?.(value)
  }

  const handleClear = () => {
    handleChange('')
    inputRef.current?.focus()
  }

  return (
    <div className={`px-8 ${className}`}>
      <div
        className="relative flex items-center"
        style={{
          height: '72px',
          borderRadius: mobileTokens.radius.full,
          background: 'rgba(255, 255, 255, 0.6)',
          border: `1px solid ${mobileTokens.cardBorder}`,
          boxShadow: mobileTokens.shadowSm,
        }}
      >
        {/* 搜索图标 */}
        <div
          className="flex items-center justify-center pl-5"
          style={{ color: mobileTokens.textMuted }}
        >
          <Search size={22} />
        </div>

        {/* 输入框 */}
        <input
          ref={inputRef}
          type="text"
          value={value}
          onChange={(e) => handleChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSubmit()
          }}
          placeholder={placeholder}
          className="flex-1 h-full bg-transparent px-3 outline-none"
          style={{
            fontSize: mobileTokens.fontSize.lg,
            color: mobileTokens.text,
          }}
        />

        {/* 清除按钮 */}
        {value && (
          <motion.button
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.8 }}
            whileTap={{ scale: 0.9 }}
            onClick={handleClear}
            className="flex items-center justify-center pr-5"
            style={{ color: mobileTokens.textMuted }}
          >
            <X size={20} />
          </motion.button>
        )}
      </div>
    </div>
  )
}
