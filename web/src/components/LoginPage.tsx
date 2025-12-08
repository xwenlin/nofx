import { Eye, EyeOff } from 'lucide-react'
import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { getIconPath } from './ModelIcons'
import { Input } from './ui/input'

export function LoginPage() {
  const { language } = useLanguage()
  const { login, loginAdmin, verifyOTP, completeRegistration } = useAuth()
  const navigate = useNavigate()
  const [step, setStep] = useState<'login' | 'setup-otp' | 'otp'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [otpCode, setOtpCode] = useState('')
  const [userID, setUserID] = useState('')
  const [otpSecret, setOtpSecret] = useState('')
  const [qrCodeURL, setQrCodeURL] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [adminPassword, setAdminPassword] = useState('')
  const adminMode = false

  const handleAdminLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = await loginAdmin(adminPassword)
    if (!result.success) {
      const msg = result.message || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    }
    setLoading(false)
  }

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await login(email, password);

    if (result.success) {
      // 需要完成OTP设置（首次登录）
      if (result.requiresOTPSetup && result.userID) {
        setUserID(result.userID)
        setOtpSecret(result.otpSecret || '')
        setQrCodeURL(result.qrCodeURL || '')
        setStep('setup-otp')
      } else if (result.requiresOTP && result.userID) {
        // 需要输入OTP验证码（已完成OTP设置）
        setUserID(result.userID)
        setStep('otp')
      }
    } else {
      const msg = result.message || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    }

    setLoading(false);
  };

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    // 如果是setup-otp步骤，使用completeRegistration
    if (step === 'setup-otp') {
      const result = await completeRegistration(userID, otpCode);
      if (!result.success) {
        const msg = result.message || t('verificationFailed', language)
        setError(msg)
        toast.error(msg)
      }
      // 成功的话AuthContext会自动处理登录状态
    } else {
      // 如果是otp步骤，使用verifyOTP
      const result = await verifyOTP(userID, otpCode);
      if (!result.success) {
        const msg = result.message || t('verificationFailed', language)
        setError(msg)
        toast.error(msg)
      }
      // 成功的话AuthContext会自动处理登录状态
    }

    setLoading(false);
  };

  return (
    <div
      className="flex items-center justify-center py-12"
      style={{ minHeight: 'calc(100vh - 64px)' }}
    >
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 mx-auto mb-4 flex items-center justify-center">
            <img
              src={getIconPath('nofx.svg')}
              alt="NoFx Logo"
              className="w-16 h-16 object-contain"
            />
          </div>
          <h1
            className="text-2xl font-bold"
            style={{ color: 'var(--brand-light-gray)' }}
          >
            登录 NOFX
          </h1>
          <p
            className="text-sm mt-2"
            style={{ color: 'var(--text-secondary)' }}
          >
            {step === 'login' && '请输入您的邮箱和密码'}
            {step === 'setup-otp' && t('setupTwoFactor', language)}
            {step === 'otp' && '请输入两步验证码'}
          </p>
        </div>

        {/* Login Form */}
        <div
          className="rounded-lg p-6"
          style={{
            background: 'var(--panel-bg)',
            border: '1px solid var(--panel-border)',
          }}
        >
          {adminMode ? (
            <form onSubmit={handleAdminLogin} className="space-y-4">
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  管理员密码
                </label>
                <input
                  type="password"
                  value={adminPassword}
                  onChange={(e) => setAdminPassword(e.target.value)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: 'var(--brand-black)',
                    border: '1px solid var(--panel-border)',
                    color: 'var(--brand-light-gray)',
                  }}
                  placeholder="请输入管理员密码"
                  required
                />
              </div>

              {error && (
                <div
                  className="text-sm px-3 py-2 rounded"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                style={{
                  background: 'var(--brand-yellow)',
                  color: 'var(--brand-black)',
                }}
              >
                {loading ? t('loading', language) : '登录'}
              </button>
            </form>
          ) : step === 'login' ? (
            <form onSubmit={handleLogin} className="space-y-4">
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('email', language)}
                </label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t('emailPlaceholder', language)}
                  required
                />
              </div>

              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('password', language)}
                </label>
                <div className="relative">
                  <Input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="pr-10"
                    placeholder={t('passwordPlaceholder', language)}
                    required
                  />
                  <button
                    type="button"
                    aria-label={showPassword ? '隐藏密码' : '显示密码'}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => setShowPassword((v) => !v)}
                    className="absolute inset-y-0 right-2 w-8 h-10 flex items-center justify-center rounded bg-transparent p-0 m-0 border-0 outline-none focus:outline-none focus:ring-0 appearance-none cursor-pointer btn-icon"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </div>
                <div className="text-right mt-2">
                  <button
                    type="button"
                    onClick={() => navigate('/reset-password')}
                    className="text-xs hover:underline"
                    style={{ color: '#F0B90B' }}
                  >
                    {t('forgotPassword', language)}
                  </button>
                </div>
              </div>

              {error && (
                <div
                  className="text-sm px-3 py-2 rounded"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                style={{
                  background: 'var(--brand-yellow)',
                  color: 'var(--brand-black)',
                }}
              >
                {loading ? t('loading', language) : t('loginButton', language)}
              </button>
            </form>
          ) : step === 'setup-otp' ? (
            <div className="space-y-4">
              <div className="text-center">
                <div className="text-4xl mb-2">📱</div>
                <h3
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('setupTwoFactor', language)}
                </h3>
                <p className="text-sm" style={{ color: '#848E9C' }}>
                  {t('setupTwoFactorDesc', language)}
                </p>
              </div>

              <div className="space-y-3">
                <div
                  className="p-3 rounded"
                  style={{
                    background: 'var(--brand-black)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <p
                    className="text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('authStep1Title', language)}
                  </p>
                  <p
                    className="text-xs"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('authStep1Desc', language)}
                  </p>
                </div>

                <div
                  className="p-3 rounded"
                  style={{
                    background: 'var(--brand-black)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <p
                    className="text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('authStep2Title', language)}
                  </p>
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {t('authStep2Desc', language)}
                  </p>

                  {qrCodeURL && (
                    <div className="mt-2">
                      <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                        {t('qrCodeHint', language)}
                      </p>
                      <div className="bg-white p-2 rounded text-center">
                        <img
                          src={`https://api.qrserver.com/v1/create-qr-code/?size=150x150&data=${encodeURIComponent(qrCodeURL)}`}
                          alt="QR Code"
                          className="mx-auto"
                        />
                      </div>
                    </div>
                  )}

                  <div className="mt-2">
                    <p className="text-xs mb-1" style={{ color: '#848E9C' }}>
                      {t('otpSecret', language)}
                    </p>
                    <div className="flex items-center gap-2">
                      <code
                        className="flex-1 px-2 py-1 text-xs rounded font-mono"
                        style={{
                          background: 'var(--panel-bg-hover)',
                          color: 'var(--brand-light-gray)',
                        }}
                      >
                        {otpSecret}
                      </code>
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(otpSecret)
                          toast.success(t('copied', language))
                        }}
                        className="px-2 py-1 text-xs rounded"
                        style={{
                          background: 'var(--brand-yellow)',
                          color: 'var(--brand-black)',
                        }}
                      >
                        {t('copy', language)}
                      </button>
                    </div>
                  </div>
                </div>

                <div
                  className="p-3 rounded"
                  style={{
                    background: 'var(--brand-black)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <p
                    className="text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('authStep3Title', language)}
                  </p>
                  <p className="text-xs mb-3" style={{ color: '#848E9C' }}>
                    {t('authStep3Desc', language)}
                  </p>

                  <form onSubmit={handleOTPVerify} className="space-y-3">
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        {t('otpCode', language)}
                      </label>
                      <input
                        type="text"
                        value={otpCode}
                        onChange={(e) =>
                          setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                        }
                        className="w-full px-3 py-2 rounded text-center text-2xl font-mono"
                        style={{
                          background: 'var(--brand-black)',
                          border: '1px solid var(--panel-border)',
                          color: 'var(--brand-light-gray)',
                        }}
                        placeholder={t('otpPlaceholder', language)}
                        maxLength={6}
                        required
                      />
                    </div>

                    {error && (
                      <div
                        className="text-sm px-3 py-2 rounded"
                        style={{
                          background: 'var(--binance-red-bg)',
                          color: 'var(--binance-red)',
                        }}
                      >
                        {error}
                      </div>
                    )}

                    <div className="flex gap-3">
                      <button
                        type="button"
                        onClick={() => setStep('login')}
                        className="flex-1 px-4 py-2 rounded text-sm font-semibold"
                        style={{
                          background: 'var(--panel-bg-hover)',
                          color: 'var(--text-secondary)',
                        }}
                      >
                        {t('back', language)}
                      </button>
                      <button
                        type="submit"
                        disabled={loading || otpCode.length !== 6}
                        className="flex-1 px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                        style={{ background: '#F0B90B', color: '#000' }}
                      >
                        {loading ? t('loading', language) : t('verifyOTP', language)}
                      </button>
                    </div>
                  </form>
                </div>
              </div>
            </div>
          ) : (
            <form onSubmit={handleOTPVerify} className="space-y-4">
              <div className="text-center mb-4">
                <div className="text-4xl mb-2">📱</div>
                <p className="text-sm" style={{ color: '#848E9C' }}>
                  {t('scanQRCodeInstructions', language)}
                  <br />
                  {t('enterOTPCode', language)}
                </p>
              </div>

              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('otpCode', language)}
                </label>
                <input
                  type="text"
                  value={otpCode}
                  onChange={(e) =>
                    setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                  }
                  className="w-full px-3 py-2 rounded text-center text-2xl font-mono"
                  style={{
                    background: 'var(--brand-black)',
                    border: '1px solid var(--panel-border)',
                    color: 'var(--brand-light-gray)',
                  }}
                  placeholder={t('otpPlaceholder', language)}
                  maxLength={6}
                  required
                />
              </div>

              {error && (
                <div
                  className="text-sm px-3 py-2 rounded"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {error}
                </div>
              )}

              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={() => setStep('login')}
                  className="flex-1 px-4 py-2 rounded text-sm font-semibold"
                  style={{
                    background: 'var(--panel-bg-hover)',
                    color: 'var(--text-secondary)',
                  }}
                >
                  {t('back', language)}
                </button>
                <button
                  type="submit"
                  disabled={loading || otpCode.length !== 6}
                  className="flex-1 px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                  style={{ background: '#F0B90B', color: '#000' }}
                >
                  {loading ? t('loading', language) : t('verifyOTP', language)}
                </button>
              </div>
            </form>
          )}
        </div>

        {/* Register Link */}
        {!adminMode && (
          <div className="text-center mt-6">
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              还没有账户？{' '}
              <button
                onClick={() => navigate('/register')}
                className="font-semibold hover:underline transition-colors"
                style={{ color: 'var(--brand-yellow)' }}
              >
                立即注册
              </button>
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
