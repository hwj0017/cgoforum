import { Navigate, Route, Routes } from 'react-router-dom'
import ArticleDetailModal from './components/ArticleDetailModal'
import ShellLayout from './components/ShellLayout'
import AuthPage from './pages/AuthPage'
import ComposePage from './pages/ComposePage'
import ExplorePage from './pages/ExplorePage'
import FeedPage from './pages/FeedPage'
import './App.css'

function App() {
  return (
    <>
      <Routes>
        <Route element={<ShellLayout />}>
          <Route index element={<ExplorePage />} />
          <Route path="feed" element={<FeedPage />} />
          <Route path="compose" element={<ComposePage />} />
          <Route path="auth" element={<AuthPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ArticleDetailModal />
    </>
  )
}

export default App
