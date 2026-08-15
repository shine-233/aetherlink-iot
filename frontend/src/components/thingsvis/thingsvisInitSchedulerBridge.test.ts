import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createThingsVisInitScheduler } from './thingsvisInitSchedulerBridge'

describe('createThingsVisInitScheduler', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('clears a pending debounce when the iframe reloads', async () => {
    const runInit = vi.fn(async () => true)
    const scheduler = createThingsVisInitScheduler({
      canInit: () => true,
      getSignature: () => 'sig-1',
      runInit,
      debounceDelay: 50
    })

    scheduler.schedule()
    scheduler.resetAfterFrameLoad()
    await vi.advanceTimersByTimeAsync(100)

    expect(runInit).toHaveBeenCalledTimes(0)
  })

  it('clears a pending retry when the iframe reloads', async () => {
    const runInit = vi.fn(async () => false)
    const scheduler = createThingsVisInitScheduler({
      canInit: () => true,
      getSignature: () => 'sig-1',
      runInit,
      debounceDelay: 10,
      retryBaseDelay: 40,
      retryMaxDelay: 40
    })

    scheduler.schedule()
    await vi.advanceTimersByTimeAsync(10)
    expect(runInit).toHaveBeenCalledTimes(1)

    scheduler.resetAfterFrameLoad()
    await vi.advanceTimersByTimeAsync(100)

    expect(runInit).toHaveBeenCalledTimes(1)
  })

  it('still allows a fresh schedule after reset', async () => {
    const runInit = vi.fn(async () => true)
    const scheduler = createThingsVisInitScheduler({
      canInit: () => true,
      getSignature: () => 'sig-1',
      runInit,
      debounceDelay: 10
    })

    scheduler.schedule()
    scheduler.resetAfterFrameLoad()
    scheduler.schedule()
    await vi.advanceTimersByTimeAsync(20)

    expect(runInit).toHaveBeenCalledTimes(1)
  })

  it('allows a completed signature to be scheduled again after invalidate', async () => {
    const runInit = vi.fn(async () => true)
    const scheduler = createThingsVisInitScheduler({
      canInit: () => true,
      getSignature: () => 'sig-1',
      runInit,
      debounceDelay: 10
    })

    scheduler.schedule()
    await vi.advanceTimersByTimeAsync(10)
    expect(runInit).toHaveBeenCalledTimes(1)

    scheduler.invalidate()
    scheduler.schedule()
    await vi.advanceTimersByTimeAsync(10)

    expect(runInit).toHaveBeenCalledTimes(2)
  })
})
