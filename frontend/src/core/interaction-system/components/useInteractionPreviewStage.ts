import { ref, onMounted, type Ref } from 'vue'

import type { InteractionResponse } from './interactionPreviewTypes'
import { applyInteractionPreviewResponse } from './interactionPreviewHelpers'

/**
 * 交互预览舞台（画布元素 + 运行时样式 + 文本）的共享实现。
 *
 * InteractionPreview 与 InteractionTemplatePreview 原本各自维护一套
 * 「保存原始样式 → 应用响应动作 → 重置」的逻辑，这里统一收敛，
 * 两个组件只保留各自差异化的编排（日志、定时器、按钮语义）。
 */

export interface InteractionPreviewStageOptions {
  /** 预览元素的初始 / 重置文本 */
  initialText: () => string
  /** 重置时写回的 class，默认 preview-element */
  elementClass?: string
  /** 重置后是否把捕获的原始样式写回内联样式（模板预览需要） */
  restoreOriginalStyles?: boolean
}

export interface InteractionPreviewStage {
  previewElement: Ref<HTMLElement | undefined>
  content: Ref<string>
  runtimeStyles: Ref<Record<string, string>>
  originalStyles: Ref<Record<string, string>>
  /** 把一个响应动作应用到预览元素 */
  applyResponse: (response: InteractionResponse) => void
  /** 清空预览元素的所有运行时效果 */
  resetStage: () => void
}

function captureComputedStyles(element: HTMLElement, properties: readonly string[]) {
  const computedStyles = window.getComputedStyle(element) as unknown as Record<string, string>
  const captured: Record<string, string> = {}
  for (const property of properties) {
    captured[property] = computedStyles[property]
  }
  return captured
}

const BASE_CAPTURED_PROPERTIES = [
  'backgroundColor',
  'color',
  'borderColor',
  'opacity',
  'transform',
  'visibility'
] as const

export function useInteractionPreviewStage(options: InteractionPreviewStageOptions): InteractionPreviewStage {
  const previewElement = ref<HTMLElement>()
  const content = ref('')
  const runtimeStyles = ref<Record<string, string>>({})
  const originalStyles = ref<Record<string, string>>({})

  const captureOriginalStyles = () => {
    if (!previewElement.value) return

    const properties = options.restoreOriginalStyles
      ? [...BASE_CAPTURED_PROPERTIES, 'width', 'height']
      : BASE_CAPTURED_PROPERTIES

    originalStyles.value = captureComputedStyles(previewElement.value, properties)
  }

  const applyResponse = (response: InteractionResponse) => {
    const element = previewElement.value
    if (!element) return

    const setRuntimeStyle = (property: string, styleValue: unknown) => {
      const normalizedValue = String(styleValue)
      const cssProperty = property.replace(/[A-Z]/g, (match) => `-${match.toLowerCase()}`)
      element.style.setProperty(cssProperty, normalizedValue)
      runtimeStyles.value = {
        ...runtimeStyles.value,
        [property]: normalizedValue
      }
    }

    applyInteractionPreviewResponse(element, response, {
      setRuntimeStyle,
      setContent: (value) => {
        content.value = value
      }
    })
  }

  const resetStage = () => {
    const element = previewElement.value
    if (!element) return

    element.style.cssText = ''
    element.className = options.elementClass ?? 'preview-element'
    runtimeStyles.value = {}
    content.value = options.initialText()

    if (options.restoreOriginalStyles) {
      Object.assign(element.style, originalStyles.value)
    }
  }

  onMounted(() => {
    content.value = options.initialText()
    captureOriginalStyles()
  })

  return {
    previewElement,
    content,
    runtimeStyles,
    originalStyles,
    applyResponse,
    resetStage
  }
}
