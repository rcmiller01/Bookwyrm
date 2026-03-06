import { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'

type NavItem = {
  section: string
  items: { label: string; path: string }[]
}

const navItems: NavItem[] = [
  { section: 'Dashboard', items: [{ label: 'Dashboard', path: '/' }] },
  {
    section: 'Library',
    items: [
      { label: 'Authors', path: '/library/authors' },
      { label: 'Books', path: '/library/books' },
      { label: 'Series', path: '/library/series' }
    ]
  },
  {
    section: 'Activity',
    items: [
      { label: 'Queue', path: '/activity/queue' },
      { label: 'History', path: '/activity/history' },
      { label: 'Import List', path: '/activity/import-list' }
    ]
  },
  { section: 'Wanted', items: [{ label: 'Missing', path: '/wanted/missing' }] },
  {
    section: 'Settings',
    items: [
      { label: 'Media Management', path: '/settings/media-management' },
      { label: 'Indexers', path: '/settings/indexers' },
      { label: 'Download Clients', path: '/settings/download-clients' },
      { label: 'Metadata', path: '/settings/metadata' },
      { label: 'General', path: '/settings/general' }
    ]
  },
  {
    section: 'System',
    items: [
      { label: 'Status', path: '/system/status' },
      { label: 'Tasks', path: '/system/tasks' },
      { label: 'Logs', path: '/system/logs' }
    ]
  }
]

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen bg-slate-950">
      <aside className="w-72 border-r border-slate-800 bg-slate-900/70 p-4">
        <h1 className="mb-6 text-xl font-semibold tracking-wide text-slate-100">Bookwyrm</h1>
        <nav className="space-y-6">
          {navItems.map((group) => (
            <div key={group.section}>
              <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">{group.section}</p>
              <ul className="space-y-1">
                {group.items.map((item) => (
                  <li key={item.path}>
                    <NavLink
                      to={item.path}
                      className={({ isActive }) =>
                        [
                          'block rounded px-3 py-2 text-sm transition-colors',
                          isActive ? 'bg-sky-600/25 text-sky-300' : 'text-slate-200 hover:bg-slate-800'
                        ].join(' ')
                      }
                    >
                      {item.label}
                    </NavLink>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
      </aside>
      <main className="flex-1 p-6">{children}</main>
    </div>
  )
}
