import { X } from 'lucide-react'
import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from 'react'

interface EditorDrawerProps {
  title: string
  isOpen: boolean
  onClose: () => void
  children: ReactNode
  widthClass?: string
}

export function EditorDrawer({ title, isOpen, onClose, children, widthClass = 'max-w-xl' }: EditorDrawerProps) {
  const titleId = useId()
  const drawerRef = useRef<HTMLDivElement>(null)
  const closeButtonRef = useRef<HTMLButtonElement>(null)
  const restoreFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!isOpen) {
      return
    }

    restoreFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    closeButtonRef.current?.focus()

    return () => {
      restoreFocusRef.current?.focus()
      restoreFocusRef.current = null
    }
  }, [isOpen])

  useEffect(() => {
    if (!isOpen) {
      return
    }

    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previousOverflow
    }
  }, [isOpen])

  if (!isOpen) {
    return null
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    event.stopPropagation()
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
      return
    }

    if (event.key !== 'Tab') {
      return
    }

    const focusableElements = getFocusableElements(drawerRef.current)
    if (focusableElements.length === 0) {
      event.preventDefault()
      drawerRef.current?.focus()
      return
    }

    const firstElement = focusableElements[0]
    const lastElement = focusableElements[focusableElements.length - 1]
    const activeElement = document.activeElement

    if (event.shiftKey && (activeElement === firstElement || !drawerRef.current?.contains(activeElement))) {
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
    <div className="fixed inset-0 z-[60] flex justify-end bg-ink-900/25" role="presentation">
      <div
        aria-labelledby={titleId}
        aria-modal="true"
        className={`flex h-[100dvh] w-full ${widthClass} flex-col overflow-hidden border-l border-ink-200 bg-white shadow-2xl`}
        onKeyDown={handleKeyDown}
        ref={drawerRef}
        role="dialog"
        tabIndex={-1}
      >
        <header className="flex shrink-0 items-center justify-between border-b border-ink-200 px-4 py-3 sm:px-5 sm:py-4">
          <h2 className="text-base font-semibold text-ink-900" id={titleId}>
            {title}
          </h2>
          <button aria-label="关闭编辑面板" className="icon-button" onClick={onClose} ref={closeButtonRef} type="button">
            <X className="h-4 w-4" />
          </button>
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-5" data-testid="editor-drawer-body">
          {children}
        </div>
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
