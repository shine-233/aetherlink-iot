import { defineComponent, h } from 'vue'
import type { VNodeChild } from 'vue'
import { NText } from 'naive-ui'
import type { FormRules, SelectRenderLabel, SelectRenderTag } from 'naive-ui'

export const COMMAND_DOWNLINK_TARGET_TOPIC = 'devices/command/{device_number}/+'

export interface TopicMapping {
  id?: string | number
  mapping_name: string
  direction: 'up' | 'down'
  original_topic: string
  target_topic: string
  data_identifier?: string
  description: string
  priority?: number
  enabled?: boolean
}

export interface TopicOptionSource {
  value: string
  label: string
  descriptionKey: string
}

export interface TopicOption {
  value: string
  label: string
  description: string
}

export const createDefaultTopicMapping = (): TopicMapping => ({
  mapping_name: '',
  direction: 'down',
  original_topic: '',
  target_topic: '',
  data_identifier: '',
  description: '',
  priority: 0,
  enabled: true
})

export const createTopicMappingFromEdit = (mapping: TopicMapping): TopicMapping => ({
  id: mapping.id,
  mapping_name: mapping.mapping_name || '',
  direction: mapping.direction || 'down',
  original_topic: mapping.original_topic || '',
  target_topic: mapping.target_topic || '',
  data_identifier: mapping.data_identifier || '',
  description: mapping.description || '',
  priority: mapping.priority ?? 0,
  enabled: mapping.enabled ?? true
})

export const createTopicMappingSnapshot = (mapping?: TopicMapping | null): TopicMapping =>
  mapping ? createTopicMappingFromEdit(mapping) : createDefaultTopicMapping()

export const isCommandDownlinkTargetTopic = (topic: string) => topic === COMMAND_DOWNLINK_TARGET_TOPIC

export const applyTopicMappingDirectionChange = (mapping: TopicMapping) => {
  mapping.target_topic = ''
  mapping.data_identifier = ''
}

export const applyTopicMappingTargetTopicChange = (mapping: TopicMapping) => {
  if (!isCommandDownlinkTargetTopic(mapping.target_topic)) {
    mapping.data_identifier = ''
  }
}

export const createTopicMappingSaveSnapshot = (mapping: TopicMapping): TopicMapping => {
  const snapshot = { ...mapping }
  applyTopicMappingTargetTopicChange(snapshot)
  return snapshot
}

export const buildTopicMappingProbeTopic = (topic: string) => {
  const normalizedTopic = topic.trim().replace(/^\/+|\/+$/g, '')
  if (!normalizedTopic) return ''
  return normalizedTopic
    .split('/')
    .map((segment) => {
      if (segment === '+') return 'first-device-001'
      if (segment.startsWith('{') && segment.endsWith('}')) {
        const variableName = segment.slice(1, -1)
        if (variableName === 'device_number') return 'first-device-001'
        if (variableName === 'message_id') return '1'
        return `probe-${variableName}`
      }
      return segment
    })
    .join('/')
}

export const createTopicMappingRules = (t: (key: string) => string): FormRules => ({
  mapping_name: [
    {
      required: true,
      message: t('generate.topicMapping.validation.mappingName'),
      trigger: 'blur'
    }
  ],
  direction: [
    {
      required: true,
      message: t('generate.topicMapping.validation.direction'),
      trigger: 'change'
    }
  ],
  original_topic: [
    {
      required: true,
      message: t('generate.topicMapping.validation.originalTopic'),
      trigger: 'blur'
    }
  ],
  target_topic: [
    {
      required: true,
      message: t('generate.topicMapping.validation.targetTopic'),
      trigger: 'change'
    }
  ]
})

export const uplinkTopicOptionSource: TopicOptionSource[] = [
  {
    label: 'devices/telemetry',
    value: 'devices/telemetry',
    descriptionKey: 'generate.topicMapping.options.uplink.devicesTelemetry'
  },
  {
    label: 'devices/attributes/{message_id}',
    value: 'devices/attributes/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.devicesAttributes'
  },
  {
    label: 'devices/event/{message_id}',
    value: 'devices/event/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.devicesEvent'
  },
  {
    label: 'ota/devices/progress',
    value: 'ota/devices/progress',
    descriptionKey: 'generate.topicMapping.options.uplink.otaProgress'
  },
  {
    label: 'gateway/telemetry',
    value: 'gateway/telemetry',
    descriptionKey: 'generate.topicMapping.options.uplink.gatewayTelemetry'
  },
  {
    label: 'gateway/attributes/{message_id}',
    value: 'gateway/attributes/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.gatewayAttributes'
  },
  {
    label: 'gateway/event/{message_id}',
    value: 'gateway/event/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.gatewayEvent'
  },
  {
    label: 'devices/command/response/{message_id}',
    value: 'devices/command/response/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.devicesCommandResponse'
  },
  {
    label: 'devices/attributes/set/response/{message_id}',
    value: 'devices/attributes/set/response/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.devicesAttributesResponse'
  },
  {
    label: 'gateway/command/response/{message_id}',
    value: 'gateway/command/response/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.gatewayCommandResponse'
  },
  {
    label: 'gateway/attributes/set/response/{message_id}',
    value: 'gateway/attributes/set/response/{message_id}',
    descriptionKey: 'generate.topicMapping.options.uplink.gatewayAttributesResponse'
  }
]

export const downlinkTopicOptionSource: TopicOptionSource[] = [
  {
    label: 'devices/telemetry/control/{device_number}',
    value: 'devices/telemetry/control/{device_number}',
    descriptionKey: 'generate.topicMapping.options.downlink.devicesTelemetryControl'
  },
  {
    label: 'devices/attributes/set/{device_number}/+',
    value: 'devices/attributes/set/{device_number}/+',
    descriptionKey: 'generate.topicMapping.options.downlink.devicesAttributesSet'
  },
  {
    label: 'devices/attributes/get/{device_number}',
    value: 'devices/attributes/get/{device_number}',
    descriptionKey: 'generate.topicMapping.options.downlink.devicesAttributesGet'
  },
  {
    label: COMMAND_DOWNLINK_TARGET_TOPIC,
    value: COMMAND_DOWNLINK_TARGET_TOPIC,
    descriptionKey: 'generate.topicMapping.options.downlink.devicesCommand'
  },
  {
    label: 'ota/devices/inform/{device_number}',
    value: 'ota/devices/inform/{device_number}',
    descriptionKey: 'generate.topicMapping.options.downlink.otaInform'
  },
  {
    label: 'gateway/telemetry/control/{device_number}',
    value: 'gateway/telemetry/control/{device_number}',
    descriptionKey: 'generate.topicMapping.options.downlink.gatewayTelemetryControl'
  },
  {
    label: 'gateway/attributes/set/{device_number}/+',
    value: 'gateway/attributes/set/{device_number}/+',
    descriptionKey: 'generate.topicMapping.options.downlink.gatewayAttributesSet'
  },
  {
    label: 'gateway/attributes/get/{device_number}',
    value: 'gateway/attributes/get/{device_number}',
    descriptionKey: 'generate.topicMapping.options.downlink.gatewayAttributesGet'
  },
  {
    label: 'gateway/command/{device_number}/+',
    value: 'gateway/command/{device_number}/+',
    descriptionKey: 'generate.topicMapping.options.downlink.gatewayCommand'
  },
  {
    label: 'devices/attributes/response/{device_number}/+',
    value: 'devices/attributes/response/{device_number}/+',
    descriptionKey: 'generate.topicMapping.options.downlink.devicesAttributesResponse'
  },
  {
    label: 'devices/event/response/{device_number}/+',
    value: 'devices/event/response/{device_number}/+',
    descriptionKey: 'generate.topicMapping.options.downlink.devicesEventResponse'
  },
  {
    label: 'gateway/attributes/response/{device_number}/+',
    value: 'gateway/attributes/response/{device_number}/+',
    descriptionKey: 'generate.topicMapping.options.downlink.gatewayAttributesResponse'
  },
  {
    label: 'gateway/event/response/{device_number}/+',
    value: 'gateway/event/response/{device_number}/+',
    descriptionKey: 'generate.topicMapping.options.downlink.gatewayEventResponse'
  }
]

export const buildTopicOptions = (sources: TopicOptionSource[], t: (key: string) => string): TopicOption[] =>
  sources.map((option) => ({
    label: option.label,
    value: option.value,
    description: t(option.descriptionKey)
  }))

const renderInlineMarkdown = (text: string): VNodeChild[] => {
  const nodes: VNodeChild[] = []
  const pattern = /(\*\*([^*]+)\*\*|`([^`]+)`)/g
  let cursor = 0
  let match: RegExpExecArray | null

  while ((match = pattern.exec(text)) !== null) {
    if (match.index > cursor) {
      nodes.push(text.slice(cursor, match.index))
    }

    if (match[2] !== undefined) {
      nodes.push(h('strong', match[2]))
    } else {
      nodes.push(h('code', match[3] || ''))
    }

    cursor = match.index + match[0].length
  }

  if (cursor < text.length) {
    nodes.push(text.slice(cursor))
  }

  return nodes
}

const renderMarkdownNodes = (text: string): VNodeChild[] => {
  if (!text) return []

  const nodes: VNodeChild[] = []
  const blockPattern = /```([\s\S]*?)```/g
  let cursor = 0
  let blockMatch: RegExpExecArray | null

  const appendInline = (segment: string) => {
    segment.split('\n').forEach((line, index) => {
      if (index > 0) nodes.push(h('br'))
      nodes.push(...renderInlineMarkdown(line))
    })
  }

  while ((blockMatch = blockPattern.exec(text)) !== null) {
    if (blockMatch.index > cursor) {
      appendInline(text.slice(cursor, blockMatch.index))
    }
    nodes.push(h('pre', { class: 'code-block' }, [h('code', blockMatch[1].trim())]))
    cursor = blockMatch.index + blockMatch[0].length
  }

  if (cursor < text.length) {
    appendInline(text.slice(cursor))
  }

  return nodes
}

export const MarkdownTip = defineComponent({
  name: 'MarkdownTip',
  props: {
    text: {
      type: String,
      required: true
    }
  },
  setup(props) {
    return () => h('div', { class: 'tip-text' }, renderMarkdownNodes(props.text))
  }
})

export const renderTopicLabel: SelectRenderLabel = (option) => {
  const topicOption = option as unknown as TopicOption
  return h(
    'div',
    {
      style: {
        display: 'flex',
        alignItems: 'center'
      }
    },
    [
      h(
        'div',
        {
          style: {
            flex: 1,
            padding: '4px 0'
          }
        },
        [
          h('div', { style: { fontSize: '14px', color: '#333', lineHeight: '1.5' } }, topicOption.label),
          h(
            NText,
            { depth: 3, tag: 'div', style: { fontSize: '12px', lineHeight: '1.4', marginTop: '2px' } },
            {
              default: () => topicOption.description
            }
          )
        ]
      )
    ]
  )
}

export const renderTopicTag: SelectRenderTag = ({ option }) => {
  const topicOption = option as unknown as TopicOption
  return h(
    'div',
    {
      style: {
        display: 'flex',
        alignItems: 'center'
      }
    },
    [h('div', { style: { fontSize: '14px' } }, topicOption.label)]
  )
}
