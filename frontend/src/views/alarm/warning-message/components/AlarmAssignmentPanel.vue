<!--
  文件用途：告警指派面板（ROADMAP TB-1 第二片：指派历史审计）。
  核心逻辑：按告警历史 ID 拉取指派流水、展示当前处理人、提交指派/改派/取消指派，并展示历史流水。
  关键注意事项：
    1. 后端流水是 append-only 审计记录：改派与取消都走同一个 POST /assignment，
       没有更新/删除接口；assignee_user_id 传 null 就是"取消指派"。
    2. 当前处理人 = 最新一条流水的 assignee_user_id（后端保证 created_at 倒序，取第 0 条）；
       为 null 或流水为空即"当前无人指派"。后端若改成正序，这里会取错人。
    3. 选人用仓库既有的 GET /user/selector（@/service/api/notification 的 getUserList），
       没有另造一份假用户列表；租户用户多时靠 name 模糊搜索。
    4. 换告警（alarmHistoryId 变化）必须清空上一次的流水与草稿，
       否则会把 A 告警的处理人显示在 B 告警下面。
    5. 不新增样式块：排版复用仓库已有的 unocss 工具类与 .alarm-resolution-card 体系。
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NEmpty, NInput, NSelect, NSpin, useMessage } from 'naive-ui'
import { assignAlarm, listAlarmAssignments, type AlarmAssignment } from '@/service/api/alarm'
import { getUserList } from '@/service/api/notification'
import { $t } from '@/locales'

defineOptions({ name: 'AlarmAssignmentPanel' })

const props = defineProps<{ alarmHistoryId: string }>()

const message = useMessage()

const records = ref<AlarmAssignment[]>([])
const loading = ref(false)
const submitting = ref(false)
const loadError = ref('')

const selectedAssignee = ref<string | null>(null)
const remark = ref('')
const userOptions = ref<{ label: string; value: string }[]>([])
const usersLoading = ref(false)

/** 当前处理人：最新一条流水的被指派人，null 表示无人指派。 */
const currentAssignee = computed<string | null>(() => records.value[0]?.assignee_user_id ?? null)

const canSubmit = computed(() => Boolean(selectedAssignee.value) && !submitting.value)
const canUnassign = computed(() => Boolean(currentAssignee.value) && !submitting.value)

function formatTime(value: string): string {
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function assigneeLabel(assigneeUserId: string | null): string {
  return assigneeUserId || $t('custom.alarmAssignment.unassigned')
}

async function loadUsers(name = '') {
  usersLoading.value = true
  try {
    const { data, error } = await getUserList({ page: 1, page_size: 20, name })
    if (error) {
      message.error($t('custom.alarmAssignment.usersLoadFailed'))
      return
    }
    const list = (data as { list?: { user_id?: string; name?: string }[] } | undefined)?.list ?? []
    userOptions.value = list
      .filter((item): item is { user_id: string; name?: string } => Boolean(item?.user_id))
      .map((item) => ({ label: item.name || item.user_id, value: item.user_id }))
  } finally {
    usersLoading.value = false
  }
}

function onSearch(query: string) {
  void loadUsers(query || '')
}

async function load() {
  if (!props.alarmHistoryId) {
    records.value = []
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const { data, error } = await listAlarmAssignments(props.alarmHistoryId)
    if (error) {
      loadError.value = $t('custom.alarmAssignment.loadFailed')
      records.value = []
      return
    }
    const payload = data as { list?: AlarmAssignment[] } | undefined
    records.value = Array.isArray(payload?.list) ? payload!.list! : []
  } finally {
    loading.value = false
  }
}

/** 指派、改派、取消指派都写同一条 append-only 流水，只有 assignee 不同。 */
async function submit(assigneeUserId: string | null) {
  if (submitting.value) return

  submitting.value = true
  try {
    const { error } = await assignAlarm(props.alarmHistoryId, {
      assignee_user_id: assigneeUserId,
      remark: remark.value.trim()
    })
    if (error) {
      message.error($t('custom.alarmAssignment.submitFailed'))
      return
    }
    remark.value = ''
    await load()
  } finally {
    submitting.value = false
  }
}

watch(
  () => props.alarmHistoryId,
  () => {
    records.value = []
    remark.value = ''
    selectedAssignee.value = null
    loadError.value = ''
    void load()
  },
  { immediate: true }
)

void loadUsers()
</script>

<template>
  <div class="alarm-assignment-panel">
    <n-spin :show="loading">
      <div class="flex items-center gap-8px text-13px">
        <span class="font-600">{{ $t('custom.alarmAssignment.currentAssignee') }}:</span>
        <span data-testid="alarm-current-assignee">{{ assigneeLabel(currentAssignee) }}</span>
      </div>

      <n-empty
        v-if="!records.length && !loading"
        :description="loadError || $t('custom.alarmAssignment.empty')"
        class="py-4"
      />
      <ul v-else class="m-0 mt-8px max-h-260px list-none overflow-y-auto p-0">
        <li v-for="record in records" :key="record.id" class="py-8px">
          <div class="flex items-center gap-8px text-12px opacity-72">
            <span>{{ formatTime(record.created_at) }}</span>
            <span>{{ $t('custom.alarmAssignment.operator') }}: {{ record.operator_user_id }}</span>
          </div>
          <div class="mt-4px whitespace-pre-wrap break-words text-13px">
            {{ assigneeLabel(record.assignee_user_id) }}
            <template v-if="record.remark">· {{ record.remark }}</template>
          </div>
        </li>
      </ul>
    </n-spin>

    <div class="mt-12px flex flex-col gap-8px">
      <n-select
        v-model:value="selectedAssignee"
        filterable
        clearable
        :options="userOptions"
        :loading="usersLoading"
        :placeholder="$t('custom.alarmAssignment.selectAssignee')"
        @search="onSearch"
      />
      <n-input
        v-model:value="remark"
        type="textarea"
        :rows="2"
        :maxlength="500"
        :placeholder="$t('custom.alarmAssignment.remarkPlaceholder')"
      />
      <div class="flex justify-end gap-8px">
        <n-button size="small" :disabled="!canUnassign" @click="submit(null)">
          {{ $t('custom.alarmAssignment.unassign') }}
        </n-button>
        <n-button
          type="primary"
          size="small"
          :disabled="!canSubmit"
          :loading="submitting"
          @click="submit(selectedAssignee)"
        >
          {{ $t('custom.alarmAssignment.submit') }}
        </n-button>
      </div>
    </div>
  </div>
</template>
