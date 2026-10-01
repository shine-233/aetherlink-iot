/**
 * 文件用途：验证构建期 gzip 预压缩插件的文件筛选与输出行为。
 * 核心逻辑：在临时目录写入文本/图片/小文件，执行 precompressDirectory 后断言只为可压缩文本生成可解压的 `.gz`。
 * 关键注意事项：nginx `gzip_static on` 依赖这些 `.gz` 与原文件内容一致。
 */
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import zlib from 'node:zlib'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { precompressDirectory, shouldPrecompress } from '../../../build/plugins/compress'

describe('build precompress plugin', () => {
  let dir = ''

  beforeEach(() => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-precompress-'))
  })

  afterEach(() => {
    fs.rmSync(dir, { recursive: true, force: true })
  })

  it('only selects text assets above the size threshold', () => {
    expect(shouldPrecompress('assets/index-abc.js', 4096)).toBe(true)
    expect(shouldPrecompress('assets/index-abc.CSS', 4096)).toBe(true)
    expect(shouldPrecompress('index.html', 4096)).toBe(true)
    expect(shouldPrecompress('assets/logo.png', 4096)).toBe(false)
    expect(shouldPrecompress('assets/tiny.js', 100)).toBe(false)
  })

  it('writes decompressible .gz siblings for text files and leaves others alone', () => {
    const js = 'export const value = "aetherlink";\n'.repeat(200)
    fs.mkdirSync(path.join(dir, 'assets'))
    fs.writeFileSync(path.join(dir, 'assets', 'index-abc.js'), js)
    fs.writeFileSync(path.join(dir, 'assets', 'tiny.css'), 'a{}')
    fs.writeFileSync(path.join(dir, 'assets', 'cover.png'), Buffer.alloc(4096, 1))

    expect(precompressDirectory(dir)).toBe(1)

    const gz = path.join(dir, 'assets', 'index-abc.js.gz')
    expect(zlib.gunzipSync(fs.readFileSync(gz)).toString()).toBe(js)
    expect(fs.existsSync(path.join(dir, 'assets', 'tiny.css.gz'))).toBe(false)
    expect(fs.existsSync(path.join(dir, 'assets', 'cover.png.gz'))).toBe(false)

    // Re-running must not compress the generated .gz files again.
    expect(precompressDirectory(dir)).toBe(1)
    expect(fs.existsSync(`${gz}.gz`)).toBe(false)
  })

  it('returns 0 for a missing output directory', () => {
    expect(precompressDirectory(path.join(dir, 'missing'))).toBe(0)
  })
})
