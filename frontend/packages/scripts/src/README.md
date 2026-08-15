# scripts CLI source

This directory contains the `@aetherlink/scripts` CLI source for local frontend
maintenance tasks.

## Main Files

- `index.ts`: registers CLI commands.
- `commands/`: implements cleanup, dependency updates, commit helpers, changelog,
  release, and route generation.
- `config/`: loads default and project-level CLI options.
- `shared/`: shared command execution helpers.
- `types/`: shared CLI option types.

## Notes

- File-writing commands should keep explicit path boundaries and prefer dry-run
  behavior when possible.
- Changelog generation is local to this package and no longer depends on the old
  template changelog package.
