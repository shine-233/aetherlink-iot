#!/usr/bin/env node

/**
 * 文件用途：集中执行前端开发阶段的轻量质量检查和架构合规检查。
 * 核心逻辑：调用子命令、读取项目文件并输出彩色检查结果，帮助开发者提交前发现问题。
 * 关键注意事项：新增检查项时保持输出可读，并避免默认格式化整个前端。
 * 重构建议：建议将检查项拆成独立函数和配置清单，便于按 CI/本地场景复用。
 */

import { execSync } from 'child_process'
import fs from 'fs'
import path from 'path'

// 颜色输出
const colors = {
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  reset: '\x1b[0m',
  bold: '\x1b[1m'
}

function log(message, color = 'reset') {}

function logSection(title) {
  log(`\n${colors.bold}${colors.cyan}${'='.repeat(60)}`, 'cyan')
  log(`${colors.bold}${colors.cyan}${title}`, 'cyan')
  log(`${colors.bold}${colors.cyan}${'='.repeat(60)}`, 'cyan')
}

function logSuccess(message) {
  log(`✅ ${message}`, 'green')
}

function logError(message) {
  log(`❌ ${message}`, 'red')
}

function logWarning(message) {
  log(`⚠️  ${message}`, 'yellow')
}

function logInfo(message) {
  log(`ℹ️  ${message}`, 'blue')
}

/**
 * 运行命令并返回结果
 */
function runCommand(command, description) {
  try {
    logInfo(`执行: ${description}`)
    const result = execSync(command, {
      encoding: 'utf8',
      stdio: 'pipe',
      timeout: 120000 // 2分钟超时
    })
    return { success: true, output: result }
  } catch (error) {
    return {
      success: false,
      error: error.message,
      output: error.stdout || error.stderr || ''
    }
  }
}

/**
 * 检查文件是否存在
 */
function checkFileExists(filePath, description) {
  if (fs.existsSync(filePath)) {
    logSuccess(`${description} 存在`)
    return true
  } else {
    logError(`${description} 不存在: ${filePath}`)
    return false
  }
}

/**
 * 检查 PanelV2 架构合规性
 */
function checkPanelV2Compliance() {
  logSection('PanelV2 架构合规性检查')

  const issues = []

  // 检查渲染器是否包含工具栏
  const rendererDir = path.join(process.cwd(), 'src/components/panelv2/renderers')
  if (!fs.existsSync(rendererDir)) {
    logInfo('panelv2 renderer directory not found; skipping PanelV2 compliance check')
    return true
  }

  const renderers = fs.readdirSync(rendererDir)

  renderers.forEach(renderer => {
    const rendererPath = path.join(rendererDir, renderer)
    if (fs.statSync(rendererPath).isDirectory()) {
      const mainRenderer = path.join(
        rendererPath,
        `${renderer.charAt(0).toUpperCase() + renderer.slice(1)}Renderer.vue`
      )

      if (fs.existsSync(mainRenderer)) {
        const content = fs.readFileSync(mainRenderer, 'utf8')

        // 检查是否包含工具栏相关代码
        if (content.includes('toolbar') && content.includes('<div') && content.includes('toolbar')) {
          issues.push(`${renderer} 渲染器可能包含内置工具栏，违反分离原则`)
        }

        // 检查是否使用主题系统
        if (!content.includes('useThemeStore') && content.includes('<style')) {
          issues.push(`${renderer} 渲染器未集成主题系统`)
        }

        // 检查图标使用是否正确
        const iconImports = content.match(/import.*from.*@vicons\/ionicons5/g)
        if (iconImports) {
          iconImports.forEach(importLine => {
            if (!importLine.includes('Outline')) {
              issues.push(`${renderer} 渲染器使用了错误的图标命名规范`)
            }
          })
        }
      }
    }
  })

  if (issues.length === 0) {
    logSuccess('PanelV2 架构合规性检查通过')
    return true
  } else {
    issues.forEach(issue => logError(issue))
    return false
  }
}

/**
 * 检查必要文件
 */
function checkRequiredFiles() {
  logSection('必要文件检查')

  const requiredFiles = [
    { path: 'README.md', desc: 'Frontend README' },
    { path: 'package.json', desc: 'Package 配置文件' }
  ]

  let allExist = true

  requiredFiles.forEach(file => {
    if (!checkFileExists(file.path, file.desc)) {
      allExist = false
    }
  })

  return allExist
}

/**
 * 代码质量检查
 */
function checkCodeQuality() {
  logSection('代码质量检查')

  const checks = [
    {
      command: 'pnpm lint --max-warnings 0',
      description: 'ESLint 代码规范检查',
      required: true
    },
    {
      command: 'pnpm typecheck',
      description: 'TypeScript 类型检查',
      required: true
    }
  ]

  let allPassed = true

  checks.forEach(check => {
    const result = runCommand(check.command, check.description)

    if (result.success) {
      logSuccess(`${check.description} 通过`)
    } else {
      logError(`${check.description} 失败`)
      if (result.output) {
        log(result.output, 'red')
      }
      if (check.required) {
        allPassed = false
      }
    }
  })

  return allPassed
}

/**
 * CSS 语法检查
 */
function checkCSSIssues() {
  logSection('CSS 语法检查')

  const vueFiles = []

  function findVueFiles(dir) {
    const items = fs.readdirSync(dir)

    items.forEach(item => {
      const fullPath = path.join(dir, item)
      const stat = fs.statSync(fullPath)

      if (stat.isDirectory() && !item.startsWith('.') && item !== 'node_modules') {
        findVueFiles(fullPath)
      } else if (item.endsWith('.vue')) {
        vueFiles.push(fullPath)
      }
    })
  }

  try {
    findVueFiles(path.join(process.cwd(), 'src'))
  } catch (error) {
    logWarning('无法扫描 Vue 文件')
    return true
  }

  let issues = []

  vueFiles.forEach(file => {
    try {
      const content = fs.readFileSync(file, 'utf8')

      // 检查常见的 CSS 语法错误
      const cssIssues = [
        {
          pattern: /justify-between;/,
          fix: 'justify-content: space-between;',
          desc: 'justify-between 应该是 justify-content: space-between'
        },
        { pattern: /align-center;/, fix: 'align-items: center;', desc: 'align-center 应该是 align-items: center' },
        { pattern: /#[0-9a-fA-F]{3,6}/, fix: 'CSS 变量', desc: '发现硬编码颜色，应使用主题变量' }
      ]

      cssIssues.forEach(issue => {
        if (issue.pattern.test(content)) {
          issues.push(`${file}: ${issue.desc}`)
        }
      })
    } catch (error) {
      // 忽略无法读取的文件
    }
  })

  if (issues.length === 0) {
    logSuccess('CSS 语法检查通过')
    return true
  } else {
    issues.forEach(issue => logWarning(issue))
    return issues.length < 5 // 少量问题不阻止提交
  }
}

/**
 * 生成质量报告
 */
function generateQualityReport(results) {
  logSection('质量检查报告')

  const passed = results.filter(r => r.passed).length
  const total = results.length
  const percentage = Math.round((passed / total) * 100)

  log(`\n检查项目: ${total}`)
  log(`通过项目: ${passed}`)
  log(`通过率: ${percentage}%`)

  if (percentage >= 90) {
    logSuccess('代码质量优秀 (A级)')
  } else if (percentage >= 80) {
    logInfo('代码质量良好 (B级)')
  } else if (percentage >= 70) {
    logWarning('代码质量一般 (C级)，建议改进')
  } else {
    logError('代码质量较差 (D级)，必须修复')
  }

  return percentage >= 70
}

/**
 * 主函数
 */
function main() {
  log(`${colors.bold}${colors.magenta}🚀 AetherLink IoT 开发质量检查工具`, 'magenta')
  log(`${colors.magenta}确保代码提交前符合项目质量标准\n`, 'magenta')

  const results = []

  // 执行各项检查
  results.push({ name: '必要文件检查', passed: checkRequiredFiles() })
  results.push({ name: 'PanelV2架构合规性', passed: checkPanelV2Compliance() })
  results.push({ name: '代码质量检查', passed: checkCodeQuality() })
  results.push({ name: 'CSS语法检查', passed: checkCSSIssues() })

  // 生成报告
  const overallPassed = generateQualityReport(results)

  // 输出建议
  logSection('改进建议')

  if (overallPassed) {
    logSuccess('恭喜！代码质量符合提交标准')
    log('\n📋 提交前请确认：')
    log('1. 已完成当前分支需要的提交前自检项')
    log('2. 功能已手动测试并正常工作')
    log('3. 在不同主题下样式显示正常')
    log('4. 浏览器控制台无错误和警告')
  } else {
    logError('代码质量不符合提交标准，请先修复问题')
    log('\n🔧 修复建议：')
    log('1. 运行 pnpm lint --fix 自动修复规范问题')
    log('2. 检查 TypeScript 类型错误并修复')
    log('3. 确保所有组件集成主题系统')
    log('4. 移除渲染器中的工具栏实现')
  }

  process.exit(overallPassed ? 0 : 1)
}

// 运行检查
main()
