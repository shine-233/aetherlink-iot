<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

defineOptions({ name: 'DashboardWorkbenchMainShortcutsCard' })

interface Props {
  label: string
  icon: string
  iconColor: string
  route?: string
}

const props = defineProps<Props>()
const router = useRouter()
const isInteractive = computed(() => Boolean(props.route))

function handleClick() {
  if (!props.route) return
  router.push(props.route)
}
</script>

<template>
  <div
    class="h-120px flex-col-center border-1px border-#efeff5 rounded-4px p-12px dark:border-#ffffff17 hover:shadow-sm"
    :class="isInteractive ? 'cursor-pointer' : 'cursor-default'"
    :role="isInteractive ? 'button' : undefined"
    :tabindex="isInteractive ? 0 : undefined"
    @click="handleClick"
    @keydown.enter.prevent="handleClick"
    @keydown.space.prevent="handleClick"
  >
    <SvgIcon :icon="icon" :style="{ color: iconColor }" class="text-30px" />
    <p class="py-8px text-16px">{{ label }}</p>
  </div>
</template>

<style scoped></style>
