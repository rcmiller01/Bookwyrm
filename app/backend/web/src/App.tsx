import { Route, Routes } from 'react-router-dom'
import { AuthorDetailPage } from './pages/AuthorDetailPage'
import { AuthorsPage } from './pages/AuthorsPage'
import { BookDetailPage } from './pages/BookDetailPage'
import { BooksPage } from './pages/BooksPage'
import { DownloadClientsPage } from './pages/DownloadClientsPage'
import { AppShell } from './layout/AppShell'
import { DashboardPage } from './pages/DashboardPage'
import { GeneralPage } from './pages/GeneralPage'
import { HistoryPage } from './pages/HistoryPage'
import { IndexersPage } from './pages/IndexersPage'
import { ImportListPage } from './pages/ImportListPage'
import { LogsPage } from './pages/LogsPage'
import { ManualSearchPage } from './pages/ManualSearchPage'
import { MediaManagementPage } from './pages/MediaManagementPage'
import { MetadataPage } from './pages/MetadataPage'
import { ProfilesPage } from './pages/ProfilesPage'
import { CutoffUnmetPage } from './pages/CutoffUnmetPage'
import { MissingPage } from './pages/MissingPage'
import { PlaceholderPage } from './pages/PlaceholderPage'
import { QueuePage } from './pages/QueuePage'
import { SeriesPage } from './pages/SeriesPage'
import { StatusPage } from './pages/StatusPage'
import { TasksPage } from './pages/TasksPage'
import { BookHistoryPage } from './pages/BookHistoryPage'

export function App() {
  return (
    <AppShell>
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
    </AppShell>
  )
}
