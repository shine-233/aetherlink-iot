<!--
文件用途: 承载场景联动卡片列表（普通场景 / 设备级告警规则双形态）。
核心逻辑: 分页拉取场景列表（isAlarm 时走设备告警列表接口），卡片展示名称/描述/启停开关，
  编辑/删除/日志入口；列表状态（分页、加载、过期请求）收口在 useListPage，
  执行日志弹窗拆到 scene-log-modal.vue 独立维护。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
-->
<script lang="tsx" setup>
import { computed, ref } from 'vue'
import { NButton, NCard, NFlex, NGrid, NGridItem, NPagination } from 'naive-ui'
import { PencilOutline as editIcon, TrashOutline as trashIcon, DocumentTextOutline } from '@vicons/ionicons5'
import { useRouterPush } from '@/hooks/common/router'
import ItemCard from '@/components/dev-card-item/index.vue'
import {
  sceneAutomationsDel,
  sceneAutomationsGet,
  sceneAutomationsSwitch
} from '@/service/api/automation'
import { $t } from '@/locales'
import { deviceAlarmList } from '@/service/api'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import SceneLogModal from './scene-log-modal.vue'
import { useDialog } from 'naive-ui'

const dialog = useDialog()
const { routerPushByKey } = useRouterPush()

interface Props {
  deviceId?: string
  deviceConfigId?: string
  isAlarm?: boolean
  backType?: string
  onboarding?: string
  starter?: string
  firstDeviceName?: string
  firstDeviceNumber?: string
  telemetryKey?: string
  telemetryValue?: string
  telemetryAt?: string
}

const props = withDefaults(defineProps<Props>(), {
  deviceId: '',
  deviceConfigId: '',
  isAlarm: false,
  backType: 'automation',
  onboarding: '',
  starter: '',
  firstDeviceName: '',
  firstDeviceNumber: '',
  telemetryKey: '',
  telemetryValue: '',
  telemetryAt: ''
})

// 新建场景
const isDeviceAutomationStarter = computed(
  () =>
    !props.isAlarm &&
    (Boolean(props.deviceId) || props.onboarding === 'first-device' || props.starter === 'first-telemetry-rule')
)

const buildLinkAddQuery = () => {
  const query: Record<string, string> = {
    device_id: props.deviceId,
    device_config_id: props.deviceConfigId,
    backType: props.backType
  }

  if (isDeviceAutomationStarter.value) {
    query.onboarding = props.onboarding || 'first-device'
    query.starter = 'first-telemetry-rule'
    if (props.firstDeviceName) query.first_device_name = props.firstDeviceName
    if (props.firstDeviceNumber) query.first_device_number = props.firstDeviceNumber
    if (props.telemetryKey) query.telemetry_key = props.telemetryKey
    if (props.telemetryValue) query.telemetry_value = props.telemetryValue
    if (props.telemetryAt) query.telemetry_at = props.telemetryAt
  }

  return query
}

const linkAdd = () => {
  routerPushByKey('automation_linkage-edit', {
    query: buildLinkAddQuery()
  })
}

// 编辑场景
const linkEdit = (item: any) => {
  routerPushByKey('automation_linkage-edit', {
    query: {
      id: item.id,
      backType: props.backType,
      device_id: props.deviceId,
      device_config_id: props.deviceConfigId
    }
  })
}

// 开启/关闭场景
const linkActivation = async (item: any) => {
  const res = await sceneAutomationsSwitch(item.id)
  if (!res.error) {
    await getData()
  }
}

// 列表查询主入口：分页、加载态与过期请求丢弃交给 useListPage。
// 卡片开关通过 v-model 原地改写 item.enabled，因此使用深响应行。
const {
  query: queryData,
  rows: sceneLinkageList,
  total: dataTotal,
  pagination,
  load: getData,
  search: handleQuery,
  setPage
} = useListPage<any, { name: string; device_id: string; device_config_id: string }>({
  initialQuery: () => ({
    name: '',
    device_id: props.deviceId,
    device_config_id: props.deviceConfigId
  }),
  initialPageSize: 12,
  deepRows: true,
  fetcher: async (params) => {
    const response = props.isAlarm ? await deviceAlarmList(params) : await sceneAutomationsGet(params)
    return fromFlatResponse<Record<string, any>>(response)
  }
})

// 查看日志
const showLog = ref(false)
const logTargetId = ref('')

const openLog = (item: any) => {
  logTargetId.value = item.id
  showLog.value = true
}

// 删除场景
const deleteLink = async (item: any) => {
  dialog.warning({
    title: $t('common.deletePrompt'),
    content: $t('common.sceneLinkageInfo'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      const res = await sceneAutomationsDel(item.id)
      if (!res.error) {
        await getData()
      }
    }
  })
}

void getData()
</script>

<template>
  <NCard class="w-full">
    <NFlex v-if="!isAlarm" justify="space-between" class="mb-4">
      <NButton type="primary" @click="linkAdd()">{{ $t('generate.+add-scene-linkage') }}</NButton>
      <NFlex align="center" justify="flex-end" :wrap="false">
        <NInput
          v-model:value="queryData.name"
          :placeholder="$t('generate.enter-scene-linkage-name')"
          class="search-input"
          type="text"
          clearable
        ></NInput>
        <NButton class="w-72px" type="primary" @click="handleQuery">{{ $t('common.search') }}</NButton>
      </NFlex>
    </NFlex>
    <n-empty v-if="sceneLinkageList.length === 0" size="huge" class="min-h-60 justify-center">
      <template #default>
        <div class="automation-empty">
          <div class="automation-empty__title">
            {{ isDeviceAutomationStarter ? $t('custom.automation.firstTelemetryRuleEmptyTitle') : $t('common.noData') }}
          </div>
          <div v-if="isDeviceAutomationStarter" class="automation-empty__desc">
            {{ $t('custom.automation.firstTelemetryRuleEmptyDesc') }}
          </div>
          <NButton v-if="isDeviceAutomationStarter" type="primary" @click="linkAdd">
            {{ $t('custom.automation.createFirstTelemetryRule') }}
          </NButton>
        </div>
      </template>
    </n-empty>
    <NGrid v-else x-gap="20px" y-gap="20px" cols="1 s:2 m:3 l:4" responsive="screen">
      <NGridItem v-for="(item, index) in sceneLinkageList" :key="index">
        <ItemCard
          :title="item.name"
          :status-active="true"
          :status-type="'success'"
          :is-status="false"
          :hide-footer-left="true"
          hoverable
        >
          <template #default>{{ item.description }}</template>
          <!-- 右上角开关 -->
          <template #top-right-icon>
            <n-switch
              v-model:value="item.enabled"
              checked-value="Y"
              unchecked-value="N"
              @update-value="() => linkActivation(item)"
            />
          </template>

          <!-- 底部操作按钮 -->
          <template #footer>
            <div class="flex items-center gap-2 w-full justify-between">
              <NTooltip trigger="hover">
                <template #trigger>
                  <NButton size="small" quaternary circle :aria-label="$t('common.edit')" @click="linkEdit(item)">
                    <template #icon>
                      <n-icon color="#888">
                        <editIcon />
                      </n-icon>
                    </template>
                  </NButton>
                </template>
                {{ $t('common.edit') }}
              </NTooltip>
              <NTooltip trigger="hover">
                <template #trigger>
                  <NButton size="small" quaternary circle :aria-label="$t('generate.log')" @click="openLog(item)">
                    <template #icon>
                      <n-icon color="#888">
                        <DocumentTextOutline />
                      </n-icon>
                    </template>
                  </NButton>
                </template>
                {{ $t('generate.log') }}
              </NTooltip>
              <NTooltip trigger="hover">
                <template #trigger>
                  <NButton size="small" quaternary circle :aria-label="$t('common.delete')" @click="deleteLink(item)">
                    <template #icon>
                      <n-icon color="#888">
                        <trashIcon />
                      </n-icon>
                    </template>
                  </NButton>
                </template>
                {{ $t('common.delete') }}
              </NTooltip>
            </div>
          </template>
        </ItemCard>
      </NGridItem>
    </NGrid>
    <NFlex justify="flex-end" class="mt-4">
      <NPagination
        :page="pagination.page"
        :page-size="pagination.pageSize"
        :item-count="dataTotal"
        @update:page="setPage"
      />
    </NFlex>
  </NCard>
  <SceneLogModal v-model:show="showLog" :scene-automation-id="logTargetId" />
</template>

<style scoped lang="scss">
.search-input {
  width: 200px;
}

.automation-empty {
  display: grid;
  justify-items: center;
  gap: 10px;
  max-width: 520px;
  text-align: center;
}

.automation-empty__title {
  font-size: 16px;
  font-weight: 700;
  color: #1f2937;
}

.automation-empty__desc {
  color: #4b5563;
  line-height: 1.55;
}
</style>
