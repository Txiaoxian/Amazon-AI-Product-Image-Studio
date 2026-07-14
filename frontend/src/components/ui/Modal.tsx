import { X } from 'lucide-react'
import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from 'react'

interface ModalProps {
  title: string
  isOpen: boolean
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  maxWidthClass?: string
}

export function Modal({ title, isOpen, onClose, children, footer, maxWidthClass = 'max-w-3xl' }: ModalProps) {
  const titleId = useId()
  const dialogRef = useRef<HTMLDivElement>(null)
  const closeButtonRef = useRef<HTMLButtonElement>(null)
  const restoreFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!isOpen) {
      return
    }

    restoreFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    closeButtonRef.current?.focus()

    return () => {
      document.body.style.overflow = previousOverflow
      restoreFocusRef.current?.focus()
      restoreFocusRef.current = null
    }
  }, [isOpen])

  if (!isOpen) {
    return null
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      onClose()
      return
    }

    if (event.key !== 'Tab') {
      return
    }

    event.stopPropagation()

    const focusableElements = getFocusableElements(dialogRef.current)
    if (focusableElements.length === 0) {
      event.preventDefault()
      dialogRef.current?.focus()
      return
    }

    const firstElement = focusableElements[0]
    const lastElement = focusableElements[focusableElements.length - 1]
    const activeElement = document.activeElement

    if (event.shiftKey && (activeElement === firstElement || !dialogRef.current?.contains(activeElement))) {
      event.preventDefault()
      lastElement.focus()
      return
    }

    if (!event.shiftKey && activeElement === lastElement) {
      event.preventDefault()
      firstElement.focus()
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-stretch justify-center bg-ink-900/45 sm:items-center sm:px-4 sm:py-6" role="presentation">
      <div
        aria-labelledby={titleId}
        aria-modal="true"
        className={`flex h-[100dvh] max-h-[100dvh] w-full ${maxWidthClass} flex-col bg-white shadow-2xl sm:h-auto sm:max-h-[90vh] sm:rounded-lg`}
        onKeyDown={handleKeyDown}
        ref={dialogRef}
        role="dialog"
        tabIndex={-1}
      >
        <div className="flex shrink-0 items-center justify-between border-b border-ink-200 px-4 py-3 sm:px-5 sm:py-4">
          <h2 className="text-base font-semibold text-ink-900" id={titleId}>
            {title}
          </h2>
          <button aria-label="关闭弹窗" className="icon-button" onClick={onClose} ref={closeButtonRef} type="button">
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-5">{children}</div>
        {footer ? <footer className="shrink-0 border-t border-ink-200 px-4 py-3 sm:px-5 sm:py-4">{footer}</footer> : null}
      </div>
    </div>
  )
}

function getFocusableElements(container: HTMLElement | null): HTMLElement[] {
  if (!container) {
    return []
  }

  return Array.from(
    container.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((element) => element.getAttribute('aria-hidden') !== 'true')
}
