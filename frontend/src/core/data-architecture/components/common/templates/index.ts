/**
 * 文件用途: 动态参数值模板注册表。
 * 核心逻辑: 定义参数模板类型并按模板组件异步加载对应编辑控件。
 * 关键注意事项: 模板类型名称和加载路径会影响动态参数编辑器的渲染与打包。
 * 重构建议: 将模板元数据、组件加载和默认值策略拆分，便于扩展新参数输入模式。
 */

import type { Component, AsyncComponentLoader } from 'vue'

// 模板类型枚举
export enum ParameterTemplateType {
  MANUAL = 'manual', // 手动输入
  DROPDOWN = 'dropdown', // 下拉选择
  PROPERTY = 'property', // 属性绑定（动态）
  COMPONENT = 'component' // 复杂组件模板
}

// 模板选项接口
export interface TemplateOption {
  label: string
  value: string | number | boolean
  description?: string
}

// 组件模板配置接口
export interface ComponentTemplateConfig {
  /** 组件名称字符串或组件导入函数或组件实例 */
  component: string | Component | AsyncComponentLoader<Component>
  /** 传递给组件的props */
  props?: Record<string, any>
  /** 组件事件监听器映射 */
  events?: Record<string, string>
  /** 组件插槽配置 */
  slots?: Record<string, any>
  /** 组件渲染配置 */
  renderConfig?: {
    /** 是否包装在容器中 */
    wrapped?: boolean
    /** 容器样式类 */
    containerClass?: string
    /** 最小高度 */
    minHeight?: string
  }
}

// 模板配置接口
export interface ParameterTemplate {
  id: string
  name: string
  type: ParameterTemplateType
  description: string
  // 下拉选择模板的选项
  options?: TemplateOption[]
  // 默认值
  defaultValue?: any
  // 是否支持自定义输入（针对下拉选择模板）
  allowCustom?: boolean
  // 🔥 新增：组件模板配置
  componentConfig?: ComponentTemplateConfig
}

/**
 * 内置模板列表
 */
export const PARAMETER_TEMPLATES: ParameterTemplate[] = [
  {
    id: 'manual',
    name: '手动输入',
    type: ParameterTemplateType.MANUAL,
    description: '直接输入固定值',
    defaultValue: ''
  },
  {
    id: 'http-methods',
    name: 'HTTP方法',
    type: ParameterTemplateType.DROPDOWN,
    description: 'HTTP请求方法选择',
    options: [
      { label: 'GET', value: 'GET', description: '获取数据' },
      { label: 'POST', value: 'POST', description: '提交数据' },
      { label: 'PUT', value: 'PUT', description: '更新数据' },
      { label: 'DELETE', value: 'DELETE', description: '删除数据' },
      { label: 'PATCH', value: 'PATCH', description: '部分更新' }
    ],
    defaultValue: 'GET'
  },
  {
    id: 'content-types',
    name: '内容类型',
    type: ParameterTemplateType.DROPDOWN,
    description: '常用的Content-Type值',
    options: [
      { label: 'application/json', value: 'application/json' },
      { label: 'application/x-www-form-urlencoded', value: 'application/x-www-form-urlencoded' },
      { label: 'multipart/form-data', value: 'multipart/form-data' },
      { label: 'text/plain', value: 'text/plain' },
      { label: 'text/html', value: 'text/html' }
    ],
    defaultValue: 'application/json',
    allowCustom: true
  },
  {
    id: 'auth-types',
    name: '认证类型',
    type: ParameterTemplateType.DROPDOWN,
    description: '常用的Authorization类型',
    options: [
      { label: 'Bearer Token', value: 'Bearer ' },
      { label: 'Basic Auth', value: 'Basic ' },
      { label: 'API Key', value: 'ApiKey ' },
      { label: 'Custom', value: '' }
    ],
    defaultValue: 'Bearer ',
    allowCustom: true
  },
  {
    id: 'boolean-values',
    name: '布尔值',
    type: ParameterTemplateType.DROPDOWN,
    description: '真假值选择',
    options: [
      { label: '是 (true)', value: 'true' },
      { label: '否 (false)', value: 'false' },
      { label: '1', value: '1' },
      { label: '0', value: '0' }
    ],
    defaultValue: 'true'
  },
  {
    id: 'property-binding',
    name: '属性绑定',
    type: ParameterTemplateType.PROPERTY,
    description: '绑定到动态属性（运行时获取值）',
    defaultValue: ''
  },
  // 🔥 新增：组件属性绑定模板
  {
    id: 'component-property-binding',
    name: '组件属性绑定',
    type: ParameterTemplateType.COMPONENT,
    description: '绑定到编辑器中已加载组件的属性',
    defaultValue: '',
    componentConfig: {
      component: 'ComponentPropertySelector',
      props: {
        placeholder: '选择要绑定的组件属性',
        // 🔥 关键修复：启用自动检测当前组件ID
        autoDetectComponentId: true
      },
      events: {
        'update:selectedValue': 'handleComponentPropertyChange'
      },
      renderConfig: {
        wrapped: true,
        containerClass: 'component-property-container',
        minHeight: '400px'
      }
    }
  },
  // 🔥 新增：组件模板
  {
    id: 'device-metrics-selector',
    name: '设备配置',
    type: ParameterTemplateType.COMPONENT,
    description: '选择设备和对应的指标数据',
    defaultValue: '',
    componentConfig: {
      component: 'DeviceMetricsSelector',
      props: {
        mode: 'single',
        showMetrics: true
      },
      events: {
        'update:selectedValue': 'handleDeviceMetricsChange'
      },
      renderConfig: {
        wrapped: true,
        containerClass: 'device-metrics-container',
        minHeight: '200px'
      }
    }
  },
  {
    id: 'device-dispatch-selector',
    name: '设备分发选择器',
    type: ParameterTemplateType.COMPONENT,
    description: '设备分发选择器组件',
    defaultValue: '',
    componentConfig: {
      component: 'DeviceDispatchSelector',
      props: {
        multiple: false,
        showDetails: true
      },
      events: {
        'update:selectedValue': 'handleDeviceSelectionChange'
      },
      renderConfig: {
        wrapped: true,
        containerClass: 'device-dispatch-container',
        minHeight: '150px'
      }
    }
  },
  {
    id: 'icon-selector',
    name: '图标选择器',
    type: ParameterTemplateType.COMPONENT,
    description: '图标选择器组件',
    defaultValue: '',
    componentConfig: {
      component: 'IconSelector',
      props: {
        size: 'small'
      },
      events: {
        'update:value': 'handleIconChange'
      },
      renderConfig: {
        wrapped: true,
        containerClass: 'icon-selector-container',
        minHeight: '100px'
      }
    }
  },
  {
    id: 'interface-template',
    name: '接口模板',
    type: ParameterTemplateType.DROPDOWN,
    description: '使用内部接口的常用参数模板',
    options: [
      { label: '设备ID', value: '{device_id}', description: '设备标识符' },
      { label: '用户ID', value: '{user_id}', description: '用户标识符' },
      { label: '租户ID', value: '{tenant_id}', description: '租户标识符' },
      { label: '看板ID', value: '{board_id}', description: '看板标识符' },
      { label: '分组ID', value: '{group_id}', description: '分组标识符' },
      { label: '时间戳', value: '{timestamp}', description: '当前时间戳' },
      { label: '页码', value: '1', description: '分页页码' },
      { label: '页大小', value: '10', description: '分页大小' }
    ],
    defaultValue: '{device_id}',
    allowCustom: true
  }
]

/**
 * 🔥 修改：根据参数类型获取推荐模板（3个选项）
 * 返回：手动输入、组件属性绑定、设备配置
 * 注意：外面有统一设备配置选择器（批量），里面有单个参数的设备配置选择
 */
export function getRecommendedTemplates(parameterType: 'header' | 'query' | 'path'): ParameterTemplate[] {
  return [
    // 1. 手动输入
    PARAMETER_TEMPLATES.find(t => t.id === 'manual')!,

    // 2. 组件属性绑定
    PARAMETER_TEMPLATES.find(t => t.id === 'component-property-binding')!,

    // 3. 设备配置（单个参数的设备配置）
    PARAMETER_TEMPLATES.find(t => t.id === 'device-metrics-selector')!
  ]
}

/**
 * 获取所有组件模板
 */
export function getComponentTemplates(): ParameterTemplate[] {
  return PARAMETER_TEMPLATES.filter(t => t.type === ParameterTemplateType.COMPONENT)
}

/**
 * 检查模板是否为组件类型
 */
export function isComponentTemplate(template: ParameterTemplate): boolean {
  return template.type === ParameterTemplateType.COMPONENT
}

/**
 * 获取模板by ID
 */
export function getTemplateById(id: string): ParameterTemplate | undefined {
  return PARAMETER_TEMPLATES.find(t => t.id === id)
}
