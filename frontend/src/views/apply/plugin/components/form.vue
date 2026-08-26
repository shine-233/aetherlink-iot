<!--
文件用途：提供 接入插件管理 页面内的 form 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'
import type { SelectMixedOption } from 'naive-ui/es/select/src/interface'

const rules = ref({})

const protocol_config = defineModel<any>('protocolConfig', { default: {} })

interface Props {
  formElements?: object | any
}

const props = defineProps<Props>()
watchEffect(() => {
  const str = '{}'
  const thejson = JSON.parse(str)
  if (props.formElements) {
    props.formElements.forEach((element) => {
      if (element.type === 'table') {
        protocol_config.value[element.dataKey] ??= thejson[element.dataKey] || []
      } else {
        rules.value[element.dataKey] = element.validate || {}
        protocol_config.value[element.dataKey] ??= thejson[element.dataKey] || ''
      }
    })
  }
})

const onCreate = () => {
  return {}
}
</script>

<template>
  <div class="connection-box">
    <!-- 插件未返回配置 schema 时给出明确空态，避免渲染成无法理解的空白表单区。 -->
    <NEmpty
      v-if="!props.formElements || !props.formElements.length"
      :description="$t('common.noData')"
      class="py-24px"
    />
    <NForm v-else :model="protocol_config" :rules="rules" label-placement="top" class="w-full">
      <div class="w-full">
        <template v-for="element in props.formElements" :key="element.dataKey">
          <div v-if="element.type === 'input'">
            <NFormItem :label="element.label" :path="element.dataKey">
              <NInputNumber
                v-if="element.validate.type === 'number'"
                v-model:value="protocol_config[element.dataKey]"
                :placeholder="element.placeholder"
              />
              <NInput v-else v-model:value="protocol_config[element.dataKey]" :placeholder="element.placeholder" />
            </NFormItem>
          </div>
          <div v-if="element.type === 'select'">
            <NFormItem :label="element.label" :path="element.dataKey">
              <NSelect
                v-model:value="protocol_config[element.dataKey]"
                :options="element.options as SelectMixedOption[]"
              />
            </NFormItem>
          </div>

          <template v-if="element.type === 'table'">
            <div class="mb-12px w-full flex justify-between">
              <n-ellipsis
                v-for="subElement in element.array"
                :key="subElement.dataKey + element.dataKey"
                class="mr-24px flex-1"
              >
                {{ subElement.label }}
              </n-ellipsis>
              <div class="ml-20px w-68px"></div>
            </div>
            <n-dynamic-input
              v-model:value="protocol_config[element.dataKey]"
              item-style="margin-bottom: 0;"
              :on-create="onCreate"
              #="{ index }"
            >
              <div class="w-full flex justify-between">
                <template v-for="subElement in element.array" :key="subElement.dataKey">
                  <template v-if="subElement.type === 'input'">
                    <n-form-item
                      class="flex-1"
                      ignore-path-change
                      :show-label="false"
                      :label="subElement.label"
                      :path="`${element.dataKey}[${index}]${subElement.dataKey}`"
                      :rule="element.validate"
                    >
                      <NInputNumber
                        v-if="subElement.validate.type === 'number'"
                        v-model:value="protocol_config[element.dataKey][index][subElement.dataKey]"
                        :placeholder="subElement.placeholder"
                        @keydown.enter.prevent
                      />
                      <NInput
                        v-else
                        v-model:value="protocol_config[element.dataKey][index][subElement.dataKey]"
                        :placeholder="subElement.placeholder"
                        @keydown.enter.prevent
                      />
                    </n-form-item>
                  </template>
                  <template v-if="subElement.type === 'select'">
                    <n-form-item
                      class="flex-1"
                      ignore-path-change
                      :show-label="false"
                      :label="subElement.label"
                      :path="`${element.dataKey}[${index}]${subElement.dataKey}`"
                      :rule="element.validate"
                    >
                      <NSelect
                        v-model:value="protocol_config[element.dataKey][index][subElement.dataKey]"
                        :options="subElement.options as SelectMixedOption[]"
                      />
                    </n-form-item>
                  </template>
                  <div class="w-12px"></div>
                </template>
              </div>
            </n-dynamic-input>
          </template>
        </template>
      </div>
    </NForm>
  </div>
</template>
<style scoped lang="scss"></style>
