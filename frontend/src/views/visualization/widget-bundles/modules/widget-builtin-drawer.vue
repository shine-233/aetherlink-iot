<!--
  文件用途：内置四部件（gauge/chart/valve/twin3d）预览抽屉与一键种子导入（从 index.vue 拆出）。
  核心逻辑：展示内置 bundle 导出描述（kind/name/version/部件数量/定义 JSON 只读），
  种子动作调用 seedBuiltinWidgetBundle（内容一致时后端幂等返回），成功后 emit seeded 并关闭。
  关键注意事项：种子导入是幂等操作，幂等命中时提示 info 而非 success。
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NDrawer, NDrawerContent, NInput, NSpace, NTag, useMessage } from 'naive-ui'
import { seedBuiltinWidgetBundle, type WidgetBundleExport } from '@/service/api'
import { $t } from '@/locales'
import { widgetCountOf } from './widget-bundle'

interface Props {
  /** 抽屉显示状态（v-model:show）。 */
  show: boolean
  /** 内置 bundle 导出描述；由父页面在打开时拉取。 */
  builtinExport: WidgetBundleExport | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'seeded'): void
}>()

const message = useMessage()
const submitting = ref(false)

/** 内置定义部件数量：解析失败按 0 展示。 */
const builtinWidgetCount = computed(() =>
  props.builtinExport ? widgetCountOf(props.builtinExport.widgets) : 0
)

const handleSeed = async () => {
  submitting.value = true
  try {
    const { data, error } = await seedBuiltinWidgetBundle()
    if (!error && data) {
      if (data.idempotent) {
        message.info($t('page.widgetBundle.seedIdempotent'))
      } else {
        message.success($t('page.widgetBundle.seedSuccess'))
      }
      emit('seeded')
      emit('update:show', false)
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <NDrawer :show="show" :width="520" @update:show="emit('update:show', $event)">
    <NDrawerContent :title="$t('page.widgetBundle.builtinPreview')" closable>
      <NSpace vertical>
        <NSpace>
          <NTag type="info">{{ builtinExport?.kind }}</NTag>
          <NTag>{{ builtinExport?.name }}</NTag>
          <NTag>v{{ builtinExport?.version }}</NTag>
          <NTag type="success">{{ $t('page.widgetBundle.widgetCount') }}: {{ builtinWidgetCount }}</NTag>
        </NSpace>
        <div v-if="builtinExport?.description" class="text-13px text-gray-500">{{ builtinExport.description }}</div>
        <NInput :value="builtinExport?.widgets ?? ''" type="textarea" :rows="16" readonly class="font-mono" />
        <div class="text-12px text-gray-400">{{ $t('page.widgetBundle.seedHint') }}</div>
      </NSpace>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="emit('update:show', false)">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="submitting" @click="handleSeed">
            {{ $t('page.widgetBundle.seedAction') }}
          </NButton>
        </NSpace>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped></style>
