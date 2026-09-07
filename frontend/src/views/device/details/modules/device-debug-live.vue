<!--
文件用途：设备实时调试面板（PHASE-D-D11）。
核心逻辑：① 复用平台遥测 WS 通道（/telemetry/datas/current/ws，设备鉴权帧 {device_id, token}）
把实时上行帧以原始 JSON 时间线呈现（与遥测图表不同，这里展示未加工帧，用于联调排查）；
② 命令下发面板：commandDataPub 下发 + 投递诊断接口展示在线状态与最近投递日志（ACK/超时可见）。
关键注意事项：WS 生命周期随组件销毁；token 取本地存储；命令参数必须为合法 JSON。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { commandDataPub, getCommandDeliveryDiagnostics, type CommandDeliveryDiagnostics } from '@/service/api/device'
import { localStg } from '@/utils/storage'
import { getWebsocketServerUrl } from '@/utils/common/tool'
import { $t } from '@/locales'

const props = defineProps<{
  /** 设备 ID（设备详情页签统一以 id 注入） */
  id: string
}>()

// ---- 实时上行帧时间线 ----
interface FrameEntry {
  seq: number
  at: string
  raw: string
}
const frames = ref<FrameEntry[]>([])
const wsState = ref<'idle' | 'connecting' | 'open' | 'closed'>('idle')
const autoScroll = ref(true)
const MAX_FRAMES = 50
let socket: WebSocket | null = null
let pingTimer: ReturnType<typeof setInterval> | null = null
let seq = 0
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

function appendFrame(raw: unknown) {
  seq += 1
  frames.value.unshift({
    seq,
    at: new Date().toLocaleTimeString(),
    raw: typeof raw === 'string' ? raw : JSON.stringify(raw)
  })
  if (frames.value.length > MAX_FRAMES) frames.value.length = MAX_FRAMES
}

function clearPing() {
  if (pingTimer) {
    clearInterval(pingTimer)
    pingTimer = null
  }
}

function startSocket() {
  if (socket) return
  const token = localStg.get('token') as string | undefined
  if (!token) {
    window.$message?.error($t('page.deviceDebug.noToken'))
    return
  }
  wsState.value = 'connecting'
  const url = `${getWebsocketServerUrl()}/telemetry/datas/current/ws`
  socket = new WebSocket(url)
  socket.onopen = () => {
    wsState.value = 'open'
    socket?.send(JSON.stringify({ device_id: props.id, token }))
    // 平台心跳窗口短，需周期 ping 保持连接（与既有实时模块一致 8s）。
    clearPing()
    pingTimer = setInterval(() => socket?.send('ping'), 8000)
  }
  socket.onmessage = event => {
    if (event.data === 'pong') return
    appendFrame(event.data)
  }
  socket.onclose = () => {
    wsState.value = 'closed'
    clearPing()
    socket = null
    // 弱网自动重连
    reconnectTimer = setTimeout(startSocket, 3000)
  }
  socket.onerror = () => {
    wsState.value = 'closed'
  }
}

function stopSocket() {
  clearPing()
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  socket?.close()
  socket = null
  wsState.value = 'idle'
}

onBeforeUnmount(stopSocket)

// ---- 命令下发与投递诊断 ----
const identify = ref('')
const paramsText = ref('{}')
const sending = ref(false)
const lastSendResult = ref<'success' | 'error' | null>(null)
const diagnostics = ref<CommandDeliveryDiagnostics | null>(null)
const diagnosticsLoading = ref(false)

const parsedParams = computed(() => {
  try {
    return { value: JSON.parse(paramsText.value || '{}') as Record<string, unknown>, valid: true }
  } catch {
    return { value: {} as Record<string, unknown>, valid: false }
  }
})

async function handleSendCommand() {
  if (!identify.value.trim()) {
    window.$message?.error($t('page.deviceDebug.identifyRequired'))
    return
  }
  if (!parsedParams.value.valid) {
    window.$message?.error($t('custom.rule_chain.invalidJson'))
    return
  }
  sending.value = true
  try {
    const { error } = await commandDataPub({
      device_id: props.id,
      identify: identify.value.trim(),
      params: parsedParams.value.value
    })
    lastSendResult.value = error ? 'error' : 'success'
    if (!error) {
      window.$message?.success($t('page.deviceDebug.commandSent'))
      await loadDiagnostics()
    }
  } finally {
    sending.value = false
  }
}

async function loadDiagnostics() {
  diagnosticsLoading.value = true
  try {
    const { data, error } = await getCommandDeliveryDiagnostics(props.id, { limit: 5 })
    if (!error && data) diagnostics.value = data
  } finally {
    diagnosticsLoading.value = false
  }
}

defineExpose({ startSocket, stopSocket })
</script>

<template>
  <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
    <!-- 左：实时上行帧 -->
    <n-card size="small" :bordered="true" class="rounded-8px" :title="$t('page.deviceDebug.liveFrames')">
      <template #header-extra>
        <n-space size="small">
          <n-tag :type="wsState === 'open' ? 'success' : wsState === 'connecting' ? 'warning' : 'default'" size="small">
            {{ wsState }}
          </n-tag>
          <n-button size="tiny" @click="startSocket">{{ $t('page.deviceDebug.connect') }}</n-button>
          <n-button size="tiny" @click="stopSocket">{{ $t('page.deviceDebug.disconnect') }}</n-button>
        </n-space>
      </template>
      <n-empty v-if="!frames.length" size="small" :description="$t('page.deviceDebug.noFrames')" />
      <div v-else class="debug-frames">
        <n-scrollbar style="max-height: 420px">
          <div v-for="entry in frames" :key="entry.seq" class="debug-frame">
            <span class="text-12px opacity-60">#{{ entry.seq }} {{ entry.at }}</span>
            <pre class="debug-frame-raw">{{ entry.raw }}</pre>
          </div>
        </n-scrollbar>
      </div>
    </n-card>

    <!-- 右：命令下发与投递诊断 -->
    <n-card size="small" :bordered="true" class="rounded-8px" :title="$t('page.deviceDebug.commandPanel')">
      <n-form label-placement="top">
        <n-form-item :label="$t('page.deviceDebug.identify')">
          <n-input v-model:value="identify" placeholder="set_speed" />
        </n-form-item>
        <n-form-item :label="$t('page.deviceDebug.params')">
          <n-input v-model:value="paramsText" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" />
        </n-form-item>
        <n-space justify="end">
          <n-button size="small" @click="loadDiagnostics" :loading="diagnosticsLoading">
            {{ $t('page.deviceDebug.refreshDiagnostics') }}
          </n-button>
          <n-button size="small" type="primary" :loading="sending" @click="handleSendCommand">
            {{ $t('page.deviceDebug.send') }}
          </n-button>
        </n-space>
      </n-form>

      <n-alert
        v-if="lastSendResult"
        :type="lastSendResult === 'success' ? 'success' : 'error'"
        class="mt-2"
        :show-icon="true"
      >
        {{ lastSendResult === 'success' ? $t('page.deviceDebug.commandSent') : $t('page.deviceDebug.commandFailed') }}
      </n-alert>

      <template v-if="diagnostics">
        <n-descriptions :column="2" size="small" bordered class="mt-3">
          <n-descriptions-item :label="$t('page.deviceDebug.isOnline')">
            <n-tag :type="diagnostics.is_online ? 'success' : 'error'" size="small">
              {{ diagnostics.is_online ? 'online' : 'offline' }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="$t('page.deviceDebug.evaluatedAt')">
            {{ diagnostics.evaluated_at }}
          </n-descriptions-item>
        </n-descriptions>
        <n-descriptions v-if="diagnostics.latest_log" :column="1" size="small" bordered class="mt-2">
          <n-descriptions-item :label="$t('page.deviceDebug.latestLog')">
            {{ diagnostics.latest_log.identify }} / {{ diagnostics.latest_log.message_id }}
          </n-descriptions-item>
        </n-descriptions>
      </template>
    </n-card>
  </div>
</template>

<style scoped>
.debug-frames {
  max-height: 420px;
}
.debug-frame {
  margin-bottom: 8px;
  padding: 6px 8px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
.debug-frame-raw {
  margin: 4px 0 0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
