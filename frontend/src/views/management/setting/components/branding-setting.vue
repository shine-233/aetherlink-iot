<!--
文件用途：系统设置页中的品牌配置组件，负责系统名称、favicon、顶部 Logo、加载页 Logo 与首页背景图等品牌资源的回显与保存。
核心逻辑：挂载时读取主题设置列表并取首条记录回填表单；保存时提交当前品牌资源配置，并在成功后刷新系统设置 store，让页签图标、标题与主题资源尽快同步到全局。
状态流说明：`loading` 管首屏回显与手动重载，`saving` 只控制保存按钮；表单本身没有脏值比较，当前实现默认每次点击保存都全量提交。
使用注意事项：品牌资源字段现在都以 URL 字符串保存，前端只做 trim，不负责校验资源可访问性；上传 GitHub 前的运维文档需要补充这些资源的部署来源与缓存策略。
静态审查建议：如果后端未来允许多套品牌记录，当前“只取 list[0]”的实现会失去表达力；更稳妥的方式是由接口返回唯一配置对象，或在这里显式选择生效记录。
-->
<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import { NButton } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { editThemeSetting, fetchThemeSetting } from '@/service/api/setting'
import {
  deleteTenantTranslations,
  fetchTenantCustomCSS,
  fetchTenantTranslations,
  upsertTenantCustomCSS,
  upsertTenantTranslations
} from '@/service/api/whitelabel'
import { useSysSettingStore } from '@/store/modules/sys-setting'
import { useThemeStore } from '@/store/modules/theme'
import { $t } from '@/locales'
import { message } from '@/utils/common/discrete'

type BrandingForm = {
  id: string
  system_name: string
  logo_cache: string
  logo_background: string
  logo_loading: string
  home_background: string
  theme_color: string
  favicon: string
}

type TranslationRow = {
  lang: string
  key: string
  value: string
}

const loading = ref(false)
const saving = ref(false)
const sysSettingStore = useSysSettingStore()
const themeStore = useThemeStore()

// 这里直接维护可编辑表单副本，避免把 store 中的全局主题状态直接暴露给输入框。
const form = reactive<BrandingForm>({
  id: '',
  system_name: '',
  logo_cache: '',
  logo_background: '',
  logo_loading: '',
  home_background: '',
  theme_color: '',
  favicon: ''
})

// ---- 白标扩展（TB-47）：翻译覆盖 + 自定义 CSS ----
// 语言白名单与后端 SupportedTenantTranslationLangs 一致；选项文案用语言自称，不随界面语言变化。
const whitelabelLangs = ['zh-cn', 'en-us', 'es-es', 'fr-fr'] as const
const langOptions = whitelabelLangs.map((value) => ({
  value,
  label: value === 'zh-cn' ? '中文' : value === 'en-us' ? 'English' : value === 'fr-fr' ? 'Français' : 'Español'
}))

const overridesLoading = ref(false)
const overrideBusy = ref(false)
const translationRows = ref<TranslationRow[]>([])
const newTranslation = reactive<TranslationRow>({ lang: 'zh-cn', key: '', value: '' })

const cssSaving = ref(false)
const customCSS = ref('')

// 操作列用 h(NButton) 渲染删除入口（模板自动注册不覆盖 render 函数场景，显式导入）。
const translationColumns: DataTableColumns<TranslationRow> = [
  { title: () => $t('custom.management.branding.translationLang'), key: 'lang', width: 100 },
  { title: () => $t('custom.management.branding.translationKey'), key: 'key' },
  { title: () => $t('custom.management.branding.translationValue'), key: 'value' },
  {
    title: () => $t('custom.management.branding.translationActions'),
    key: 'actions',
    width: 90,
    render: (row) =>
      h(
        NButton,
        {
          size: 'tiny',
          quaternary: true,
          type: 'error',
          disabled: overrideBusy.value,
          onClick: () => removeTranslationRow(row)
        },
        { default: () => $t('custom.management.branding.translationDelete') }
      )
  }
]

// NDataTable row-key：lang+key 是覆盖行的业务唯一键（对齐后端 UNIQUE(tenant_id,lang,key)）
function translationRowKey(row: TranslationRow) {
  return `${row.lang}#${row.key}`
}

// 加载当前作用域的全部翻译覆盖（跨语言一起展示，便于对照维护）。
async function loadTranslationOverrides() {
  overridesLoading.value = true
  try {
    const { error, data } = await fetchTenantTranslations()
    if (!error && data) {
      translationRows.value = (data.list || []).map((row) => ({
        lang: row.lang,
        key: row.key,
        value: row.value
      }))
    } else if (error) {
      message.error($t('custom.management.branding.overridesLoadFailed'))
    }
  } finally {
    overridesLoading.value = false
  }
}

// 加载当前作用域的自定义 CSS 回显文本。
async function loadCustomCSS() {
  const { error, data } = await fetchTenantCustomCSS()
  if (!error && data) {
    customCSS.value = data.css || ''
  }
}

// 新增/更新一条覆盖（同 lang+key 即更新 value，后端 UPSERT 幂等）；单条即时落库。
async function upsertTranslationRow() {
  const row: TranslationRow = {
    lang: newTranslation.lang,
    key: newTranslation.key.trim(),
    value: newTranslation.value
  }
  if (!row.key || !row.value.trim()) {
    message.error($t('custom.management.branding.translationRowInvalid'))
    return
  }
  overrideBusy.value = true
  try {
    const { error } = await upsertTenantTranslations([row])
    if (!error) {
      message.success($t('custom.management.branding.translationSaved'))
      newTranslation.key = ''
      newTranslation.value = ''
      await loadTranslationOverrides()
    }
  } finally {
    overrideBusy.value = false
  }
}

// 删除一条覆盖（按 lang+key 定位，后端按作用域隔离）。
async function removeTranslationRow(row: TranslationRow) {
  overrideBusy.value = true
  try {
    const { error } = await deleteTenantTranslations([{ lang: row.lang, key: row.key }])
    if (!error) {
      message.success($t('custom.management.branding.translationDeleted'))
      await loadTranslationOverrides()
    }
  } finally {
    overrideBusy.value = false
  }
}

// 保存自定义 CSS：空串即清除；成功后重新拉取覆盖，让新样式经 textContent 立即注入生效。
async function saveCustomCSS() {
  cssSaving.value = true
  try {
    const { error } = await upsertTenantCustomCSS(customCSS.value.trim())
    if (!error) {
      message.success($t('custom.management.branding.customCssSaved'))
      await sysSettingStore.initWhitelabelOverrides()
    }
  } finally {
    cssSaving.value = false
  }
}

// 主题设置接口当前按列表返回，这里只接管第一条记录作为“当前生效品牌配置”。
function assignForm(record?: Api.GeneralSetting.ThemeSetting) {
  form.id = record?.id || ''
  form.system_name = record?.system_name || ''
  form.logo_cache = record?.logo_cache || ''
  form.logo_background = record?.logo_background || ''
  form.logo_loading = record?.logo_loading || ''
  form.home_background = record?.home_background || ''
  form.theme_color = record?.theme_color || ''
  form.favicon = record?.favicon || ''
}

// 回显链路只负责把远端品牌配置落入局部表单，不直接写全局 store，避免编辑中的脏值提前污染全局展示。
async function loadBrandingSetting() {
  loading.value = true
  try {
    const { error, data } = await fetchThemeSetting()
    if (!error) assignForm(data?.list?.[0])
  } finally {
    loading.value = false
  }
}

// 保存成功后需要重新初始化系统设置 store，确保导航标题、图标和登录页背景等全局展示拿到最新资源。
// 静态审查建议：当前仅校验 id 是否存在，没有检测资源字段是否为空或 URL 是否有效，后续可按部署要求补充更明确的输入约束。
async function saveBrandingSetting() {
  if (!form.id) {
    message.error($t('custom.management.branding.missingRecord'))
    return
  }
  saving.value = true
  try {
    const { error } = await editThemeSetting({
      id: form.id,
      system_name: form.system_name.trim(),
      logo_cache: form.logo_cache.trim(),
      logo_background: form.logo_background.trim(),
      logo_loading: form.logo_loading.trim(),
      home_background: form.home_background.trim(),
      theme_color: form.theme_color.trim(),
      favicon: form.favicon.trim()
    })
    if (!error) {
      message.success($t('custom.management.branding.saved'))
      await sysSettingStore.initSysSetting()
      // 白标联动：theme_color 非空时同步 Naive 主题系统主色（登录页/全局主题即时生效）。
      const rawColor = form.theme_color.trim()
      if (rawColor && /^#[0-9a-fA-F]{6}$/.test(rawColor)) {
        themeStore.updateThemeColors('primary', rawColor)
      }
    }
  } finally {
    saving.value = false
  }
}

// 首次进入系统设置页就读取当前品牌配置与白标覆盖，避免表单出现“空白后再闪现”的体验割裂。
onMounted(() => {
  void loadBrandingSetting()
  void loadTranslationOverrides()
  void loadCustomCSS()
})
</script>

<template>
  <NSpin :show="loading">
    <NForm class="branding-form" label-placement="left" :label-width="180">
      <NFormItem :label="$t('custom.management.branding.systemTitle')">
        <NInput v-model:value="form.system_name" maxlength="99" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.faviconUrl')">
        <NInput v-model:value="form.logo_cache" maxlength="255" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.themeColor')">
        <NInput v-model:value="form.theme_color" maxlength="32" placeholder="#1677ff" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.browserFavicon')">
        <NInput v-model:value="form.favicon" maxlength="255" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.headerLogoUrl')">
        <NInput v-model:value="form.logo_background" maxlength="255" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.loadingLogoUrl')">
        <NInput v-model:value="form.logo_loading" maxlength="255" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.homeBackgroundUrl')">
        <NInput v-model:value="form.home_background" maxlength="255" clearable />
      </NFormItem>

      <!-- 白标扩展（TB-47）：租户翻译覆盖 -->
      <NDivider title-placement="left" :title="$t('custom.management.branding.translationOverrides')" />
      <p class="branding-hint">{{ $t('custom.management.branding.translationOverridesHint') }}</p>
      <NDataTable
        size="small"
        :columns="translationColumns"
        :data="translationRows"
        :loading="overridesLoading"
        :row-key="translationRowKey"
      >
        <template #empty>{{ $t('custom.management.branding.translationEmpty') }}</template>
      </NDataTable>
      <NSpace class="branding-translation-editor" :size="8">
        <NSelect
          v-model:value="newTranslation.lang"
          class="branding-lang-select"
          :options="langOptions"
          :disabled="overrideBusy"
        />
        <NInput
          v-model:value="newTranslation.key"
          class="branding-key-input"
          :placeholder="$t('custom.management.branding.translationNewKeyPlaceholder')"
          :disabled="overrideBusy"
          clearable
        />
        <NInput
          v-model:value="newTranslation.value"
          class="branding-value-input"
          :placeholder="$t('custom.management.branding.translationNewValuePlaceholder')"
          :disabled="overrideBusy"
          clearable
        />
        <NButton :loading="overrideBusy" @click="upsertTranslationRow">
          {{ $t('custom.management.branding.translationUpsert') }}
        </NButton>
      </NSpace>

      <!-- 白标扩展（TB-47）：自定义 CSS -->
      <NDivider title-placement="left" :title="$t('custom.management.branding.customCss')" />
      <p class="branding-hint">{{ $t('custom.management.branding.customCssHint') }}</p>
      <NFormItem :label="$t('custom.management.branding.customCss')">
        <NInput
          v-model:value="customCSS"
          type="textarea"
          class="branding-css-input"
          :rows="8"
          :placeholder="$t('custom.management.branding.customCssPlaceholder')"
          :disabled="cssSaving"
        />
      </NFormItem>
      <NSpace class="branding-actions">
        <NButton :loading="cssSaving" @click="saveCustomCSS">
          {{ $t('custom.management.branding.customCssSave') }}
        </NButton>
      </NSpace>

      <NSpace class="branding-actions">
        <NButton :loading="loading" @click="loadBrandingSetting">
          {{ $t('custom.management.branding.reload') }}
        </NButton>
        <NButton type="primary" :loading="saving" @click="saveBrandingSetting">
          {{ $t('custom.management.branding.save') }}
        </NButton>
      </NSpace>
    </NForm>
  </NSpin>
</template>

<style scoped>
.branding-form {
  width: min(760px, 100%);
  padding-top: 12px;
}

.branding-actions {
  padding-left: 180px;
}

.branding-hint {
  margin: 0 0 12px;
  padding-left: 0;
  color: var(--n-text-color-disabled, #999);
  font-size: 12px;
}

.branding-lang-select {
  width: 120px;
}

.branding-key-input {
  width: 260px;
}

.branding-value-input {
  width: 200px;
}

.branding-translation-editor {
  margin-top: 12px;
}

.branding-css-input {
  font-family: Consolas, Monaco, monospace;
}

@media (max-width: 640px) {
  .branding-actions {
    padding-left: 0;
  }

  .branding-lang-select,
  .branding-key-input,
  .branding-value-input {
    width: 100%;
  }
}
</style>
