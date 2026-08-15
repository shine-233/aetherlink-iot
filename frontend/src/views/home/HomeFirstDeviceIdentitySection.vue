<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'
import type { HomeFirstRunProtocol } from './homeFirstRunWizard'

type ProtocolOption = {
  label: string
  value: HomeFirstRunProtocol
  description: string
}

const props = defineProps<{
  firstDevice: any
  firstDeviceFocusMode: boolean
  firstDeviceWorkbenchLoaded: boolean
  firstDeviceLoading: boolean
  firstRunProtocol: HomeFirstRunProtocol
  firstRunCreateLoading: boolean
  firstRunCreateTenantRequired: boolean
  deploymentHealthOk: boolean
  firstRunCreateResult: any
  firstRunSetupBlockerStep?: any
  firstRunSetupBlockerTitle: string
  firstRunSetupBlockerDescription: string
  firstRunSetupBlockerAction: string
}>()

const emit = defineEmits<{
  refreshFirstDeviceWorkbench: []
  refreshDeploymentHealth: []
  updateFirstRunProtocol: [protocol: HomeFirstRunProtocol]
  createFirstRunFirstDevice: []
  openManualDeviceAdd: []
  openThingsModel: []
  openHomeGuideStep: [step: any]
}>()

const firstRunProtocolOptions = computed<ProtocolOption[]>(() => [
  {
    label: 'MQTT',
    value: 'MQTT',
    description: $t('custom.home.firstDevice.identity.protocol.mqttDescription')
  },
  {
    label: 'HTTP',
    value: 'HTTP',
    description: $t('custom.home.firstDevice.identity.protocol.httpDescription')
  }
])

const selectedFirstRunProtocolOption = computed(
  () =>
    firstRunProtocolOptions.value.find((option) => option.value === props.firstRunProtocol) ||
    firstRunProtocolOptions.value[0]
)

const firstDeviceSetupSequence = computed(() => [
  {
    order: '1',
    label: $t('custom.home.firstDevice.identity.sequence.adminTenant'),
    detail: props.firstRunCreateTenantRequired
      ? $t('custom.home.firstDevice.identity.sequence.completeInitialization')
      : $t('custom.home.firstDevice.identity.sequence.ready'),
    state: props.firstRunCreateTenantRequired ? 'active' : 'done'
  },
  {
    order: '2',
    label: $t('custom.home.firstDevice.identity.sequence.deploymentHealth'),
    detail: props.deploymentHealthOk
      ? $t('custom.home.firstDevice.identity.sequence.componentsAvailable')
      : $t('custom.home.firstDevice.identity.sequence.checkDependencies'),
    state: props.deploymentHealthOk ? 'done' : props.firstRunCreateTenantRequired ? 'todo' : 'active'
  },
  {
    order: '3',
    label: $t('custom.home.firstDevice.identity.sequence.defaultProductModel'),
    detail: props.firstDevice
      ? $t('custom.home.firstDevice.identity.sequence.generated')
      : $t('custom.home.firstDevice.identity.sequence.generatedWithDevice'),
    state: props.firstDevice ? 'done' : props.firstRunCreateTenantRequired || !props.deploymentHealthOk ? 'todo' : 'active'
  },
  {
    order: '4',
    label: $t('custom.home.firstDevice.identity.sequence.firstDevice'),
    detail: props.firstDevice
      ? $t('custom.home.firstDevice.identity.sequence.parametersCopyable')
      : $t('custom.home.firstDevice.identity.sequence.generateOnce'),
    state: props.firstDevice ? 'done' : props.firstRunCreateTenantRequired || !props.deploymentHealthOk ? 'todo' : 'active'
  }
])

const updateFirstRunProtocol = (value: HomeFirstRunProtocol) => {
  emit('updateFirstRunProtocol', value)
}
</script>

<template>
  <div v-if="!firstDevice && !firstDeviceWorkbenchLoaded" class="rounded-6px bg-gray-50 px-12px py-10px">
    <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.common.step1') }}</div>
    <div class="mt-2px font-600">
      {{
        firstDeviceFocusMode
          ? $t('custom.home.firstDevice.identity.preparingWorkbench')
          : $t('custom.home.firstDevice.identity.loadWorkbench')
      }}
    </div>
    <div class="mt-4px text-gray-500">
      {{
        firstDeviceFocusMode
          ? $t('custom.home.firstDevice.identity.preparingWorkbenchDesc')
          : $t('custom.home.firstDevice.identity.loadWorkbenchDesc')
      }}
    </div>
    <div class="mt-10px flex flex-wrap gap-8px">
      <n-button size="small" type="primary" :loading="firstDeviceLoading" @click="emit('refreshFirstDeviceWorkbench')">
        {{
          firstDeviceFocusMode
            ? $t('custom.home.firstDevice.identity.reprepareWorkbench')
            : $t('custom.home.firstDevice.identity.loadWorkbenchAction')
        }}
      </n-button>
      <n-button size="small" secondary @click="emit('refreshDeploymentHealth')">
        {{ $t('custom.home.firstDevice.identity.checkHealthFirst') }}
      </n-button>
    </div>
  </div>

  <div v-else-if="!firstDevice" class="rounded-6px bg-gray-50 px-12px py-10px">
    <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.common.step1') }}</div>
    <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.identity.createFirstDeviceTitle') }}</div>
    <div class="mt-4px text-gray-500">
      {{ $t('custom.home.firstDevice.identity.createFirstDeviceDesc') }}
    </div>
    <div class="mt-10px first-device-setup-sequence">
      <div v-for="step in firstDeviceSetupSequence" :key="step.order" class="first-device-setup-step" :data-state="step.state">
        <span>{{ step.order }}</span>
        <strong>{{ step.label }}</strong>
        <small>{{ step.detail }}</small>
      </div>
    </div>
    <div class="mt-10px rounded-6px border border-gray-200 bg-white px-10px py-8px">
      <div class="flex flex-col gap-8px sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.identity.accessProtocol') }}</div>
          <div class="mt-3px text-12px line-height-18px text-gray-600">
            {{ selectedFirstRunProtocolOption.description }}
          </div>
        </div>
        <n-radio-group
          :value="firstRunProtocol"
          size="small"
          name="first-run-protocol"
          :disabled="firstRunCreateLoading"
          @update:value="updateFirstRunProtocol"
        >
          <n-radio-button v-for="option in firstRunProtocolOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </n-radio-button>
        </n-radio-group>
      </div>
    </div>
    <div class="mt-10px flex flex-wrap gap-8px">
      <n-button
        size="small"
        type="primary"
        :loading="firstRunCreateLoading"
        :disabled="firstRunCreateTenantRequired || !deploymentHealthOk"
        @click="emit('createFirstRunFirstDevice')"
      >
        {{ $t('custom.home.firstDevice.identity.createFirstDeviceAction') }}
      </n-button>
      <n-button size="small" @click="emit('openManualDeviceAdd')">
        {{ $t('custom.home.firstDevice.identity.manualAddDevice') }}
      </n-button>
      <n-button size="small" quaternary @click="emit('openThingsModel')">
        {{ $t('custom.home.firstDevice.identity.editThingsModel') }}
      </n-button>
    </div>
    <div
      v-if="!deploymentHealthOk"
      class="mt-8px flex flex-wrap items-center gap-8px text-12px text-orange-600"
    >
      <span>{{ $t('custom.home.firstDevice.identity.healthBlockedHint') }}</span>
      <n-button size="tiny" secondary :loading="firstDeviceLoading" @click="emit('refreshDeploymentHealth')">
        {{ $t('custom.home.firstDevice.common.checkDeploymentHealth') }}
      </n-button>
    </div>
    <div v-if="firstRunCreateResult" class="mt-8px text-12px text-gray-500">
      {{
        $t('custom.home.firstDevice.identity.createdResult', {
          deviceName: firstRunCreateResult.deviceName,
          protocol: firstRunCreateResult.protocol || 'MQTT'
        })
      }}
    </div>
    <div v-if="firstRunCreateTenantRequired" class="mt-10px rounded-6px border border-orange-200 bg-orange-50 px-10px py-8px">
      <div class="text-13px text-orange-700 font-600">{{ firstRunSetupBlockerTitle }}</div>
      <div class="mt-3px text-12px line-height-18px text-orange-700">
        {{ firstRunSetupBlockerDescription }}
      </div>
      <n-button
        size="tiny"
        type="primary"
        class="mt-8px"
        @click="firstRunSetupBlockerStep && emit('openHomeGuideStep', firstRunSetupBlockerStep)"
      >
        {{ firstRunSetupBlockerAction }}
      </n-button>
    </div>
  </div>

  <template v-else>
    <div>
      <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.common.step1') }}</div>
      <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.identity.confirmIdentityTitle') }}</div>
    </div>
    <div class="first-device-status">
      <div>
        <span>{{ $t('custom.home.firstDevice.identity.device') }}</span>
        <strong>{{ firstDevice.name }}</strong>
      </div>
      <div>
        <span>{{ $t('custom.home.firstDevice.identity.number') }}</span>
        <strong>{{ firstDevice.number }}</strong>
      </div>
      <div>
        <span>{{ $t('custom.devicePage.online') }}</span>
        <strong>
          {{
            firstDevice.online
              ? $t('custom.devicePage.online')
              : $t('custom.home.firstDevice.identity.notOnline')
          }}
        </strong>
      </div>
      <div>
        <span>{{ $t('custom.home.firstDevice.identity.thingsModel') }}</span>
        <strong>{{ firstDevice.configName || $t('custom.home.firstDevice.identity.unbound') }}</strong>
      </div>
    </div>
  </template>
</template>

<style scoped>
.first-device-status {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.first-device-status > div {
  min-width: 0;
  padding: 10px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.first-device-status span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.first-device-status strong {
  display: block;
  overflow-wrap: anywhere;
  color: #0f172a;
}

.first-device-setup-sequence {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.first-device-setup-step {
  min-width: 0;
  padding: 9px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #fff;
}

.first-device-setup-step span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  margin-bottom: 6px;
  border-radius: 999px;
  background: #eef2ff;
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
}

.first-device-setup-step strong,
.first-device-setup-step small {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-setup-step strong {
  color: #0f172a;
  font-size: 13px;
}

.first-device-setup-step small {
  margin-top: 3px;
  color: #64748b;
  font-size: 12px;
  line-height: 18px;
}

.first-device-setup-step[data-state='done'] {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-setup-step[data-state='done'] span {
  background: #dcfce7;
  color: #15803d;
}

.first-device-setup-step[data-state='active'] {
  border-color: #fed7aa;
  background: #fff7ed;
}

.first-device-setup-step[data-state='active'] span {
  background: #ffedd5;
  color: #c2410c;
}

@media (max-width: 640px) {
  .first-device-status,
  .first-device-setup-sequence {
    grid-template-columns: 1fr;
  }
}
</style>
