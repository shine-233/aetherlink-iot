# Frontend Workspace Packages

`frontend/packages` contains local workspace packages shared by the Vue app.

## Folder Map

- `axios/`: request client abstraction and adapters.
- `hooks/`: reusable Vue/composition helpers.
- `materials/`: shared UI materials and layout assets.
- `scripts/`: frontend CLI, generation, and maintenance scripts.
- `utils/`: shared utility functions.
- `color-palette/` and `uno-preset/`: color support and build/style presets.

## Maintenance Notes

- Package-level changes can affect the whole frontend even when the page-level
  diff is small.
- Keep exported interfaces stable; breaking changes should update callers,
  README notes, and type checks together.
- Prefer pure functions or clear side effects inside packages so pages and
  focused checks can reuse them safely.

## Audit Notes

- `@aetherlink/fetch` / `packages/ofetch` was retired after no main-source
  importers were found. Do not re-add a workspace package unless there is a
  real caller and a clear interface.
