import { buildManualSearchPath } from './manualSearch'

describe('buildManualSearchPath', () => {
  it('includes autoGrab when requested', () => {
    expect(buildManualSearchPath({ workID: 'work-1', autorun: true, autoGrab: true })).toBe(
      '/library/books/manual-search?workID=work-1&autorun=1&autoGrab=1'
    )
  })
})
