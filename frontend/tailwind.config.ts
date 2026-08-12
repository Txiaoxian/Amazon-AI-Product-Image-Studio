import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: {
          50: '#f8fafc',
          100: '#eef2f7',
          200: '#dbe3ee',
          300: '#b8c4d4',
          400: '#8392a8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1f2937',
          900: '#111827',
        },
        amazon: {
          50: '#fff7e6',
          100: '#ffebc2',
          200: '#ffd58a',
          300: '#ffc45c',
          400: '#f7b733',
          500: '#ff9900',
          600: '#dd7d00',
          700: '#b86600',
          800: '#8f4f00',
        },
      },
      borderRadius: {
        md: '8px',
        lg: '8px',
        xl: '12px',
      },
      boxShadow: {
        panel: '0 1px 2px rgba(15, 23, 42, 0.06), 0 8px 24px rgba(15, 23, 42, 0.06)',
      },
    },
  },
  plugins: [],
} satisfies Config
