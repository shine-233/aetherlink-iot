<script setup lang="ts">
import { computed, ref, useAttrs, watch } from 'vue'
import { Icon } from '@iconify/vue'

defineOptions({ name: 'SvgIcon', inheritAttrs: false })

/**
 * Props
 *
 * - Support iconify and local svg icon
 * - If icon and localIcon are passed at the same time, localIcon will be rendered first
 */
interface Props {
  /** Iconify icon name */
  icon?: string
  /** Local svg icon name */
  localIcon?: string
}

const props = defineProps<Props>()

const attrs = useAttrs()
const localIconReady = ref(!props.localIcon)

let localIconsRegisterPromise: Promise<void> | null = null

async function ensureLocalIconsRegistered() {
  if (!localIconsRegisterPromise) {
    localIconsRegisterPromise = import('virtual:svg-icons-register').then(() => undefined)
  }

  await localIconsRegisterPromise
}

const bindAttrs = computed<{ class: string; style: string }>(() => ({
  class: (attrs.class as string) || '',
  style: (attrs.style as string) || ''
}))

const symbolId = computed(() => {
  const { VITE_ICON_LOCAL_PREFIX: prefix } = import.meta.env

  const defaultLocalIcon = 'no-icon'

  const icon = props.localIcon || defaultLocalIcon

  return `#${prefix}-${icon}`
})

/** If localIcon is passed, render localIcon first */
const renderLocalIcon = computed(() => props.localIcon || !props.icon)

watch(
  () => props.localIcon,
  async iconName => {
    if (!iconName) {
      localIconReady.value = true
      return
    }

    localIconReady.value = false
    await ensureLocalIconsRegistered()
    localIconReady.value = true
  },
  { immediate: true }
)
</script>

<template>
  <template v-if="renderLocalIcon && localIconReady">
    <svg aria-hidden="true" width="1em" height="1em" v-bind="bindAttrs">
      <use :xlink:href="symbolId" fill="currentColor" />
    </svg>
  </template>
  <template v-else>
    <Icon v-if="icon" :icon="icon" v-bind="bindAttrs" />
  </template>
</template>

<style scoped></style>
