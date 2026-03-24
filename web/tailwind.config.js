/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        display: ['Orbitron', 'sans-serif'],
        body: ['Inter', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
      },
      colors: {
        // 「深空流体」赛博朋克色彩系统
        neon: {
          blue: '#00F0FF',
          purple: '#8A2BE2',
          pink: '#FF00FF',
          green: '#00FF88',
        },
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
        accent: {
          400: '#A855F7',
          500: '#8A2BE2',
          600: '#7C3AED',
          glow: 'rgba(138, 43, 226, 0.4)',
        },
        surface: {
          50: 'var(--surface-50, #f0f4f8)',
          100: 'var(--surface-100, #d9e2ec)',
          200: 'var(--surface-200, #bcccdc)',
          300: 'var(--surface-300, #9fb3c8)',
          400: 'var(--surface-400, #829ab1)',
          500: 'var(--surface-500, #627d98)',
          600: 'var(--surface-600, #486581)',
          700: 'var(--surface-700, #1a2332)',
          800: 'var(--surface-800, #121a27)',
          900: 'var(--surface-900, #0b1120)',
          950: 'var(--surface-950, #060a13)',
        },
      },
      backgroundImage: {
        'neon-gradient': 'linear-gradient(135deg, #8A2BE2, #FF00FF)',
        'neon-gradient-h': 'linear-gradient(90deg, #00F0FF, #8A2BE2)',
        'neon-gradient-v': 'linear-gradient(180deg, #00F0FF, #8A2BE2)',
        'deep-space': 'radial-gradient(ellipse at 20% 50%, rgba(0,240,255,0.03) 0%, transparent 50%), radial-gradient(ellipse at 80% 20%, rgba(138,43,226,0.03) 0%, transparent 50%)',
        'glass-border': 'linear-gradient(135deg, rgba(0,240,255,0.2), rgba(138,43,226,0.2))',
      },
      boxShadow: {
        'neon-blue': '0 0 15px rgba(0, 240, 255, 0.3), 0 0 30px rgba(0, 240, 255, 0.1)',
        'neon-purple': '0 0 15px rgba(138, 43, 226, 0.3), 0 0 30px rgba(138, 43, 226, 0.1)',
        'neon-glow': '0 0 20px rgba(0, 240, 255, 0.2), 0 0 60px rgba(0, 240, 255, 0.05)',
        'card-hover': '0 8px 32px rgba(0, 240, 255, 0.15), 0 0 1px rgba(0, 240, 255, 0.3)',
        'glass': '0 8px 32px rgba(0, 0, 0, 0.3)',
        'inner-glow': 'inset 0 1px 0 0 rgba(255,255,255,0.05)',
      },
      animation: {
        'fade-in': 'fadeIn 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'slide-up': 'slideUp 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'scale-in': 'scaleIn 0.3s cubic-bezier(0.22, 1, 0.36, 1)',
        'glow-pulse': 'glowPulse 2s ease-in-out infinite',
        'neon-breathe': 'neonBreathe 3s ease-in-out infinite',
        'slide-right': 'slideRight 0.3s cubic-bezier(0.22, 1, 0.36, 1)',
        'float': 'float 6s ease-in-out infinite',
        'shimmer': 'shimmer 2s linear infinite',
        'energy-flow': 'energyFlow 2s linear infinite',
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
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.9)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'scale(1)', filter: 'blur(0)' },
        },
        glowPulse: {
          '0%, 100%': { boxShadow: '0 0 15px rgba(0, 240, 255, 0.2)' },
          '50%': { boxShadow: '0 0 25px rgba(0, 240, 255, 0.4), 0 0 50px rgba(0, 240, 255, 0.1)' },
        },
        neonBreathe: {
          '0%, 100%': { opacity: '0.5' },
          '50%': { opacity: '1' },
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
        energyFlow: {
          '0%': { backgroundPosition: '0% 50%' },
          '100%': { backgroundPosition: '200% 50%' },
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
