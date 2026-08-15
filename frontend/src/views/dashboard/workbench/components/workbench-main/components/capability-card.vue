<script setup lang="ts">
import { useRouter } from 'vue-router'

defineOptions({ name: 'DashboardWorkbenchMainCapabilityCard' })

interface Props {
  name: string
  description: string
  actionLabel: string
  route: string
  icon: string
  iconColor?: string
}

const props = defineProps<Props>()
const router = useRouter()

function handleOpenCapability() {
  if (props.route.startsWith('/')) {
    void router.push(props.route)
    return
  }
  window.open(props.route, '_blank')
}
</script>

<template>
  <div
    class="min-h-120px cursor-pointer border-1px border-#efeff5 rounded-4px p-4px dark:border-#ffffff17 hover:shadow-sm"
    @click="handleOpenCapability"
  >
    <header class="flex-y-center">
      <SvgIcon :icon="icon" :style="{ color: iconColor }" class="text-30px" />
      <h3 class="pl-12px text-18px font-semibold">{{ name }}</h3>
    </header>
    <p class="min-h-56px break-words py-8px text-#999">{{ description }}</p>
    <div class="flex justify-end">
      <span>{{ actionLabel }}</span>
    </div>
  </div>
</template>

<style scoped></style>
