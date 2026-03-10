import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PageHeader } from '../components/PageHeader'
import { useToast } from '../components/ToastProvider'
import { fetchJSON, postJSON } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'
import { buildWantedPayload } from '../lib/wantedPayload'

type SearchAuthor = { id?: string; name?: string }
type SearchWork = { id?: string; title?: string; authors?: SearchAuthor[] }
type SearchResponse = { works?: SearchWork[] }

type WantedAuthor = { author_id: string; formats?: string[]; languages?: string[] }
type WantedAuthorsResponse = { items: WantedAuthor[] }
type WantedWork = { work_id: string; formats?: string[]; languages?: string[] }
type WantedWorksResponse = { items: WantedWork[] }

export function AddNewPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [query, setQuery] = useState('')

  const wantedAuthorsQuery = useQuery({
    queryKey: ['wanted', 'authors', 'add-new'],
    queryFn: () => fetchJSON<WantedAuthorsResponse>('/ui-api/indexer/wanted/authors'),
    refetchInterval: 30000
  })
  const wantedWorksQuery = useQuery({
    queryKey: ['wanted', 'works', 'add-new'],
    queryFn: () => fetchJSON<WantedWorksResponse>('/ui-api/indexer/wanted/works'),
    refetchInterval: 30000
  })

  const searchQuery = useQuery({
    queryKey: ['library', 'add-new', 'search', query.trim()],
    queryFn: () => fetchJSON<SearchResponse>(`/api/v1/search?q=${encodeURIComponent(query.trim())}`),
    enabled: query.trim().length >= 2
  })

  const addAuthorMutation = useMutation({
    mutationFn: async (authorID: string) => {
      await postJSON(
        `/ui-api/indexer/wanted/authors/${encodeURIComponent(authorID)}`,
        buildWantedPayload({
          enabled: true
        })
      )
    },
    onSuccess: async () => {
      pushToast('Author added to wanted list')
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'authors'] })
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'authors', 'add-new'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const addWorkMutation = useMutation({
    mutationFn: async (payload: { workID: string; title: string }) => {
      await postJSON(
        `/ui-api/indexer/wanted/works/${encodeURIComponent(payload.workID)}`,
        buildWantedPayload({
          enabled: true
        })
      )
      await postJSON(`/ui-api/indexer/search/work/${encodeURIComponent(payload.workID)}`, { title: payload.title })
    },
    onSuccess: async () => {
      pushToast('Book added and search queued')
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'works'] })
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'works', 'add-new'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const wantedAuthorSet = useMemo(() => new Set((wantedAuthorsQuery.data?.items ?? []).map((item) => item.author_id)), [wantedAuthorsQuery.data?.items])
  const wantedWorkSet = useMemo(() => new Set((wantedWorksQuery.data?.items ?? []).map((item) => item.work_id)), [wantedWorksQuery.data?.items])

  const rows = useMemo(() => {
    const works = searchQuery.data?.works ?? []
    const authorMap = new Map<string, string>()
    for (const work of works) {
      for (const author of work.authors ?? []) {
        const authorID = author.id?.trim()
        const name = author.name?.trim()
        if (!authorID || !name || authorMap.has(authorID)) continue
        authorMap.set(authorID, name)
      }
    }
    return {
      works,
      authors: Array.from(authorMap.entries()).map(([id, name]) => ({ id, name }))
    }
  }, [searchQuery.data?.works])

  return (
    <section className="space-y-4">
      <PageHeader title="Add New" subtitle="Search authors/books and add them to monitoring." />

      <div className="rounded border border-slate-800 bg-slate-900/50 p-3">
        <label className="block text-sm text-slate-300">
          Search
          <input
            className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-100"
            placeholder="Title, author, ISBN..."
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </label>
      </div>

      {query.trim().length > 0 && query.trim().length < 2 ? <p className="text-sm text-slate-400">Type at least 2 characters.</p> : null}
      {searchQuery.isLoading ? <p className="text-sm text-slate-400">Searching...</p> : null}

      {rows.works.length > 0 ? (
        <div className="rounded border border-slate-800 bg-slate-900/50">
          <div className="border-b border-slate-800 px-3 py-2 text-sm font-medium text-slate-200">Books</div>
          <ul>
            {rows.works.map((work) => {
              const workID = work.id?.trim() || ''
              const title = work.title?.trim() || workID || 'Unknown'
              const subtitle = (work.authors ?? []).map((author) => author.name?.trim()).filter(Boolean).join(', ')
              const alreadyAdded = workID ? wantedWorkSet.has(workID) : false
              return (
                <li key={`work:${workID || title}`} className="flex items-center justify-between gap-3 border-t border-slate-800 px-3 py-2 text-sm">
                  <div className="min-w-0">
                    <p className="truncate text-slate-100">{title}</p>
                    <p className="truncate text-xs text-slate-400">{subtitle || workID}</p>
                  </div>
                  <button
                    className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300 disabled:opacity-60"
                    disabled={!workID || alreadyAdded || addWorkMutation.isPending}
                    onClick={() => addWorkMutation.mutate({ workID, title })}
                  >
                    {alreadyAdded ? 'Added' : 'Add'}
                  </button>
                </li>
              )
            })}
          </ul>
        </div>
      ) : null}

      {rows.authors.length > 0 ? (
        <div className="rounded border border-slate-800 bg-slate-900/50">
          <div className="border-b border-slate-800 px-3 py-2 text-sm font-medium text-slate-200">Authors</div>
          <ul>
            {rows.authors.map((author) => {
              const alreadyAdded = wantedAuthorSet.has(author.id)
              return (
                <li key={`author:${author.id}`} className="flex items-center justify-between gap-3 border-t border-slate-800 px-3 py-2 text-sm">
                  <div className="min-w-0">
                    <p className="truncate text-slate-100">{author.name}</p>
                    <p className="truncate text-xs text-slate-400">{author.id}</p>
                  </div>
                  <button
                    className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300 disabled:opacity-60"
                    disabled={alreadyAdded || addAuthorMutation.isPending}
                    onClick={() => addAuthorMutation.mutate(author.id)}
                  >
                    {alreadyAdded ? 'Added' : 'Add'}
                  </button>
                </li>
              )
            })}
          </ul>
        </div>
      ) : null}

      {query.trim().length >= 2 && !searchQuery.isLoading && rows.works.length === 0 && rows.authors.length === 0 ? (
        <p className="text-sm text-slate-400">No results found.</p>
      ) : null}
    </section>
  )
}
