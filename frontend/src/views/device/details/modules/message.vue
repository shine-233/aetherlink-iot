<!--
设备附加信息与地理位置面板，负责维护设备实例级经纬度与 additional_info 扩展字段。
核心链路：读取设备详情中的 location 和 additional_info -> 如存在 deviceConfigId 再读取设备配置的扩展字段 schema -> 按 schema 类型归一历史值并渲染表单 -> 保存时把经纬度与扩展字段统一回写到设备位置接口。
静态维护重点：
1. 这里编辑的是设备实例级数据，而扩展字段定义来自设备配置；实例值与配置默认值是两套来源，后续改动不要混淆。
2. `additional_info` 需要兼容对象、数组、`extendedInfo` 包装三种历史形态，若后续清理结构，必须先设计迁移与回填策略。
3. 当前没有独立 loading 和错误态区分，详情读取、模板读取、保存失败都依赖通用提示，弱网下不容易定位具体失败阶段。
4. 保存时会整体重组扩展字段对象并直接提交，后端若新增其他附加字段结构，这里需要同步补更明确的合并策略。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, getCurrentInstance, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import type { FormInst } from 'naive-ui'
import { NButton, NSpace, useMessage, NInputNumber, NTooltip, NInput, NSelect, NSwitch } from 'naive-ui'
import { deviceConfigInfo, deviceDetail, deviceLocation } from '@/service/api'
import { $t } from '@/locales'
import { getCoordinateStringValidationError } from '@/utils/common/map-validator'

const props = defineProps<{
  id: string
  deviceConfigId: string
}>()

const latitude = ref('')
const longitude = ref('')
const isShow = ref(false)
const additionInfo = ref([] as ExtensionInfo[])
const extensionFormRef = ref<HTMLElement & FormInst>()

interface ExtensionInfo {
  name: string
  type: 'String' | 'Number' | 'Boolean' | 'Enum'
  default_value: string
  value?: string | number | boolean | null
  desc?: string
  enable: boolean
  options?: Array<{ label: string; value: string }>
}

// 设备详情与设备配置的 JSON 字段都可能出现历史脏数据，统一走安全解析避免整页回显中断。
const safeParseJSON = <T,>(payload: string | null | undefined, fallback: T): T => {
  if (!payload) return fallback

  try {
    return JSON.parse(payload) as T
  } catch (error) {
    console.warn('Failed to parse JSON payload:', error)
    return fallback
  }
}

// 兼容 additional_info.extendedInfo 的对象/数组双形态，统一转成 name-value 列表。
const normalizeExtendedInfo = (payload: unknown): Array<{ name: string; value: any }> => {
  if (Array.isArray(payload)) {
    return payload as Array<{ name: string; value: any }>
  }

  if (payload && typeof payload === 'object') {
    return Object.entries(payload as Record<string, any>).map(([name, value]) => ({
      name,
      value
    }))
  }

  return []
}

// 扩展字段值会按 schema.type 归一化成前端表单更容易编辑的值类型，减少字符串历史值带来的控件回显歧义。
const coerceValueByType = (value: unknown, type: ExtensionInfo['type']) => {
  if (value === null || value === undefined || value === '') {
    return undefined
  }

  switch (type) {
    case 'Number': {
      const numberValue = Number(value)
      return Number.isNaN(numberValue) ? undefined : numberValue
    }
    case 'Boolean': {
      if (typeof value === 'boolean') return value
      if (value === 'true' || value === 'false') {
        return value === 'true'
      }
      return Boolean(value)
    }
    default:
      return String(value)
  }
}

const getTextValue = (item: ExtensionInfo) => {
  return item.value === null || item.value === undefined ? '' : String(item.value)
}

const getNumberValue = (item: ExtensionInfo) => {
  if (item.value === null || item.value === undefined || item.value === '') return null
  const numericValue = Number(item.value)
  return Number.isFinite(numericValue) ? numericValue : null
}

const getBooleanValue = (item: ExtensionInfo) => {
  return item.value === true || item.value === 'true' || item.value === 1
}

const setItemValue = (item: ExtensionInfo, value: string | number | boolean | null) => {
  item.value = value
}

const { query } = useRoute()
const message = useMessage()
const TencentMap = defineAsyncComponent(() => import('./public/tencent-map.vue'))

// 保存链路会把页面上编辑过的扩展字段收敛回对象，并与经纬度一起提交，保证这两组附加信息一次落库。
const handleSave = async () => {
  try {
    if (latitude.value && longitude.value) {
      const error = getCoordinateStringValidationError(latitude.value, longitude.value)
      if (error) {
        message.error(`${$t('generate.invalidCoordinates')} ${error}`)
        return
      }
    }

    if (extensionFormRef.value) {
      await extensionFormRef.value.validate()
    }

    const extentedInfoObject = additionInfo.value.reduce<Record<string, string | number | boolean | null | undefined>>(
      (acc, item) => {
        acc[item.name] = item.value
        return acc
      },
      {}
    )

    const res = await deviceLocation({
      id: props.id,
      location: `${longitude.value},${latitude.value}`,
      additional_info: JSON.stringify(extentedInfoObject)
    })

    if (!res.error) {
      message.success($t('common.modifySuccess'))
    }
  } catch {
    message.error($t('common.saveFailed'))
  }
}

// 地图选点后直接回填文本框，用户仍可在保存前继续手动微调坐标。
const onPositionSelected = (position: { lat: number; lng: number }) => {
  latitude.value = position.lat.toString()
  longitude.value = position.lng.toString()
  isShow.value = false
}

// 打开地图前先校验当前坐标是否可解析，避免把脏坐标带入选点弹窗。
const openMapAndGetPosition = () => {
  if (latitude.value && longitude.value) {
    const error = getCoordinateStringValidationError(latitude.value, longitude.value)
    if (error) {
      window.$message?.error(`${$t('generate.currentCoordinatesInvalid')} ${error}`)
      return
    }
  }

  isShow.value = true
}

// 首屏初始化链路:
// 1. 读取设备详情，回填 location 与设备已保存的 additional_info。
// 2. 若设备绑定了设备配置，再读取配置里的扩展字段 schema。
// 3. 采用“设备已存值优先，模板默认值兜底”的策略生成最终表单状态。
const getConfigInfo = async () => {
  const configInfoPromise = props.deviceConfigId ? deviceConfigInfo({ id: props.deviceConfigId }) : Promise.resolve(null)
  const [result, resultData] = await Promise.all([deviceDetail(query.d_id as string), configInfoPromise])
  const location = result?.data?.location || ''
  const deviceAdditionalInfo = safeParseJSON<Record<string, any>>(result?.data?.additional_info, {})
  const locationData = location?.split(',') || []
  latitude.value = locationData[1] || ''
  longitude.value = locationData[0] || ''

  if (resultData) {
    const parsedAdditionalInfo = safeParseJSON<ExtensionInfo[]>(resultData?.data?.additional_info, [])
    const extendedInfoCandidates = deviceAdditionalInfo?.extendedInfo ?? deviceAdditionalInfo ?? []
    const extendedInfo = normalizeExtendedInfo(extendedInfoCandidates)
    const extendedInfoMap = new Map(extendedInfo.map(info => [info.name, info.value]))

    additionInfo.value = parsedAdditionalInfo.map(item => {
      const resolvedValue = extendedInfoMap.has(item.name) ? extendedInfoMap.get(item.name) : item.default_value

      return {
        ...item,
        value: coerceValueByType(resolvedValue, item.type),
        options: item.options || []
      }
    })
  }
}

// 地图弹窗宽度依赖宿主平台能力，避免嵌入端与桌面端共用固定尺寸时出现遮挡。
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})

onMounted(getConfigInfo)
</script>

<template>
  <div>
    <NCard :title="$t('generate.device-location')" class="mb-4">
      <NSpace>
        <NInput v-model:value="longitude" :placeholder="$t('generate.longitude')" class="w-140px" />
        <NInput v-model:value="latitude" :placeholder="$t('generate.latitude')" class="w-140px" />

        <NButton type="primary" @click="openMapAndGetPosition">{{ $t('generate.location') }}</NButton>
      </NSpace>
    </NCard>

    <NCard :title="$t('generate.extension-info')" class="mb-4">
      <div v-if="additionInfo.filter(item => item.enable === true).length > 0">
        <NForm ref="extensionFormRef" class="mt-4">
          <div class="space-y-4">
            <div
              v-for="item in additionInfo.filter(item => item.enable === true)"
              :key="item.name"
              class="flex items-center gap-3"
            >
              <div class="w-40 text-sm font-medium text-gray-700 flex-shrink-0 flex items-center gap-1">
                <span class="truncate" :title="item.name">{{ item.name }}</span>
                <NTooltip trigger="hover">
                  <template #trigger>
                    <SvgIcon icon="mdi:help-circle" class="text-14px text-gray-400 cursor-help" />
                  </template>
                  <div class="max-w-xs">
                    <div class="text-sm font-medium mb-1">{{ $t('generate.extensionFieldName') }}: {{ item.name }}</div>

                    <div class="text-sm font-medium mb-1">{{ $t('generate.extensionFieldType') }}: {{ item.type }}</div>

                    <div class="text-sm mb-1">{{ $t('generate.extensionFieldDefault') }}: {{ item.default_value }}</div>

                    <div class="text-sm text-gray-600">{{ item.desc || $t('generate.extensionNoDesc') }}</div>
                  </div>
                </NTooltip>
              </div>

              <div class="flex-1">
                <NInput
                  v-if="item.type === 'String'"
                  :value="getTextValue(item)"
                  :placeholder="`${$t('generate.extensionPlaceholderDefault')} ${item.default_value || ''}`"
                  @update:value="value => setItemValue(item, value)"
                />
                <NInputNumber
                  v-else-if="item.type === 'Number'"
                  :value="getNumberValue(item)"
                  :placeholder="`${$t('generate.extensionPlaceholderDefault')} ${item.default_value || ''}`"
                  class="w-full"
                  @update:value="value => setItemValue(item, value)"
                />
                <NSwitch
                  v-else-if="item.type === 'Boolean'"
                  :value="getBooleanValue(item)"
                  :checked-value="true"
                  :unchecked-value="false"
                  @update:value="value => setItemValue(item, value)"
                />
                <NSelect
                  v-else-if="item.type === 'Enum'"
                  :value="getTextValue(item)"
                  :options="item.options || []"
                  :placeholder="`${$t('generate.extensionPlaceholderDefault')} ${item.default_value || ''}`"
                  @update:value="value => setItemValue(item, value as string)"
                />
                <NInput
                  v-else
                  :value="getTextValue(item)"
                  :placeholder="`${$t('generate.extensionPlaceholderDefault')} ${item.default_value || ''}`"
                  @update:value="value => setItemValue(item, value)"
                />
              </div>
            </div>
          </div>
        </NForm>
      </div>

      <div v-else class="text-center text-gray-400 py-8">
        {{ $t('common.noData') }}
      </div>
    </NCard>

    <NButton type="primary" @click="handleSave">{{ $t('common.save') }}</NButton>
    <NModal v-model:show="isShow" class="flex-center" :class="getPlatform ? 'max-w-90%' : 'max-w-720px'">
      <NCard class="flex flex-1">
        <TencentMap
          v-show="isShow"
          class="flex-1 h-440px w-680px"
          :longitude="longitude"
          :latitude="latitude"
          @position-selected="onPositionSelected"
        />
      </NCard>
    </NModal>
  </div>
</template>
