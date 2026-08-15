<!--
物模型市场详情抽屉，负责展示市场中的单个物模型详情、版本历史与安装入口。
核心链路：监听物模型 ID 与抽屉显隐 -> 拉取详情 -> 展示封面、品牌、型号、版本历史和标签 -> 从抽屉底部触发安装。
静态维护重点：
1. 当前同时监听 `templateId` 和 `visible` 两个源，会在打开时重复触发详情请求，后续可收敛成单一加载入口。
2. 详情数据结构依赖市场接口返回，若后端字段改名，封面、作者、版本历史和标签区域都会同步受影响。
3. 抽屉只负责展示与触发安装，不在内部处理登录态；登录校验仍由上层列表组件统一协调。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { NAlert, NDrawer, NDrawerContent, NButton, NDescriptions, NDescriptionsItem, NTag, NSpace, NSpin } from 'naive-ui'
import { $t } from '@/locales'
import { getMarketTemplateDetail } from '@/service/api/market'
import defaultCover from '@/assets/imgs/default_template_cover.png'

const props = defineProps<{
  visible: boolean
  templateId: string
  installing?: boolean
}>()

const emit = defineEmits(['update:visible', 'install'])

const loading = ref(false)
const detail = ref<any>(null)
const detailError = ref('')
const loadedTemplateId = ref('')
let detailRequestSeq = 0

const getDetailErrorMessage = (error: any) => error?.msg || error?.message || $t('market.detailLoadFailed')

const loadTemplateDetail = async (id: string) => {
  if (!id) return

  const requestSeq = ++detailRequestSeq
  loading.value = true
  detail.value = null
  detailError.value = ''
  loadedTemplateId.value = ''
  try {
    const res: any = await getMarketTemplateDetail(id)
    if (requestSeq === detailRequestSeq && res && !res.error) {
      detail.value = res.data
      loadedTemplateId.value = id
    } else if (requestSeq === detailRequestSeq) {
      detailError.value = getDetailErrorMessage(res?.error)
    }
  } catch (e) {
    console.error(e)
    if (requestSeq === detailRequestSeq) {
      detailError.value = getDetailErrorMessage(e)
    }
  } finally {
    if (requestSeq === detailRequestSeq) {
      loading.value = false
    }
  }
}

// 抽屉打开或模板切换时统一拉详情，避免 templateId/visible 双 watcher 在同一轮重复请求。
watch(
  () => [props.visible, props.templateId] as const,
  ([visible, templateId]) => {
    if (visible && templateId) {
      loadTemplateDetail(templateId)
    }
  },
  { immediate: true }
)

// 抽屉关闭只回传显隐状态，不主动清空 detail，便于再次打开时减少闪烁。
const handleClose = () => {
  emit('update:visible', false)
}

const canInstallCurrentTemplate = () => Boolean(detail.value && loadedTemplateId.value === props.templateId && !loading.value)
</script>

<template>
  <NDrawer :show="visible" :width="480" @update:show="handleClose">
    <NDrawerContent :title="$t('market.templateDetail')" closable>
      <NSpin :show="loading">
        <template v-if="detail">
          <!-- 封面 -->
          <div class="drawer-cover">
            <img v-if="detail.cover_url" :src="detail.cover_url" :alt="detail.name" />
            <img v-else :src="defaultCover" :alt="detail.name" class="opacity-60" />
          </div>

          <!-- 基本信息 -->
          <NDescriptions :column="1" label-placement="left" bordered size="small" class="mb-4">
            <NDescriptionsItem :label="$t('device_template.templateName')">{{ detail.name }}</NDescriptionsItem>
            <NDescriptionsItem :label="$t('device_template.brand')">{{ detail.brand || '--' }}</NDescriptionsItem>
            <NDescriptionsItem :label="$t('device_template.modelNumber')">{{ detail.model || '--' }}</NDescriptionsItem>
            <NDescriptionsItem :label="$t('device_template.author')">
              {{ detail.author_name || detail.author_id || '--' }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('device_template.version')">
              {{ detail.latest_version || '--' }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('market.installCount')">{{ detail.install_count || 0 }}</NDescriptionsItem>
          </NDescriptions>

          <!-- 描述 -->
          <div v-if="detail.description" class="section">
            <h4>{{ $t('generate.description') }}</h4>
            <p class="desc-text">{{ detail.description }}</p>
          </div>

          <!-- 版本历史 -->
          <div v-if="detail.versions && detail.versions.length" class="section">
            <h4>{{ $t('market.versionHistory') }}</h4>
            <div v-for="v in detail.versions" :key="v.version" class="version-item">
              <NSpace align="center" :size="8">
                <NTag size="small" type="primary">v{{ v.version }}</NTag>
                <span class="version-date">
                  {{ v.created_at ? new Date(v.created_at).toLocaleDateString('zh-CN') : '' }}
                </span>
              </NSpace>
              <p v-if="v.changelog" class="version-changelog">{{ v.changelog }}</p>
            </div>
          </div>

          <!-- 标签 -->
          <div v-if="detail.tags && detail.tags.length" class="section">
            <NSpace :size="6">
              <NTag v-for="tag in detail.tags" :key="tag" size="small">{{ tag }}</NTag>
            </NSpace>
          </div>
        </template>
        <div v-else-if="!loading && detailError" class="detail-error">
          <NAlert type="error" :show-icon="false">
            <template #header>{{ $t('market.detailLoadFailed') }}</template>
            <div class="detail-error-content">
              <span>{{ detailError }}</span>
              <NButton size="small" secondary @click="loadTemplateDetail(templateId)">
                {{ $t('market.retry') }}
              </NButton>
            </div>
          </NAlert>
        </div>
      </NSpin>

      <template #footer>
        <NButton
          type="primary"
          block
          :loading="installing"
          :disabled="installing || !canInstallCurrentTemplate()"
          @click="emit('install', templateId)"
        >
          {{ $t('market.install') }}
        </NButton>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped lang="scss">
.drawer-cover {
  margin-bottom: 16px;
  border-radius: 8px;
  overflow: hidden;
  img {
    width: 100%;
    max-height: 200px;
    object-fit: cover;
  }
}

.section {
  margin-bottom: 16px;
  h4 {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 8px;
    color: #333;
  }
}

.desc-text {
  color: #666;
  font-size: 13px;
  line-height: 1.6;
}

.version-item {
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
  &:last-child {
    border-bottom: none;
  }
}

.version-date {
  font-size: 12px;
  color: #999;
}

.version-changelog {
  margin: 4px 0 0;
  font-size: 12px;
  color: #666;
}

.detail-error {
  padding: 24px 0;
}

.detail-error-content {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
</style>
