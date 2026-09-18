import { describe, expect, it } from 'vitest'
import {
  adaptDashboardLayout,
  collides,
  getBreakpointForWidth,
  getColumnCountForBreakpoint,
  scaleWidgetLayout,
  type BaseLayoutItem
} from '../breakpoints'

describe('Responsive Breakpoints 2.0', () => {
  describe('getBreakpointForWidth', () => {
    it('maps pixel widths to standard breakpoints', () => {
      expect(getBreakpointForWidth(1920)).toBe('lg')
      expect(getBreakpointForWidth(1200)).toBe('lg')
      expect(getBreakpointForWidth(1199)).toBe('md')
      expect(getBreakpointForWidth(768)).toBe('md')
      expect(getBreakpointForWidth(767)).toBe('sm')
      expect(getBreakpointForWidth(375)).toBe('sm')
    })

    it('returns correct column counts for breakpoints', () => {
      expect(getColumnCountForBreakpoint('lg')).toBe(24)
      expect(getColumnCountForBreakpoint('md')).toBe(12)
      expect(getColumnCountForBreakpoint('sm')).toBe(6)
    })
  })

  describe('collides', () => {
    it('detects overlapping items accurately', () => {
      const a: BaseLayoutItem = { id: 'a', x: 0, y: 0, w: 4, h: 4 }
      const b: BaseLayoutItem = { id: 'b', x: 2, y: 2, w: 4, h: 4 }
      const c: BaseLayoutItem = { id: 'c', x: 4, y: 0, w: 4, h: 4 }

      expect(collides(a, b)).toBe(true)
      expect(collides(a, c)).toBe(false)
      expect(collides(a, a)).toBe(false) // self does not collide
    })
  })

  describe('scaleWidgetLayout', () => {
    it('scales 24-col coordinates into 12-col coordinates', () => {
      const original: BaseLayoutItem = { id: 'w1', x: 12, y: 0, w: 12, h: 4 }
      const scaled = scaleWidgetLayout(original, 24, 12)
      expect(scaled.w).toBe(6)
      expect(scaled.x).toBe(6)
      expect(scaled.h).toBe(4)
    })

    it('prevents widget from exceeding boundary column limit', () => {
      const original: BaseLayoutItem = { id: 'w2', x: 20, y: 0, w: 8, h: 4 }
      const scaled = scaleWidgetLayout(original, 24, 12)
      expect(scaled.x + scaled.w).toBeLessThanOrEqual(12)
    })
  })

  describe('adaptDashboardLayout', () => {
    it('reorganizes 24-col layout to 12-col tablet layout without collisions', () => {
      const widgets: BaseLayoutItem[] = [
        { id: 'w1', x: 0, y: 0, w: 12, h: 4 },
        { id: 'w2', x: 12, y: 0, w: 12, h: 4 },
        { id: 'w3', x: 0, y: 4, w: 24, h: 6 }
      ]

      const adapted = adaptDashboardLayout(widgets, 12, 24)
      expect(adapted).toHaveLength(3)

      // Ensure no two widgets collide in adapted layout
      for (let i = 0; i < adapted.length; i++) {
        for (let j = i + 1; j < adapted.length; j++) {
          expect(collides(adapted[i], adapted[j])).toBe(false)
        }
      }
    })

    it('reorganizes layout to 6-col mobile layout cleanly with push-down', () => {
      const widgets: BaseLayoutItem[] = [
        { id: 'w1', x: 0, y: 0, w: 8, h: 4 },
        { id: 'w2', x: 8, y: 0, w: 8, h: 4 },
        { id: 'w3', x: 16, y: 0, w: 8, h: 4 }
      ]

      const adapted = adaptDashboardLayout(widgets, 6, 24)
      expect(adapted).toHaveLength(3)

      // In 6-col, 3 widgets of width 8 in 24-col will take width 3 or 4.
      // They should stack cleanly downwards without collision.
      for (let i = 0; i < adapted.length; i++) {
        for (let j = i + 1; j < adapted.length; j++) {
          expect(collides(adapted[i], adapted[j])).toBe(false)
        }
      }
    })

    it('returns empty array when input is empty', () => {
      expect(adaptDashboardLayout([], 12, 24)).toEqual([])
    })

    it('returns exact clone when fromCols === toCols', () => {
      const widgets: BaseLayoutItem[] = [{ id: 'w1', x: 0, y: 0, w: 10, h: 4 }]
      const adapted = adaptDashboardLayout(widgets, 24, 24)
      expect(adapted).toEqual(widgets)
    })
  })
})
