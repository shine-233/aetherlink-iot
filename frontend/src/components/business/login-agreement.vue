<!--
  文件用途：登录/注册入口使用的协议确认组件，负责展示勾选框、协议文案和跳转事件。
  核心逻辑：通过 `computed` 把外层 `value` 映射为可双向绑定的 `checked`，再把协议与隐私政策点击事件抛回父层。
  关键数据流：
  1. 外层页面通过 `value` 控制是否勾选。
  2. 组件内部把勾选变化透传为 `update:value`。
  3. 协议与隐私链接点击事件由父页面决定是打开路由、外链还是弹窗。
  使用注意：
  - 当前默认勾选值为 `true`，接入页面若有更严格的合规要求，需要显式覆盖。
  - 默认协议地址是 `/terms` 和 `/privacy`，若项目存在多站点或品牌化协议页，建议由外层显式传入。
  静态审查建议：
  - 保持该组件只做交互表达，不在内部承载登录阻断、埋点或合规策略判断。
  - 后续若增加更多协议项，优先考虑配置化文本和链接，而不是继续硬编码多个字段。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

defineOptions({ name: 'LoginAgreement' })

interface Props {
  /** 是否勾选 */
  value?: boolean
  protocolHref?: string
  policyHref?: string
}

const props = withDefaults(defineProps<Props>(), {
  value: true,
  protocolHref: '/terms',
  policyHref: '/privacy'
})

interface Emits {
  (e: 'update:value', value: boolean): void

  /** 点击协议 */
  (e: 'click-protocol'): void

  /** 点击隐私政策 */
  (e: 'click-policy'): void
}

const emit = defineEmits<Emits>()

// 用 computed 包装双向绑定，避免模板直接操作 props。
const checked = computed({
  get() {
    return props.value
  },
  set(newValue: boolean) {
    emit('update:value', newValue)
  }
})

function handleClickProtocol() {
  emit('click-protocol')
  window.open(props.protocolHref, '_blank', 'noopener,noreferrer')
}

function handleClickPolicy() {
  emit('click-policy')
  window.open(props.policyHref, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div class="w-full text-14px">
    <NCheckbox v-model:checked="checked">{{ $t('page.login.register.agreement') }}</NCheckbox>

    <NButton :text="true" type="primary" @click="handleClickProtocol">{{ $t('generate.user-agreement') }}</NButton>
    <NButton :text="true" type="primary" @click="handleClickPolicy">{{ $t('generate.privacy-policy') }}</NButton>
  </div>
</template>

<style scoped></style>
