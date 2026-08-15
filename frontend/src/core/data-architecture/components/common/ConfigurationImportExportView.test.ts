/**
 * 文件用途: ConfigurationImportExportView 的行为测试。
 * 核心逻辑: 构造局部 fixture 和 mock 依赖，验证导入导出公开契约。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节。
 */

import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  exportConfiguration: vi.fn(),
  generateImportPreview: vi.fn(),
  importConfiguration: vi.fn(),
  getAvailableDataSources: vi.fn(),
  exportSingleDataSource: vi.fn(),
  generateSingleImportPreview: vi.fn(),
  importSingleDataSource: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  messageWarning: vi.fn(),
  t: (key: string) => key
}))

vi.mock('../../utils/ConfigurationImportExport', () => ({
  configurationExporter: {
    exportConfiguration: hoisted.exportConfiguration
  },
  configurationImporter: {
    generateImportPreview: hoisted.generateImportPreview,
    importConfiguration: hoisted.importConfiguration
  },
  singleDataSourceExporter: {
    getAvailableDataSources: hoisted.getAvailableDataSources,
    exportSingleDataSource: hoisted.exportSingleDataSource
  },
  singleDataSourceImporter: {
    generateImportPreview: hoisted.generateSingleImportPreview,
    importSingleDataSource: hoisted.importSingleDataSource
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: hoisted.t }),
  createI18n: () => ({ global: { t: hoisted.t, locale: { value: 'en-US' } } })
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({
    success: hoisted.messageSuccess,
    error: hoisted.messageError,
    warning: hoisted.messageWarning
  }),
  NSpace: defineComponent({
    props: ['align', 'vertical', 'size'],
    setup(_, { slots }) {
      return () => h('div', { class: 'n-space-stub' }, slots.default?.())
    }
  }),
  NButton: defineComponent({
    props: {
      type: String,
      size: String,
      loading: Boolean,
      disabled: Boolean
    },
    emits: ['click'],
    setup(props, { emit, slots }) {
      return () =>
        h(
          'button',
          {
            class: ['n-button-stub', props.type ? `n-button-${props.type}` : ''],
            disabled: props.disabled,
            type: 'button',
            onClick: () => {
              if (!props.disabled) emit('click')
            }
          },
          [slots.icon?.(), slots.default?.(), slots.suffix?.()]
        )
    }
  }),
  NIcon: defineComponent({
    props: ['size'],
    setup(_, { slots }) {
      return () => h('i', { class: 'n-icon-stub' }, slots.default?.())
    }
  }),
  NModal: defineComponent({
    props: ['show', 'title', 'preset', 'size'],
    emits: ['update:show'],
    setup(props, { slots }) {
      return () =>
        props.show
          ? h('div', { class: 'n-modal-stub', 'data-title': props.title }, [
              props.title ? h('h3', props.title) : null,
              slots.default?.(),
              slots.action?.()
            ])
          : null
    }
  }),
  NCard: defineComponent({
    props: ['title', 'size', 'hoverable', 'bordered'],
    emits: ['click'],
    setup(props, { attrs, emit, slots }) {
      return () =>
        h(
          'section',
          {
            class: ['n-card-stub', attrs.class],
            onClick: () => emit('click')
          },
          [props.title ? h('h4', props.title) : null, slots.default?.(), slots.action?.()]
        )
    }
  }),
  NDescriptions: defineComponent({
    props: ['column', 'size'],
    setup(_, { slots }) {
      return () => h('dl', { class: 'n-descriptions-stub' }, slots.default?.())
    }
  }),
  NDescriptionsItem: defineComponent({
    props: ['label'],
    setup(props, { slots }) {
      return () => h('div', { class: 'n-descriptions-item-stub' }, [h('dt', props.label), h('dd', slots.default?.())])
    }
  }),
  NTag: defineComponent({
    props: ['type', 'size'],
    setup(props, { slots }) {
      return () => h('span', { class: 'n-tag-stub', 'data-type': props.type }, slots.default?.())
    }
  }),
  NText: defineComponent({
    props: ['strong', 'depth'],
    setup(_, { slots }) {
      return () => h('span', { class: 'n-text-stub' }, slots.default?.())
    }
  }),
  NAlert: defineComponent({
    props: ['type', 'title'],
    setup(props, { slots }) {
      return () => h('div', { class: 'n-alert-stub', 'data-type': props.type }, [h('strong', props.title), slots.default?.()])
    }
  }),
  NDropdown: defineComponent({
    props: ['options', 'disabled'],
    emits: ['select'],
    setup(props, { emit, slots }) {
      return () =>
        h('div', { class: 'n-dropdown-stub' }, [
          slots.default?.(),
          h(
            'button',
            {
              class: 'dropdown-full',
              disabled: props.disabled,
              type: 'button',
              onClick: () => emit('select', 'full')
            },
            'full'
          ),
          h(
            'button',
            {
              class: 'dropdown-single',
              disabled: props.disabled,
              type: 'button',
              onClick: () => emit('select', 'single')
            },
            'single'
          )
        ])
    }
  }),
  NSelect: defineComponent({
    props: ['value', 'options', 'placeholder'],
    emits: ['update:value'],
    setup(props, { emit }) {
      return () =>
        h(
          'select',
          {
            class: 'n-select-stub',
            value: props.value,
            onChange: (event: Event) => emit('update:value', (event.target as HTMLSelectElement).value)
          },
          (props.options || []).map((option: any) =>
            h('option', { value: option.value, 'data-occupied': String(option.occupied) }, option.label)
          )
        )
    }
  }),
  NForm: defineComponent({
    setup(_, { slots }) {
      return () => h('form', { class: 'n-form-stub' }, slots.default?.())
    }
  }),
  NFormItem: defineComponent({
    props: ['label'],
    setup(props, { slots }) {
      return () => h('label', { class: 'n-form-item-stub' }, [props.label, slots.default?.()])
    }
  })
}))

vi.mock('@vicons/ionicons5', () => ({
  DownloadOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-download' }) }),
  ChevronDownOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-down' }) }),
  UploadOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-upload' }) })
}))

// Keep the parent test focused on the import orchestration contract while
// preserving the child modal's observable props/events.
vi.mock('./SingleDataSourceImportPreviewModal.vue', () => ({
  default: defineComponent({
    props: {
      show: Boolean,
      preview: { type: Object, default: null },
      selectedTargetSlot: { type: String, default: '' },
      targetSlotOptions: { type: Array, default: () => [] },
      isProcessing: Boolean
    },
    emits: ['update:show', 'update:selectedTargetSlot', 'confirm'],
    setup(props, { emit }) {
      return () => {
        if (!props.show || !props.preview) return null
        const conflicts = Array.isArray((props.preview as any).conflicts) ? (props.preview as any).conflicts : []
        const disabled = !props.selectedTargetSlot || props.isProcessing || conflicts.length > 0
        return h('div', { class: 'single-data-source-import-preview-stub' }, [
          h('h3', 'configuration.import.singleDataSourcePreview'),
          h(
            'select',
            {
              class: 'n-select-stub',
              'data-placeholder': 'configuration.import.selectTargetSlot',
              value: props.selectedTargetSlot,
              onChange: (event: Event) =>
                emit('update:selectedTargetSlot', (event.target as HTMLSelectElement).value)
            },
            (props.targetSlotOptions as any[]).map((option) => h('option', { value: option.value }, option.label))
          ),
          h(
            'button',
            {
              type: 'button',
              disabled,
              onClick: () => {
                if (!disabled) emit('confirm')
              }
            },
            'configuration.import.importToSlot'
          )
        ])
      }
    }
  })
}))

import ConfigurationImportExportView from './ConfigurationImportExportView.vue'

const mountedWrappers: VueWrapper[] = []

const configurationManager = {
  getConfiguration: vi.fn()
}

const mountView = (props: Record<string, unknown> = {}) => {
  const wrapper = mount(ConfigurationImportExportView, {
    props: {
      configuration: { dataSource: { dataSources: [] } },
      componentId: 'component-abcdef123456',
      componentType: 'rdi-card',
      configurationManager,
      ...props
    },
    attachTo: document.body,
    global: {
      mocks: {
        $t: hoisted.t
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const setFileInput = async (wrapper: VueWrapper, file: File) => {
  const input = wrapper.get<HTMLInputElement>('input[type="file"]')
  Object.defineProperty(input.element, 'files', {
    value: [file],
    configurable: true
  })
  await input.trigger('change')
  await vi.runOnlyPendingTimersAsync()
  await flushPromises()
  await nextTick()
}

const jsonFile = (name: string, data: unknown) => new File([JSON.stringify(data)], name, { type: 'application/json' })

const buttonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find(item => item.text().includes(text))
  if (!button) throw new Error(`Button not found: ${text}`)
  return button
}

const getState = (wrapper: VueWrapper) => wrapper.vm.$.setupState as Record<string, any>

describe('ConfigurationImportExportView.vue', () => {
  let originalCreateObjectURL: typeof URL.createObjectURL | undefined
  let originalRevokeObjectURL: typeof URL.revokeObjectURL | undefined
  let originalFileReader: typeof FileReader
  let clickSpy: ReturnType<typeof vi.spyOn>
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T05:00:00.000Z'))
    vi.clearAllMocks()
    document.body.innerHTML = ''
    configurationManager.getConfiguration.mockReturnValue({})
    hoisted.exportConfiguration.mockResolvedValue({ version: '1.0.0', data: { full: true } })
    hoisted.exportSingleDataSource.mockResolvedValue({ exportType: 'single-datasource', dataSourceConfig: {} })
    hoisted.getAvailableDataSources.mockReturnValue([
      { sourceId: 'main', sourceIndex: 0, hasData: true, dataItemCount: 2 },
      { sourceId: 'backup', sourceIndex: 1, hasData: false, dataItemCount: 0 }
    ])
    hoisted.generateImportPreview.mockReturnValue({
      basicInfo: {
        version: '1.0.0',
        exportTime: 1782536400000,
        componentType: 'rdi-card',
        exportSource: 'SimpleConfigurationEditor'
      },
      statistics: {
        dataSourceCount: 1,
        interactionCount: 2,
        httpConfigCount: 3
      },
      dependencies: ['component-dependency-123456'],
      conflicts: []
    })
    hoisted.importConfiguration.mockReturnValue({ success: true, importedData: { ok: true } })
    hoisted.generateSingleImportPreview.mockReturnValue({
      basicInfo: {
        version: '1.0.0',
        exportType: 'single-datasource',
        exportTime: 1782536400000,
        originalSourceId: 'main',
        sourceIndex: 0,
        exportSource: 'SingleDataSourceExporter'
      },
      configSummary: {
        dataItemCount: 2,
        mergeStrategy: 'object',
        hasProcessing: true
      },
      relatedConfig: {
        interactionCount: 1,
        httpBindingCount: 1
      },
      dependencies: ['external-component'],
      conflicts: [],
      availableSlots: [
        { slotId: 'occupied', slotIndex: 0, isEmpty: false },
        { slotId: 'empty', slotIndex: 1, isEmpty: true }
      ]
    })
    hoisted.importSingleDataSource.mockResolvedValue(undefined)
    originalCreateObjectURL = URL.createObjectURL
    originalRevokeObjectURL = URL.revokeObjectURL
    originalFileReader = globalThis.FileReader
    URL.createObjectURL = vi.fn(() => 'blob:configuration')
    URL.revokeObjectURL = vi.fn()
    class ImmediateFileReader {
      result: string | ArrayBuffer | null = null
      onload: ((this: FileReader, ev: ProgressEvent<FileReader>) => any) | null = null
      onerror: ((this: FileReader, ev: ProgressEvent<FileReader>) => any) | null = null

      async readAsText(file: File) {
        try {
          this.result = await file.text()
          this.onload?.call(this as unknown as FileReader, {} as ProgressEvent<FileReader>)
        } catch {
          this.onerror?.call(this as unknown as FileReader, {} as ProgressEvent<FileReader>)
        }
      }
    }
    globalThis.FileReader = ImmediateFileReader as unknown as typeof FileReader
    clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    URL.createObjectURL = originalCreateObjectURL as typeof URL.createObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL as typeof URL.revokeObjectURL
    globalThis.FileReader = originalFileReader
    clickSpy.mockRestore()
    consoleErrorSpy.mockRestore()
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  it('exports a full configuration file and emits the exported payload', async () => {
    const wrapper = mountView()

    await wrapper.get('.dropdown-full').trigger('click')
    await flushPromises()

    expect(hoisted.exportConfiguration).toHaveBeenCalledWith(
      'component-abcdef123456',
      configurationManager,
      'rdi-card'
    )
    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob))
    expect(clickSpy).toHaveBeenCalledTimes(1)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:configuration')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('configuration.export.success')
    expect(wrapper.emitted('exportSuccess')?.[0]?.[0]).toEqual({ version: '1.0.0', data: { full: true } })
  })

  it('emits operationError when full export is attempted without a manager', async () => {
    const wrapper = mountView({ configurationManager: undefined })

    await wrapper.get('.dropdown-full').trigger('click')
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith(
      'configuration.export.error: configuration.export.noManagerError'
    )
    expect(wrapper.emitted('operationError')?.[0]?.[0]).toBeInstanceOf(Error)
  })

  it('shows single data sources, exports a chosen source, and closes the chooser', async () => {
    const wrapper = mountView()

    await wrapper.get('.dropdown-single').trigger('click')
    await nextTick()

    expect(hoisted.getAvailableDataSources).toHaveBeenCalledWith('component-abcdef123456', configurationManager)
    expect(wrapper.text()).toContain('main')
    expect(wrapper.text()).toContain('configuration.export.hasData')
    expect(wrapper.text()).toContain('backup')
    expect(wrapper.text()).toContain('configuration.export.noData')

    const sourceCard = wrapper.findAll('.datasource-item').find(card => card.text().includes('main'))
    if (!sourceCard) throw new Error('main data source card not found')

    await sourceCard.trigger('click')
    await flushPromises()

    expect(hoisted.exportSingleDataSource).toHaveBeenCalledWith(
      'component-abcdef123456',
      'main',
      configurationManager,
      'rdi-card'
    )
    expect(wrapper.emitted('exportSuccess')?.[0]?.[0]).toEqual({
      exportType: 'single-datasource',
      dataSourceConfig: {}
    })
    expect(wrapper.findAll('.datasource-selection')).toHaveLength(0)
  })

  it('warns when no data source is available for single-source export', async () => {
    hoisted.getAvailableDataSources.mockReturnValueOnce([])
    const wrapper = mountView()

    await wrapper.get('.dropdown-single').trigger('click')
    await nextTick()

    expect(hoisted.messageWarning).toHaveBeenCalledWith('configuration.export.noDataSources')
    expect(wrapper.findAll('.datasource-selection')).toHaveLength(0)
  })

  it('rejects non-json import files before preview generation', async () => {
    const wrapper = mountView()

    await setFileInput(wrapper, new File(['not json'], 'config.txt', { type: 'text/plain' }))

    expect(hoisted.messageError).toHaveBeenCalledWith('configuration.import.invalidFileType')
    expect(hoisted.generateImportPreview).toHaveBeenCalledTimes(0)
  })

  it('previews and confirms a full configuration import when no conflicts exist', async () => {
    const wrapper = mountView()
    const importData = { version: '1.0.0', data: { imported: true } }

    await setFileInput(wrapper, jsonFile('config.json', importData))

    expect(hoisted.generateImportPreview).toHaveBeenCalledWith(
      importData,
      'component-abcdef123456',
      configurationManager,
      { dataSource: { dataSources: [] } }
    )
    expect(wrapper.text()).toContain('SimpleConfigurationEditor')
    expect(wrapper.text()).toContain('component')
    expect(wrapper.text()).toContain('configuration.import.noConflicts')

    await buttonByText(wrapper, 'common.confirm').trigger('click')
    await flushPromises()

    expect(hoisted.importConfiguration).toHaveBeenCalledWith(importData, 'component-abcdef123456', configurationManager)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('configuration.import.success')
    expect(wrapper.emitted('importSuccess')?.[0]?.[0]).toEqual({ success: true, importedData: { ok: true } })
    expect(wrapper.findAll('.import-preview')).toHaveLength(0)
  })

  it('disables full import confirmation when preview conflicts are present', async () => {
    hoisted.generateImportPreview.mockReturnValueOnce({
      basicInfo: {
        version: '1.0.0',
        exportTime: 1782536400000,
        componentType: '',
        exportSource: 'SimpleConfigurationEditor'
      },
      statistics: {
        dataSourceCount: 1,
        interactionCount: 0,
        httpConfigCount: 0
      },
      dependencies: [],
      conflicts: ['data source conflict']
    })
    const wrapper = mountView()

    await setFileInput(wrapper, jsonFile('config.json', { version: '1.0.0' }))

    expect(wrapper.text()).toContain('data source conflict')
    expect(buttonByText(wrapper, 'common.confirm').attributes('disabled')).toBe('')
  })

  it('previews a single-data-source import, chooses the first empty slot, and imports to that slot', async () => {
    const wrapper = mountView()
    const importData = { exportType: 'single-datasource', dataSourceConfig: { dataItems: [{ item: 'a' }] } }

    await setFileInput(wrapper, jsonFile('datasource.json', importData))

    expect(hoisted.generateSingleImportPreview).toHaveBeenCalledWith(
      importData,
      'component-abcdef123456',
      configurationManager
    )
    expect(wrapper.text()).toContain('configuration.import.singleDataSourcePreview')
    expect(wrapper.get<HTMLSelectElement>('.n-select-stub').element.value).toBe('empty')

    await buttonByText(wrapper, 'configuration.import.importToSlot').trigger('click')
    await flushPromises()

    expect(hoisted.importSingleDataSource).toHaveBeenCalledWith(
      importData,
      'component-abcdef123456',
      'empty',
      configurationManager
    )
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('configuration.import.success')
    expect(wrapper.emitted('importSuccess')?.[0]?.[0]).toEqual(importData)
    expect(wrapper.findAll('.n-select-stub')).toHaveLength(0)
  })

  it('falls back to exporter slot discovery when single-source preview has no availableSlots', async () => {
    hoisted.generateSingleImportPreview.mockReturnValueOnce({
      basicInfo: {
        version: '1.0.0',
        exportType: 'single-datasource',
        exportTime: 1782536400000,
        originalSourceId: 'main',
        sourceIndex: 0,
        exportSource: 'SingleDataSourceExporter'
      },
      configSummary: {
        dataItemCount: 1,
        mergeStrategy: 'array',
        hasProcessing: false
      },
      relatedConfig: {
        interactionCount: 0,
        httpBindingCount: 0
      },
      dependencies: [],
      conflicts: [],
      availableSlots: []
    })
    hoisted.getAvailableDataSources.mockReturnValueOnce([
      { sourceId: 'occupied-from-exporter', sourceIndex: 0, hasData: true, dataItemCount: 1 },
      { sourceId: 'empty-from-exporter', sourceIndex: 1, hasData: false, dataItemCount: 0 }
    ])
    const wrapper = mountView()
    const importData = { type: 'singleDataSource', dataSourceConfig: { dataItems: [{ item: 'a' }] } }

    await setFileInput(wrapper, jsonFile('datasource.json', importData))

    expect(wrapper.get<HTMLSelectElement>('.n-select-stub').element.value).toBe('empty-from-exporter')
  })

  it('emits operationError for malformed import preview and failed single-source import', async () => {
    const wrapper = mountView()

    await setFileInput(wrapper, new File(['{bad json'], 'config.json', { type: 'application/json' }))

    expect(hoisted.messageError).toHaveBeenCalledWith(expect.stringContaining('configuration.import.previewError'))
    expect(wrapper.emitted('operationError')?.[0]?.[0]).toBeInstanceOf(Error)

    hoisted.importSingleDataSource.mockRejectedValueOnce(new Error('slot write failed'))
    await setFileInput(wrapper, jsonFile('datasource.json', { exportType: 'single-datasource' }))
    await buttonByText(wrapper, 'configuration.import.importToSlot').trigger('click')
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('configuration.import.error: slot write failed')
    expect(wrapper.emitted('operationError')?.[1]?.[0]).toBeInstanceOf(Error)
  })

  it('opens the hidden file picker from the import button', async () => {
    const inputClickSpy = vi.spyOn(HTMLInputElement.prototype, 'click').mockImplementation(() => undefined)
    const wrapper = mountView()

    try {
      await buttonByText(wrapper, 'common.import').trigger('click')

      expect(inputClickSpy).toHaveBeenCalledTimes(1)
    } finally {
      inputClickSpy.mockRestore()
    }
  })

  it('ignores confirm actions when required import state is missing', async () => {
    const wrapper = mountView()
    const state = getState(wrapper)

    await state.handleConfirmImport()
    expect(hoisted.importConfiguration).toHaveBeenCalledTimes(0)

    state.importFile = jsonFile('datasource.json', { exportType: 'single-datasource' })
    state.singleDataSourceImportPreview = {
      basicInfo: {
        version: '1.0.0',
        exportType: 'single-datasource',
        exportTime: 1782536400000,
        originalSourceId: 'main',
        sourceIndex: 0,
        exportSource: 'SingleDataSourceExporter'
      },
      configSummary: {
        dataItemCount: 1,
        mergeStrategy: 'object',
        hasProcessing: false
      },
      relatedConfig: {
        interactionCount: 0,
        httpBindingCount: 0
      },
      dependencies: [],
      conflicts: [],
      availableSlots: []
    }
    state.selectedTargetSlot = ''

    await state.handleSingleDataSourceImport()

    expect(hoisted.importSingleDataSource).toHaveBeenCalledTimes(0)
    expect(wrapper.emitted('importSuccess')).toBeUndefined()
  })

  it('reports file reader failures during import preview', async () => {
    const OriginalFileReader = globalThis.FileReader
    class FailingFileReader {
      result: string | ArrayBuffer | null = null
      onload: ((this: FileReader, ev: ProgressEvent<FileReader>) => any) | null = null
      onerror: ((this: FileReader, ev: ProgressEvent<FileReader>) => any) | null = null

      readAsText() {
        this.onerror?.call(this as unknown as FileReader, {} as ProgressEvent<FileReader>)
      }
    }

    globalThis.FileReader = FailingFileReader as unknown as typeof FileReader
    const wrapper = mountView()

    try {
      await setFileInput(wrapper, jsonFile('config.json', { version: '1.0.0' }))

      expect(hoisted.messageError).toHaveBeenCalledWith(
        expect.stringContaining('configuration.import.previewError')
      )
      expect(wrapper.emitted('operationError')?.[0]?.[0]).toBeInstanceOf(Error)
    } finally {
      globalThis.FileReader = OriginalFileReader
    }
  })
})
