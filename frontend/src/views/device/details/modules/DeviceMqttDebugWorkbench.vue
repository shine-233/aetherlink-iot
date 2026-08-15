<!--
  文件用途：为当前设备提供隔离的 MQTT 连接、订阅、发布和有界消息日志工作台。
  核心逻辑：显式开启短时后端会话，所有 topic 由后端按设备/租户校验，页面仅展示受容量与长度限制的状态和原始设备载荷。
  关键注意事项：工作台不读取 broker 凭据，不复用生产订阅连接，离开页面会尽力关闭会话且服务端仍有 TTL 兜底。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  applyDeviceMQTTDebugCommand,
  closeDeviceMQTTDebugSession,
  getDeviceMQTTDebugSession,
  openDeviceMQTTDebugSession,
  type DeviceMQTTDebugMessage,
  type DeviceMQTTDebugSnapshot,
  type DeviceMQTTDebugSubscription
} from '@/service/api/device'
import { $t } from '@/locales'

const props = defineProps<{
  deviceId: string
  defaultSubscribeTopic?: string
  defaultPublishTopic?: string
  defaultPayload?: string
}>()

const session = ref<DeviceMQTTDebugSnapshot | null>(null)
const messages = ref<DeviceMQTTDebugMessage[]>([])
const opening = ref(false)
const actionLoading = ref(false)
const refreshLoading = ref(false)
const subscribeTopic = ref(props.defaultSubscribeTopic || '')
const publishTopic = ref(props.defaultPublishTopic || '')
const publishPayload = ref(props.defaultPayload || '{}')
const subscribeQoS = ref(0)
const publishQoS = ref(0)
const qosOptions = [
  { label: 'QoS 0', value: 0 },
  { label: 'QoS 1', value: 1 }
]
let refreshTimer: ReturnType<typeof setInterval> | null = null
let deviceEpoch = 0
let subscribeTopicTouched = false
let publishTopicTouched = false
let publishPayloadTouched = false

const active = computed(() => Boolean(session.value?.session_id))
const connected = computed(() => Boolean(session.value?.connected))
const hasPlatformDeviceOnline = computed(() => typeof session.value?.platform_device_online === 'boolean')
const platformDeviceOnline = computed(() => Boolean(session.value?.platform_device_online))
const subscriptionItems = computed<DeviceMQTTDebugSubscription[]>(() => {
  if (session.value?.subscription_details?.length) return session.value.subscription_details
  return (session.value?.subscriptions || []).map(topic => ({
    topic,
    mode: 'broker_subscription',
    qos: undefined
  }))
})

watch(
  () => props.defaultSubscribeTopic,
  value => {
    if (!subscribeTopicTouched) subscribeTopic.value = value || ''
  }
)

watch(
  () => props.defaultPublishTopic,
  value => {
    if (!publishTopicTouched) publishTopic.value = value || ''
  }
)

watch(
  () => props.defaultPayload,
  value => {
    if (!publishPayloadTouched) publishPayload.value = value || '{}'
  }
)

watch(
  () => props.deviceId,
  (_deviceId, previousDeviceId) => {
    const previousSessionId = session.value?.session_id
    deviceEpoch += 1
    stopPolling()
    session.value = null
    messages.value = []
    opening.value = false
    actionLoading.value = false
    refreshLoading.value = false
    subscribeTopicTouched = false
    publishTopicTouched = false
    publishPayloadTouched = false
    subscribeTopic.value = props.defaultSubscribeTopic || ''
    publishTopic.value = props.defaultPublishTopic || ''
    publishPayload.value = props.defaultPayload || '{}'
    if (previousSessionId && previousDeviceId) {
      void closeDeviceMQTTDebugSession(previousDeviceId, previousSessionId)
    }
  }
)

function applySnapshot(snapshot: DeviceMQTTDebugSnapshot) {
  const sameSession = session.value?.session_id === snapshot.session_id
  const messageBySequence = new Map<number, DeviceMQTTDebugMessage>()
  if (sameSession) {
    for (const message of messages.value) messageBySequence.set(message.sequence, message)
  }
  for (const message of Array.isArray(snapshot.messages) ? snapshot.messages : []) {
    messageBySequence.set(message.sequence, message)
  }
  const capacity = snapshot.message_capacity > 0 ? snapshot.message_capacity : 200
  messages.value = [...messageBySequence.values()]
    .sort((left, right) => left.sequence - right.sequence)
    .slice(-capacity)
  session.value = snapshot
}

function lastReceivedSequence() {
  return messages.value.length > 0 ? messages.value[messages.value.length - 1].sequence : 0
}

function stopPolling() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

function startPolling() {
  stopPolling()
  refreshTimer = setInterval(() => {
    void refreshSession(false)
  }, 3000)
}

type MQTTDebugRequestFailure = {
  status?: number
  data?: { code?: number }
  response?: { status?: number; data?: { code?: number } }
}

function isMQTTDebugSessionNotFound(failure: unknown) {
  if (!failure || typeof failure !== 'object') return false
  const wrapped = failure as { error?: MQTTDebugRequestFailure }
  const error = wrapped.error || (failure as MQTTDebugRequestFailure)
  return error.status === 404 || error.response?.status === 404 || error.data?.code === 100404 || error.response?.data?.code === 100404
}

function clearMissingSession(deviceId: string, sessionId: string, epoch: number) {
  if (epoch !== deviceEpoch || deviceId !== props.deviceId || session.value?.session_id !== sessionId) return
  stopPolling()
  session.value = null
  messages.value = []
}

async function openSession() {
  if (opening.value || active.value) return
  const deviceId = props.deviceId
  const epoch = deviceEpoch
  opening.value = true
  try {
    const { data, error } = await openDeviceMQTTDebugSession(deviceId)
    if (!error && data) {
      if (epoch !== deviceEpoch || deviceId !== props.deviceId) {
        void closeDeviceMQTTDebugSession(deviceId, data.session_id)
        return
      }
      applySnapshot(data)
      startPolling()
      window.$message?.success($t('custom.device_details.mqttDebug.opened'))
    }
  } finally {
    if (epoch === deviceEpoch) opening.value = false
  }
}

async function refreshSession(showFeedback = true) {
  const sessionId = session.value?.session_id
  if (!sessionId || refreshLoading.value || actionLoading.value) return
  const deviceId = props.deviceId
  const epoch = deviceEpoch
  refreshLoading.value = true
  try {
    const { data, error } = await getDeviceMQTTDebugSession(
      deviceId,
      sessionId,
      { after_sequence: lastReceivedSequence(), limit: 200 },
      { silentError: !showFeedback }
    )
    if (!error && data) {
      if (epoch !== deviceEpoch || deviceId !== props.deviceId || session.value?.session_id !== sessionId) return
      applySnapshot(data)
      if (showFeedback) window.$message?.success($t('custom.device_details.mqttDebug.refreshed'))
    } else if (error && isMQTTDebugSessionNotFound(error)) {
      clearMissingSession(deviceId, sessionId, epoch)
    }
  } catch (failure) {
    if (isMQTTDebugSessionNotFound(failure)) clearMissingSession(deviceId, sessionId, epoch)
  } finally {
    if (epoch === deviceEpoch) refreshLoading.value = false
  }
}

async function closeSession(showFeedback = true) {
  const sessionId = session.value?.session_id
  if (!sessionId || actionLoading.value) return
  const deviceId = props.deviceId
  const epoch = deviceEpoch
  actionLoading.value = true
  try {
    const { error } = await closeDeviceMQTTDebugSession(deviceId, sessionId)
    if (epoch === deviceEpoch && deviceId === props.deviceId && session.value?.session_id === sessionId) {
      stopPolling()
      if (!error && showFeedback) window.$message?.success($t('custom.device_details.mqttDebug.closed'))
      session.value = null
      messages.value = []
    }
  } finally {
    if (epoch === deviceEpoch) actionLoading.value = false
  }
}

async function applyCommand(command: {
  action: 'subscribe' | 'unsubscribe' | 'publish'
  topic: string
  qos?: number
  payload?: string
}) {
  const sessionId = session.value?.session_id
  if (actionLoading.value || refreshLoading.value) return
  if (!sessionId || !command.topic.trim()) {
    window.$message?.warning($t('custom.device_details.mqttDebug.topicRequired'))
    return
  }
  const deviceId = props.deviceId
  const epoch = deviceEpoch
  actionLoading.value = true
  try {
    const { data, error } = await applyDeviceMQTTDebugCommand(deviceId, sessionId, {
      ...command,
      topic: command.topic.trim()
    })
    if (
      !error &&
      data &&
      epoch === deviceEpoch &&
      deviceId === props.deviceId &&
      session.value?.session_id === sessionId
    ) {
      applySnapshot(data)
      window.$message?.success(commandSuccessMessage(command.action))
    }
  } finally {
    if (epoch === deviceEpoch) actionLoading.value = false
  }
}

function commandSuccessMessage(action: 'subscribe' | 'unsubscribe' | 'publish') {
  if (action === 'subscribe') return $t('custom.device_details.mqttDebug.subscribeSuccess')
  if (action === 'unsubscribe') return $t('custom.device_details.mqttDebug.unsubscribeSuccess')
  return $t('custom.device_details.mqttDebug.publishSuccess')
}

function markSubscribeTopicTouched() {
  subscribeTopicTouched = true
}

function markPublishTopicTouched() {
  publishTopicTouched = true
}

function markPublishPayloadTouched() {
  publishPayloadTouched = true
}

function subscribe() {
  void applyCommand({ action: 'subscribe', topic: subscribeTopic.value, qos: subscribeQoS.value })
}

function unsubscribe(topic: string) {
  void applyCommand({ action: 'unsubscribe', topic })
}

function publish() {
  void applyCommand({
    action: 'publish',
    topic: publishTopic.value,
    qos: publishQoS.value,
    payload: publishPayload.value
  })
}

function formatTime(value?: string) {
  if (!value) return '--'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

function messageDirectionLabel(direction: string) {
  if (direction === 'inbound') return $t('custom.device_details.mqttDebug.directionInbound')
  if (direction === 'outbound') return $t('custom.device_details.mqttDebug.directionOutbound')
  if (direction === 'system') return $t('custom.device_details.mqttDebug.directionSystem')
  return direction
}

function messageOutcomeLabel(outcome?: string) {
  if (!outcome) return ''
  if (outcome === 'session_opened') return $t('custom.device_details.mqttDebug.outcomeSessionOpened')
  if (outcome === 'connected') return $t('custom.device_details.mqttDebug.outcomeConnected')
  if (outcome === 'connection_lost') return $t('custom.device_details.mqttDebug.outcomeConnectionLost')
  if (outcome === 'subscribed') return $t('custom.device_details.mqttDebug.outcomeSubscribed')
  if (outcome === 'subscribe_failed') return $t('custom.device_details.mqttDebug.outcomeSubscribeFailed')
  if (outcome === 'unsubscribed') return $t('custom.device_details.mqttDebug.outcomeUnsubscribed')
  if (outcome === 'unsubscribe_failed') return $t('custom.device_details.mqttDebug.outcomeUnsubscribeFailed')
  if (outcome === 'published') return $t('custom.device_details.mqttDebug.outcomePublished')
  if (outcome === 'publish_failed') return $t('custom.device_details.mqttDebug.outcomePublishFailed')
  if (outcome === 'received') return $t('custom.device_details.mqttDebug.outcomeReceived')
  if (outcome === 'resubscribe_failed') return $t('custom.device_details.mqttDebug.outcomeResubscribeFailed')
  return outcome
}

function subscriptionModeLabel(mode?: string) {
  if (mode === 'accepted_application_uplink_observer') {
    return $t('custom.device_details.mqttDebug.modeAcceptedUplinkObserver')
  }
  if (mode === 'broker_subscription') return $t('custom.device_details.mqttDebug.modeBrokerSubscription')
  if (mode === 'broker_publish') return $t('custom.device_details.mqttDebug.modeBrokerPublish')
  return mode || ''
}

onBeforeUnmount(() => {
  const deviceId = props.deviceId
  const sessionId = session.value?.session_id
  deviceEpoch += 1
  stopPolling()
  if (sessionId) void closeDeviceMQTTDebugSession(deviceId, sessionId)
})
</script>

<template>
  <NCard class="mb-4" data-testid="device-mqtt-debug-workbench">
    <template #header>
      <div class="mqtt-debug-header">
        <div>
          <strong>{{ $t('custom.device_details.mqttDebug.title') }}</strong>
          <div class="mqtt-debug-subtitle">{{ $t('custom.device_details.mqttDebug.description') }}</div>
        </div>
        <NSpace>
          <NTag v-if="active" :type="connected ? 'success' : 'warning'">
            {{ connected ? $t('custom.device_details.mqttDebug.connected') : $t('custom.device_details.mqttDebug.disconnected') }}
          </NTag>
          <NTag v-if="active && hasPlatformDeviceOnline" :type="platformDeviceOnline ? 'success' : 'default'">
            {{
              platformDeviceOnline
                ? $t('custom.device_details.mqttDebug.platformDeviceOnline')
                : $t('custom.device_details.mqttDebug.platformDeviceOffline')
            }}
          </NTag>
          <NButton v-if="!active" type="primary" :loading="opening" @click="openSession">
            {{ $t('custom.device_details.mqttDebug.open') }}
          </NButton>
          <template v-else>
            <NButton :loading="refreshLoading" :disabled="actionLoading" @click="refreshSession(true)">
              {{ $t('custom.device_details.mqttDebug.refresh') }}
            </NButton>
            <NButton type="error" secondary :loading="actionLoading" @click="closeSession(true)">
              {{ $t('custom.device_details.mqttDebug.close') }}
            </NButton>
          </template>
        </NSpace>
      </div>
    </template>

    <NAlert type="info" :show-icon="false" class="mb-4">
      {{ $t('custom.device_details.mqttDebug.scopeHint') }}
    </NAlert>

    <NEmpty v-if="!active" :description="$t('custom.device_details.mqttDebug.inactive')" />

    <template v-else>
      <div class="mqtt-debug-session-grid">
        <div>
          <span>{{ $t('custom.device_details.mqttDebug.session') }}</span>
          <strong>{{ session?.session_id }}</strong>
        </div>
        <div>
          <span>{{ $t('custom.device_details.mqttDebug.expires') }}</span>
          <strong>{{ formatTime(session?.expires_at) }}</strong>
        </div>
        <div>
          <span>{{ $t('custom.device_details.mqttDebug.dropped') }}</span>
          <strong>{{ session?.dropped_messages || 0 }}</strong>
        </div>
        <div>
          <span>{{ $t('custom.device_details.mqttDebug.uplinkObserverDropped') }}</span>
          <strong>{{ session?.uplink_observer_dropped_messages || 0 }}</strong>
        </div>
      </div>

      <div class="mqtt-debug-columns">
        <section>
          <h4>{{ $t('custom.device_details.mqttDebug.subscribeTitle') }}</h4>
          <div class="mqtt-debug-row">
            <NInput
              v-model:value="subscribeTopic"
              :placeholder="$t('custom.device_details.mqttDebug.subscribePlaceholder')"
              @update:value="markSubscribeTopicTouched"
            />
            <NSelect v-model:value="subscribeQoS" class="mqtt-debug-qos" :options="qosOptions" />
            <NButton type="primary" :disabled="!connected || refreshLoading" :loading="actionLoading" @click="subscribe">
              {{ $t('custom.device_details.mqttDebug.subscribe') }}
            </NButton>
          </div>
          <div class="mqtt-debug-subscriptions">
            <NTag
              v-for="item in subscriptionItems"
              :key="item.topic"
              :closable="connected && !actionLoading && !refreshLoading"
              @close="unsubscribe(item.topic)"
            >
              {{ item.topic }} · {{ subscriptionModeLabel(item.mode) }}<template v-if="item.qos !== undefined"> · QoS {{ item.qos }}</template>
            </NTag>
            <NEmpty
              v-if="subscriptionItems.length === 0"
              size="small"
              :description="$t('custom.device_details.mqttDebug.noSubscriptions')"
            />
          </div>
        </section>

        <section>
          <h4>{{ $t('custom.device_details.mqttDebug.publishTitle') }}</h4>
          <div class="mqtt-debug-row">
            <NInput
              v-model:value="publishTopic"
              :placeholder="$t('custom.device_details.mqttDebug.publishPlaceholder')"
              @update:value="markPublishTopicTouched"
            />
            <NSelect v-model:value="publishQoS" class="mqtt-debug-qos" :options="qosOptions" />
          </div>
          <NInput
            v-model:value="publishPayload"
            class="mt-3"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 10 }"
            maxlength="65536"
            show-count
            @update:value="markPublishPayloadTouched"
          />
          <NAlert type="warning" :show-icon="false" class="mt-3">
            {{ $t('custom.device_details.mqttDebug.publishSafetyHint') }}
          </NAlert>
          <div class="mt-3 flex justify-end">
            <NButton type="primary" :disabled="!connected || refreshLoading" :loading="actionLoading" @click="publish">
              {{ $t('custom.device_details.mqttDebug.publish') }}
            </NButton>
          </div>
        </section>
      </div>

      <section class="mt-5">
        <div class="mqtt-debug-log-heading">
          <h4>{{ $t('custom.device_details.mqttDebug.logsTitle') }}</h4>
          <span>{{ messages.length }}/{{ session?.message_capacity || 0 }}</span>
        </div>
        <NEmpty v-if="messages.length === 0" :description="$t('custom.device_details.mqttDebug.noMessages')" />
        <NScrollbar v-else class="mqtt-debug-log-scroll">
          <div v-for="message in messages" :key="message.sequence" class="mqtt-debug-message">
            <div class="mqtt-debug-message-meta">
              <NTag size="small" :type="message.direction === 'inbound' ? 'success' : message.direction === 'outbound' ? 'info' : 'default'">
                {{ messageDirectionLabel(message.direction) }}
              </NTag>
              <strong v-if="message.topic">{{ message.topic }}</strong>
              <NTag v-if="message.outcome" size="small" :bordered="false">
                {{ messageOutcomeLabel(message.outcome) }}
              </NTag>
              <NTag v-if="message.source" size="small" :bordered="false">
                {{ subscriptionModeLabel(message.source) }}
              </NTag>
              <span v-if="message.topic && message.source !== 'accepted_application_uplink_observer'">
                QoS {{ message.qos || 0 }}
              </span>
              <span>{{ formatTime(message.timestamp) }}</span>
            </div>
            <pre v-if="message.payload" class="mqtt-debug-payload">{{ message.payload }}</pre>
            <span v-if="message.truncated" class="mqtt-debug-truncated">
              {{ $t('custom.device_details.mqttDebug.truncated') }}
            </span>
          </div>
        </NScrollbar>
      </section>
    </template>
  </NCard>
</template>

<style scoped>
.mqtt-debug-header,
.mqtt-debug-log-heading,
.mqtt-debug-message-meta,
.mqtt-debug-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.mqtt-debug-header,
.mqtt-debug-log-heading {
  justify-content: space-between;
}

.mqtt-debug-subtitle,
.mqtt-debug-session-grid span,
.mqtt-debug-log-heading span,
.mqtt-debug-message-meta span,
.mqtt-debug-truncated {
  color: #64748b;
  font-size: 12px;
}

.mqtt-debug-session-grid,
.mqtt-debug-columns {
  display: grid;
  gap: 14px;
}

.mqtt-debug-session-grid {
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  margin-bottom: 16px;
}

.mqtt-debug-session-grid > div {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.mqtt-debug-session-grid strong,
.mqtt-debug-session-grid span {
  display: block;
  overflow-wrap: anywhere;
}

.mqtt-debug-columns {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.mqtt-debug-columns section {
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #f8fafc;
}

.mqtt-debug-columns h4,
.mqtt-debug-log-heading h4 {
  margin: 0 0 12px;
}

.mqtt-debug-qos {
  width: 100px;
  flex: 0 0 100px;
}

.mqtt-debug-subscriptions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.mqtt-debug-log-scroll {
  max-height: 420px;
}

.mqtt-debug-message {
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.mqtt-debug-message-meta {
  flex-wrap: wrap;
}

.mqtt-debug-payload {
  overflow-wrap: anywhere;
  margin: 8px 0 0;
  padding: 8px;
  border-radius: 6px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 12px;
  white-space: pre-wrap;
}

@media (max-width: 900px) {
  .mqtt-debug-columns {
    grid-template-columns: 1fr;
  }

  .mqtt-debug-row {
    align-items: stretch;
    flex-direction: column;
  }

  .mqtt-debug-qos {
    width: 100%;
    flex-basis: auto;
  }
}
</style>
