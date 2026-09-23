import type { ReactNode } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { LoadingSpinner } from './components/LoadingSpinner'
import { useAuth } from './context/AuthContext'
import { HistoryPage } from './pages/HistoryPage'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { MissionFormPage } from './pages/MissionFormPage'
import { MissionsPage } from './pages/MissionsPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { RegisterPage } from './pages/RegisterPage'
import { SettingsPage } from './pages/SettingsPage'

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <LoadingSpinner label="読み込み中..." />
      </div>
    )
  }
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

function RedirectIfAuthed({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) return null
  if (user) return <Navigate to="/" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <RedirectIfAuthed>
            <LoginPage />
          </RedirectIfAuthed>
        }
      />
      <Route
        path="/register"
        element={
          <RedirectIfAuthed>
            <RegisterPage />
          </RedirectIfAuthed>
        }
      />

      <Route
        path="/"
        element={
          <RequireAuth>
            <Layout>
              <HomePage />
            </Layout>
          </RequireAuth>
        }
      />
      <Route
        path="/missions"
        element={
          <RequireAuth>
            <Layout>
              <MissionsPage />
            </Layout>
          </RequireAuth>
        }
      />
      <Route
        path="/missions/new"
        element={
          <RequireAuth>
            <Layout>
              <MissionFormPage />
            </Layout>
          </RequireAuth>
        }
      />
      <Route
        path="/missions/:id/edit"
        element={
          <RequireAuth>
            <Layout>
              <MissionFormPage />
            </Layout>
          </RequireAuth>
        }
      />
      <Route
        path="/history"
        element={
          <RequireAuth>
            <Layout>
              <HistoryPage />
            </Layout>
          </RequireAuth>
        }
      />
      <Route
        path="/settings"
        element={
          <RequireAuth>
            <Layout>
              <SettingsPage />
            </Layout>
          </RequireAuth>
        }
      />

      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
