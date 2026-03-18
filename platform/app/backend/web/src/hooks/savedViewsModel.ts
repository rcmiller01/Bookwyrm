import { PresetView } from '../presets/views'

export type UserSavedView<T> = {
  id: string
  name: string
  state: T
  createdAt: string
  updatedAt: string
}

export type SavedViewsStorage<T> = {
  myViews: UserSavedView<T>[]
  hiddenPresetIDs: string[]
  defaultViewID?: string
}

export type ResolvedView<T> = {
  id: string
  name: string
  state: T
  source: 'default' | 'my'
}

export function mergeViews<T>(
  presets: PresetView<T>[],
  userViews: UserSavedView<T>[],
  hiddenPresetIDs: string[]
): { defaultViews: ResolvedView<T>[]; myViews: ResolvedView<T>[]; allViews: ResolvedView<T>[] } {
  const hiddenSet = new Set(hiddenPresetIDs)
  const userByID = new Map(userViews.map((view) => [view.id, view]))

  const defaultViews: ResolvedView<T>[] = []
  for (const preset of presets) {
    if (hiddenSet.has(preset.id)) continue
    const userOverride = userByID.get(preset.id)
    if (userOverride) {
      defaultViews.push({
        id: userOverride.id,
        name: userOverride.name,
        state: userOverride.state,
        source: 'default'
      })
      continue
    }
    defaultViews.push({
      id: preset.id,
      name: preset.name,
      state: preset.state,
      source: 'default'
    })
  }

  const myViews: ResolvedView<T>[] = userViews
    .filter((view) => !presets.some((preset) => preset.id === view.id))
    .map((view) => ({
      id: view.id,
      name: view.name,
      state: view.state,
      source: 'my'
    }))

  return {
    defaultViews,
    myViews,
    allViews: [...defaultViews, ...myViews]
  }
}

export function clonePresetToMyView<T>(preset: PresetView<T>, name: string): UserSavedView<T> {
  const now = new Date().toISOString()
  return {
    id: `my:${preset.pageKey}:${Date.now()}`,
    name: name.trim(),
    state: preset.state,
    createdAt: now,
    updatedAt: now
  }
}
