import type { ButtonHTMLAttributes, ReactNode } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  icon?: ReactNode
}

const variantClasses: Record<ButtonVariant, string> = {
  primary: 'border-amazon-600 bg-amazon-500 text-ink-900 hover:bg-amazon-400',
  secondary: 'border-ink-200 bg-white text-ink-800 hover:bg-ink-50',
  danger: 'border-red-200 bg-red-50 text-red-700 hover:bg-red-100',
  ghost: 'border-transparent bg-transparent text-ink-600 hover:bg-ink-100 hover:text-ink-900',
}

export function Button({ children, className = '', icon, variant = 'secondary', ...props }: ButtonProps) {
  return (
    <button
      className={`inline-flex min-h-11 items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold transition focus:outline-none focus:ring-2 focus:ring-amazon-500/30 disabled:opacity-55 ${variantClasses[variant]} ${className}`}
      type="button"
      {...props}
    >
      {icon}
      {children}
    </button>
  )
}
