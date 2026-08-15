<!--
文件用途：系统设置页中的功能开关组件，负责回显后端下发的系统功能列表，并以开关形式控制验证码、前端资源保护等能力。
核心逻辑：进入组件时拉取功能配置，把 enable_flag 映射为开关布尔值；切换任一开关时提交对应功能 ID，再重新回读整份功能配置并刷新本地缓存。
状态流说明：当前实现没有单独的 loading / saving 标记，而是依靠“切换后整表回读”保持最终一致；如果接口响应较慢，开关会短时间停留在用户刚操作的本地态。
使用注意事项：`enableZcAndYzm` 被写入 localStorage，仓库内其他组件会直接依赖这份缓存判断是否启用密码加密等策略，因此这里属于系统级配置源，而不是普通 UI 开关。
静态审查建议：建议后续补充列表加载态、单项提交态和失败回滚，避免当前接口失败时出现“界面已切换但远端未生效”的感知偏差。
-->
<script setup lang="tsx">
import { reactive, ref } from 'vue'
import { editFunction, getFunction } from '@/service/api/setting'

// 后端切换接口只接收 function_id，当前组件通过复用这个对象完成单项切换提交。
const queryParam = reactive({
  function_id: ''
})

interface FunctionOption {
  id: string
  description: string
  enable_flag: string
  value: boolean
  [key: string]: any
}

// 切换开关后重新回读整份功能表，保证本地布尔值、后端 enable_flag 和 localStorage 缓存三者重新对齐。
// 静态审查建议：这里没有失败回滚；如果 editFunction 报错，UI 会暂时保留用户刚切换的值，后续更适合补充 optimistic update 回退策略。
const changeFunc = async (item: FunctionOption) => {
  queryParam.function_id = String(item.id ?? '')
  const res = await editFunction(queryParam)
  if (!res.error) {
    getFunctionOption()
  }
}
const funcOptions = ref<FunctionOption[]>([])

// 功能列表既驱动当前页面渲染，也作为其他模块读取的本地能力快照，因此每次回读都会刷新 localStorage。
async function getFunctionOption() {
  const { data } = await getFunction()
  if (data) {
    localStorage.setItem('enableZcAndYzm', JSON.stringify(data))
    funcOptions.value = data.map((v: any): FunctionOption => {
      return {
        ...v,
        id: String(v.id ?? ''),
        description: String(v.description ?? ''),
        value: v.enable_flag === 'enable'
      }
    })
  }
}

// 当前组件在 setup 末尾直接触发首屏读取，后续如需增强错误提示和 loading 态，可考虑改成 onMounted + 统一请求封装。
getFunctionOption()
</script>

<template>
  <NFlex class="function-setting-section">
    <NForm class="function-setting-form" label-placement="left" :label-width="260">
      <NGrid :cols="24" :x-gap="18">
        <NFormItemGridItem v-for="(item, index) in funcOptions" :key="index" :span="24" :label="item.description">
          <n-switch v-model:value="item.value" @change="val => changeFunc(item)" />
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="start"></NSpace>
    </NForm>
  </NFlex>
</template>

<style lang="scss" scoped>
.function-setting-section {
  width: 100%;
  padding-top: 12px;
}

.function-setting-form {
  width: min(640px, 100%);
}
</style>
