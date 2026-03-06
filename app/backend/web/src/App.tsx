import { Route, Routes } from 'react-router-dom'
import { DownloadClientsPage } from './pages/DownloadClientsPage'
import { AppShell } from './layout/AppShell'
import { DashboardPage } from './pages/DashboardPage'
import { GeneralPage } from './pages/GeneralPage'
import { HistoryPage } from './pages/HistoryPage'
import { IndexersPage } from './pages/IndexersPage'
import { ImportListPage } from './pages/ImportListPage'
import { ManualSearchPage } from './pages/ManualSearchPage'
import { MediaManagementPage } from './pages/MediaManagementPage'
import { MetadataPage } from './pages/MetadataPage'
import { PlaceholderPage } from './pages/PlaceholderPage'
import { QueuePage } from './pages/QueuePage'

export function App() {
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/library/authors" element={<PlaceholderPage title="Authors" />} />
        <Route path="/library/books" element={<ManualSearchPage />} />
        <Route path="/library/series" element={<PlaceholderPage title="Series" />} />
        <Route path="/activity/queue" element={<QueuePage />} />
        <Route path="/activity/history" element={<HistoryPage />} />
        <Route path="/activity/import-list" element={<ImportListPage />} />
        <Route path="/wanted/missing" element={<PlaceholderPage title="Missing" />} />
        <Route path="/settings/media-management" element={<MediaManagementPage />} />
        <Route path="/settings/indexers" element={<IndexersPage />} />
        <Route path="/settings/download-clients" element={<DownloadClientsPage />} />
        <Route path="/settings/metadata" element={<MetadataPage />} />
        <Route path="/settings/general" element={<GeneralPage />} />
        <Route path="/system/status" element={<PlaceholderPage title="Status" />} />
        <Route path="/system/tasks" element={<PlaceholderPage title="Tasks" />} />
        <Route path="/system/logs" element={<PlaceholderPage title="Logs" />} />
        <Route path="*" element={<PlaceholderPage title="Not Found" />} />
      </Routes>
    </AppShell>
  )
}
