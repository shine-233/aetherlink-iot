#!/usr/bin/env node

/**
 * File purpose: audit live frontend imports for grid/drag packages before any
 * dependency slimming pass.
 * Core logic: scan frontend/src imports, map them to selected packages, and
 * print which packages still have real callers.
 * Key note: this is a read-only audit script; it should never rewrite source
 * or lockfiles.
 */

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const projectRoot = path.resolve(__dirname, '..')
const srcRoot = path.join(projectRoot, 'src')
const packageJsonPath = path.join(projectRoot, 'package.json')

const trackedPackages = [
  {
    name: 'grid-layout-plus',
    category: 'grid',
    canonical: true,
    note: 'current canonical grid wrapper path'
  },
  {
    name: 'vue3-grid-layout',
    category: 'grid',
    canonical: false,
    note: 'retired legacy panel grid detector'
  },
  {
    name: 'gridstack',
    category: 'grid',
    canonical: false,
    note: 'retired package / visual-editor compatibility detector'
  },
  {
    name: 'vue-draggable-plus',
    category: 'drag',
    canonical: true,
    note: 'current canonical list drag path'
  },
  {
    name: 'vuedraggable',
    category: 'drag',
    canonical: false,
    note: 'retired legacy drag path'
  }
]

const sourceExtensions = new Set(['.ts', '.tsx', '.js', '.jsx', '.vue'])
const importPattern = /(?:import|export)\s+(?:[^'"`]*?\s+from\s+)?['"]([^'"`]+)['"]|import\(\s*['"]([^'"`]+)['"]\s*\)/g

function readJson(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'))
}

function walkFiles(rootDir) {
  const stack = [rootDir]
  const files = []

  while (stack.length > 0) {
    const currentDir = stack.pop()
    if (!currentDir) continue

    for (const entry of fs.readdirSync(currentDir, { withFileTypes: true })) {
      const fullPath = path.join(currentDir, entry.name)
      if (entry.isDirectory()) {
        stack.push(fullPath)
        continue
      }

      if (sourceExtensions.has(path.extname(entry.name))) {
        files.push(fullPath)
      }
    }
  }

  return files.sort((a, b) => a.localeCompare(b))
}

function collectImports(filePath) {
  const content = fs.readFileSync(filePath, 'utf8')
  const matches = []

  for (const match of content.matchAll(importPattern)) {
    const specifier = match[1] || match[2]
    if (specifier) matches.push(specifier)
  }

  return matches
}

function formatRelative(filePath) {
  return path.relative(projectRoot, filePath).replaceAll('\\', '/')
}

function createAuditState(packageManifest) {
  const manifestDependencies = {
    ...packageManifest.dependencies,
    ...packageManifest.devDependencies
  }

  return new Map(
    trackedPackages.map((pkg) => [
      pkg.name,
      {
        ...pkg,
        declared: Object.hasOwn(manifestDependencies, pkg.name),
        callers: new Set()
      }
    ])
  )
}

function applyImportHit(specifier, filePath, auditState) {
  for (const [packageName, pkg] of auditState.entries()) {
    if (specifier === packageName || specifier.startsWith(`${packageName}/`)) {
      pkg.callers.add(formatRelative(filePath))
    }
  }
}

function printHeader(title) {
  console.log(`\n${title}`)
  console.log('-'.repeat(title.length))
}

function printSummary(auditState) {
  printHeader('Grid / drag dependency audit')
  console.log(`Project root: ${projectRoot}`)
  console.log(`Source root : ${srcRoot}`)

  const rows = [...auditState.values()].map((pkg) => ({
    packageName: pkg.name,
    declared: pkg.declared ? 'yes' : 'no',
    callers: pkg.callers.size,
    canonical: pkg.canonical ? 'yes' : 'no',
    note: pkg.note
  }))

  const nameWidth = Math.max(...rows.map((row) => row.packageName.length), 'package'.length)
  const declaredWidth = 'declared'.length
  const callersWidth = 'callers'.length
  const canonicalWidth = 'canonical'.length

  console.log('')
  console.log(
    `${'package'.padEnd(nameWidth)}  ${'declared'.padEnd(declaredWidth)}  ${'callers'.padStart(callersWidth)}  ${'canonical'.padEnd(canonicalWidth)}  note`
  )
  console.log(
    `${'-'.repeat(nameWidth)}  ${'-'.repeat(declaredWidth)}  ${'-'.repeat(callersWidth)}  ${'-'.repeat(canonicalWidth)}  ${'-'.repeat(24)}`
  )

  for (const row of rows) {
    console.log(
      `${row.packageName.padEnd(nameWidth)}  ${row.declared.padEnd(declaredWidth)}  ${String(row.callers).padStart(callersWidth)}  ${row.canonical.padEnd(canonicalWidth)}  ${row.note}`
    )
  }
}

function printCallers(auditState) {
  for (const pkg of auditState.values()) {
    printHeader(pkg.name)
    if (pkg.callers.size === 0) {
      console.log('No live imports under frontend/src')
      continue
    }

    for (const caller of [...pkg.callers].sort((a, b) => a.localeCompare(b))) {
      console.log(`- ${caller}`)
    }
  }
}

function main() {
  const packageManifest = readJson(packageJsonPath)
  const auditState = createAuditState(packageManifest)
  const files = walkFiles(srcRoot)

  for (const filePath of files) {
    for (const specifier of collectImports(filePath)) {
      applyImportHit(specifier, filePath, auditState)
    }
  }

  printSummary(auditState)
  printCallers(auditState)
}

main()
