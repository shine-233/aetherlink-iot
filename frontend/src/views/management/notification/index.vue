<!--
通知服务管理页，负责聚合邮件、短信和推送三类通知通道配置。
核心链路：页面只承担 tab 壳层职责，具体配置读写分别下沉到 Email、ShortMessage、PushNotification 子组件。
静态维护重点：
1. 当前 tab 顺序直接对应三类通知能力，新增通道时要同步检查国际化、默认 tab 与目录 README。
2. 各子组件都直接读写远端通知服务配置，后续若要补权限禁用态或统一保存反馈，优先从本页抽公共壳层。
3. Tab 内容使用 show:lazy 首次进入才挂载，减少首屏配置请求；切换后保留实例，避免表单回显反复重拉。
-->
<script setup lang="ts">
import Email from './components/email.vue'
import ShortMessage from './components/short-message.vue'
import PushNotification from './components/push-notification.vue'
import { $t } from '~/src/locales'
</script>

<template>
  <div class="overflow-hidden">
    <NCard :bordered="false" class="h-full rounded-8px shadow-sm">
      <div class="h-full flex-col">
        <NTabs type="line" animated>
          <NTabPane
            name="1"
            :tab="$t('page.manage.notification.email.title')"
            class="pannel-content"
            display-directive="show:lazy"
          >
            <Email></Email>
          </NTabPane>
          <NTabPane
            name="2"
            :tab="$t('page.manage.notification.shortMessage.title')"
            class="pannel-content"
            display-directive="show:lazy"
          >
            <ShortMessage></ShortMessage>
          </NTabPane>
          <NTabPane
            name="3"
            :tab="$t('page.manage.notification.pushNotification.title')"
            class="pannel-content"
            display-directive="show:lazy"
          >
            <PushNotification></PushNotification>
          </NTabPane>
        </NTabs>
      </div>
    </NCard>
  </div>
</template>

<style lang="scss" scoped>
.pannel-content {
  padding-top: 16px !important;
}
</style>
