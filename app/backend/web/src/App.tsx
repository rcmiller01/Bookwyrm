import { lazy, Suspense } from 'react'
import { Route, Routes } from 'react-router-dom'
import { AppShell } from './layout/AppShell'

const DashboardPage = lazy(() => import('./pages/DashboardPage').then(m => ({ default: m.DashboardPage })))
const AuthorsPage = lazy(() => import('./pages/AuthorsPage').then(m => ({ default: m.AuthorsPage })))
const AuthorDetailPage = lazy(() => import('./pages/AuthorDetailPage').then(m => ({ default: m.AuthorDetailPage })))
const BooksPage = lazy(() => import('./pages/BooksPage').then(m => ({ default: m.BooksPage })))
const ManualSearchPage = lazy(() => import('./pages/ManualSearchPage').then(m => ({ default: m.ManualSearchPage })))
const BookDetailPage = lazy(() => import('./pages/BookDetailPage').then(m => ({ default: m.BookDetailPage })))
const BookHistoryPage = lazy(() => import('./pages/BookHistoryPage').then(m => ({ default: m.BookHistoryPage })))
const SeriesPage = lazy(() => import('./pages/SeriesPage').then(m => ({ default: m.SeriesPage })))
const QueuePage = lazy(() => import('./pages/QueuePage').then(m => ({ default: m.QueuePage })))
const HistoryPage = lazy(() => import('./pages/HistoryPage').then(m => ({ default: m.HistoryPage })))
const ImportListPage = lazy(() => import('./pages/ImportListPage').then(m => ({ default: m.ImportListPage })))
const MissingPage = lazy(() => import('./pages/MissingPage').then(m => ({ default: m.MissingPage })))
const CutoffUnmetPage = lazy(() => import('./pages/CutoffUnmetPage').then(m => ({ default: m.CutoffUnmetPage })))
const MediaManagementPage = lazy(() => import('./pages/MediaManagementPage').then(m => ({ default: m.MediaManagementPage })))
const ProfilesPage = lazy(() => import('./pages/ProfilesPage').then(m => ({ default: m.ProfilesPage })))
const IndexersPage = lazy(() => import('./pages/IndexersPage').then(m => ({ default: m.IndexersPage })))
const DownloadClientsPage = lazy(() => import('./pages/DownloadClientsPage').then(m => ({ default: m.DownloadClientsPage })))
const MetadataPage = lazy(() => import('./pages/MetadataPage').then(m => ({ default: m.MetadataPage })))
const GeneralPage = lazy(() => import('./pages/GeneralPage').then(m => ({ default: m.GeneralPage })))
const StatusPage = lazy(() => import('./pages/StatusPage').then(m => ({ default: m.StatusPage })))
const TasksPage = lazy(() => import('./pages/TasksPage').then(m => ({ default: m.TasksPage })))
const LogsPage = lazy(() => import('./pages/LogsPage').then(m => ({ default: m.LogsPage })))
const PlaceholderPage = lazy(() => import('./pages/PlaceholderPage').then(m => ({ default: m.PlaceholderPage })))

function LoadingFallback() {
  return (
    <div className="flex h-64 items-center justify-center">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-sky-400 border-t-transparent" />
    </div>
  )
}

export function App() {
  return (
    <AppShell>
      <Suspense fallback={<LoadingFallback />}>
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/library/authors" element={<AuthorsPage />} />
          <Route path="/library/authors/:authorID" element={<AuthorDetailPage />} />
          <Route path="/library/books" element={<BooksPage />} />
          <Route path="/library/books/manual-search" element={<ManualSearchPage />} />
          <Route path="/library/books/:workID" element={<BookDetailPage />} />
          <Route path="/library/books/:workID/history" element={<BookHistoryPage />} />
          <Route path="/library/series" element={<SeriesPage />} />
          <Route path="/activity/queue" element={<QueuePage />} />
          <Route path="/activity/history" element={<HistoryPage />} />
          <Route path="/activity/import-list" element={<ImportListPage />} />
          <Route path="/wanted/missing" element={<MissingPage />} />
          <Route path="/wanted/cutoff-unmet" element={<CutoffUnmetPage />} />
          <Route path="/settings/media-management" element={<MediaManagementPage />} />
          <Route path="/settings/profiles" element={<ProfilesPage />} />
          <Route path="/settings/indexers" element={<IndexersPage />} />
          <Route path="/settings/download-clients" element={<DownloadClientsPage />} />
          <Route path="/settings/metadata" element={<MetadataPage />} />
          <Route path="/settings/general" element={<GeneralPage />} />
          <Route path="/system/status" element={<StatusPage />} />
          <Route path="/system/tasks" element={<TasksPage />} />
          <Route path="/system/logs" element={<LogsPage />} />
          <Route path="*" element={<PlaceholderPage title="Not Found" />} />
        </Routes>
      </Suspense>
    </AppShell>
  )
}
