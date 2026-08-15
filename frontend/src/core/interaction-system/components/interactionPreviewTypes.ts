export type InteractionEventType = 'click' | 'hover' | 'focus' | 'blur' | 'custom' | 'dataChange' | string

export type InteractionActionType =
  | 'changeBackgroundColor'
  | 'changeTextColor'
  | 'changeBorderColor'
  | 'changeSize'
  | 'changeOpacity'
  | 'changeTransform'
  | 'changeVisibility'
  | 'changeContent'
  | 'triggerAnimation'
  | 'custom'
  | string

export interface InteractionResponse {
  action: InteractionActionType
  value?: any
  duration?: number
  delay?: number
  easing?: string
  [key: string]: any
}

export interface InteractionConfig {
  id?: string
  componentId?: string
  event: InteractionEventType
  eventType?: InteractionEventType
  responses: InteractionResponse[]
  enabled?: boolean
  priority?: number
  name?: string
  watchedProperty?: string
  condition?: any
  [key: string]: any
}
