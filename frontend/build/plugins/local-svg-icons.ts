import fs from 'node:fs'
import path from 'node:path'
import { Window } from 'happy-dom'
import type { Plugin } from 'vite'

const MODULE_ID = 'virtual:svg-icons-register'
const RESOLVED_MODULE_ID = `\0${MODULE_ID}`
const SVG_NAMESPACE = 'http://www.w3.org/2000/svg'
const XLINK_NAMESPACE = 'http://www.w3.org/1999/xlink'

const SVG_ELEMENT_NAMES = new Map([
  ['clippath', 'clipPath'],
  ['defs', 'defs'],
  ['ellipse', 'ellipse'],
  ['feblend', 'feBlend'],
  ['feflood', 'feFlood'],
  ['fegaussianblur', 'feGaussianBlur'],
  ['filter', 'filter'],
  ['g', 'g'],
  ['image', 'image'],
  ['line', 'line'],
  ['lineargradient', 'linearGradient'],
  ['mask', 'mask'],
  ['path', 'path'],
  ['pattern', 'pattern'],
  ['radialgradient', 'radialGradient'],
  ['rect', 'rect'],
  ['stop', 'stop'],
  ['text', 'text'],
  ['tspan', 'tspan'],
  ['use', 'use'],
  ['circle', 'circle']
])

const SVG_ATTRIBUTE_NAMES = new Map([
  ['aria-hidden', 'aria-hidden'],
  ['class', 'class'],
  ['clip-path', 'clip-path'],
  ['cx', 'cx'],
  ['cy', 'cy'],
  ['d', 'd'],
  ['enable-background', 'enable-background'],
  ['fill', 'fill'],
  ['fill-opacity', 'fill-opacity'],
  ['fill-rule', 'fill-rule'],
  ['filter', 'filter'],
  ['filterunits', 'filterUnits'],
  ['flood-opacity', 'flood-opacity'],
  ['font-family', 'font-family'],
  ['font-size', 'font-size'],
  ['gradienttransform', 'gradientTransform'],
  ['gradientunits', 'gradientUnits'],
  ['height', 'height'],
  ['id', 'id'],
  ['in', 'in'],
  ['in2', 'in2'],
  ['letter-spacing', 'letter-spacing'],
  ['mask', 'mask'],
  ['offset', 'offset'],
  ['opacity', 'opacity'],
  ['p-id', 'p-id'],
  ['patterncontentunits', 'patternContentUnits'],
  ['preserveaspectratio', 'preserveAspectRatio'],
  ['r', 'r'],
  ['result', 'result'],
  ['rx', 'rx'],
  ['ry', 'ry'],
  ['stddeviation', 'stdDeviation'],
  ['stop-color', 'stop-color'],
  ['stop-opacity', 'stop-opacity'],
  ['stroke', 'stroke'],
  ['stroke-dasharray', 'stroke-dasharray'],
  ['stroke-linecap', 'stroke-linecap'],
  ['stroke-linejoin', 'stroke-linejoin'],
  ['stroke-miterlimit', 'stroke-miterlimit'],
  ['stroke-opacity', 'stroke-opacity'],
  ['stroke-width', 'stroke-width'],
  ['t', 't'],
  ['transform', 'transform'],
  ['viewbox', 'viewBox'],
  ['width', 'width'],
  ['x', 'x'],
  ['x1', 'x1'],
  ['x2', 'x2'],
  ['xlink:href', 'xlink:href'],
  ['xml:space', 'xml:space'],
  ['xmlns:svgjs', 'xmlns:svgjs'],
  ['y', 'y'],
  ['y1', 'y1'],
  ['y2', 'y2']
])

const SVG_FRAGMENT_REFERENCE = /^url\(\s*#[A-Za-z_][\w:.-]*\s*\)$/
const SVG_LOCAL_REFERENCE = /^#[A-Za-z_][\w:.-]*$/
const EMBEDDED_RASTER_IMAGE = /^data:image\/(?:png|gif|jpe?g|webp);base64,[A-Za-z0-9+/]+={0,2}$/i
const SVG_ELEMENT_NODE = 1
const SVG_TEXT_NODE = 3
const SVG_CDATA_SECTION_NODE = 4
const svgParser = new Window().DOMParser()

interface LocalSvgIconsOptions {
  iconDirs: string[]
  symbolId: string
  customDomId: string
}

function escapeXmlAttribute(value: string): string {
  return value.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
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

function isSafeSvgAttributeValue(name: string, value: string): boolean {
  const normalized = value.trim()
  if (name === 'href' || name === 'xlink:href') {
    return SVG_LOCAL_REFERENCE.test(normalized) || EMBEDDED_RASTER_IMAGE.test(normalized)
  }

  // Fragment-only paint/filter references are safe; external URLs and CSS
  // payloads are not valid inputs for the local icon sprite.
  if (/url\s*\(/i.test(normalized) && !SVG_FRAGMENT_REFERENCE.test(normalized)) {
    return false
  }

  return !/[<>]/.test(value) && !value.includes('\u0000')
}

function copySafeSvgAttributes(source: Element, target: Element): void {
  for (const attribute of Array.from(source.attributes)) {
    const normalizedName = attribute.name.toLowerCase()
    const name = SVG_ATTRIBUTE_NAMES.get(normalizedName)
    if (!name || !isSafeSvgAttributeValue(name, attribute.value)) continue

    if (name === 'xlink:href') {
      target.setAttributeNS(XLINK_NAMESPACE, name, attribute.value)
    } else {
      target.setAttribute(name, attribute.value)
    }
  }
}

function sanitizeSvgNode(node: Node, document: Document): Node | null {
  if (node.nodeType === SVG_TEXT_NODE || node.nodeType === SVG_CDATA_SECTION_NODE) {
    return document.createTextNode(node.textContent ?? '')
  }
  if (node.nodeType !== SVG_ELEMENT_NODE) return null

  const source = node as Element
  const elementName = SVG_ELEMENT_NAMES.get(source.localName.toLowerCase())
  if (!elementName) return null

  const target = document.createElementNS(SVG_NAMESPACE, elementName)
  copySafeSvgAttributes(source, target)
  for (const child of Array.from(source.childNodes)) {
    const sanitizedChild = sanitizeSvgNode(child, document)
    if (sanitizedChild) target.appendChild(sanitizedChild)
  }
  return target
}

function createSymbol(source: string, symbolId: string): string {
  const parsed = svgParser.parseFromString(source, 'image/svg+xml')
  const root = parsed.documentElement
  if (!root || root.localName.toLowerCase() !== 'svg' || parsed.querySelector('parsererror')) {
    throw new Error(`Invalid SVG source for ${symbolId}`)
  }

  const symbol = parsed.createElementNS(SVG_NAMESPACE, 'symbol')
  copySafeSvgAttributes(root, symbol)
  symbol.setAttribute('id', symbolId)
  for (const child of Array.from(root.childNodes)) {
    const sanitizedChild = sanitizeSvgNode(child, parsed)
    if (sanitizedChild) symbol.appendChild(sanitizedChild)
  }
  return symbol.outerHTML
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
  if (typeof document === 'undefined' || document.getElementById(domId) || !document.body) return
  const parsed = new DOMParser().parseFromString(sprite, 'image/svg+xml')
  if (parsed.querySelector('parsererror') || parsed.documentElement?.localName !== 'svg') return
  const element = document.importNode(parsed.documentElement, true)
  document.body.appendChild(element)
}

registerLocalSvgIcons()
export default registerLocalSvgIcons
`
    },
    handleHotUpdate(context) {
      if (!options.iconDirs.some((directory) => context.file.startsWith(directory))) {
        return undefined
      }
      rebuild()
      const module = context.server.moduleGraph.getModuleById(RESOLVED_MODULE_ID)
      if (module) context.server.moduleGraph.invalidateModule(module)
      return module ? [module] : undefined
    }
  }
}
