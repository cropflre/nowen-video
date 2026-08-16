/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  // 项目使用 <html data-theme="dark|light"> 切换主题，
  // 通过 selector 模式让 Tailwind 的 dark: 变体与此对齐。
  darkMode: ['selector', '[data-theme="dark"]'],
  theme: {
    extend: {
      // 字体栈的唯一来源是 design-system.css 的 --nv-font-* 令牌。
      fontFamily: {
        display: ['var(--nv-font-display)'],
        body: ['var(--nv-font-sans)'],
        mono: ['var(--nv-font-mono)'],
      },
      colors: {
        primary: {
          50: '#ecfeff',
          100: '#cffafe',
          200: '#a5f3fc',
          300: '#67e8f9',
          400: '#00F0FF',
          500: '#00D4E0',
          600: '#00A8B8',
          700: '#008899',
          800: '#006B7A',
          900: '#004D5A',
          950: '#002B33',
        },
        // Purple 仅保留为装饰/氛围色，不再作为功能主色。
        accent: {
          400: '#A855F7',
          500: '#8A2BE2',
          600: '#7C3AED',
          glow: 'rgba(138, 43, 226, 0.4)',
        },
      },
      backgroundImage: {
        'deep-space': 'radial-gradient(ellipse at 20% 50%, var(--nv-ambient-cyan) 0%, transparent 50%), radial-gradient(ellipse at 80% 20%, var(--nv-ambient-purple-soft) 0%, transparent 50%)',
      },
      boxShadow: {
        'card-hover': 'var(--nv-shadow-card-hover)',
      },
      animation: {
        'fade-in': 'fadeIn 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'slide-up': 'slideUp 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'slide-down': 'slideDown 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'scale-in': 'scaleIn 0.3s cubic-bezier(0.22, 1, 0.36, 1)',
        'slide-right': 'slideRight 0.3s cubic-bezier(0.22, 1, 0.36, 1)',
        'float': 'float 6s ease-in-out infinite',
        'shimmer': 'shimmer 2s linear infinite',
        'ripple': 'ripple 0.6s ease-out',
        'particle-burst': 'particleBurst 0.5s ease-out forwards',
        'page-enter': 'pageEnter 0.5s cubic-bezier(0.22, 1, 0.36, 1)',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0', filter: 'blur(4px)' },
          '100%': { opacity: '1', filter: 'blur(0)' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(20px)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)', filter: 'blur(0)' },
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-20px)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)', filter: 'blur(0)' },
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.9)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'scale(1)', filter: 'blur(0)' },
        },
        slideRight: {
          '0%': { opacity: '0', transform: 'translateX(-10px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-10px)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
        ripple: {
          '0%': { transform: 'scale(0.8)', opacity: '1' },
          '100%': { transform: 'scale(2)', opacity: '0' },
        },
        particleBurst: {
          '0%': { transform: 'scale(1)', opacity: '1' },
          '100%': { transform: 'scale(1.5)', opacity: '0' },
        },
        pageEnter: {
          '0%': { opacity: '0', transform: 'translateY(12px)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)', filter: 'blur(0)' },
        },
      },
      backdropBlur: {
        xs: '2px',
      },
      borderRadius: {
        '2xl': '1rem',
        '3xl': '1.5rem',
      },
    },
  },
  plugins: [],
}
