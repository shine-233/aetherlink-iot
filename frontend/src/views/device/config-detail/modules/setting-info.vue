<!--
  文件用途: 设备配置基础设置信息面板。
  核心逻辑: 展示设备配置基础字段、物模型和接入设置等信息。
  关键注意事项: 字段来源混合配置详情与物模型详情，展示空值时需保持可追踪。
  重构建议: 抽出字段展示配置，补不同设备类型和缺物模型测试。
-->
<script setup lang="ts">
import { computed, getCurrentInstance, onMounted, reactive, ref } from 'vue'
import { NButton, useDialog, useMessage } from 'naive-ui'
import { useRoute } from 'vue-router'
import { useTabStore } from '@/store/modules/tab'
import { useRouterPush } from '@/hooks/common/router'
import { deviceConfigDel, deviceConfigEdit } from '@/service/api/device'
import { $t } from '@/locales'
import type { UploadFileInfo } from 'naive-ui'
import { localStg } from '@/utils/storage'
import { getPlatformApiBaseUrl } from '@/utils/common/tool'
import { writeClipboardText } from '@/utils/clipboard'

interface Props {
  configInfo?: object | any
}
const emit = defineEmits(['change'])
const props = withDefaults(defineProps<Props>(), {
  configInfo: null
})
const dialog = useDialog()
const message = useMessage()
const route = useRoute()
const tabStore = useTabStore()
const { routerPushByKey } = useRouterPush()

// 删除链路会直接影响当前详情页可用性，因此成功后需要同步关闭标签并跳回列表页。
const deleteConfig = () => {
  dialog.warning({
    title: $t('common.tip'),
    content: $t('common.deleteDeviceConfig'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      const res: any = await deviceConfigDel({ id: props.configInfo.id })

      if (!res || !res.error) {
        message.success($t('custom.grouping_details.operationSuccess'))
        await tabStore.removeTab(route.path)
        await routerPushByKey('device_config')
      }
    }
  })
}

// showModal / modalIndex 共同描述当前弹窗状态：1 为自动注册配置，2 为在线判定配置。
const showModal = ref(false)
const modalIndex = ref(1)

// auto_register 与 onlinejson 分别对应两类基础设置，保存时根据 modalIndex 分流提交。
const auto_register = ref(props.configInfo?.auto_register === 1 || false)
const onlinejson = reactive({
  online_timeout: 0,
  heartbeat: 0
})

// BUG-25: Mutual exclusion — only one of timeout / heartbeat can be set
const isTimeoutDisabled = computed(() => onlinejson.heartbeat > 0)
const isHeartbeatDisabled = computed(() => onlinejson.online_timeout > 0)

// 图片上传相关
const platformApiBaseUrl = getPlatformApiBaseUrl()
const platformAssetBaseUrl: any = ref(platformApiBaseUrl)
const imagePath: any = ref('')
const customRequest = ({ event }: { file: UploadFileInfo; event?: ProgressEvent }) => {
  if (!event || !event.target) return

  const xhr = event.target as XMLHttpRequest
  const response = JSON.parse(xhr.response)

  // 保存图片路径
  const relativePath = response.data.path.replace(/^\.\//, '')
  imagePath.value = `${platformAssetBaseUrl.value.replace('api/v1', '') + relativePath}`

  // 直接保存图片路径到服务器
  saveImagePath(relativePath)
}

// 图片保存链路：上传接口返回相对路径 -> 本地拼接预览地址 -> 复用 deviceConfigEdit 持久化 image_url。
const saveImagePath = async (relativePath: string) => {
  const { error }: any = await deviceConfigEdit({
    id: props.configInfo.id,
    image_url: relativePath
  })

  if (!error) {
    message.success($t('custom.grouping_details.operationSuccess'))
    emit('change')
  }
}

const onDialogVisble = () => {
  showModal.value = !showModal.value
}

// 打开在线配置弹窗时，从 other_config 回填超时与心跳参数，保证二次编辑可见当前值。
const onOpenDialogModal = (val: number) => {
  modalIndex.value = val
  onDialogVisble()
  if (modalIndex.value !== 1) {
    const { online_timeout, heartbeat }: any = JSON.parse(props.configInfo?.other_config || {})
    onlinejson.online_timeout = online_timeout || 0
    onlinejson.heartbeat = heartbeat || 0
  }
}
const copyOneTypeOneSecretDevicePassword = async () => {
  const textToCopy = props.configInfo?.template_secret || ''

  if (!textToCopy) {
    message.error($t('common.noContentToCopy'))
    return
  }

  if (await writeClipboardText(textToCopy)) {
    message.success($t('custom.grouping_details.operationSuccess'))
  } else {
    message.error($t('common.copyFailed'))
  }
}

const onSubmit = async () => {
  onDialogVisble()
  if (modalIndex.value !== 1) {
    const { error }: any = await deviceConfigEdit({
      id: props.configInfo.id,
      other_config: JSON.stringify({
        online_timeout: onlinejson.online_timeout,
        heartbeat: onlinejson.heartbeat
      })
    })
    !error && emit('change')
  } else {
    const { error }: any = await deviceConfigEdit({
      id: props.configInfo.id,
      auto_register: auto_register.value ? 1 : 0
    })
    message.success($t('custom.grouping_details.operationSuccess'))
    !error && emit('change')
  }
}
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})

// 初始化回填基础状态，只做本地展示同步，不在挂载时主动触发保存。
onMounted(() => {
  auto_register.value = props.configInfo?.auto_register === 1 || false
  // 初始化图片路径
  imagePath.value = props.configInfo?.image_url ? `${platformAssetBaseUrl.value.replace('api/v1', '') + props.configInfo.image_url}` : ''
})
</script>

<template>
  <div class="flex-col gap-30px p-10px">
    <div class="">
      <div class="m-b-10px">{{ $t('generate.auto-create-device') }}</div>
      <div class="m-b-10px">{{ $t('generate.auto-create-device-via-one-type-one-secret') }}</div>
      <NButton class="" type="primary" @click="onOpenDialogModal(1)">{{ $t('generate.configuration') }}</NButton>
    </div>
    <div class="">
      <div class="m-b-10px">{{ $t('generate.onlineDeviceConfig') }}</div>
      <NButton class="" type="primary" @click="onOpenDialogModal(2)">{{ $t('generate.configuration') }}</NButton>
    </div>
    <div class="">
      <div class="m-b-10px">{{ $t('generate.deviceConfigImage') }}</div>

      <n-upload
        :action="platformAssetBaseUrl + '/file/up'"
        :headers="{ 'x-token': localStg.get('token') || '' }"
        :data="{ type: 'image' }"
        class="upload"
        :show-file-list="false"
        accept="image/png, image/jpeg, image/jpg, image/gif"
        @finish="customRequest"
      >
        <n-upload-dragger class="upload-dragger">
          <div class="upload-content">
            <img v-if="imagePath && imagePath !== ''" :src="imagePath" class="slt" />
            <template v-else>
              <n-icon size="35" :depth="3">
                <SvgIcon local-icon="picture" class="more" />
              </n-icon>
              <p class="upload-text">{{ $t('generate.deviceConfigImage') }}</p>
            </template>
          </div>
        </n-upload-dragger>
      </n-upload>
    </div>
    <div>
      <!-- <div class="m-b-10px color-error-500">{{ $t('generate.delete-device-configuration') }}</div> -->
      <NButton type="error" @click="deleteConfig">{{ $t('common.delete') }}</NButton>
    </div>

    <n-modal
      v-model:show="showModal"
      preset="dialog"
      :class="getPlatform ? '90%' : 'w-400px'"
      :title="modalIndex === 1 ? $t('generate.configure-auto-create-device') : $t('generate.onlineDeviceConfig')"
      :show-icon="false"
    >
      <template v-if="modalIndex === 1">
        <dl class="flex-col gap-20px">
          <dd>{{ $t('generate.allow-device-auto-create') }}</dd>
          <dd>
            <n-switch v-model:value="auto_register" />
          </dd>
          <dd>{{ $t('generate.copy-one-type-one-secret-device-password') }}</dd>
          <dd>
            <NButton type="primary" @click="copyOneTypeOneSecretDevicePassword">{{ $t('generate.copy') }}</NButton>
          </dd>
        </dl>
      </template>
      <template v-else>
        <n-alert type="info" class="m-b-15px" :show-icon="true">
          {{ $t('generate.timeoutHeartbeatHint') }}
        </n-alert>
        <dl class="m-b-20px flex-col">
          <dt class="m-b-5px font-900">{{ $t('generate.timeoutMinutes') }}</dt>
          <dd class="m-b-10px">
            {{ $t('generate.timeoutThreshold') }}
          </dd>
          <dd class="m-b-20px max-w-220px">
            <n-input-number
              v-model:value="onlinejson.online_timeout"
              :disabled="isTimeoutDisabled"
              :min="0"
            ></n-input-number>
          </dd>
          <dt class="m-b-5px font-900">{{ $t('generate.heartbeatIntervalSeconds') }}</dt>
          <dd class="m-b-10px">{{ $t('generate.heartbeatThreshold') }}</dd>
          <dd class="max-w-220px">
            <n-input-number
              v-model:value="onlinejson.heartbeat"
              :disabled="isHeartbeatDisabled"
              :min="0"
            ></n-input-number>
          </dd>
        </dl>
      </template>

      <NFlex justify="end">
        <NButton @click="onDialogVisble">{{ $t('generate.cancel') }}</NButton>
        <NButton type="primary" @click="onSubmit">{{ $t('common.save') }}</NButton>
      </NFlex>
    </n-modal>
  </div>
</template>

<style lang="scss" scoped>
.upload {
  width: 200px;
  height: 150px;
  position: relative;

  .upload-dragger {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 200px;
    height: 150px;
    cursor: pointer;
  }

  .upload-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
  }

  .upload-text {
    margin-top: 8px;
    font-size: 14px;
  }

  .slt {
    position: absolute;
    top: 0;
    left: 0;
    width: 200px;
    height: 150px;
    object-fit: cover;
  }
}
</style>
