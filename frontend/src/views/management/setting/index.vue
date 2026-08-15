<!--
系统设置页，负责聚合数据清理、账号资料、账号邮箱、告警邮箱、品牌配置和功能开关等系统级配置子页。
核心链路：本页只承担 tab 壳层职责，各类设置的拉取、保存和校验分别下沉到对应组件中执行。
静态维护重点：
1. tab 顺序已经隐含了“高风险运维操作 -> 账号信息 -> 品牌与功能配置”的分组方式，调整时要同步检查 README 和国际化文案。
2. 这里聚合的子页既有个人资料，也有平台级配置，后续新增设置项时要先确认属于账号域还是系统域，避免继续混杂。
3. 页面当前不做懒加载，切换 tab 会保留子组件状态；如果以后改成按需挂载，需要同步评估未保存表单丢失问题。
-->
<script setup lang="ts">
import { $t } from '@/locales'
import FunctionSetting from './components/function-setting.vue'
import DataClearSetting from './components/data-clear-setting.vue'
import AccountEmailSetting from './components/account-email-setting.vue'
import AccountProfileSetting from './components/account-profile-setting.vue'
import BrandingSetting from './components/branding-setting.vue'
import TelemetryDeadLetterSetting from './components/telemetry-dead-letter-setting.vue'
import WarningEmailSetting from './components/warning-email-setting.vue'
</script>

<template>
  <div class="overflow-hidden">
    <NCard :bordered="false" class="h-full rounded-8px shadow-sm">
      <div class="h-full flex-col">
        <NTabs type="line" animated>
          <NTabPane name="data-clear" :tab="$t('page.manage.setting.dataClearSetting.title')">
            <DataClearSetting></DataClearSetting>
          </NTabPane>
          <NTabPane name="telemetry-dead-letter" :tab="$t('custom.management.telemetryDeadLetter.title')">
            <TelemetryDeadLetterSetting />
          </NTabPane>
          <NTabPane name="account-profile" :tab="$t('custom.management.accountProfile.title')">
            <AccountProfileSetting />
          </NTabPane>
          <NTabPane name="account-email" :tab="$t('custom.personalCenter.changeAccountEmail')">
            <AccountEmailSetting />
          </NTabPane>
          <NTabPane name="warning-email" :tab="$t('custom.management.warningEmail')">
            <WarningEmailSetting />
          </NTabPane>
          <NTabPane name="branding" :tab="$t('custom.management.branding')">
            <BrandingSetting />
          </NTabPane>
          <NTabPane name="function" :tab="$t('custom.management.configSetting')">
            <FunctionSetting></FunctionSetting>
          </NTabPane>
        </NTabs>
      </div>
    </NCard>
  </div>
</template>

<style lang="scss"></style>
