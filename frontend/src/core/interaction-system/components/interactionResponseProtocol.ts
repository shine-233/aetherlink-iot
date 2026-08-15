export type InteractionActionType = '' | 'jump' | 'modify' | 'custom' | 'none'
export type InteractionUrlType = 'external' | 'internal'

export type JumpEditState = {
  urlType: InteractionUrlType
  url: string
  target: string
  selectedMenuPath: string
  shouldLoadMenuOptions: boolean
}

export type ModifyEditState = {
  targetComponentId: string
  targetProperty: string
  updateValue: string
}

export type JumpResponseInput = {
  urlType: InteractionUrlType
  url: string
  selectedMenuPath: string
  target: string
}

export type ModifyResponseInput = ModifyEditState & {
  bindingPath: string
}

export function getInteractionActionType(interaction: any): InteractionActionType {
  const firstResponse = interaction?.responses?.[0]
  if (!firstResponse) return 'none'

  if (firstResponse.action === 'jump' || firstResponse.action === 'navigateToUrl') return 'jump'
  if (firstResponse.action === 'modify' || firstResponse.action === 'updateComponentData') return 'modify'

  return 'custom'
}

export function isExternalInteractionUrl(url: string) {
  return url.startsWith('http') || url.startsWith('https')
}

export function isInternalInteractionUrl(url: string) {
  return url.startsWith('/')
}

export function readCompatibilityJumpEditState(url: string, target?: string): JumpEditState {
  const isInternal = Boolean(url) && !isExternalInteractionUrl(url)

  return {
    urlType: isInternal ? 'internal' : 'external',
    url,
    target: target || '_blank',
    selectedMenuPath: isInternal ? url : '',
    shouldLoadMenuOptions: isInternal
  }
}

export function readJumpEditState(response: any): JumpEditState {
  if (!response?.jumpConfig) {
    return readCompatibilityJumpEditState(response?.value || '', response?.target)
  }

  const jumpConfig = response.jumpConfig
  const urlType: InteractionUrlType = jumpConfig.jumpType === 'internal' ? 'internal' : 'external'
  const url = urlType === 'external' ? jumpConfig.url || '' : jumpConfig.internalPath || ''

  return {
    urlType,
    url,
    target: jumpConfig.target || '_self',
    selectedMenuPath: urlType === 'internal' ? jumpConfig.internalPath || '' : '',
    shouldLoadMenuOptions: urlType === 'internal'
  }
}

export function readModifyEditState(response: any): ModifyEditState {
  const modifyConfig = response?.modifyConfig ?? response ?? {}

  return {
    targetComponentId: modifyConfig.targetComponentId || '',
    targetProperty: modifyConfig.targetProperty || '',
    updateValue: modifyConfig.updateValue || ''
  }
}

export function buildJumpResponse(input: JumpResponseInput) {
  const jumpConfig: Record<string, any> = {
    jumpType: input.urlType === 'external' ? 'external' : 'internal',
    target: input.target || '_self'
  }

  if (input.urlType === 'external') {
    jumpConfig.url = input.url
  } else {
    jumpConfig.internalPath = input.selectedMenuPath || input.url
  }

  return {
    action: 'jump',
    jumpConfig,
    value: input.url,
    target: input.target
  }
}

export function buildModifyResponse(input: ModifyResponseInput) {
  const modifyConfig = {
    targetComponentId: input.targetComponentId,
    targetProperty: input.targetProperty,
    updateValue: input.updateValue,
    updateMode: 'replace',
    bindingPath: input.bindingPath
  }

  return {
    action: 'modify',
    modifyConfig,
    targetComponentId: input.targetComponentId,
    targetProperty: input.targetProperty,
    updateValue: input.updateValue
  }
}
