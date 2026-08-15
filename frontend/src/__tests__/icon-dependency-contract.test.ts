import { readFileSync, readdirSync, statSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const packageJsonPath = path.join(frontendRoot, 'package.json')
const lockfilePath = path.join(frontendRoot, 'pnpm-lock.yaml')
const sourceRoot = path.join(frontendRoot, 'src')
const removedCollections = ['carbon', 'ph', 'simple-icons', 'uil']

function collectSourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = path.join(directory, entry.name)
    if (entry.isDirectory()) return collectSourceFiles(entryPath)
    return statSync(entryPath).isFile() && /\.(ts|tsx|vue|js|jsx)$/.test(entry.name) ? [entryPath] : []
  })
}

describe('icon dependency contract', () => {
  const manifest = JSON.parse(readFileSync(packageJsonPath, 'utf8')) as {
    dependencies?: Record<string, string>
    devDependencies?: Record<string, string>
  }
  const lockfile = readFileSync(lockfilePath, 'utf8')
  const declaredDependencies = {
    ...manifest.dependencies,
    ...manifest.devDependencies
  }

  it('does not retain removed Iconify JSON collections in manifest or lockfile', () => {
    for (const collection of removedCollections) {
      const packageName = `@iconify-json/${collection}`
      expect(declaredDependencies).not.toHaveProperty(packageName)
      expect(lockfile).not.toContain(`'${packageName}'`)
      expect(lockfile).not.toContain(`'${packageName}@`)
    }
  })

  it('declares an installed Iconify JSON package for every ~icons source collection', () => {
    const source = collectSourceFiles(sourceRoot)
      .map((filePath) => readFileSync(filePath, 'utf8'))
      .join('\n')
    const collections = [...source.matchAll(/~icons\/([a-z0-9-]+)\//gi)]
      .map((match) => match[1].toLowerCase())
      .filter((collection) => collection !== 'local')

    for (const collection of new Set(collections)) {
      expect(declaredDependencies).toHaveProperty(`@iconify-json/${collection}`)
    }
  })
})
