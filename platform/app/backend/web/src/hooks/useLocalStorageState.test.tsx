import { fireEvent, render, screen } from '@testing-library/react'
import { useLocalStorageState } from './useLocalStorageState'

function TestLocalStorageState() {
  const [value, setValue] = useLocalStorageState<string>('test.localStorage.key', 'initial')
  return (
    <div>
      <span data-testid="value">{value}</span>
      <button onClick={() => setValue('updated')}>update</button>
    </div>
  )
}

describe('useLocalStorageState', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('hydrates from initial value and persists updates', () => {
    render(<TestLocalStorageState />)
    expect(screen.getByTestId('value').textContent).toBe('initial')

    fireEvent.click(screen.getByText('update'))
    expect(screen.getByTestId('value').textContent).toBe('updated')
    expect(window.localStorage.getItem('test.localStorage.key')).toBe(JSON.stringify('updated'))
  })
})
