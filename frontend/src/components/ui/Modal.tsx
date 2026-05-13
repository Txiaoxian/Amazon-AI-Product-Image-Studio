import { X } from 'lucide-react'
import type { ReactNode } from 'react'

interface ModalProps {
  title: string
  isOpen: boolean
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  maxWidthClass?: string
}

export function Modal({ title, isOpen, onClose, children, footer, maxWidthClass = 'max-w-3xl' }: ModalProps) {
  if (!isOpen) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/45 px-4 py-6" role="presentation">
      <div
        aria-modal="true"
        className={`flex max-h-[90vh] w-full ${maxWidthClass} flex-col rounded-lg bg-white shadow-2xl`}
        role="dialog"
      >
        <header className="flex items-center justify-between border-b border-ink-200 px-5 py-4">
          <h2 className="text-base font-semibold text-ink-900">{title}</h2>
          <button aria-label="关闭弹窗" className="icon-button" onClick={onClose} type="button">
            <X className="h-4 w-4" />
          </button>
        </header>
        <div className="overflow-y-auto px-5 py-4">{children}</div>
        {footer ? <footer className="border-t border-ink-200 px-5 py-4">{footer}</footer> : null}
      </div>
    </div>
  )
}
