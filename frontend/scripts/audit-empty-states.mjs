/**
 * ROADMAP A4/B4：空态覆盖率审计脚本（只读扫描，不自动改码）。
 * 判定一个 Vue 列表/表格视图是否缺少空态：模板含 table/tabs/卡片列表类容器，
 *   且同文件中没有 n-empty / NEmpty / empty 相关占位或 description 提示时，标记为 gap。
 * 用法: node frontend/scripts/audit-empty-states.mjs [--report path]
 * 输出: 控制台统计 + 可选 JSON 报告（gap 清单按视图路径排序）。
 */
import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { join, extname } from 'node:path'

const root = process.cwd()
const viewsDir = join(root, 'frontend/src/views')
const reportPathArg = process.argv.find(a => a.startsWith('--report='))
const reportPath = reportPathArg ? reportPathArg.split('=')[1] : join(root, 'frontend/scripts/empty-state-audit-report.json')

function walk(dir) {
  const out = []
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (name.startsWith('.')) continue
    if (statSync(full).isDirectory()) {
      if (['node_modules', '__tests__', 'test', 'assets'].includes(name)) continue
      out.push(...walk(full))
    } else if (extname(full) === '.vue') {
      out.push(full)
    }
  }
  return out
}

function looksListy(src) {
  return /<n-table|el-table|<table\b|v-for=|:columns=|n-data-table/.test(src)
}

function hasEmptyState(src) {
  return /n-empty|NEmpty|empty|description\s*[:=]|noData|emptyData|isEmpty/.test(src)
}

const files = walk(viewsDir)
const gaps = []
for (const file of files) {
  const src = readFileSync(file, 'utf8')
  if (!looksListy(src)) continue
  if (!hasEmptyState(src)) {
    gaps.push(file.replaceAll('\\', '/').slice(root.length + 1))
  }
}

const totalListy = files.filter(f => looksListy(readFileSync(f, 'utf8'))).length
const report = {
  generatedAt: new Date().toISOString(),
  totalVueViews: files.length,
  listyViews: totalListy,
  viewsWithEmptyState: totalListy - gaps.length,
  viewsMissingEmptyState: gaps.length,
  coverageRate: totalListy ? ((totalListy - gaps.length) / totalListy) * 100 : 0,
  gaps
}
writeFileSync(reportPath, JSON.stringify(report, null, 2), 'utf8')
console.log(
  `listy=${totalListy} withEmpty=${totalListy - gaps.length} missing=${gaps.length} rate=${report.coverageRate.toFixed(1)}%`
)
console.log('report ->', reportPath)
