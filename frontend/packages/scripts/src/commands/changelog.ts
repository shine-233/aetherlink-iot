import fs from 'node:fs/promises'
import path from 'node:path'
import { execa } from 'execa'
import type { ChangelogOptions } from '../types'

interface GitEntry {
  hash: string
  subject: string
  author: string
  date: string
}

async function runGit(args: string[], cwd: string) {
  return execa('git', args, { cwd, reject: false })
}

async function resolveLatestTagRange(cwd: string, options: ChangelogOptions, total: boolean) {
  if (total) return undefined
  if (options.from || options.to) return `${options.from || ''}..${options.to || 'HEAD'}`

  const latestTag = await runGit(['describe', '--tags', '--abbrev=0'], cwd)

  return latestTag.exitCode === 0 && latestTag.stdout.trim() ? `${latestTag.stdout.trim()}..HEAD` : undefined
}

function parseLog(stdout: string): GitEntry[] {
  return stdout
    .split('\n')
    .map(line => line.trim())
    .filter(Boolean)
    .map(line => {
      const [hash = '', subject = '', author = '', date = ''] = line.split('\t')

      return { hash, subject, author, date }
    })
}

function formatChangelog(entries: GitEntry[], options: ChangelogOptions, range?: string) {
  const title = options.title || 'AetherLink IoT Changelog'
  const generatedAt = new Date().toISOString()
  const lines = [`# ${title}`, '', `Generated: ${generatedAt}`, range ? `Range: ${range}` : 'Range: all commits', '']

  if (!entries.length) {
    lines.push('No commits found for this range.', '')
    return lines.join('\n')
  }

  lines.push('## Changes', '')

  for (const entry of entries) {
    const meta = [entry.hash, entry.date, entry.author].filter(Boolean).join(' | ')
    lines.push(`- ${entry.subject}${meta ? ` (${meta})` : ''}`)
  }

  lines.push('')

  return lines.join('\n')
}

export async function genChangelog(options: ChangelogOptions = {}, total = false) {
  const cwd = options.cwd ? path.resolve(options.cwd) : process.cwd()
  const output = path.resolve(cwd, options.output || 'CHANGELOG.md')
  const range = await resolveLatestTagRange(cwd, options, total)
  const maxCount = options.maxCount && options.maxCount > 0 ? options.maxCount : 200
  const args = ['log', `--max-count=${maxCount}`, '--pretty=format:%h%x09%s%x09%an%x09%ad', '--date=short']

  if (range) args.push(range)

  const result = await runGit(args, cwd)

  if (result.exitCode !== 0) {
    throw new Error(`Unable to generate changelog: ${result.stderr || result.stdout || 'git log failed'}`)
  }

  await fs.writeFile(output, formatChangelog(parseLog(result.stdout), options, range), 'utf8')
}
