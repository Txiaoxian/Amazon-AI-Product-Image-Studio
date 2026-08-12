import type { ReactNode } from 'react'

interface PageHeaderProps {
  actions?: ReactNode
  description: string
  eyebrow?: string
  title: string
}

export function PageHeader({ actions, description, eyebrow, title }: PageHeaderProps) {
  return (
    <header className="workspace-page-header">
      <div className="min-w-0">
        {eyebrow ? <p className="text-xs font-semibold text-amazon-600">{eyebrow}</p> : null}
        <h1 className="workspace-page-title">{title}</h1>
        <p className="workspace-page-description">{description}</p>
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
    </header>
  )
}
