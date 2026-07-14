import { Loader2, LogIn, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { authApi as defaultAuthApi, type AuthApi } from '../../api/auth'
import type { CaptchaChallenge, LoginRequest } from '../../types/auth'
import { Button } from '../ui/Button'

interface LoginPanelProps {
  error?: string | null
  isSubmitting: boolean
  onSubmit: (request: LoginRequest) => Promise<boolean>
  authApi?: Pick<AuthApi, 'createCaptcha'>
}

export function LoginPanel({ authApi = defaultAuthApi, error, isSubmitting, onSubmit }: LoginPanelProps) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [captchaCode, setCaptchaCode] = useState('')
  const [captcha, setCaptcha] = useState<CaptchaChallenge | null>(null)
  const [isLoadingCaptcha, setLoadingCaptcha] = useState(false)
  const [captchaError, setCaptchaError] = useState<string | null>(null)

  const refreshCaptcha = useCallback(async () => {
    setLoadingCaptcha(true)
    setCaptchaError(null)
    setCaptchaCode('')
    try {
      setCaptcha(await authApi.createCaptcha())
    } catch {
      setCaptcha(null)
      setCaptchaError('验证码加载失败，管理员仍可直接登录；普通用户请刷新重试。')
    } finally {
      setLoadingCaptcha(false)
    }
  }, [authApi])

  useEffect(() => {
    void refreshCaptcha()
  }, [refreshCaptcha])

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const normalizedCaptchaCode = captchaCode.trim()

    const didLogin = await onSubmit({
      email: email.trim(),
      password,
      ...(captcha && normalizedCaptchaCode
        ? { captchaId: captcha.captchaId, captchaCode: normalizedCaptchaCode }
        : {}),
    })

    if (didLogin) {
      setPassword('')
      setCaptchaCode('')
    } else {
      void refreshCaptcha()
    }
  }

  return (
    <div className="mx-auto flex min-h-[calc(100dvh-180px)] w-full max-w-md items-center">
      <section className="panel w-full p-6">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-ink-900">登录</h2>
          <p className="text-sm leading-6 text-ink-500">使用平台账号进入图片工作台，无需填写租户信息。</p>
        </div>

        {error ? (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm leading-6 text-red-700" role="alert">
            {error}
          </div>
        ) : null}

        <form className="mt-5 space-y-4" onSubmit={handleSubmit}>
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

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <label className="field-label" htmlFor="login-captcha">验证码（普通用户必填）</label>
              <button
                aria-label="刷新验证码"
                className="icon-button h-8 w-8"
                disabled={isLoadingCaptcha || isSubmitting}
                onClick={() => void refreshCaptcha()}
                title="刷新验证码"
                type="button"
              >
                {isLoadingCaptcha ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              </button>
            </div>
            <div className="grid grid-cols-[minmax(0,1fr)_160px] gap-2">
              <input
                autoComplete="off"
                className="field-input"
                disabled={isSubmitting}
                id="login-captcha"
                inputMode="numeric"
                maxLength={8}
                onChange={(event) => setCaptchaCode(event.target.value)}
                placeholder="管理员可留空"
                value={captchaCode}
              />
              <div className="flex h-14 items-center justify-center overflow-hidden rounded-md border border-ink-200 bg-ink-50">
                {captcha ? <img alt="登录验证码" className="h-full w-full object-cover" src={captcha.imageUrl} /> : null}
                {!captcha && isLoadingCaptcha ? <Loader2 className="h-5 w-5 animate-spin text-ink-400" /> : null}
                {!captcha && !isLoadingCaptcha ? <span className="text-xs text-ink-400">未加载</span> : null}
              </div>
            </div>
            {captchaError ? <p className="text-xs leading-5 text-red-600">{captchaError}</p> : null}
            <p className="text-xs leading-5 text-ink-400">验证码一次有效；管理员账号可以不填写。</p>
          </div>

          <Button className="w-full" disabled={isSubmitting} icon={<LogIn className="h-4 w-4" />} type="submit" variant="primary">
            {isSubmitting ? '登录中...' : '登录'}
          </Button>
        </form>
      </section>
    </div>
  )
}
