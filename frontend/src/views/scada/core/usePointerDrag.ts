/**
 * Window-level mouse drag tracking.
 *
 * The previous inline implementation registered `mousemove`/`mouseup` on window and only
 * removed them on mouseup; unmounting the view mid-drag (route change on a keyboard
 * shortcut, for example) leaked both listeners and kept mutating a dead editor. Here the
 * active drag is always cancelled on unmount.
 */
import { getCurrentInstance, onBeforeUnmount } from 'vue'

export interface DragDelta {
  dx: number
  dy: number
}

export function usePointerDrag() {
  let cleanup: (() => void) | null = null

  function cancel() {
    cleanup?.()
    cleanup = null
  }

  /** Starts a drag from `event`; `onDelta` receives screen-space offsets from the start point. */
  function begin(event: Pick<MouseEvent, 'clientX' | 'clientY'>, onDelta: (delta: DragDelta) => void) {
    cancel()
    const startX = event.clientX
    const startY = event.clientY
    const onMove = (moveEvent: MouseEvent) => onDelta({ dx: moveEvent.clientX - startX, dy: moveEvent.clientY - startY })
    const onUp = () => cancel()
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    cleanup = () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }

  if (getCurrentInstance()) onBeforeUnmount(cancel)

  return { begin, cancel, isDragging: () => cleanup !== null }
}
