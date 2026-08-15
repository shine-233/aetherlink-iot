<!--
文件用途: 物模型配置完成步骤。
核心逻辑: 读取模板信息并展示完成态，提示用户后续可继续管理模板。
关键注意事项: 完成页依赖模板 ID 和接口返回数据，缺失时要保持安全空态。
重构建议: 将完成态数据加载和展示文案拆分，便于复用到导入或复制模板流程。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getTemplat } from '@/service/api/system-data'
import { $t } from '@/locales'
import { message } from '@/utils/common/discrete'

const props = defineProps({
  stepCurrent: { type: Number, required: true },
  modalVisible: {
    type: Boolean,
    required: true
  },
  deviceTemplateId: { type: String, required: true }
})
const emit = defineEmits(['update:modalVisible', 'update:stepCurrent'])
const code = ref<string>('')

const parseTemplateJsonField = (value: unknown) => {
  if (!value || typeof value !== 'string') return value

  try {
    return JSON.parse(value)
  } catch {
    // Keep malformed chart JSON visible in the final export instead of failing the wizard completion step.
    return value
  }
}

const getTemplate = async () => {
  const { data, error } = await getTemplat(props.deviceTemplateId)
  if (!error) {
    data.app_chart_config = parseTemplateJsonField(data.app_chart_config)
    data.web_chart_config = parseTemplateJsonField(data.web_chart_config)
    code.value = JSON.stringify(data, null, 2)
  }
}
const back: () => void = async () => {
  emit('update:stepCurrent', 4)
}
const copyText = (): void => {
  const textElement = document.getElementById('text-to-copy')
  if (textElement) {
    const text: string | null = textElement.textContent
    if (window.isSecureContext && navigator.clipboard) {
      navigator.clipboard
        .writeText(typeof text === 'string' ? text : '')
        .then(() => {
          message.info($t('common.copiedClipboard'))
        })
        .catch((err) => {
          message.error(`${$t('common.copyingFailed')}:`, err)
        })
    } else {
      const range = document.createRange()
      range.selectNodeContents(textElement!)
      const selection = document.getSelection()
      selection?.removeAllRanges()
      selection?.addRange(range)
      document.execCommand('Copy')
      window.$message?.success($t('theme.configOperation.copySuccess'))
    }
  }
}
onMounted(getTemplate)
</script>

<template>
  <div>
    <n-card class="mt-4">
      <n-scrollbar class="h-400px">
        <n-code id="text-to-copy" :code="code" language="json" />
      </n-scrollbar>
      <template #footer>
        <div class="flex justify-between border-t pt-3">
          <div>
            <n-button type="primary" class="mr-4" @click="copyText">{{ $t('generate.copy-json') }}</n-button>
          </div>
          <div>
            <n-button class="mr-4" @click="back">{{ $t('generate.previous-step') }}</n-button>
            <n-button type="primary" @click="emit('update:modalVisible', false)">
              {{ $t('common.complete') }}
            </n-button>
          </div>
        </div>
      </template>
    </n-card>
  </div>
</template>

<style scoped></style>
