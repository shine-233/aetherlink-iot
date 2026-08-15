export interface ChangelogOptions {
  /** Project root used for git log and output resolution. Defaults to the current process cwd. */
  cwd?: string
  /** Output markdown file. Defaults to CHANGELOG.md under cwd. */
  output?: string
  /** Optional lower git range bound, for example v1.0.0. */
  from?: string
  /** Optional upper git range bound. Defaults to HEAD when from is set. */
  to?: string
  /** Markdown title for the generated changelog. */
  title?: string
  /** Maximum number of commits to include. Defaults to 200. */
  maxCount?: number
}

export interface CliOption {
  /** The project root directory */
  cwd: string
  /**
   * Cleanup dirs
   *
   * Glob pattern syntax {@link https://github.com/isaacs/minimatch}
   *
   * @default
   * ```json
   * ["** /dist", "** /pnpm-lock.yaml", "** /node_modules", "!node_modules/**"]
   * ```
   */
  cleanupDirs: string[]
  /** Git commit types */
  gitCommitTypes: [string, string][]
  /** Git commit scopes */
  gitCommitScopes: [string, string][]
  /**
   * Npm-check-updates command args
   *
   * @default ['--deep', '-u']
   */
  ncuCommandArgs: string[]
  /** Local changelog generation options. */
  changelogOptions: ChangelogOptions
}
