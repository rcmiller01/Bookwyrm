import { clonePresetToMyView, mergeViews, UserSavedView } from './savedViewsModel'
import { PresetView } from '../presets/views'

type State = { query: string }

const presets: PresetView<State>[] = [
  {
    id: 'preset.books.all.v1',
    pageKey: 'books',
    name: 'All Books',
    state: { query: '' },
    isDefault: true,
    version: 1
  },
  {
    id: 'preset.books.monitored.v1',
    pageKey: 'books',
    name: 'Monitored',
    state: { query: 'm' },
    isDefault: true,
    version: 1
  }
]

describe('savedViewsModel', () => {
  it('shows presets when no user views exist', () => {
    const merged = mergeViews(presets, [], [])
    expect(merged.defaultViews).toHaveLength(2)
    expect(merged.myViews).toHaveLength(0)
  })

  it('keeps user view when it shares the same id', () => {
    const user: UserSavedView<State> = {
      id: 'preset.books.monitored.v1',
      name: 'Monitored (Custom)',
      state: { query: 'custom' },
      createdAt: '2026-03-06T00:00:00.000Z',
      updatedAt: '2026-03-06T00:00:00.000Z'
    }
    const merged = mergeViews(presets, [user], [])
    const overridden = merged.defaultViews.find((view) => view.id === 'preset.books.monitored.v1')
    expect(overridden?.name).toBe('Monitored (Custom)')
    expect(overridden?.state.query).toBe('custom')
  })

  it('hides a preset when preset id is in hidden set', () => {
    const merged = mergeViews(presets, [], ['preset.books.monitored.v1'])
    expect(merged.defaultViews.some((view) => view.id === 'preset.books.monitored.v1')).toBe(false)
  })

  it('clones preset into my view', () => {
    const cloned = clonePresetToMyView(presets[0], 'All Books Copy')
    expect(cloned.id.startsWith('my:books:')).toBe(true)
    expect(cloned.name).toBe('All Books Copy')
    expect(cloned.state.query).toBe('')
  })
})
