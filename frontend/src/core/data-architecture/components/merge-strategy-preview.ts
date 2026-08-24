import { defaultScriptEngine } from '@/core/script-engine'

export type PreviewMergeStrategy =
  | { type: 'object' }
  | { type: 'array' }
  | { type: 'condition' }
  | { type: 'script'; script?: string }

export interface MergePreviewResult {
  success: boolean
  data?: unknown
  error?: string
}

/**
 * Preview a merge strategy without evaluating code in the browser host context.
 * Script previews use the same local script-engine policy as runtime merges.
 */
export async function previewMergeStrategy(
  items: unknown[],
  strategy: PreviewMergeStrategy
): Promise<MergePreviewResult> {
  try {
    switch (strategy.type) {
      case 'object':
        return {
          success: true,
          data: Object.assign(
            {},
            ...items.filter(item => item !== null && typeof item === 'object' && !Array.isArray(item))
          )
        }
      case 'array':
        return { success: true, data: items }
      case 'condition':
        return { success: true, data: items.find(item => item !== null && item !== undefined) ?? {} }
      case 'script': {
        if (!strategy.script?.trim()) {
          return { success: false, error: '请输入合并脚本后再预览' }
        }

        const result = await defaultScriptEngine.execute(strategy.script, { items })
        if (!result.success) {
          const failure = result.error
          return {
            success: false,
            error: typeof failure === 'string' ? failure : failure instanceof Error ? failure.message : '脚本预览执行失败'
          }
        }

        return { success: true, data: result.data }
      }
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '合并预览执行失败'
    }
  }
}
