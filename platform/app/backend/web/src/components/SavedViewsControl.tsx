type ViewOption = { id: string; name: string }

export function SavedViewsControl({
  selectedViewID,
  defaultViewID,
  defaultViews,
  myViews,
  onSelectView,
  onSaveCurrentAsMyView,
  onClonePresetToMyView,
  onUpdateCurrentMyView,
  onDeleteCurrentMyView,
  onSetDefaultView,
  onHidePreset
}: {
  selectedViewID: string
  defaultViewID: string
  defaultViews: ViewOption[]
  myViews: ViewOption[]
  onSelectView: (id: string) => boolean
  onSaveCurrentAsMyView: (name: string) => boolean
  onClonePresetToMyView: (presetID: string, name: string) => boolean
  onUpdateCurrentMyView: () => boolean
  onDeleteCurrentMyView: () => boolean
  onSetDefaultView: (id: string) => boolean
  onHidePreset: (id: string) => boolean
}) {
  const selectedIsMyView = selectedViewID.startsWith('my:')
  const selectedIsDefaultPreset =
    !selectedIsMyView && defaultViews.some((view) => view.id === selectedViewID)

  return (
    <div className="flex flex-wrap items-center gap-2">
      <select
        className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
        value={selectedViewID}
        onChange={(event) => onSelectView(event.target.value)}
      >
        <option value="">Views</option>
        {defaultViews.length > 0 ? (
          <optgroup label="Default">
            {defaultViews.map((view) => (
              <option key={view.id} value={view.id}>
                {view.name}
                {defaultViewID === view.id ? ' (default)' : ''}
              </option>
            ))}
          </optgroup>
        ) : null}
        {myViews.length > 0 ? (
          <optgroup label="My Views">
            {myViews.map((view) => (
              <option key={view.id} value={view.id}>
                {view.name}
                {defaultViewID === view.id ? ' (default)' : ''}
              </option>
            ))}
          </optgroup>
        ) : null}
      </select>

      <button
        className="rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300"
        onClick={() => {
          const name = window.prompt('Save current as My View:', '')
          if (!name) return
          onSaveCurrentAsMyView(name)
        }}
      >
        Save as My View...
      </button>

      <button
        className="rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300 disabled:opacity-50"
        disabled={!selectedIsDefaultPreset}
        onClick={() => {
          const selected = defaultViews.find((view) => view.id === selectedViewID)
          if (!selected) return
          const suggested = `${selected.name} (Copy)`
          const name = window.prompt('Clone preset to My View:', suggested)
          if (!name) return
          onClonePresetToMyView(selected.id, name)
        }}
      >
        Clone to My View
      </button>

      <button
        className="rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300 disabled:opacity-50"
        disabled={!selectedIsMyView}
        onClick={() => onUpdateCurrentMyView()}
      >
        Update My View
      </button>

      <button
        className="rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300 disabled:opacity-50"
        disabled={!selectedIsMyView}
        onClick={() => {
          const ok = window.confirm('Delete this My View?')
          if (ok) onDeleteCurrentMyView()
        }}
      >
        Delete My View
      </button>

      <button
        className="rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300 disabled:opacity-50"
        disabled={!selectedViewID}
        onClick={() => onSetDefaultView(selectedViewID)}
      >
        Set default view
      </button>

      <button
        className="rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300 disabled:opacity-50"
        disabled={!selectedIsDefaultPreset}
        onClick={() => onHidePreset(selectedViewID)}
      >
        Hide preset
      </button>
    </div>
  )
}
