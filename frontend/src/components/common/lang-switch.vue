<!--
  文件用途：提供应用语言切换入口。
  核心逻辑：读取当前语言选项并触发本地化状态切换，让界面文案跟随 locale 更新。
  关键注意事项：语言 key 需与 locales 配置一致，调整选项时需同步语言包和持久化逻辑。
  重构建议：可将语言选项和持久化逻辑收敛到 i18n 工具层。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

defineOptions({
  name: 'LangSwitch'
})

interface Props {
  /** Current language */
  lang: App.I18n.LangType
  /** Language options */
  langOptions: App.I18n.LangOption[]
  /** Show tooltip */
  showTooltip?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showTooltip: true
})

type Emits = {
  (e: 'changeLang', lang: App.I18n.LangType): void
}

const emit = defineEmits<Emits>()

const tooltipContent = computed(() => {
  if (!props.showTooltip) return ''

  return $t('icon.lang')
})

function changeLang(lang: App.I18n.LangType) {
  emit('changeLang', lang)
}
</script>

<template>
  <NDropdown :value="lang" :options="langOptions" trigger="hover" @select="changeLang">
    <div>
      <ButtonIcon :tooltip-content="tooltipContent" tooltip-placement="left">
        <SvgIcon icon="mdi:translate" />
      </ButtonIcon>
    </div>
  </NDropdown>
</template>

<style scoped></style>
