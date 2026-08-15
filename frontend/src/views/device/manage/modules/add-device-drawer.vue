<script setup lang="ts">
import { computed, type StyleValue } from 'vue'
import type { DrawerPlacement, StepsProps } from 'naive-ui'
import AddDevicesStep1 from './add-devices-step1.vue'
import AddDevicesStep2 from './add-devices-step2.vue'
import AddDevicesStep3 from './add-devices-step3.vue'

const props = defineProps<{
  show: boolean
  addKey?: string | number
  placement: DrawerPlacement
  manualStep: number
  manualStatus: StepsProps['status']
  configOptions?: any[]
  deviceId?: string | null
  deviceConfigId?: string | null
  manualDeviceNumber?: string
  deviceFormData?: Record<string, unknown>
  formElements?: any
  isSuccess: boolean
  deviceNumber: string
  buttonDisabled: boolean
  showMessage: boolean
  messageStyle: StyleValue
  firstDeviceOnboarding?: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  'update:manualStep': [value: number]
  'update:deviceNumber': [value: string]
  afterLeave: []
  setUpId: [deviceId: string, configId: string, deviceObject: string, deviceNumber: string]
  setIsSuccess: [value: boolean]
  completeNumberAdd: []
}>()

const showModel = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value)
})

const manualStepModel = computed({
  get: () => props.manualStep,
  set: (value: number) => emit('update:manualStep', value)
})

const deviceNumberModel = computed({
  get: () => props.deviceNumber,
  set: (value: string) => emit('update:deviceNumber', value)
})

const goNextManualStep = () => {
  manualStepModel.value += 1
}

const goBackManualStep = () => {
  manualStepModel.value -= 1
}

const closeDrawer = () => {
  showModel.value = false
}
</script>

<template>
  <NDrawer v-model:show="showModel" :height="720" :placement="placement" @after-leave="$emit('afterLeave')">
    <NDrawerContent v-if="addKey === 'hands'" :title="$t('generate.manually-add-device')" class="flex-center pt-24px">
      <NSteps :current="manualStepModel" :status="manualStatus">
        <NStep :title="$t('custom.devicePage.step1Title')" :description="$t('custom.devicePage.step1Desc')" />
        <NStep :title="$t('custom.devicePage.step2Title')" :description="$t('custom.devicePage.step2Desc')" />
        <NStep :title="$t('custom.devicePage.step3Title')" :description="$t('custom.devicePage.step3Desc')" />
      </NSteps>
      <NCard class="mt-6" bordered border>
        <div v-if="manualStepModel === 1">
          <AddDevicesStep1
            :set-id-callback="(dId, cId, dobj, deviceNumber) => $emit('setUpId', dId, cId, dobj, deviceNumber)"
            :config-options="configOptions ?? []"
            :next-callback="goNextManualStep"
          />
        </div>
        <div v-if="manualStepModel === 2">
          <AddDevicesStep2
            :set-is-success="(value) => $emit('setIsSuccess', value)"
            :device_id="deviceId ?? ''"
            :device-number="manualDeviceNumber"
            :form-data="deviceFormData ?? {}"
            :form-elements="formElements"
            :next-callback="goNextManualStep"
          />
        </div>
        <div v-if="manualStepModel === 3">
          <AddDevicesStep3
            :is-success="isSuccess"
            :device_id="deviceId ?? ''"
            :device_config_id="deviceConfigId ?? undefined"
            :first-device-onboarding="firstDeviceOnboarding"
            :close-callback="closeDrawer"
            :back-callback="goBackManualStep"
          />
        </div>
      </NCard>
    </NDrawerContent>

    <NDrawerContent
      v-if="addKey === 'number'"
      class="flex-left pt-24px"
      style="margin-left: 500px"
      :title="$t('custom.devicePage.addByNumber')"
    >
      <NH4 align-text>
        <NLi>
          <NText strong>{{ $t('custom.devicePage.tips') }}</NText>
        </NLi>
      </NH4>
      <div style="display: flex; margin-bottom: 20px">
        <NInput
          v-model:value="deviceNumberModel"
          :placeholder="$t('custom.devicePage.enterDeviceNumber')"
          maxlength="12"
          class="max-w-240px"
        />
        <NText v-if="showMessage" :style="messageStyle">
          {{
            buttonDisabled
              ? $t('custom.devicePage.deviceNumberNotAvailable')
              : $t('custom.devicePage.enterDeviceNumber')
          }}
        </NText>
      </div>
      <NButton type="primary" :disabled="buttonDisabled" @click="$emit('completeNumberAdd')">
        {{ $t('custom.devicePage.finish') }}
      </NButton>
    </NDrawerContent>
  </NDrawer>
</template>
