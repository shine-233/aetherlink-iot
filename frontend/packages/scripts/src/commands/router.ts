/**
 * 文件用途：生成前端路由页面和模块文件。
 * 核心逻辑：通过 prompts 收集页面名称、模块和路由标题，并写入对应 Vue 与路由文件。
 * 关键注意事项：该命令会修改应用源码目录，重复名称或路径错误可能覆盖预期外文件。
 * 重构建议：可先抽出路径计算与文件内容生成逻辑，并增加存在文件时的更明确确认。
 */
import process from 'node:process'
import path from 'node:path'
import { writeFile } from 'node:fs/promises'
import { existsSync, mkdirSync } from 'node:fs'
import prompts from 'prompts'
import { green, red } from 'kolorist'

/** generate route */
export async function generateRoute() {
  const result = await prompts([
    {
      type: 'text',
      name: 'routeName',
      message: 'please enter route name',
      initial: 'demo-route_child'
    },
    {
      type: 'confirm',
      name: 'addRouteParams',
      message: 'add route params?',
      initial: false
    },
    {
      type: pre => (pre ? 'text' : null),
      name: 'routeParams',
      message: 'please enter route params',
      initial: 'id'
    }
  ])

  const PAGE_DIR_NAME_PATTERN = /^[\w-]+[0-9a-zA-Z]+$/

  if (!PAGE_DIR_NAME_PATTERN.test(result.routeName)) {
    throw new Error(`${red('route name is invalid, it only allow letters, numbers, "-" or "_"')}.
For example:
(1) one level route: ${green('demo-route')}
(2) two level route: ${green('demo-route_child')}
(3) multi level route: ${green('demo-route_child_child')}
(4) group route: ${green('_ignore_demo-route')}'
`)
  }

  const PARAM_REG = /^\w+$/g

  if (!PARAM_REG.test(result.routeParams)) {
    throw new Error(red('route params is invalid, it only allow letters, numbers or "_".'))
  }

  const cwd = process.cwd()

  const [dir, ...rest] = result.routeName.split('_') as string[]

  let routeDir = path.join(cwd, 'src', 'views', dir)

  if (rest.length) {
    routeDir = path.join(routeDir, rest.join('_'))
  }

  if (!existsSync(routeDir)) {
    mkdirSync(routeDir, { recursive: true })
  } else {
    throw new Error(red('route already exists'))
  }

  const fileName = result.routeParams ? `[${result.routeParams}].vue` : 'index.vue'

  const vueTemplate = `<script setup lang="ts"></script>

<template>
  <div>${result.routeName}</div>
</template>

<style scoped></style>
`

  const filePath = path.join(routeDir, fileName)

  await writeFile(filePath, vueTemplate)
}
