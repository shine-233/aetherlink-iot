/**
 * 文件用途：axios paramsSerializer 的零依赖实现，替代 `qs.stringify`。
 * 核心逻辑：逐字节复刻 qs@6 默认选项（arrayFormat=indices、RFC3986、utf-8、
 *   allowDots=false、skipNulls=false）的输出：
 *   - 数组/对象递归展开为 `a[0]=1&b[c]=2`，整段 key（含方括号）一起 percent-encode；
 *   - null → `k=`；undefined、函数被跳过；空数组/空对象不产出任何片段；
 *   - Date → toISOString()；其余标量走 String()。
 * 关键注意事项：qs 及其依赖链（side-channel / get-intrinsic / object-inspect…）
 *   约 90KB 未压缩，此前全部压在登录页入口 chunk 上；这里保持线上 query 串完全不变。
 *   唯一有意偏差：孤立代理项（lone surrogate）qs 会拼出错误 UTF-8，这里替换为 U+FFFD。
 */

type ParamValue = unknown

const RFC3986_EXTRA = /[!'()*]/g

function encodeRfc3986(input: string): string {
  let encoded: string
  try {
    encoded = encodeURIComponent(input)
  } catch {
    // encodeURIComponent 对孤立代理项抛 URIError；替换后再编码，避免整次请求失败。
    encoded = encodeURIComponent(input.replace(/[\uD800-\uDFFF]/g, '\uFFFD'))
  }
  return encoded.replace(RFC3986_EXTRA, (ch) => `%${ch.charCodeAt(0).toString(16).toUpperCase()}`)
}

function appendValue(out: string[], prefix: string, value: ParamValue, seen: WeakSet<object>) {
  if (value === undefined || typeof value === 'function') return

  if (value === null) {
    out.push(`${encodeRfc3986(prefix)}=`)
    return
  }

  if (value instanceof Date) {
    out.push(`${encodeRfc3986(prefix)}=${encodeRfc3986(value.toISOString())}`)
    return
  }

  if (typeof value === 'object') {
    if (seen.has(value)) throw new RangeError('Cyclic object value')
    seen.add(value)
    for (const key of Object.keys(value)) {
      appendValue(out, `${prefix}[${key}]`, (value as Record<string, unknown>)[key], seen)
    }
    seen.delete(value)
    return
  }

  out.push(`${encodeRfc3986(prefix)}=${encodeRfc3986(String(value))}`)
}

/** 与 `qs.stringify(params)`（默认选项）输出一致的 query 串，不带前导 `?`。 */
export function serializeParams(params: unknown): string {
  if (params === null || typeof params !== 'object') return ''

  const out: string[] = []
  const seen = new WeakSet<object>()
  seen.add(params)
  for (const key of Object.keys(params)) {
    appendValue(out, key, (params as Record<string, unknown>)[key], seen)
  }
  return out.join('&')
}
