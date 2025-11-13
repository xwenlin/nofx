import { RouterProvider } from 'react-router-dom'
import { ConfirmDialogProvider } from './components/ConfirmDialog'
import { getIconPath } from './components/ModelIcons'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { useSystemConfig } from './hooks/useSystemConfig'
import { t } from './i18n/translations'
import { router } from './routes'

function LoadingScreen() {
  const { language } = useLanguage()

  return (
    <div
      className="min-h-screen flex items-center justify-center"
      style={{ background: '#0B0E11' }}
    >
      <div className="text-center">
        <img
          src={getIconPath('nofx.svg')}
          alt="NoFx Logo"
          className="w-16 h-16 mx-auto mb-4 animate-pulse"
        />
        <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
      </div>
    </div>
  )
}

function AppContent() {
  const { isLoading } = useAuth()
  const { loading: configLoading } = useSystemConfig()

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return <LoadingScreen />
  }

  return <RouterProvider router={router} />
}

export default function App() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <AppContent />
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}
