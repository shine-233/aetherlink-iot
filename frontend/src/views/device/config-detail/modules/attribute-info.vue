<!--
  文件用途: 设备配置详情页的物模型绑定与属性来源区域。
  核心逻辑: 拉取可选物模型列表，展示当前绑定的物模型，并在用户切换物模型后回写设备配置，再通知父页面刷新属性信息。
  主要链路: 页面挂载 -> 拉取物模型菜单与当前配置详情 -> NSelect 展示当前绑定 -> 用户搜索/切换物模型 -> 保存成功后 emit('upDateConfig') 让父层整页回刷。
  关键注意事项:
  1. 这里绑定的是设备配置级物模型，不是单台设备的临时视图，切换后会影响同配置下详情页可见的属性、事件和命令。
  2. `id: ''` 被显式当作“解绑物模型”语义，前后端必须对这个约定保持一致。
  3. 当前页面通过重新查询 configInfo 回显 device_template_id，若父层上下文变化但组件未重建，旧 selectValue 可能残留。
  静态审查建议:
  1. getTableData 与 onMounted 缺少 loading 和 try/finally 收口，后续可统一成可观察的异步状态。
  2. emit 名称 `upDateConfig` 语义偏历史化，后续适合标准化为更稳定的刷新事件名。
  3. 目前只取物模型列表和详情接口的直接结果，建议补空配置、接口失败、物模型被删除等场景说明或保护。
-->
<script setup lang="tsx">
import { onMounted, ref } from 'vue'
import { NFlex } from 'naive-ui'
import { deviceConfigEdit, deviceConfigInfo, deviceConfigMenu } from '@/service/api/device'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
// eslint-disable-next-line vue/valid-define-emits
const emit = defineEmits()
const { routerPushByKey } = useRouterPush()

interface Props {
  configInfo?: object | any
}

const props = withDefaults(defineProps<Props>(), {
  configInfo: null
})

// 空字符串 id 代表解绑物模型，这个约定会直接进入更新接口 payload。
const unbindOption = () => ({ name: $t('generate.unbind'), id: '' })

// 物模型下拉列表，始终保留“解绑”入口，避免用户只能在已有物模型之间切换。
const plugList = ref([unbindOption()])

// 当前选中的物模型 ID，首次挂载时通过详情接口回显。
const selectValue = ref()

// 搜索物模型时只替换选项列表，不主动改当前值；真正的绑定关系以后端详情回读为准。
const getTableData = async (name: string) => {
  const res = await deviceConfigMenu({ name })
  const list = Array.isArray(res.data) ? res.data : []
  plugList.value = [unbindOption(), ...list]
}

// NSelect 的搜索词直接透传给物模型菜单接口，当前没有做输入防抖与异常提示。
const searchPlug = v => {
  getTableData(v)
}

// 切换物模型后立即调用更新接口；成功时让父页面重新拉取配置详情和物模型相关区域。
const choseTemp = async v => {
  const res = await deviceConfigEdit({ device_template_id: v, id: props.configInfo.id })
  if (!res.error) {
    emit('upDateConfig')
  }
}

// 当现有物模型不足时跳转到物模型管理页，当前组件本身不负责创建物模型。
const toTemplate = () => {
  routerPushByKey('device_template')
}

// 首屏同时完成“候选物模型列表”和“当前绑定物模型”回显；后者决定下拉框初始显示值。
onMounted(async () => {
  await getTableData('')
  const res = await deviceConfigInfo({ id: props.configInfo.id })
  selectValue.value = res.data?.device_template_id
})
</script>

<template>
  <div class="attribute-box">
    <NFlex align="center">
      <div>{{ $t('generate.bind-device-function-template') }}</div>
      <!-- 这里展示的是设备配置与物模型的绑定关系，变更后会影响下游属性/事件/命令展示来源。 -->
      <NSelect
        v-model:value="selectValue"
        class="w-300px"
        value-field="id"
        label-field="name"
        :options="plugList"
        filterable
        @update:value="choseTemp"
        @search="
          v => {
            searchPlug(v)
          }
        "
      />
      <div class="to-create" @click="toTemplate">{{ $t('generate.not-found-create') }}</div>
    </NFlex>
  </div>
</template>

<style scoped lang="scss">
.attribute-box {
  padding: 50px 10px;

  .to-create {
    color: #999999;
  }

  .to-create:hover {
    cursor: pointer;
    text-decoration: underline;
    color: #646cff;
  }

  .m-b-10 {
    margin-bottom: 10px;
  }
}

.pagination-box {
  display: flex;
  justify-content: flex-end;
}

.m-tb-10 {
  margin: 10px 0;
}

.w-300px {
  width: 300px;
  margin: 0 15px;
}

.w-500 {
  width: 500px;
}
</style>
