<!--
  DeviceManageCardView: the device card grid ("card" view) for the device-manage page.

  Extracted from the shared <data-table-page> wrapper when the page moved onto `useListPage`.
  Renders the DevCardItem grid (device icon, warning bell, config image) and shows the
  parent-provided `empty` slot while rows are empty and not loading.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NGrid, NGridItem, NScrollbar, NSpin } from 'naive-ui'
import { formatDateTime } from '@/utils/common/datetime'
import { getPlatformApiBaseUrl } from '@/utils/common/tool'
import DevCardItem from '@/components/dev-card-item/index.vue'
import SvgIcon from '@/components/custom/svg-icon.vue'
import type { DeviceManageRow } from './useDeviceManageListPage'

const props = defineProps<{
  rows: DeviceManageRow[]
  loading?: boolean
  rowClick?: (row: DeviceManageRow) => void
}>()

const showEmpty = computed(() => !props.loading && props.rows.length === 0)

const router = useRouter()

// 设备类型到图标名称的映射 (使用项目标准图标系统)
const deviceTypeIcons = {
  '1': 'direct', // 直连设备图标
  '2': 'gateway', // 网关图标
  '3': 'subdevice', // 网关子设备图标
  default: 'defaultdevice' // 默认设备图标
}

// 获取设备图标名称的函数，针对"默认配置"使用直连设备图标
const getDeviceIconName = (deviceType: string, deviceConfigName?: string): string => {
  // 当配置是默认配置时，强制使用直连设备图标
  if (!deviceConfigName || deviceConfigName === '默认配置') {
    return deviceTypeIcons['1'] // 直连设备图标
  }
  return deviceTypeIcons[deviceType] || deviceTypeIcons.default
}

// 获取配置图片URL的函数
const platformAssetBaseUrl = ref(getPlatformApiBaseUrl())
const getConfigImageUrl = (imagePath: string | undefined): string => {
  if (!imagePath) return '' // 返回空字符串，让模板使用默认图标
  const relativePath = imagePath.replace(/^\.?\//, '')
  return `${platformAssetBaseUrl.value.replace('api/v1', '') + relativePath}`
}

// 处理告警铃铛图标点击事件
const handleWarningClick = (item: DeviceManageRow) => {
  if (item.warn_status === 'Y') {
    // 有告警时跳转到具体设备的告警详情
    router.push(`/alarm/warning-message?device_id=${item.id}`)
  } else {
    // 无告警时可能跳转到告警管理页面
    router.push('/alarm/warning-message')
  }
}
</script>

<template>
  <n-scrollbar style="height: calc(100vh - 442px)" :size="1">
    <n-spin :show="loading">
      <slot v-if="showEmpty" name="empty" />
      <NGrid v-else x-gap="20px" y-gap="20px" cols="1 s:2 m:3 l:4" responsive="screen">
        <NGridItem v-for="item in rows" :key="item.id">
          <DevCardItem
            :title="item.name || 'N/A'"
            :status-active="item.is_online === 1"
            :subtitle="item.device_config_name || '--'"
            :footer-text="(item.ts ? formatDateTime(item.ts) : null) ?? '--'"
            :warn-status="item.warn_status"
            :device-id="item.id"
            @click-card="() => rowClick && rowClick(item)"
            @click-top-right-icon="handleWarningClick(item)"
          >
            <template #subtitle-icon>
              <SvgIcon :local-icon="getDeviceIconName(item.device_type, item.device_config_name)" class="image-icon" />
            </template>

            <!-- 右上角铃铛图标插槽 -->
            <template #top-right-icon>
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                :fill="item.warn_status === 'Y' ? '#ff4d4f' : '#d9d9d9'"
                class="bell-icon"
              >
                <!-- 铃铛图标 SVG 路径 -->
                <path
                  d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.89 2 2 2zm6-6v-5c0-3.07-1.64-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.63 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z"
                />
              </svg>
            </template>
            <template #footer-icon>
              <div class="footer-icon-container">
                <img
                  v-if="item.image_url"
                  :src="getConfigImageUrl(item.image_url)"
                  alt="config image"
                  loading="lazy"
                  decoding="async"
                  class="config-image"
                />
                <SvgIcon v-else local-icon="defaultdevice" class="config-image" />
              </div>
            </template>
          </DevCardItem>
        </NGridItem>
      </NGrid>
    </n-spin>
  </n-scrollbar>
</template>

<style scoped lang="scss">
.image-icon {
  max-width: 100%;
  max-height: 100%;
  width: 24px;
  height: 24px;
  object-fit: contain;
}

.bell-icon {
  transition: fill 0.3s ease;
}

// 底部图标容器 - 固定40x40正方形
.footer-icon-container {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 6px;
  background-color: #f8f9fa;
  border: 1px solid #e9ecef;
}

.config-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
}
</style>
