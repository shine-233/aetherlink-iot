import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('useRdiTemperatureUnit', () => {
  beforeEach(() => {
    window.localStorage.clear()
    vi.resetModules()
  })

  it('shares the persisted RDI temperature unit across detail modules', async () => {
    window.localStorage.setItem('rdi-temperature-unit', 'F')
    const { useRdiTemperatureUnit } = await import('../useRdiTemperatureUnit')

    const firstConsumer = useRdiTemperatureUnit()
    const secondConsumer = useRdiTemperatureUnit()

    expect(firstConsumer).toBe(secondConsumer)
    expect(firstConsumer.value).toBe('F')

    firstConsumer.value = 'C'
    await nextTick()

    expect(secondConsumer.value).toBe('C')
    expect(window.localStorage.getItem('rdi-temperature-unit')).toBe('C')
  })
})
