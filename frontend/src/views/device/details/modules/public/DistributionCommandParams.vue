<!--
  文件用途: 命令下发弹窗“可视化配置”页签：按命令参数模板渲染 string / Number / Boolean / Enum 输入。
  核心逻辑: 参数值直接写回 paramsData 每一项的 item[item.data_identifier]（与 useDistributionSubmitFlow 折叠规则一致）。
-->
<script setup lang="ts">
import { getDescriptionText } from './distributionAttributePayload'

defineProps<{
  commandValue: string
  params: any[]
}>()

// naive-ui 选项类型只声明 string | number 值，布尔参数沿用历史行为按原值提交。
const BOOLEAN_OPTIONS = [
  { label: 'true', value: true },
  { label: 'false', value: false }
] as unknown as Array<{ label: string; value: string }>

const enumOptions = (item: any) => item.enum_config?.map((option: any) => ({ ...option, label: option.desc })) || []
</script>

<template>
  <div v-if="commandValue !== ''">
    <div v-for="item in params" :key="item.id" class="form_box">
      <div class="form_table">
        <NFormItem :label="item.data_name" label-placement="left" label-width="80px" label-align="left">
          <NInput v-if="item.param_type === 'string'" v-model:value="item[item.data_identifier]" />
          <n-input-number v-else-if="item.param_type === 'Number'" v-model:value="item[item.data_identifier]" />
          <n-select
            v-else-if="item.param_type === 'Boolean'"
            v-model:value="item[item.data_identifier]"
            :options="BOOLEAN_OPTIONS"
          />
          <n-select
            v-else-if="item.param_type === 'Enum'"
            v-model:value="item[item.data_identifier]"
            :options="enumOptions(item)"
            :placeholder="$t('generate.please-select')"
          />
          <div class="description">
            {{ $t('generate.description-label') }}：{{ getDescriptionText(item) || $t('generate.description-empty') }}
          </div>
        </NFormItem>
      </div>
    </div>
    <div v-if="params.length === 0" class="empty-params">
      <p>{{ $t('generate.no-params-available') }}</p>
    </div>
  </div>
  <div v-else class="empty-params">
    <p>{{ $t('generate.select-command-first') }}</p>
  </div>
</template>

<style lang="scss" scoped>
.form_box {
  width: 100%;
}

.form_table {
  display: flex;
  gap: 12px;
  margin-bottom: 8px;

  .n-form-item {
    flex: 1;
    margin-right: 0;

    :deep(.n-form-item-blank) {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
    }

    .description {
      margin-top: 4px;
      font-size: 11px;
      color: #6b7280;
      line-height: 1.3;
    }

    :deep(.n-input),
    :deep(.n-input-number),
    :deep(.n-select) {
      .n-input__input-el,
      .n-input-number-input,
      .n-base-selection {
        height: 32px;
        border-radius: 4px;
        font-size: 13px;
      }
    }

    :deep(.n-input--textarea) {
      .n-input__textarea-el {
        min-height: 60px;
        border-radius: 4px;
        font-size: 13px;
        line-height: 1.4;
      }
    }
  }

  .n-input-number {
    width: 100%;
  }
}

.empty-params {
  text-align: center;
  padding: 20px 16px;
  color: #999;

  p {
    margin: 0;
    font-size: 13px;
  }
}
</style>
