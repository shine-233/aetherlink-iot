# scripts CLI types

This directory defines shared option types for `@aetherlink/scripts`.

## Main File

- `index.ts`: defines `CliOption` and `ChangelogOptions`.

## Notes

- `cleanupDirs` can delete generated files, so callers must keep path boundaries
  narrow and auditable.
- `changelogOptions` configures the local Git-log based changelog generator.
