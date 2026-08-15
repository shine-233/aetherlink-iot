<!--
  文件用途：集中提供应用级 Naive UI 上下文和全局挂载能力。
  核心逻辑：包裹全局 provider 组件，让消息、弹窗、通知和加载条等能力在应用内可用。
  关键注意事项：provider 层级影响全局反馈能力，调整顺序时需确认主题、语言和弹窗行为。
  重构建议：若全局 provider 继续增多，可按主题、反馈和布局职责拆分包装组件。
-->
<script setup lang="ts">
import { createTextVNode, defineComponent } from 'vue'
import { useDialog, useLoadingBar, useMessage, useNotification } from 'naive-ui'

defineOptions({
  name: 'AppProvider'
})

const ContextHolder = defineComponent({
  name: 'ContextHolder',
  setup() {
    function register() {
      window.$loadingBar = useLoadingBar()
      window.$dialog = useDialog()
      window.$message = useMessage()
      window.$notification = useNotification()
    }

    register()

    return () => createTextVNode()
  }
})
</script>

<template>
  <NLoadingBarProvider>
    <NDialogProvider>
      <NNotificationProvider>
        <NMessageProvider>
          <ContextHolder />
          <slot></slot>
        </NMessageProvider>
      </NNotificationProvider>
    </NDialogProvider>
  </NLoadingBarProvider>
</template>

<style scoped></style>
