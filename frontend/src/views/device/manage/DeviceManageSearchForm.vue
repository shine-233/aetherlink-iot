<!--
  DeviceManageSearchForm: the device-manage search controls rendered from `SearchConfig[]`.

  Extracted from the shared <data-table-page> wrapper when the page moved onto `useListPage`
  (see useDeviceManageListPage.ts). Behavior parity with the old wrapper:
  - input controls search on a 400ms debounce; select/tree-select search immediately;
  - `loadOptions` configs lazy-load once when their dropdown is first shown;
  - control values are written straight into the list-page `query`, and `search`/`reset`
    bubble up so the page drives useListPage through the table bridge.
-->
<script setup lang="ts">
import { onUnmounted } from 'vue'
import { debounce } from 'lodash-es'
import { NButton, NDatePicker, NGi, NGrid, NInput, NSelect, NSpace, NTreeSelect } from 'naive-ui'
import type { SearchConfig } from '@/components/data-table-page/types'
import { $t } from '@/locales'

const props = defineProps<{
  configs: SearchConfig[]
  /** The list-page query object (useListPage `query`); controls mutate it in place. */
  criteria: Record<string, any>
}>()

// searchConfigs 数组对象会被 service-access filters 原地更新 options，保持引用传递。
const { configs, criteria } = props

const emit = defineEmits<{
  search: []
  reset: []
}>()

const doSearch = () => emit('search')
const debouncedInputSearch = debounce(doSearch, 400)

onUnmounted(() => {
  debouncedInputSearch.cancel()
})

const handleInputChange = () => {
  debouncedInputSearch()
}

const handleSelectChange = () => {
  doSearch()
}

const handleTreeSelectUpdate = () => {
  doSearch()
}

// 用于加载动态选项的函数，适用于select和tree-select类型的搜索配置
const loadedSearchOptionKeys = new Set<string>()
const pendingSearchOptionLoads = new Map<string, Promise<void>>()

const ensureSearchOptionsLoaded = async (config: any) => {
  if (!config?.loadOptions || loadedSearchOptionKeys.has(config.key)) {
    return
  }

  const pending = pendingSearchOptionLoads.get(config.key)
  if (pending) {
    await pending
    return
  }

  const load = (async () => {
    const opts = config.type === 'select' ? await config.loadOptions('') : await config.loadOptions()
    config.options = [...(config.options || []), ...opts]
    loadedSearchOptionKeys.add(config.key)
  })().finally(() => {
    pendingSearchOptionLoads.delete(config.key)
  })

  pendingSearchOptionLoads.set(config.key, load)
  await load
}

const ensureSearchOptionsLoadedWhenShown = (show: boolean, config: any) => {
  if (show) {
    void ensureSearchOptionsLoaded(config)
  }
}

// 修复 NSelect 的 filter 函数类型错误
const filterSelectOption = (pattern: string, option: any) => {
  const label = typeof option.label === 'string' ? option.label : ''
  return label.includes(pattern)
}

</script>

<template>
  <n-grid cols="1 s:2 m:3 l:4 xl:6 2xl:8" x-gap="18" y-gap="18" responsive="screen">
    <n-gi v-for="config in configs" :key="config.key">
      <template v-if="config.type === 'input'">
        <NInput
          v-model:value="criteria[config.key]"
          :placeholder="$t(config.label)"
          class="input-style"
          @update:value="handleInputChange"
        />
      </template>
      <template v-else-if="config.type === 'date-range'">
        <NDatePicker
          v-model:value="criteria[config.key]"
          type="daterange"
          :placeholder="$t(config.label)"
          class="input-style"
        />
      </template>
      <template v-else-if="config.type === 'select'">
        <NSelect
          v-model:value="criteria[config.key]"
          :value-field="config.valueField"
          :label-field="config.labelField"
          filterable
          :filter="filterSelectOption"
          :options="config.options"
          :render-label="config.renderLabel"
          :render-tag="config.renderTag"
          :placeholder="$t(config.label)"
          class="input-style"
          @update:show="(show) => ensureSearchOptionsLoadedWhenShown(show, config)"
          @update:value="handleSelectChange"
        />
      </template>
      <template v-else-if="config.type === 'date'">
        <NDatePicker
          v-model:value="criteria[config.key]"
          type="date"
          :placeholder="$t(config.label)"
          class="input-style"
        />
      </template>
      <template v-else-if="config.type === 'tree-select'">
        <NTreeSelect
          v-model:value="criteria[config.key]"
          filterable
          :options="config.options"
          :multiple="config.multiple"
          class="input-style"
          @update:show="(show) => ensureSearchOptionsLoadedWhenShown(show, config)"
          @update:value="handleTreeSelectUpdate"
        />
      </template>
    </n-gi>
    <n-gi>
      <NSpace>
        <NButton type="primary" @click="doSearch">{{ $t('generate.query') }}</NButton>
        <NButton type="default" @click="emit('reset')">{{ $t('generate.reset') }}</NButton>
      </NSpace>
    </n-gi>
  </n-grid>
</template>
