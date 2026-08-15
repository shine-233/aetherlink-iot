<!--
  文件用途: 设备配置详情页的动态协议表单组件。
  核心逻辑: 根据后端返回的 formElements schema 动态渲染输入框、下拉框和表格型字段，并把结果持续同步到 protocolConfig。
  主要链路: 父页面传入 schema -> watchEffect 按字段类型补默认值与校验规则 -> 模板分支按 type 渲染 -> 保存时由父页面统一提交 protocolConfig。
  关键注意事项:
  1. 这里直接决定协议配置 payload 的字段结构，schema 漂移会立即影响新增、编辑和详情回显。
  2. edit 只控制前端禁用态，不代表字段绝对不可修改，真正的写入边界仍以后端校验为准。
  3. table 类型字段使用数组承载多行配置，默认值和 path 拼接一旦不一致，保存和校验都会同时失真。
  静态审查建议:
  1. 当前 schema 类型约束较弱，建议补明确的字段类型声明，减少 element.validate、element.options 空值分支的隐式依赖。
  2. watchEffect 每次依赖变化都会尝试补默认值，后续可拆成更显式的初始化流程，避免 schema 热更新时掩盖异常数据。
  3. table 子项校验仍复用了外层 element.validate，建议补独立校验映射与无 schema、只读态场景测试。
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import type { SelectMixedOption } from 'naive-ui/es/select/src/interface'
import { find } from 'lodash-es'
import { $t } from '@/locales'

// 动态表单的校验规则字典，键名与 schema.dataKey 保持一致。
const rules = ref({})

// 父层通过 v-model:protocolConfig 共享协议配置，当前组件只负责结构化编辑，不直接提交。
const protocol_config = defineModel<any>('protocolConfig', { default: {} })

interface Props {
  formElements?: object | any
  edit?: boolean
}

const props = defineProps<Props>()

const formElementList = computed(() => (Array.isArray(props.formElements) ? props.formElements : []))
const defaultRule = { required: false }
const ruleFor = (element: any) => element?.validate || defaultRule
const isNumberField = (element: any) => element?.validate?.type === 'number'
const tableFieldPath = (parentKey: string, index: number, childKey: string) => `${parentKey}[${index}].${childKey}`

// schema 一旦到位，就按字段类型为 protocolConfig 补初始结构，避免模板直接访问时报空。
watchEffect(() => {
  const str = '{}'
  const thejson = JSON.parse(str)
  rules.value = {}
  if (formElementList.value.length > 0) {
    formElementList.value.forEach(element => {
      // table 字段始终用数组承载多行配置，详情页回显和新增空行都依赖这里的默认结构。
      if (element.type === 'table') {
        protocol_config.value[element.dataKey] ??= thejson[element.dataKey] || []
      } else {
        // 非 table 字段同时建立 NForm 校验映射；如果 schema.validate 漂移，这里会直接影响提交流程。
        rules.value[element.dataKey] = element.validate || {}
        protocol_config.value[element.dataKey] ??= thejson[element.dataKey] || ''
      }
    })
  }
})

// NDynamicInput 新增一行时使用空对象作为行模型，后续由各子字段双向绑定逐步补全。
const onCreate = () => {
  return {}
}
</script>

<template>
  <div class="connection-box h-full w-full">
    <NForm :model="protocol_config" :rules="rules" label-placement="top" class="w-full">
      <div class="w-full">
        <template v-for="element in formElementList" :key="element.dataKey">
          <!-- 单值输入字段：根据 validate.type 决定是数字输入还是普通文本输入。 -->
          <template v-if="element.type === 'input'">
            <NFormItem
              :label="element.label"
              :path="element.dataKey"
              class="w-300"
              :rules="[ruleFor(element)]"
            >
              <NTooltip trigger="hover" placement="top">
                <template #trigger>
                  <NInputNumber
                    v-if="isNumberField(element)"
                    v-model:value="protocol_config[element.dataKey]"
                    :disabled="props.edit"
                    :placeholder="element.placeholder"
                  />
                  <NInput
                    v-else
                    v-model:value="protocol_config[element.dataKey]"
                    :placeholder="element.placeholder"
                    :disabled="props.edit"
                  />
                </template>
                <template #default>
                  <span>{{ protocol_config[element.dataKey] }}</span>
                </template>
              </NTooltip>
            </NFormItem>
          </template>
          <!-- 枚举字段：依赖 schema.options 渲染，下方 tooltip 负责把 value 翻译成 label。 -->
          <template v-if="element.type === 'select'">
            <NFormItem
              :label="element.label"
              :path="props.edit ? undefined : element.dataKey"
              :rules="props.edit ? undefined : [ruleFor(element)]"
              :show-feedback="!props.edit"
            >
              <NTooltip trigger="hover" placement="top">
                <template #trigger>
                  <NSelect
                    v-model:value="protocol_config[element.dataKey]"
                    :disabled="props.edit"
                    :options="(element.options || []) as SelectMixedOption[]"
                  />
                </template>
                <template #default>
                  <span>
                    {{
                      find(element.options, {
                        value: protocol_config[element.dataKey]
                      })?.label
                    }}
                  </span>
                </template>
              </NTooltip>
            </NFormItem>
          </template>

          <!-- 表格字段：用于展示一对多协议参数，行级数据最终仍落在 protocolConfig[element.dataKey] 数组内。 -->
          <template v-if="element.type === 'table'">
            <div class="w-full flex flex-col overflow-auto">
              <div class="mb-12px flex flex-1 justify-between">
                <n-ellipsis
                  v-for="subElement in element.array"
                  :key="subElement.dataKey + element.dataKey"
                  class="mr-24px min-w-[100px] flex-1"
                >
                  <span v-if="subElement?.validate?.required" class="text-[#FF3838]">*</span>

                  {{ subElement.label }}
                  <span>{{ subElement?.validate?.required ? $t('card.required') : $t('card.notRequired') }}</span>
                </n-ellipsis>
                <div class="mr-20px min-w-[68px] w-[68px]"></div>
              </div>
              <n-dynamic-input
                v-model:value="protocol_config[element.dataKey]"
                item-style="margin-bottom: 0;"
                :on-create="onCreate"
                :disabled="props.edit"
                #="{ index }"
              >
                <!-- 每一行都复用子 schema 渲染，path 通过 dataKey + index 拼接，供 NForm 做逐项校验。 -->
                <div class="mb-12px flex flex-1 justify-between">
                  <template v-for="subElement in element.array" :key="subElement.dataKey">
                    <template v-if="subElement.type === 'input'">
                      <div class="mr-24px min-w-[100px] flex-1">
                        <n-form-item
                          ignore-path-change
                          :show-label="false"
                          :label="subElement.label"
                          :path="tableFieldPath(element.dataKey, index, subElement.dataKey)"
                          :rules="[ruleFor(subElement)]"
                        >
                          <NTooltip trigger="hover" placement="top">
                            <template #trigger>
                              <NInputNumber
                                v-if="isNumberField(subElement)"
                                v-model:value="protocol_config[element.dataKey][index][subElement.dataKey]"
                                :disabled="props.edit"
                                :placeholder="subElement.placeholder"
                                @keydown.enter.prevent
                              />
                              <NInput
                                v-else
                                v-model:value="protocol_config[element.dataKey][index][subElement.dataKey]"
                                :disabled="props.edit"
                                :placeholder="subElement.placeholder"
                                @keydown.enter.prevent
                              />
                            </template>
                            <template #default>
                              <span>{{ protocol_config[element.dataKey][index][subElement.dataKey] }}</span>
                            </template>
                          </NTooltip>
                        </n-form-item>
                      </div>
                    </template>
                    <template v-if="subElement.type === 'select'">
                      <div class="mr-24px min-w-[100px] flex-1">
                        <n-form-item
                          style="margin-right: 24px"
                          class="mr-24px min-w-[100px] flex-1"
                          ignore-path-change
                          :show-label="false"
                          :label="subElement.label"
                          :path="tableFieldPath(element.dataKey, index, subElement.dataKey)"
                          :rules="[ruleFor(subElement)]"
                        >
                          <NTooltip trigger="hover" placement="top">
                            <template #trigger>
                              <NSelect
                                v-model:value="protocol_config[element.dataKey][index][subElement.dataKey]"
                                :disabled="props.edit"
                                :options="(subElement.options || []) as SelectMixedOption[]"
                              />
                            </template>
                            <template #default>
                              <span>
                                {{
                                  find(subElement.options, {
                                    value: protocol_config[element.dataKey][index][subElement.dataKey]
                                  })?.label
                                }}
                              </span>
                            </template>
                          </NTooltip>
                        </n-form-item>
                      </div>
                    </template>
                  </template>
                </div>
              </n-dynamic-input>
            </div>
          </template>
        </template>
      </div>
    </NForm>
  </div>
</template>

<style scoped lang="scss"></style>
