<script lang="ts" setup>
import { computed, ref } from 'vue'
import { $t } from '@/locales'

defineOptions({ name: 'IconSelect' })

interface Props {
  /** Selected icon name. */
  value: string
  /** Available icon names. */
  icons: string[]
  /** Fallback icon when no value is selected. */
  emptyIcon?: string
}

const props = withDefaults(defineProps<Props>(), {
  emptyIcon: 'mdi:apps'
})

interface Emits {
  (e: 'update:value', val: string): void
}

const emit = defineEmits<Emits>()

const modelValue = computed({
  get() {
    return props.value
  },
  set(val: string) {
    emit('update:value', val)
  }
})

const selectedIcon = computed(() => modelValue.value || props.emptyIcon)

const searchValue = ref('')

const iconsList = computed(() => props.icons.filter((v) => v.includes(searchValue.value)))

function handleChange(iconItem: string) {
  modelValue.value = iconItem
}
</script>

<template>
  <NPopover placement="bottom-end" trigger="click">
    <template #trigger>
      <NInput v-model:value="modelValue" readonly :placeholder="$t('generate.click-to-select-icon')">
        <template #suffix>
          <SvgIcon :icon="selectedIcon" class="p-5px text-30px" />
        </template>
      </NInput>
    </template>
    <template #header>
      <NInput v-model:value="searchValue" :placeholder="$t('generate.search-icon')"></NInput>
    </template>
    <div v-if="iconsList.length > 0" class="grid grid-cols-9 h-auto overflow-auto">
      <button
        v-for="iconItem in iconsList"
        :key="iconItem"
        type="button"
        class="icon-option m-2px cursor-pointer border-1px border-#d9d9d9 p-5px text-30px"
        :class="{ 'border-primary': modelValue === iconItem }"
        :aria-label="iconItem"
        :aria-pressed="modelValue === iconItem"
        @click="handleChange(iconItem)"
      >
        <SvgIcon :icon="iconItem" aria-hidden="true" />
      </button>
    </div>
    <NEmpty v-else class="w-306px" :description="$t('card.cannotFound')" />
  </NPopover>
</template>

<style lang="scss" scoped>
:deep(.n-input-wrapper) {
  padding-right: 0;
}

:deep(.n-input__suffix) {
  border: 1px solid #d9d9d9;
}

.icon-option {
  background: transparent;
  line-height: 1;
}

.icon-option:focus-visible {
  outline: 2px solid var(--primary-color);
  outline-offset: 2px;
}
</style>
