import { X } from 'lucide-react'
import { useEffect, useId, useRef, type ReactNode } from 'react'

interface ContextDrawerProps {
  children: ReactNode
  description?: string
  isOpen: boolean
  onClose: () => void
  title: string
}

export function ContextDrawer({ children, description, isOpen, onClose, title }: ContextDrawerProps) {
  const titleId = useId()
  const restoreFocusRef = useRef<HTMLElement | null>(null)
  const closeButtonRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!isOpen) return

    restoreFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    closeButtonRef.current?.focus()
    const handleEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handleEscape)
    return () => {
      window.removeEventListener('keydown', handleEscape)
      restoreFocusRef.current?.focus()
      restoreFocusRef.current = null
    }
  }, [isOpen, onClose])

  if (!isOpen) return null

  return (
    <aside aria-labelledby={titleId} className="context-drawer" role="complementary">
      <header className="context-drawer-header">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold text-ink-900" id={titleId}>{title}</h2>
          {description ? <p className="mt-1 text-xs text-ink-500">{description}</p> : null}
        </div>
        <button aria-label="关闭快捷面板" className="icon-button shrink-0" onClick={onClose} ref={closeButtonRef} type="button">
          <X className="h-4 w-4" />
        </button>
      </header>
      <div className="context-drawer-body">{children}</div>
    </aside>
  )
}
