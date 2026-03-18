import { useEffect, useMemo, useRef, useState } from 'react'
import { PageKey, PresetView } from '../presets/views'
import { clonePresetToMyView, mergeViews, SavedViewsStorage, UserSavedView } from './savedViewsModel'

function toKey(pageKey: PageKey): string {
  return `savedViews:${pageKey}`
}

function defaultViewKey(pageKey: PageKey): string {
  return `defaultView:${pageKey}`
}

function deepEqual<T>(a: T, b: T): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

function readStorage<T>(pageKey: PageKey): SavedViewsStorage<T> {
  try {
    const raw = window.localStorage.getItem(toKey(pageKey))
    if (!raw) {
      return { myViews: [], hiddenPresetIDs: [] }
    }
    const parsed = JSON.parse(raw) as
      | SavedViewsStorage<T>
      | {
          views?: Array<{ name: string; state: T; createdAt: string; updatedAt: string }>
          defaultViewName?: string
        }

    if ('myViews' in parsed || 'hiddenPresetIDs' in parsed) {
      return {
        myViews: (parsed as SavedViewsStorage<T>).myViews ?? [],
        hiddenPresetIDs: (parsed as SavedViewsStorage<T>).hiddenPresetIDs ?? [],
        defaultViewID: (parsed as SavedViewsStorage<T>).defaultViewID
      }
    }

    const legacy = parsed as {
      views?: Array<{ name: string; state: T; createdAt: string; updatedAt: string }>
    }
    return {
      myViews: (legacy.views ?? []).map((view, idx) => ({
        id: `my:${pageKey}:legacy:${idx}`,
        ...view
      })),
      hiddenPresetIDs: []
    }
  } catch {
    return { myViews: [], hiddenPresetIDs: [] }
  }
}

function writeStorage<T>(pageKey: PageKey, payload: SavedViewsStorage<T>): void {
  window.localStorage.setItem(toKey(pageKey), JSON.stringify(payload))
}

export function useSavedViews<T>({
  pageKey,
  currentState,
  applyState,
  presetViews
}: {
  pageKey: PageKey
  currentState: T
  applyState: (state: T) => void
  presetViews: PresetView<T>[]
}) {
  const [storage, setStorage] = useState<SavedViewsStorage<T>>(() => readStorage<T>(pageKey))
  const [activeViewID, setActiveViewID] = useState<string>('')
  const bootApplied = useRef(false)

  useEffect(() => {
    writeStorage(pageKey, storage)
  }, [pageKey, storage])

  const merged = useMemo(
    () => mergeViews(presetViews, storage.myViews, storage.hiddenPresetIDs),
    [presetViews, storage.hiddenPresetIDs, storage.myViews]
  )

  const defaultPresetID = useMemo(
    () => presetViews.find((view) => view.name.toLowerCase().includes('all'))?.id || presetViews[0]?.id || '',
    [presetViews]
  )

  useEffect(() => {
    if (bootApplied.current) return
    const explicitDefaultID = window.localStorage.getItem(defaultViewKey(pageKey)) || storage.defaultViewID || ''
    const fallbackID = explicitDefaultID || defaultPresetID
    const fallback = merged.allViews.find((view) => view.id === fallbackID)
    if (fallback) {
      applyState(fallback.state)
      setActiveViewID(fallback.id)
    }
    bootApplied.current = true
  }, [applyState, defaultPresetID, merged.allViews, pageKey, storage.defaultViewID])

  const matchedActive = useMemo(() => {
    const byState = merged.allViews.find((view) => deepEqual(view.state, currentState))
    return byState?.id ?? ''
  }, [currentState, merged.allViews])

  const selectedViewID = activeViewID || matchedActive

  const selectView = (id: string) => {
    const found = merged.allViews.find((view) => view.id === id)
    if (!found) {
      setActiveViewID('')
      return false
    }
    applyState(found.state)
    setActiveViewID(found.id)
    return true
  }

  const saveCurrentAsMyView = (name: string) => {
    const trimmed = name.trim()
    if (!trimmed) return false
    const duplicate = storage.myViews.some((view) => view.name.toLowerCase() === trimmed.toLowerCase())
    if (duplicate) return false
    const now = new Date().toISOString()
    const next: UserSavedView<T> = {
      id: `my:${pageKey}:${Date.now()}`,
      name: trimmed,
      state: currentState,
      createdAt: now,
      updatedAt: now
    }
    setStorage((prev) => ({ ...prev, myViews: [...prev.myViews, next] }))
    setActiveViewID(next.id)
    return true
  }

  const clonePresetToMy = (presetID: string, name: string) => {
    const preset = presetViews.find((view) => view.id === presetID)
    if (!preset || !name.trim()) return false
    const duplicate = storage.myViews.some((view) => view.name.toLowerCase() === name.trim().toLowerCase())
    if (duplicate) return false
    const cloned = clonePresetToMyView(preset, name)
    setStorage((prev) => ({ ...prev, myViews: [...prev.myViews, cloned] }))
    setActiveViewID(cloned.id)
    applyState(cloned.state)
    return true
  }

  const updateCurrentMyView = () => {
    const targetID = selectedViewID
    if (!targetID.startsWith('my:')) return false
    const now = new Date().toISOString()
    let changed = false
    const updated = storage.myViews.map((view) => {
      if (view.id !== targetID) return view
      changed = true
      return { ...view, state: currentState, updatedAt: now }
    })
    if (!changed) return false
    setStorage((prev) => ({ ...prev, myViews: updated }))
    return true
  }

  const deleteCurrentMyView = () => {
    const targetID = selectedViewID
    if (!targetID.startsWith('my:')) return false
    setStorage((prev) => ({ ...prev, myViews: prev.myViews.filter((view) => view.id !== targetID) }))
    setActiveViewID('')
    return true
  }

  const hidePreset = (presetID: string) => {
    if (!presetViews.some((view) => view.id === presetID)) return false
    setStorage((prev) => ({
      ...prev,
      hiddenPresetIDs: prev.hiddenPresetIDs.includes(presetID) ? prev.hiddenPresetIDs : [...prev.hiddenPresetIDs, presetID]
    }))
    if (selectedViewID === presetID) {
      setActiveViewID('')
    }
    return true
  }

  const setDefaultView = (viewID: string) => {
    const exists = merged.allViews.some((view) => view.id === viewID)
    if (!exists) return false
    window.localStorage.setItem(defaultViewKey(pageKey), viewID)
    setStorage((prev) => ({ ...prev, defaultViewID: viewID }))
    return true
  }

  return {
    defaultViews: merged.defaultViews,
    myViews: merged.myViews,
    allViews: merged.allViews,
    selectedViewID,
    defaultViewID: window.localStorage.getItem(defaultViewKey(pageKey)) || storage.defaultViewID || defaultPresetID,
    selectView,
    saveCurrentAsMyView,
    clonePresetToMy,
    updateCurrentMyView,
    deleteCurrentMyView,
    hidePreset,
    setDefaultView
  }
}
