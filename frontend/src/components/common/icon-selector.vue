<!--
  文件用途：提供可展开的本地图标选择器。
  核心逻辑：懒加载 icons 注册表，展示图标选项，并在选择后通过 iconSelected 事件回传名称。
  关键注意事项：默认图标和懒加载状态会影响首次渲染，修改 icons 导出时需确认初始化选择仍有效。
  重构建议：可将图标加载缓存和选项生成拆成 composable，减少组件状态分支。
-->
<script setup lang="ts">
import type { Component } from 'vue'
import { computed, onMounted, ref, shallowRef } from 'vue'
import { NButton, NIcon, NSpin } from 'naive-ui'
import { CaretDownOutline, CaretUpOutline } from '@vicons/ionicons5'
import { $t } from '@/locales'

const emit = defineEmits<{
  iconSelected: [value: string]
  'update:value': [value: string]
}>()
const props = defineProps({
  // optional default icon name
  defaultIcon: {
    type: String,
    default: null
  }
})

const loadedIcons = shallowRef<Record<string, Component>>({})
const selectedIcon = shallowRef<Component | null>(null)
const selectedIconName = ref<string | null>(null)
const isExpanded = ref(false)
const isLoadingIcons = ref(false)

let loadIconsPromise: Promise<void> | null = null

async function ensureIconsLoaded() {
  if (Object.keys(loadedIcons.value).length > 0) {
    return
  }

  if (!loadIconsPromise) {
    isLoadingIcons.value = true
    loadIconsPromise = import('./icons').then(module => {
      loadedIcons.value = module.icons as Record<string, Component>
      isLoadingIcons.value = false
    })
  }

  await loadIconsPromise
}

const iconOptions = computed(() =>
  Object.keys(loadedIcons.value).map(key => ({
    name: key,
    component: loadedIcons.value[key]
  }))
)

const selectIcon = (option: { name: string; component: Component }) => {
  selectedIcon.value = option.component
  selectedIconName.value = option.name
  emit('iconSelected', option.name)
  // Keep the template-driven parameter editor contract while preserving the legacy event.
  emit('update:value', option.name)
  // 选择后自动收起面板
  isExpanded.value = false
}

const toggleExpand = async () => {
  isExpanded.value = !isExpanded.value
  if (isExpanded.value) {
    await ensureIconsLoaded()
  }
}

// Set the default icon if provided
onMounted(async () => {
  if (props.defaultIcon) {
    await ensureIconsLoaded()

    const defaultOption = iconOptions.value.find(option => option.name === props.defaultIcon)
    if (defaultOption) {
      selectedIcon.value = defaultOption.component
      selectedIconName.value = defaultOption.name
      // 不需要emit，因为这是初始化设置
    }
  }
})
</script>

<template>
  <div>
    <div class="icon-display">
      <span>{{ $t('card.selectedIcon') }}：</span>
      <NIcon v-if="selectedIcon" size="30" :component="selectedIcon" />
      <span v-else>{{ $t('card.notSelected') }}</span>
      <NButton class="icon-picker-btn" @click="toggleExpand">
        {{ isExpanded ? $t('card.collapse') : $t('card.expand') }}
        <template #icon>
          <NIcon>
            <component :is="isExpanded ? CaretUpOutline : CaretDownOutline" />
          </NIcon>
        </template>
      </NButton>
    </div>
    <div v-if="isExpanded" class="icon-picker-dialog">
      <div v-if="isLoadingIcons" class="py-16px flex-center">
        <NSpin size="small" />
      </div>
      <div class="icon-grid">
        <div
          v-for="(option, index) in iconOptions"
          :key="index"
          class="icon-cell"
          :class="{ selected: selectedIconName === option.name }"
          :title="option.name"
          @click="selectIcon(option)"
        >
          <NIcon size="24" :component="option.component" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.icon-display {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 8px;
  background: var(--card-color);
  border: 1px solid var(--border-color);
  border-radius: 6px;
}

.icon-display span {
  font-size: 14px;
  color: var(--text-color);
  white-space: nowrap;
}

.icon-picker-dialog {
  margin-top: 8px;
  padding: 12px;
  background: var(--card-color);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  max-height: 300px;
  overflow-y: auto;
}

.icon-picker-btn {
  margin-left: auto;
  flex-shrink: 0;
}

.icon-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(48px, 1fr));
  gap: 8px;
  justify-content: center;
}

.icon-cell {
  width: 48px;
  height: 48px;
  display: flex;
  justify-content: center;
  align-items: center;
  cursor: pointer;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--input-color);
  transition: all 0.2s ease;
  position: relative;
}

.icon-cell:hover {
  background: var(--primary-color-hover);
  border-color: var(--primary-color);
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.icon-cell:active {
  transform: translateY(0);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1);
}

/* 选中状态样式 */
.icon-cell.selected {
  background: var(--primary-color);
  border-color: var(--primary-color);
  box-shadow: 0 2px 8px rgba(24, 144, 255, 0.3);
}

.icon-cell.selected :deep(.n-icon) {
  color: white;
}

/* 图标颜色适配 */
.icon-cell :deep(.n-icon) {
  color: var(--text-color);
  transition: color 0.2s ease;
}

.icon-cell:hover :deep(.n-icon) {
  color: var(--primary-color);
}

/* 当前选中的图标样式 */
.icon-display :deep(.n-icon) {
  color: var(--primary-color);
  border: 1px solid var(--primary-color);
  border-radius: 4px;
  padding: 2px;
}

/* 滚动条美化 */
.icon-picker-dialog::-webkit-scrollbar {
  width: 6px;
}

.icon-picker-dialog::-webkit-scrollbar-track {
  background: var(--fill-color-1);
  border-radius: 3px;
}

.icon-picker-dialog::-webkit-scrollbar-thumb {
  background: var(--fill-color-3);
  border-radius: 3px;
  transition: background-color 0.2s;
}

.icon-picker-dialog::-webkit-scrollbar-thumb:hover {
  background: var(--primary-color);
}

/* 响应式设计 */
@media (max-width: 480px) {
  .icon-grid {
    grid-template-columns: repeat(auto-fill, minmax(40px, 1fr));
    gap: 6px;
  }

  .icon-cell {
    width: 40px;
    height: 40px;
  }

  .icon-picker-dialog {
    max-height: 200px;
    padding: 8px;
  }

  .icon-display {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .icon-picker-btn {
    margin-left: 0;
    width: 100%;
  }
}

/* 暗主题适配 */
[data-theme='dark'] .icon-cell {
  background: var(--input-color);
  border-color: var(--border-color);
}

[data-theme='dark'] .icon-cell:hover {
  background: var(--primary-color-hover);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
}

[data-theme='dark'] .icon-picker-dialog {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}
</style>
