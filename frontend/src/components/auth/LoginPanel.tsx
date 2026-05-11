import { LogIn } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import type { LoginRequest } from '../../types/auth'
import { Button } from '../ui/Button'

interface LoginPanelProps {
  error?: string | null
  isSubmitting: boolean
  onSubmit: (request: LoginRequest) => Promise<boolean>
}

export function LoginPanel({ error, isSubmitting, onSubmit }: LoginPanelProps) {
  const [tenantId, setTenantId] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const trimmedTenantId = tenantId.trim()

    const didLogin = await onSubmit({
      tenantId: trimmedTenantId || undefined,
      email: email.trim(),
      password,
    })

    if (didLogin) {
      setPassword('')
    }
  }

  return (
    <div className="mx-auto flex min-h-[calc(100dvh-180px)] w-full max-w-md items-center">
      <section className="panel w-full p-6">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-ink-900">登录</h2>
          <p className="text-sm leading-6 text-ink-500">使用平台账号进入图片工作台。</p>
        </div>

        {error ? (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm leading-6 text-red-700" role="alert">
            {error}
          </div>
        ) : null}

        <form className="mt-5 space-y-4" onSubmit={handleSubmit}>
          <label className="space-y-2">
            <span className="field-label">租户 ID</span>
            <input
              autoComplete="organization"
              className="field-input"
              disabled={isSubmitting}
              onChange={(event) => setTenantId(event.target.value)}
              placeholder="多租户环境需要填写"
              value={tenantId}
            />
          </label>

          <label className="space-y-2">
            <span className="field-label">邮箱</span>
            <input
              autoComplete="email"
              className="field-input"
              disabled={isSubmitting}
              onChange={(event) => setEmail(event.target.value)}
              required
              type="email"
              value={email}
            />
          </label>

          <label className="space-y-2">
            <span className="field-label">密码</span>
            <input
              autoComplete="current-password"
              className="field-input"
              disabled={isSubmitting}
              onChange={(event) => setPassword(event.target.value)}
              required
              type="password"
              value={password}
            />
          </label>

          <Button className="w-full" disabled={isSubmitting} icon={<LogIn className="h-4 w-4" />} type="submit" variant="primary">
            {isSubmitting ? '登录中...' : '登录'}
          </Button>
        </form>
      </section>
    </div>
  )
}
