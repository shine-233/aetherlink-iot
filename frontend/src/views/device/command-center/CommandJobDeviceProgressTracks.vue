<script setup lang="ts">
import type { CommandJobDeviceProgressTrack } from './commandCenterJobView'

defineProps<{
  tracks: CommandJobDeviceProgressTrack[]
}>()

const emit = defineEmits<{
  openDeviceDiagnosis: [deviceId: string]
}>()
</script>

<template>
  <div class="command-job-device-progress">
    <div
      v-for="track in tracks"
      :key="track.key"
      class="command-job-device-progress__track"
    >
      <div class="command-job-device-progress__head">
        <div>
          <strong>{{ track.device }}</strong>
          <span>{{ track.summary }}</span>
        </div>
        <NTag :type="track.type" size="small">{{ track.nextAction }}</NTag>
      </div>
      <div class="command-job-device-progress__steps">
        <div
          v-for="step in track.steps"
          :key="step.key"
          class="command-job-device-progress__step"
        >
          <NTag :type="step.type" size="small">{{ step.state }}</NTag>
          <div>
            <strong>{{ step.label }}</strong>
            <span>{{ step.detail }}</span>
          </div>
        </div>
      </div>
      <NButton
        v-if="track.deviceId"
        size="tiny"
        secondary
        type="primary"
        @click="emit('openDeviceDiagnosis', track.deviceId)"
      >
        {{ $t('custom.commandCenter.openDeviceDiagnosis') }}
      </NButton>
    </div>
  </div>
</template>

<style scoped>
.command-job-device-progress {
  display: grid;
  gap: 8px;
}

.command-job-device-progress__track {
  display: grid;
  gap: 10px;
  padding: 10px;
  border: 1px solid #bae6fd;
  border-radius: 8px;
  background: #ffffff;
}

.command-job-device-progress__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.command-job-device-progress__head > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.command-job-device-progress__head strong,
.command-job-device-progress__head span,
.command-job-device-progress__step strong,
.command-job-device-progress__step span {
  overflow-wrap: anywhere;
}

.command-job-device-progress__head strong {
  color: #0c4a6e;
  font-size: 13px;
}

.command-job-device-progress__head span {
  color: #475569;
  font-size: 12px;
}

.command-job-device-progress__steps {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.command-job-device-progress__step {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  padding: 8px;
  border: 1px solid #e0f2fe;
  border-radius: 8px;
  background: #f8fafc;
}

.command-job-device-progress__step > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.command-job-device-progress__step strong {
  color: #0f172a;
  font-size: 12px;
}

.command-job-device-progress__step span {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .command-job-device-progress__steps {
    grid-template-columns: 1fr;
  }
}
</style>
