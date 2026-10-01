<!--
文件用途: 数据处理脚本的新增/编辑弹窗（表单 + Lua 脚本编辑器 + 调试区）。
核心逻辑: 接收父级的脚本表单与编辑器选项，把保存/调试动作以事件回抛给父级的状态机。
关键注意事项: 表单对象由父级拥有，这里通过 v-model 透传，保存/调试的副作用仍在父级执行。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { $t } from '@/locales'

const ScriptEditor = defineAsyncComponent(() => import('@/components/LuaScriptEditor.vue'))

const props = defineProps<{
  title: string
  rules: Record<string, any>
  editorOptions: Record<string, any>
  scriptTypeOptions: Array<{ label: string; value: any }>
  /**
   * Function ref for the inner NForm: templates unwrap refs, so the parent hands down a setter
   * and keeps validation inside the page state machine.
   */
  setFormRef: (instance: any) => void
  /** Narrow (mobile) layout switches the form to a single column. */
  wide: boolean
}>()

const show = defineModel<boolean>('show', { required: true })
const form = defineModel<Record<string, any>>('form', { required: true })

const emit = defineEmits<{
  close: []
  submit: []
  quiz: []
  'toggle-word-wrap': []
  'toggle-minimap': []
  'change-font-size': [delta: number]
}>()

const bodyStyle = computed(() => ({ width: props.wide ? '90%' : '800px' }))
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="dialog"
    :width="800"
    :title="title + $t('common.dataProces')"
    :show-icon="false"
    :style="bodyStyle"
    :closable="false"
  >
    <NForm
      :ref="setFormRef"
      class="flex-wrap"
      :class="wide ? 'flex-col' : 'flex'"
      :model="form"
      :rules="rules"
      label-placement="left"
      label-width="auto"
    >
      <NFormItem :class="wide ? 'w-100%' : 'w-50%'" :label="$t('page.manage.menu.form.title')" path="name">
        <NInput v-model:value="form.name" :placeholder="$t('generate.enter-title')" />
      </NFormItem>
      <NFormItem :class="wide ? 'w-100%' : 'w-50%'" :label="$t('generate.processing-type')" path="script_type">
        <NSelect
          v-model:value="form.script_type"
          :options="scriptTypeOptions"
          :placeholder="$t('generate.select-processing-type')"
        ></NSelect>
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('device_template.table_header.description')" path="description">
        <NInput
          v-model:value="form.description"
          type="textarea"
          :rows="2"
          :placeholder="$t('generate.enter-description')"
        />
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('generate.parse-script')" :rules="rules" path="content">
        <div class="editor-container">
          <!-- 编辑器工具栏 -->
          <div class="editor-toolbar">
            <div class="toolbar-left">
              <NButton size="small" tertiary @click="emit('toggle-word-wrap')">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24">
                      <path
                        fill="currentColor"
                        d="M4 19h6v-2H4v2zM20 5H4v2h16V5zm-3 6H4v2h13.25c1.1 0 2 .9 2 2s-.9 2-2 2H15v-2l-3 3l3 3v-2h2.25c2.3 0 4.25-2.05 4.25-4.5S19.55 11 17.25 11z"
                      />
                    </svg>
                  </n-icon>
                </template>
                自动换行
              </NButton>
              <NButton size="small" tertiary @click="emit('toggle-minimap')">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24">
                      <path
                        fill="currentColor"
                        d="M3 3h18v18H3V3zm16 16V5H5v14h14zM7 7h2v2H7V7zm0 4h2v2H7v-2zm0 4h2v2H7v-2zm4-8h6v2h-6V7zm0 4h6v2h-6v-2zm0 4h6v2h-6v-2z"
                      />
                    </svg>
                  </n-icon>
                </template>
                小地图
              </NButton>
            </div>
            <div class="toolbar-right">
              <NButton size="small" tertiary @click="emit('change-font-size', -1)">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24"><path fill="currentColor" d="M19 13H5v-2h14v2z" /></svg>
                  </n-icon>
                </template>
              </NButton>
              <span class="font-size-display">{{ editorOptions.fontSize }}px</span>
              <NButton size="small" tertiary @click="emit('change-font-size', 1)">
                <template #icon>
                  <n-icon>
                    <svg viewBox="0 0 24 24"><path fill="currentColor" d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" /></svg>
                  </n-icon>
                </template>
              </NButton>
            </div>
          </div>
          <!-- Script editor -->
          <div class="editor-wrapper">
            <ScriptEditor
              v-model:value="form.content"
              :options="editorOptions"
              height="300"
              language="lua"
              class="custom-script-editor"
            />
          </div>
        </div>
      </NFormItem>
      <NFormItem
        v-if="0"
        class="w-100%"
        :label="$t('page.manage.setting.dataClearSetting.form.enabled')"
        path="enable_flag"
      >
        <NSwitch v-model:value="form.enable_flag" checked-value="Y" unchecked-value="N" />
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('generate.simulate-input')" path="last_analog_input">
        <NInput v-model:value="form.last_analog_input" type="textarea" :rows="2" />
      </NFormItem>
      <NFormItem class="w-100%" :label="$t('generate.debug-run-result')" path="resolt_analog_input">
        <NInput v-model:value="form.resolt_analog_input" :rows="5" :disabled="true" type="textarea" />
      </NFormItem>
      <NFormItem>
        <NButton type="primary" @click="emit('quiz')">{{ $t('common.debug') }}</NButton>
      </NFormItem>
    </NForm>
    <NFlex justify="end">
      <NButton @click="emit('close')">{{ $t('generate.cancel') }}</NButton>
      <NButton type="primary" @click="emit('submit')">{{ $t('common.save') }}</NButton>
    </NFlex>
  </n-modal>
</template>

<style scoped lang="scss">
/* 编辑器容器样式 */
.editor-container {
  width: 100%;
  border: 1px solid #e0e0e6;
  border-radius: 6px;
  overflow: hidden;
  background: #fff;
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e6;
  min-height: 40px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.font-size-display {
  font-size: 12px;
  color: #666;
  min-width: 35px;
  text-align: center;
}

.editor-wrapper {
  position: relative;
  background: #fff;
  width: 100%;
}

.custom-script-editor {
  border: none !important;
  width: 100% !important;
}

/* 编辑器工具栏按钮样式优化 */
.editor-toolbar .n-button {
  height: 28px;
  padding: 0 8px;
  font-size: 12px;
}

.editor-toolbar .n-button .n-icon {
  font-size: 14px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .editor-toolbar {
    flex-direction: column;
    gap: 8px;
    padding: 12px;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
    justify-content: center;
  }
}
</style>
