# scripts CLI commands

This directory contains the command implementations behind `@aetherlink/scripts`.

## Commands

- `cleanup.ts`: removes generated dependencies and build artifacts by glob.
- `update-pkg.ts`: runs dependency version update tooling.
- `git-commit.ts`: helps create and verify Conventional Commit messages.
- `changelog.ts`: writes a local Git-log based `CHANGELOG.md`.
- `release.ts`: runs the version bump and release preparation flow.
- `router.ts`: generates route source files.

## Maintenance

Treat commands that write files, bump versions, or touch Git state as high-impact
developer tooling. Prefer small pure helpers and path-boundary checks before
expanding those flows.
