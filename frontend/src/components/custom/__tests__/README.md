# Custom Component Tests

This directory holds focused Vitest coverage for shared UI components under
`frontend/src/components/custom`.

Current coverage is intentionally narrow: `svg-icon.test.ts` verifies the public
rendering contract for local SVG and Iconify fallback behavior. Add tests here
when a shared component gains new public behavior, edge-case state, or regression
risk.

Mocks should mirror the real component contract rather than only proving that a
component mounted.
