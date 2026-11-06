import type { ReactNode } from 'react'
import { Link, Navigate, Route, Routes } from 'react-router-dom'
import { Sparkles } from 'lucide-react'
import Layout from './components/Layout'
import { EmptyState, Loading } from './components/Status'
import { useMe } from './hooks'
import CollectionPage from './pages/CollectionPage'
import ExplorePage from './pages/ExplorePage'
import ItemDetailPage from './pages/ItemDetailPage'
import ItemFormPage from './pages/ItemFormPage'
import LoginPage from './pages/LoginPage'
import { btnPrimary } from './ui'

function RequireAuth({ children }: { children: ReactNode }) {
  const { data: me, isPending } = useMe()
  if (isPending) return <Loading />
  if (!me) return <Navigate to="/login" replace />
  return children
}

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Navigate to="/collection" replace />} />
        <Route path="login" element={<LoginPage />} />
        <Route path="explore" element={<ExplorePage />} />
        <Route path="items/:id" element={<ItemDetailPage />} />
        <Route
          path="collection"
          element={
            <RequireAuth>
              <CollectionPage />
            </RequireAuth>
          }
        />
        <Route
          path="items/new"
          element={
            <RequireAuth>
              <ItemFormPage />
            </RequireAuth>
          }
        />
        <Route
          path="items/:id/edit"
          element={
            <RequireAuth>
              <ItemFormPage />
            </RequireAuth>
          }
        />
        <Route
          path="*"
          element={
            <EmptyState icon={<Sparkles size={40} />} title="Page not found">
              <Link to="/" className={`${btnPrimary} mt-3`}>
                Go home
              </Link>
            </EmptyState>
          }
        />
      </Route>
    </Routes>
  )
}
