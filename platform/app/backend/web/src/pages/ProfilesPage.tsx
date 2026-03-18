import { FormEvent, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useToast } from '../components/ToastProvider'
import { deleteNoContent, fetchJSON, postJSON } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'

type ProfileQuality = {
  profile_id: string
  quality: string
  rank: number
}

type ProfileRecord = {
  id: string
  name: string
  cutoff_quality: string
  upgrade_action: string
  default_profile: boolean
}

type ProfileWithQualities = {
  profile: ProfileRecord
  qualities: ProfileQuality[]
}

type ProfilesResponse = {
  items: ProfileWithQualities[]
  default_profile_id: string
}

function normalizeQualities(value: string): { quality: string; rank: number }[] {
  return value
    .split(',')
    .map((q) => q.trim().toLowerCase())
    .filter(Boolean)
    .map((quality, idx) => ({ quality, rank: idx + 1 }))
}

const upgradeActionLabels: Record<string, string> = {
  ask: 'Ask (manual review)',
  replace: 'Replace existing',
  keep_both: 'Keep both'
}

export function ProfilesPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [newID, setNewID] = useState('')
  const [newName, setNewName] = useState('')
  const [newCutoff, setNewCutoff] = useState('epub')
  const [newUpgradeAction, setNewUpgradeAction] = useState('ask')
  const [newQualities, setNewQualities] = useState('epub,azw3,mobi,pdf')
  const [editingID, setEditingID] = useState<string | null>(null)

  const profilesQuery = useQuery({
    queryKey: ['settings', 'profiles'],
    queryFn: () => fetchJSON<ProfilesResponse>('/ui-api/indexer/profiles')
  })

  const createMutation = useMutation({
    mutationFn: async () => {
      await postJSON('/ui-api/indexer/profiles', {
        id: newID.trim(),
        name: newName.trim(),
        cutoff_quality: newCutoff.trim().toLowerCase(),
        upgrade_action: newUpgradeAction,
        qualities: normalizeQualities(newQualities),
        default_profile: false
      })
    },
    onSuccess: async () => {
      pushToast('Profile created')
      setNewID('')
      setNewName('')
      setNewCutoff('epub')
      setNewUpgradeAction('ask')
      setNewQualities('epub,azw3,mobi,pdf')
      await queryClient.invalidateQueries({ queryKey: ['settings', 'profiles'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const updateMutation = useMutation({
    mutationFn: async (payload: { id: string; name: string; cutoff: string; upgradeAction: string; qualities: string; defaultProfile: boolean }) => {
      await postJSON(`/ui-api/indexer/profiles/${encodeURIComponent(payload.id)}`, {
        id: payload.id,
        name: payload.name.trim(),
        cutoff_quality: payload.cutoff.trim().toLowerCase(),
        upgrade_action: payload.upgradeAction,
        qualities: normalizeQualities(payload.qualities),
        default_profile: payload.defaultProfile
      })
    },
    onSuccess: async () => {
      pushToast('Profile updated')
      setEditingID(null)
      await queryClient.invalidateQueries({ queryKey: ['settings', 'profiles'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      await deleteNoContent(`/ui-api/indexer/profiles/${encodeURIComponent(id)}`)
    },
    onSuccess: async () => {
      pushToast('Profile deleted')
      await queryClient.invalidateQueries({ queryKey: ['settings', 'profiles'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const items = useMemo(() => profilesQuery.data?.items ?? [], [profilesQuery.data?.items])
  const editable = useMemo(() => items.find((item) => item.profile.id === editingID) ?? null, [editingID, items])
  const [editName, setEditName] = useState('')
  const [editCutoff, setEditCutoff] = useState('')
  const [editUpgradeAction, setEditUpgradeAction] = useState('ask')
  const [editQualities, setEditQualities] = useState('')
  const [editDefault, setEditDefault] = useState(false)

  const startEdit = (item: ProfileWithQualities) => {
    setEditingID(item.profile.id)
    setEditName(item.profile.name)
    setEditCutoff(item.profile.cutoff_quality)
    setEditUpgradeAction(item.profile.upgrade_action || 'ask')
    setEditQualities(item.qualities.map((q) => q.quality).join(','))
    setEditDefault(item.profile.default_profile)
  }

  const onCreate = (event: FormEvent) => {
    event.preventDefault()
    if (!newID.trim()) {
      pushToast('Profile ID is required')
      return
    }
    createMutation.mutate()
  }

  const onSaveEdit = () => {
    if (!editingID) {
      return
    }
    updateMutation.mutate({
      id: editingID,
      name: editName,
      cutoff: editCutoff,
      upgradeAction: editUpgradeAction,
      qualities: editQualities,
      defaultProfile: editDefault
    })
  }

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Profiles</h2>
        <p className="text-sm text-slate-400">Quality ordering and cutoff profiles used by wanted items and upgrades.</p>
      </header>

      <form className="grid gap-3 rounded border border-slate-800 bg-slate-900/60 p-3 md:grid-cols-6" onSubmit={onCreate}>
        <label className="text-sm text-slate-300">
          ID
          <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={newID} onChange={(e) => setNewID(e.target.value)} />
        </label>
        <label className="text-sm text-slate-300">
          Name
          <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={newName} onChange={(e) => setNewName(e.target.value)} />
        </label>
        <label className="text-sm text-slate-300">
          Cutoff
          <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={newCutoff} onChange={(e) => setNewCutoff(e.target.value)} />
        </label>
        <label className="text-sm text-slate-300">
          Upgrade action
          <select className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={newUpgradeAction} onChange={(e) => setNewUpgradeAction(e.target.value)}>
            <option value="ask">Ask (manual review)</option>
            <option value="replace">Replace existing</option>
            <option value="keep_both">Keep both</option>
          </select>
        </label>
        <label className="text-sm text-slate-300 md:col-span-2">
          Quality order (best to worst)
          <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={newQualities} onChange={(e) => setNewQualities(e.target.value)} />
        </label>
        <div className="md:col-span-6">
          <button className="rounded border border-sky-700 px-3 py-1.5 text-sm text-sky-300" type="submit">
            Create Profile
          </button>
        </div>
      </form>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">ID</th>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Cutoff</th>
              <th className="px-3 py-2">Upgrade</th>
              <th className="px-3 py-2">Order</th>
              <th className="px-3 py-2">Default</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.profile.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{item.profile.id}</td>
                <td className="px-3 py-2">{item.profile.name}</td>
                <td className="px-3 py-2">{item.profile.cutoff_quality}</td>
                <td className="px-3 py-2">{upgradeActionLabels[item.profile.upgrade_action] ?? item.profile.upgrade_action}</td>
                <td className="px-3 py-2">{item.qualities.map((q) => q.quality).join(' > ')}</td>
                <td className="px-3 py-2">{item.profile.default_profile ? 'yes' : 'no'}</td>
                <td className="px-3 py-2">
                  <div className="flex gap-2">
                    <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => startEdit(item)}>Edit</button>
                    <button className="rounded border border-red-700 px-2 py-1 text-xs text-red-300" onClick={() => deleteMutation.mutate(item.profile.id)}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
            {items.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-3 py-6 text-center text-slate-400">No profiles available.</td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {editable ? (
        <div className="rounded border border-slate-800 bg-slate-900/60 p-3">
          <h3 className="text-sm font-semibold text-slate-100">Edit Profile: {editable.profile.id}</h3>
          <div className="mt-2 grid gap-3 md:grid-cols-5">
            <label className="text-sm text-slate-300">
              Name
              <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={editName} onChange={(e) => setEditName(e.target.value)} />
            </label>
            <label className="text-sm text-slate-300">
              Cutoff
              <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={editCutoff} onChange={(e) => setEditCutoff(e.target.value)} />
            </label>
            <label className="text-sm text-slate-300">
              Upgrade action
              <select className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={editUpgradeAction} onChange={(e) => setEditUpgradeAction(e.target.value)}>
                <option value="ask">Ask (manual review)</option>
                <option value="replace">Replace existing</option>
                <option value="keep_both">Keep both</option>
              </select>
            </label>
            <label className="text-sm text-slate-300 md:col-span-2">
              Quality order
              <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={editQualities} onChange={(e) => setEditQualities(e.target.value)} />
            </label>
            <label className="flex items-center gap-2 text-sm text-slate-300 md:col-span-5">
              <input type="checkbox" checked={editDefault} onChange={(e) => setEditDefault(e.target.checked)} />
              Set as default profile
            </label>
          </div>
          <div className="mt-3 flex gap-2">
            <button className="rounded border border-emerald-700 px-3 py-1 text-sm text-emerald-300" onClick={onSaveEdit}>Save</button>
            <button className="rounded border border-slate-700 px-3 py-1 text-sm text-slate-300" onClick={() => setEditingID(null)}>Cancel</button>
          </div>
        </div>
      ) : null}

      {profilesQuery.isLoading ? <p className="text-sm text-slate-400">Loading profiles...</p> : null}
      {profilesQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load profiles.</div> : null}
    </section>
  )
}
