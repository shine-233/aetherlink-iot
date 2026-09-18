/**
 * 文件用途：本地可视化组件 HTML 与 CSS 安全净化器（HTML Sanitizer & Scoped CSS Engine）。
 *
 * 安全前置准则（ROADMAP TB-11 / ThingsBoard 4.3.1.2 #15556 对标）：
 * 1. 严格 Fail-Closed 白名单过滤：仅允许安全排版/结构标签，剥离 script/iframe/object/embed/form 等；
 * 2. 剥离所有 on* 事件处理器；
 * 3. 严格校验 href 与 src 协议（仅允许 http/https/mailto/tel 或安全相对路径，无条件拦截 javascript:/vbscript:/危险 data:）；
 * 4. 样式净化与隔离：过滤 CSS expression/behavior/import，支持基于 [data-widget-id="..."] 的 Scoped CSS 作用域重写；
 * 5. 防御 DOM Clobbering 与原型污染关键命名。
 */

export const ALLOWED_TAGS: ReadonlySet<string> = new Set([
  'a', 'abbr', 'article', 'aside', 'b', 'bdi', 'bdo', 'blockquote', 'br',
  'caption', 'cite', 'code', 'col', 'colgroup', 'data', 'dd', 'del', 'details',
  'dfn', 'div', 'dl', 'dt', 'em', 'figcaption', 'figure', 'footer', 'h1', 'h2',
  'h3', 'h4', 'h5', 'h6', 'header', 'hr', 'i', 'img', 'ins', 'kbd', 'li',
  'main', 'mark', 'ol', 'p', 'pre', 'q', 'rp', 'rt', 'ruby', 's', 'samp',
  'section', 'small', 'span', 'strong', 'sub', 'summary', 'sup', 'table',
  'tbody', 'td', 'tfoot', 'th', 'thead', 'time', 'tr', 'u', 'ul', 'var', 'wbr'
])

export const DANGEROUS_TAGS: ReadonlySet<string> = new Set([
  'script', 'iframe', 'object', 'embed', 'style', 'base', 'meta', 'link',
  'form', 'input', 'button', 'textarea', 'select', 'option', 'optgroup',
  'canvas', 'svg', 'math', 'applet', 'frame', 'frameset', 'noscript'
])

export const ALLOWED_ATTRIBUTES: ReadonlySet<string> = new Set([
  'class', 'id', 'title', 'dir', 'lang',
  'width', 'height', 'align', 'valign', 'colspan', 'rowspan',
  'border', 'cellpadding', 'cellspacing',
  'href', 'target', 'rel', 'download',
  'src', 'alt', 'loading',
  'style'
])

export const FORBIDDEN_IDENTIFIERS: ReadonlySet<string> = new Set([
  'window', 'document', 'location', 'top', 'parent', 'self', 'frames',
  'content', 'defaultview', '__proto__', 'prototype', 'constructor',
  'valueof', 'tostring', 'hasownproperty'
])

const SAFE_IMAGE_DATA_URI = /^data:image\/(?:png|jpeg|jpg|webp|gif);base64,[a-z0-9+/=]+$/i

/**
 * 校验 URL 是否安全（防止 javascript:、vbscript: 及危险伪协议）
 */
export function isSafeUrl(url: string): boolean {
  if (typeof url !== 'string') return false
  const trimmed = url.trim().replace(/[\u0000-\u001F\u007F-\u009F\s]+/g, '')
  const lower = trimmed.toLowerCase()

  if (lower.startsWith('javascript:') || lower.startsWith('vbscript:')) {
    return false
  }

  if (lower.startsWith('data:')) {
    return SAFE_IMAGE_DATA_URI.test(trimmed)
  }

  // 显式允许的安全网络/通信协议
  if (
    lower.startsWith('http://') ||
    lower.startsWith('https://') ||
    lower.startsWith('mailto:') ||
    lower.startsWith('tel:') ||
    lower.startsWith('/') ||
    lower.startsWith('./') ||
    lower.startsWith('../') ||
    lower.startsWith('#') ||
    lower.startsWith('?')
  ) {
    return true
  }

  // 无 scheme 相对路径
  if (!lower.includes(':')) {
    return true
  }

  return false
}

/**
 * 安全清理内联 style 属性
 */
export function cleanInlineStyle(style: string): string {
  if (!style || typeof style !== 'string') return ''
  const declarations = style.split(';')
  const safeDeclarations: string[] = []

  for (const decl of declarations) {
    const trimmed = decl.trim()
    if (!trimmed) continue
    const lower = trimmed.toLowerCase()
    if (
      lower.includes('expression') ||
      lower.includes('behavior') ||
      lower.includes('-moz-binding') ||
      lower.includes('@import') ||
      lower.includes('javascript:') ||
      lower.includes('vbscript:') ||
      /url\s*\(\s*['"]?data:text\/html/i.test(lower)
    ) {
      continue
    }
    safeDeclarations.push(trimmed)
  }

  return safeDeclarations.join('; ')
}

/**
 * 纯原生 Fail-Closed HTML 净化器
 */
export function sanitizeHtml(rawHtml: string): string {
  if (!rawHtml || typeof rawHtml !== 'string') return ''
  if (typeof document === 'undefined') return ''

  const container = document.createElement('template')
  container.innerHTML = rawHtml

  function cleanNode(node: Node): void {
    const children = Array.from(node.childNodes)
    for (const child of children) {
      if (child.nodeType === Node.ELEMENT_NODE) {
        const el = child as HTMLElement
        const tag = el.tagName.toLowerCase()

        // 1. 危险标签 -> 连同子孙节点彻底移除
        if (DANGEROUS_TAGS.has(tag)) {
          el.remove()
          continue
        }

        // 2. 非白名单标签 -> 展开保留安全子孙（unwrap）
        if (!ALLOWED_TAGS.has(tag)) {
          cleanNode(el)
          const docFrag = document.createDocumentFragment()
          while (el.firstChild) {
            docFrag.appendChild(el.firstChild)
          }
          el.replaceWith(docFrag)
          continue
        }

        // 3. 白名单标签 -> 属性净化
        const attrs = Array.from(el.attributes)
        for (const attr of attrs) {
          const attrName = attr.name.toLowerCase()

          // 事件处理器一律剥离
          if (attrName.startsWith('on')) {
            el.removeAttribute(attr.name)
            continue
          }

          // 不在属性白名单一律剥离
          if (!ALLOWED_ATTRIBUTES.has(attrName)) {
            el.removeAttribute(attr.name)
            continue
          }

          // DOM Clobbering 关键字防护
          if (attrName === 'id' || attrName === 'name') {
            if (FORBIDDEN_IDENTIFIERS.has(attr.value.trim().toLowerCase())) {
              el.removeAttribute(attr.name)
              continue
            }
          }

          // URL 协议合法性检测
          if (attrName === 'href' || attrName === 'src') {
            if (!isSafeUrl(attr.value)) {
              el.removeAttribute(attr.name)
              continue
            }
          }

          // 新窗口链接强制注入 rel="noopener noreferrer"
          if (attrName === 'target' && attr.value.toLowerCase() === '_blank') {
            el.setAttribute('rel', 'noopener noreferrer')
          }

          // 内联样式安全清洗
          if (attrName === 'style') {
            const cleaned = cleanInlineStyle(attr.value)
            if (cleaned) {
              el.setAttribute('style', cleaned)
            } else {
              el.removeAttribute('style')
            }
          }
        }

        // 递归清洗子节点
        cleanNode(el)
      } else if (child.nodeType === Node.COMMENT_NODE) {
        // 移除所有 HTML 注释（防范 IE 条件注释或潜在利用点）
        child.remove()
      } else if (child.nodeType !== Node.TEXT_NODE) {
        // 移除 CDATA 等非常规节点
        child.remove()
      }
    }
  }

  cleanNode(container.content)
  return container.innerHTML
}

/**
 * CSS 净化与 Scoped 作用域重写引擎
 *
 * 将用户定义的自定义 CSS 限制在当前小部件内部，彻底防止影响宿主页面全局样式。
 */
export function sanitizeCss(rawCss: string, scopeSelector?: string): string {
  if (!rawCss || typeof rawCss !== 'string') return ''

  // 1. 过滤危险 @ 规则与表达式
  let css = rawCss
    .replace(/@import\s+[^;]+;/gi, '')
    .replace(/@charset\s+[^;]+;/gi, '')
    .replace(/expression\s*\([^)]*\)/gi, '')
    .replace(/behavior\s*:[^;]+;/gi, '')
    .replace(/url\s*\(\s*['"]?(?:javascript|vbscript):[^)]*\)/gi, '')

  if (!scopeSelector) return css.trim()

  const safeScope = scopeSelector.trim()
  if (!safeScope) return css.trim()

  // 2. 遍历 CSS 规则块，为每个选择器添加作用域前缀
  return css
    .replace(/([^{}]+)\{([^{}]*)\}/g, (_match, selectors, block) => {
      const trimmedSelectors = selectors.trim()
      if (trimmedSelectors.startsWith('@')) {
        return `${trimmedSelectors} {${block}}`
      }
      const scoped = trimmedSelectors
        .split(',')
        .map((s: string) => {
          const sel = s.trim()
          if (!sel) return ''
          // 针对根节点选择器匹配优化
          if (sel === ':root' || sel === 'body' || sel === 'html') {
            return safeScope
          }
          return `${safeScope} ${sel}`
        })
        .filter(Boolean)
        .join(', ')

      return `${scoped} {\n  ${block.trim()}\n}`
    })
    .trim()
}
