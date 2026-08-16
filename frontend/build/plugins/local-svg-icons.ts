import fs from 'node:fs'
import path from 'node:path'
import type { Plugin } from 'vite'

const MODULE_ID = 'virtual:svg-icons-register'
const RESOLVED_MODULE_ID = `\0${MODULE_ID}`

interface LocalSvgIconsOptions {
  iconDirs: string[]
  symbolId: string
  customDomId: string
}

function escapeXmlAttribute(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
}

function collectSvgFiles(directory: string): string[] {
  if (!fs.existsSync(directory)) return []

  const files: string[] = []
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      files.push(...collectSvgFiles(fullPath))
    } else if (entry.isFile() && entry.name.toLowerCase().endsWith('.svg')) {
      files.push(fullPath)
    }
  }
  return files.sort()
}

function createSymbol(source: string, symbolId: string): string {
  const withoutDeclaration = source.replace(/^\s*<\?xml[^>]*>\s*/i, '')
  const openingTag = withoutDeclaration.match(/<svg\b([^>]*)>/i)
  const closingTag = withoutDeclaration.lastIndexOf('</svg>')
  if (!openingTag || closingTag < 0) {
    throw new Error(`Invalid SVG source for ${symbolId}`)
  }

  const attributes = openingTag[1]
    .replace(/\s+xmlns(?::\w+)?\s*=\s*(?:"[^"]*"|'[^']*')/gi, '')
    .trim()
  const body = withoutDeclaration
    .slice(openingTag.index! + openingTag[0].length, closingTag)
    .replace(/<script\b[\s\S]*?<\/script>/gi, '')
    .replace(/\s+on[a-z]+\s*=\s*(?:"[^"]*"|'[^']*')/gi, '')
    .trim()

  return `<symbol id="${escapeXmlAttribute(symbolId)}"${attributes ? ` ${attributes}` : ''}>${body}</symbol>`
}

function createSprite(options: LocalSvgIconsOptions): string {
  const symbols: string[] = []
  for (const directory of options.iconDirs) {
    for (const file of collectSvgFiles(directory)) {
      const relativeName = path.relative(directory, file).replaceAll(path.sep, '/')
      const name = relativeName.replace(/\.svg$/i, '').replaceAll('/', '-')
      const symbolId = options.symbolId.replace('[name]', name)
      symbols.push(createSymbol(fs.readFileSync(file, 'utf8'), symbolId))
    }
  }

  return `<svg xmlns="http://www.w3.org/2000/svg" id="${escapeXmlAttribute(options.customDomId)}" aria-hidden="true" style="position:absolute;width:0;height:0;overflow:hidden"><defs>${symbols.join('')}</defs></svg>`
}

/**
 * Registers local SVGs as a small virtual sprite module.
 *
 * This intentionally replaces the abandoned vite-plugin-svg-icons chain. The
 * old chain pulled an unmaintained svg-baker/image-size dependency tree into
 * every frontend install even though AetherLink only needs one flat local
 * icon directory and one virtual registration module.
 */
export function createLocalSvgIconsPlugin(options: LocalSvgIconsOptions): Plugin {
  let sprite = ''

  const rebuild = () => {
    sprite = createSprite(options)
  }

  return {
    name: 'aetherlink-local-svg-icons',
    buildStart() {
      rebuild()
    },
    resolveId(id) {
      return id === MODULE_ID ? RESOLVED_MODULE_ID : undefined
    },
    load(id) {
      if (id !== RESOLVED_MODULE_ID) return undefined
      if (!sprite) rebuild()

      return `
const sprite = ${JSON.stringify(sprite)}
const domId = ${JSON.stringify(options.customDomId)}

function registerLocalSvgIcons() {
  if (typeof document === 'undefined' || document.getElementById(domId)) return
  const template = document.createElement('template')
  template.innerHTML = sprite
  const element = template.content.firstElementChild
  if (element) document.body.appendChild(element)
}

registerLocalSvgIcons()
export default registerLocalSvgIcons
`
    },
    handleHotUpdate(context) {
      if (!options.iconDirs.some(directory => context.file.startsWith(directory))) {
        return undefined
      }
      rebuild()
      const module = context.server.moduleGraph.getModuleById(RESOLVED_MODULE_ID)
      if (module) context.server.moduleGraph.invalidateModule(module)
      return module ? [module] : undefined
    }
  }
}
