<!--
文件用途: 承载Add Devices Step3相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'
import { useRouterPush } from '@/hooks/common/router'
import { buildAddDeviceSuccessNextSteps } from './add-device-success-next-steps'

const props = defineProps<{
  isSuccess: boolean
  device_id: string
  device_config_id?: string
  firstDeviceOnboarding?: boolean
  closeCallback: () => void
  backCallback: () => void
}>()

const { routerPushByKey } = useRouterPush()
const successNextSteps = computed(() =>
  buildAddDeviceSuccessNextSteps(props.device_id, {
    firstDeviceOnboarding: props.firstDeviceOnboarding,
    deviceConfigId: props.device_config_id
  })
)
const primaryNextStep = computed(() => successNextSteps.value[0])
const followUpNextSteps = computed(() => successNextSteps.value.slice(1))

const openNextStep = async (step: ReturnType<typeof buildAddDeviceSuccessNextSteps>[number]) => {
  await routerPushByKey(step.routeKey, { query: step.query })
  props.closeCallback()
}
</script>

<template>
  <n-result
    v-if="isSuccess"
    status="success"
    :title="$t('custom.devicePage.success')"
    :description="$t('custom.devicePage.deviceConfigSuccess')"
  >
    <template #footer>
      <n-card class="success-next-steps" :bordered="false">
        <template #header>{{ $t('custom.devicePage.nextStepsTitle') }}</template>
        <div class="success-next-steps__intro">
          {{ $t('custom.devicePage.nextStepsIntro') }}
        </div>
        <div v-if="primaryNextStep" class="success-next-steps__primary">
          <div>
            <div class="success-next-step__title">{{ $t(primaryNextStep.titleKey) }}</div>
            <div class="success-next-step__desc">{{ $t(primaryNextStep.descriptionKey) }}</div>
          </div>
          <n-button type="primary" @click="openNextStep(primaryNextStep)">
            {{ $t(primaryNextStep.actionKey) }}
          </n-button>
        </div>
        <n-grid cols="1 s:2" responsive="screen" :x-gap="12" :y-gap="12">
          <n-grid-item v-for="step in followUpNextSteps" :key="step.key">
            <n-card size="small" embedded class="success-next-step">
              <div class="success-next-step__title">{{ $t(step.titleKey) }}</div>
              <div class="success-next-step__desc">{{ $t(step.descriptionKey) }}</div>
              <n-button class="mt-3" size="small" :type="step.type" secondary @click="openNextStep(step)">
                {{ $t(step.actionKey) }}
              </n-button>
            </n-card>
          </n-grid-item>
        </n-grid>
      </n-card>
      <n-button class="mt-4" @click="closeCallback">{{ $t('custom.devicePage.close') }}</n-button>
    </template>
  </n-result>
  <n-result
    v-if="!isSuccess"
    status="error"
    :title="$t('custom.devicePage.fail')"
    :description="$t('custom.devicePage.deviceConfigFail')"
  >
    <template #footer>
      <n-button @click="backCallback">{{ $t('custom.devicePage.back') }}</n-button>
      <n-button @click="closeCallback">{{ $t('custom.devicePage.close') }}</n-button>
    </template>
  </n-result>
</template>

<style scoped>
.success-next-steps {
  width: min(760px, 100%);
  margin: 0 auto;
  text-align: left;
  background: rgba(24, 160, 88, 0.06);
}

.success-next-steps__intro {
  margin-bottom: 12px;
  color: var(--text-color-2);
  line-height: 1.6;
}

.success-next-steps__primary {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
  padding: 14px;
  border: 1px solid rgba(24, 160, 88, 0.32);
  border-radius: 8px;
  background: rgba(24, 160, 88, 0.1);
}

.success-next-step {
  height: 100%;
}

.success-next-step__title {
  font-weight: 600;
  margin-bottom: 6px;
}

.success-next-step__desc {
  color: var(--text-color-2);
  line-height: 1.5;
}

@media (max-width: 720px) {
  .success-next-steps__primary {
    grid-template-columns: 1fr;
  }
}
</style>
