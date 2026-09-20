<!--
  文件用途：告警评论面板（ROADMAP TB-1 第一片）。
  核心逻辑：按告警历史 ID 拉取评论、按对话顺序展示、提交新评论、删除自己的评论。
  关键注意事项：
    1. 评论挂 alarm_history，不是已废弃的 alarm_info；组件只认 alarmHistoryId。
    2. 删除入口只对自己发的评论显示 —— 后端虽然允许租户管理员删他人评论，
       但前端不该把"管理员能删别人的话"做成默认可见的操作，避免误触。
    3. 提交空/纯空白内容在本地就拦下，不浪费一次往返；后端也会独立校验。
    4. 换告警（alarmHistoryId 变化）必须清空上一次的列表与草稿，
       否则会把 A 告警的评论显示在 B 告警下面。
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NEmpty, NInput, NSpin, useMessage } from 'naive-ui'
import { createAlarmComment, deleteAlarmComment, listAlarmComments, type AlarmComment } from '@/service/api/alarm'
import { useAuthStore } from '@/store/modules/auth'
import { $t } from '@/locales'

defineOptions({ name: 'AlarmCommentPanel' })

const props = defineProps<{ alarmHistoryId: string }>()

const message = useMessage()
const authStore = useAuthStore()

const comments = ref<AlarmComment[]>([])
const draft = ref('')
const loading = ref(false)
const submitting = ref(false)
const loadError = ref('')

const currentUserId = computed(() => String(authStore.userInfo?.id || ''))
const canSubmit = computed(() => draft.value.trim().length > 0 && !submitting.value)

/** 只有自己的评论显示删除入口。 */
function canDelete(comment: AlarmComment): boolean {
  return Boolean(currentUserId.value) && comment.author_user_id === currentUserId.value
}

function formatTime(value: string): string {
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

async function load() {
  if (!props.alarmHistoryId) {
    comments.value = []
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const { data, error } = await listAlarmComments(props.alarmHistoryId)
    if (error) {
      loadError.value = $t('custom.alarmComment.loadFailed')
      comments.value = []
      return
    }
    const payload = data as { list?: AlarmComment[] } | undefined
    comments.value = Array.isArray(payload?.list) ? payload!.list! : []
  } finally {
    loading.value = false
  }
}

async function submit() {
  const content = draft.value.trim()
  if (!content || submitting.value) return

  submitting.value = true
  try {
    const { error } = await createAlarmComment(props.alarmHistoryId, content)
    if (error) {
      message.error($t('custom.alarmComment.submitFailed'))
      return
    }
    draft.value = ''
    await load()
  } finally {
    submitting.value = false
  }
}

async function remove(comment: AlarmComment) {
  const { error } = await deleteAlarmComment(props.alarmHistoryId, comment.id)
  if (error) {
    message.error($t('custom.alarmComment.deleteFailed'))
    return
  }
  await load()
}

// 换告警时清空：草稿与列表都不能跨告警复用。
watch(
  () => props.alarmHistoryId,
  () => {
    comments.value = []
    draft.value = ''
    loadError.value = ''
    void load()
  },
  { immediate: true }
)
</script>

<template>
  <div class="alarm-comment-panel">
    <n-spin :show="loading">
      <n-empty
        v-if="!comments.length && !loading"
        :description="loadError || $t('custom.alarmComment.empty')"
        class="py-4"
      />
      <ul v-else class="comment-list">
        <li v-for="comment in comments" :key="comment.id" class="comment-item">
          <div class="comment-meta">
            <span class="comment-author">{{ comment.author_user_id }}</span>
            <span class="comment-time">{{ formatTime(comment.created_at) }}</span>
            <n-button v-if="canDelete(comment)" text size="tiny" type="error" @click="remove(comment)">
              {{ $t('common.delete') }}
            </n-button>
          </div>
          <div class="comment-content">{{ comment.content }}</div>
        </li>
      </ul>
    </n-spin>

    <div class="comment-compose">
      <n-input
        v-model:value="draft"
        type="textarea"
        :rows="2"
        :maxlength="2000"
        show-count
        :placeholder="$t('custom.alarmComment.placeholder')"
      />
      <div class="comment-actions">
        <n-button type="primary" size="small" :disabled="!canSubmit" :loading="submitting" @click="submit">
          {{ $t('custom.alarmComment.submit') }}
        </n-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.comment-list {
  margin: 0 0 12px;
  padding: 0;
  list-style: none;
  max-height: 260px;
  overflow-y: auto;
}

.comment-item {
  padding: 8px 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.16);
}

.comment-item:last-child {
  border-bottom: none;
}

.comment-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  opacity: 0.72;
}

.comment-author {
  font-weight: 600;
}

.comment-content {
  margin-top: 4px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 13px;
}

.comment-compose {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.comment-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
