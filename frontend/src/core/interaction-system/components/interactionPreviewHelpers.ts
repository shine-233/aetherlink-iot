import type {
  InteractionConfig,
  InteractionEventType,
  InteractionResponse
} from './interactionPreviewTypes'

type Translate = (key: string) => string
type InteractionActionType = InteractionResponse['action']

export type PreviewLogType = 'info' | 'success' | 'warning' | 'error'

export function getInteractionEventTagType(event: InteractionEventType) {
  const typeMap: Record<string, string> = {
    click: 'success',
    hover: 'info',
    focus: 'warning',
    blur: 'default',
    custom: 'error'
  }
  return typeMap[event] || 'default'
}

export function getInteractionEventDisplayName(event: InteractionEventType, t: Translate) {
  const nameMap: Record<string, string> = {
    click: t('interaction.events.click'),
    hover: t('interaction.events.hover'),
    focus: t('interaction.events.focus'),
    blur: t('interaction.events.blur'),
    custom: t('interaction.events.custom')
  }
  return nameMap[event] || event
}

export function getInteractionActionDisplayName(action: InteractionActionType, t: Translate) {
  const nameMap: Record<string, string> = {
    changeBackgroundColor: t('interaction.actions.changeBackgroundColor'),
    changeTextColor: t('interaction.actions.changeTextColor'),
    changeBorderColor: t('interaction.actions.changeBorderColor'),
    changeSize: t('interaction.actions.changeSize'),
    changeOpacity: t('interaction.actions.changeOpacity'),
    changeTransform: t('interaction.actions.changeTransform'),
    changeVisibility: t('interaction.actions.changeVisibility'),
    changeContent: t('interaction.actions.changeContent'),
    triggerAnimation: t('interaction.actions.triggerAnimation'),
    custom: t('interaction.actions.custom')
  }
  return nameMap[action] || action
}

export function formatInteractionResponseValue(response: InteractionResponse, t: Translate) {
  const { action, value } = response

  switch (action) {
    case 'changeBackgroundColor':
    case 'changeTextColor':
    case 'changeBorderColor':
      return value
    case 'changeSize':
      if (typeof value === 'object' && value) {
        const sizeValue = value as { width?: number | string; height?: number | string }
        return `${sizeValue.width || '?'}\u00d7${sizeValue.height || '?'}`
      }
      return String(value)
    case 'changeOpacity':
      return `${Math.round((value as number) * 100)}%`
    case 'changeTransform':
      return String(value)
    case 'changeVisibility':
      return value === 'visible' ? t('interaction.visibility.visible') : t('interaction.visibility.hidden')
    case 'changeContent':
      return String(value).substring(0, 20) + (String(value).length > 20 ? '...' : '')
    case 'triggerAnimation':
      return String(value)
    case 'custom':
      try {
        return JSON.stringify(value).substring(0, 30) + '...'
      } catch {
        return String(value)
      }
    default:
      return String(value)
  }
}

export function formatInteractionPreviewTime(date: Date) {
  return date.toLocaleTimeString('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

export function getEnabledInteractionsByEvent(interactions: InteractionConfig[], eventType: InteractionEventType) {
  return interactions
    .map((interaction, index) => ({ interaction, index }))
    .filter(({ interaction }) => interaction.event === eventType && interaction.enabled)
    .sort((a, b) => (b.interaction.priority || 0) - (a.interaction.priority || 0))
}

interface ApplyPreviewResponseOptions {
  setRuntimeStyle: (property: string, styleValue: unknown) => void
  setContent: (value: string) => void
}

export function applyInteractionPreviewResponse(
  element: HTMLElement,
  response: InteractionResponse,
  options: ApplyPreviewResponseOptions
) {
  const { action, value, duration = 300, easing = 'ease' } = response
  const { setRuntimeStyle, setContent } = options

  element.style.transition = `all ${duration}ms ${easing}`

  switch (action) {
    case 'changeBackgroundColor':
      setRuntimeStyle('backgroundColor', value)
      break
    case 'changeTextColor':
      setRuntimeStyle('color', value)
      break
    case 'changeBorderColor':
      setRuntimeStyle('borderStyle', 'solid')
      setRuntimeStyle('borderTopStyle', 'solid')
      setRuntimeStyle('borderRightStyle', 'solid')
      setRuntimeStyle('borderBottomStyle', 'solid')
      setRuntimeStyle('borderLeftStyle', 'solid')
      setRuntimeStyle('borderColor', value)
      setRuntimeStyle('borderTopColor', value)
      setRuntimeStyle('borderRightColor', value)
      setRuntimeStyle('borderBottomColor', value)
      setRuntimeStyle('borderLeftColor', value)
      break
    case 'changeSize':
      if (typeof value === 'object' && value) {
        const sizeValue = value as { width?: number | string; height?: number | string }
        if (sizeValue.width) setRuntimeStyle('width', `${sizeValue.width}px`)
        if (sizeValue.height) setRuntimeStyle('height', `${sizeValue.height}px`)
      }
      break
    case 'changeOpacity':
      setRuntimeStyle('opacity', value)
      break
    case 'changeTransform':
      setRuntimeStyle('transform', value)
      break
    case 'changeVisibility':
      setRuntimeStyle('visibility', value)
      break
    case 'changeContent':
      setContent(String(value))
      break
    case 'triggerAnimation':
      element.style.animation = ''
      void element.offsetHeight
      setRuntimeStyle('animation', `${value} ${duration}ms ${easing}`)
      break
    case 'custom':
      if (typeof value === 'object' && value) {
        for (const [property, styleValue] of Object.entries(value)) {
          setRuntimeStyle(property, styleValue)
        }
      }
      break
  }
}
