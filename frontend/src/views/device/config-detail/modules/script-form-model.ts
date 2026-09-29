/*
 * Static pieces of the data-handle script form: type options, validation rules, the default Lua
 * template seeded into new scripts, and the CodeMirror editor options/toggles.
 */
import { ref } from 'vue'
import { $t } from '@/locales'

export function createScriptTypeOptions() {
  return [
    { label: $t('generate.all'), value: '' },
    { label: $t('custom.devicePage.reportPreprocessing'), value: 'A' },
    { label: $t('custom.devicePage.transmissionPreprocessing'), value: 'B' },
    { label: $t('custom.devicePage.attributeReporting'), value: 'C' },
    { label: $t('custom.devicePage.attributeDistribution'), value: 'D' },
    { label: $t('custom.devicePage.commandDeliveryPreprocessing'), value: 'E' },
    { label: $t('custom.devicePage.eventReportPreprocessing'), value: 'F' }
  ]
}

/**
 * Baseline form for both "new script" and "edit script": opening the modal starts from this
 * value so a previous session's script content, debug input or result cannot leak in.
 */
export function defaultScriptForm() {
  return {
    id: null,
    content: `function encodeInp(msg,topic)
 -- 说明：该函数为编码函数，将输入的消息编码为平台可识别的消息格式或者设备可识别的消息格式，请根据实际需求编写编码逻辑
 -- 入参：输入的msg，可以是任意数据类型的字符串。
 -- 出参：返回值为编码后的消息,需要是json字符串形式
 -- 注意：string与jsonObj互转需导入json库：local json = require("json")
 -- 例，string转jsonObj：local jsonData = json.decode(msgString)
 -- 例，jsonObj转string：local jsonStr = json.encode(jsonTable)
 local json = require("json")
 local jsonData = json.decode(msg)
 -- 例 if jsonData.temp then
 -- 例 jsonData.temp = jsonData.temp * 10
 -- 例 end
 local newJsonString = json.encode(jsonData)
 return newJsonString
 end`,
    description: null,
    device_config_id: null,
    enable_flag: 'Y',
    analog_input: null,
    last_analog_input: null,
    name: null,
    remark: null,
    script_type: null,
    resolt_analog_input: ''
  }
}

export function createScriptFormRules() {
  return {
    name: {
      required: true,
      message: $t('generate.enter-title'),
      trigger: 'blur'
    },
    content: {
      required: true,
      message: $t('generate.parse-script'),
      trigger: 'blur'
    },
    enable_flag: {
      required: true,
      message: $t('common.select'),
      trigger: 'change'
    },
    script_type: {
      required: true,
      message: $t('generate.select-processing-type'),
      trigger: 'change'
    }
  }
}

// Script editor options. The compatibility component renders CodeMirror.
export function createScriptEditorOptions() {
  return {
    automaticLayout: true,
    theme: 'vs',
    language: 'lua',
    fontSize: 14,
    lineHeight: 20,
    fontFamily: 'Consolas, "Courier New", monospace',
    wordWrap: 'on',
    lineNumbers: 'on',
    glyphMargin: true,
    folding: true,
    lineDecorationsWidth: 10,
    lineNumbersMinChars: 3,
    minimap: {
      enabled: true,
      side: 'right',
      size: 'proportional',
      showSlider: 'mouseover'
    },
    scrollBeyondLastLine: false,
    readOnly: false,
    cursorStyle: 'line',
    cursorBlinking: 'blink',
    renderWhitespace: 'selection',
    renderControlCharacters: false,
    fontLigatures: true,
    suggestOnTriggerCharacters: true,
    acceptSuggestionOnEnter: 'on',
    tabCompletion: 'on',
    wordBasedSuggestions: true,
    parameterHints: {
      enabled: true
    },
    quickSuggestions: {
      other: true,
      comments: false,
      strings: false
    },
    bracketPairColorization: {
      enabled: true
    },
    guides: {
      bracketPairs: true,
      indentation: true
    },
    formatOnPaste: true,
    formatOnType: true
  }
}

type EditorOptions = ReturnType<typeof createScriptEditorOptions>

/** Toolbar toggles for the script editor: minimap, word wrap and font size. */
export function useScriptEditorControls() {
  const editorOptions = ref<EditorOptions>(createScriptEditorOptions())

  // 当前本地编辑器未引入 Lua formatter；不展示会静默成功的伪格式化操作。
  const toggleMinimap = () => {
    editorOptions.value.minimap.enabled = !editorOptions.value.minimap.enabled
  }

  const toggleWordWrap = () => {
    editorOptions.value.wordWrap = editorOptions.value.wordWrap === 'on' ? 'off' : 'on'
  }

  const changeFontSize = (delta: number) => {
    const newSize = editorOptions.value.fontSize + delta
    if (newSize >= 10 && newSize <= 24) {
      editorOptions.value.fontSize = newSize
    }
  }

  return { editorOptions, toggleMinimap, toggleWordWrap, changeFontSize }
}
