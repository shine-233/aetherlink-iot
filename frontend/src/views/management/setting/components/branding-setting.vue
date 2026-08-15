<!--
文件用途：系统设置页中的品牌配置组件，负责系统名称、favicon、顶部 Logo、加载页 Logo 与首页背景图等品牌资源的回显与保存。
核心逻辑：挂载时读取主题设置列表并取首条记录回填表单；保存时提交当前品牌资源配置，并在成功后刷新系统设置 store，让页签图标、标题与主题资源尽快同步到全局。
状态流说明：`loading` 管首屏回显与手动重载，`saving` 只控制保存按钮；表单本身没有脏值比较，当前实现默认每次点击保存都全量提交。
使用注意事项：品牌资源字段现在都以 URL 字符串保存，前端只做 trim，不负责校验资源可访问性；上传 GitHub 前的运维文档需要补充这些资源的部署来源与缓存策略。
静态审查建议：如果后端未来允许多套品牌记录，当前“只取 list[0]”的实现会失去表达力；更稳妥的方式是由接口返回唯一配置对象，或在这里显式选择生效记录。
-->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { editThemeSetting, fetchThemeSetting } from '@/service/api/setting'
import { useSysSettingStore } from '@/store/modules/sys-setting'
import { $t } from '@/locales'
import { message } from '@/utils/common/discrete'

type BrandingForm = {
  id: string
  system_name: string
  logo_cache: string
  logo_background: string
  logo_loading: string
  home_background: string
}

const loading = ref(false)
const saving = ref(false)
const sysSettingStore = useSysSettingStore()

// 这里直接维护可编辑表单副本，避免把 store 中的全局主题状态直接暴露给输入框。
const form = reactive<BrandingForm>({
  id: '',
  system_name: '',
  logo_cache: '',
  logo_background: '',
  logo_loading: '',
  home_background: ''
})

// 主题设置接口当前按列表返回，这里只接管第一条记录作为“当前生效品牌配置”。
function assignForm(record?: Api.GeneralSetting.ThemeSetting) {
  form.id = record?.id || ''
  form.system_name = record?.system_name || ''
  form.logo_cache = record?.logo_cache || ''
  form.logo_background = record?.logo_background || ''
  form.logo_loading = record?.logo_loading || ''
  form.home_background = record?.home_background || ''
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
      home_background: form.home_background.trim()
    })
    if (!error) {
      message.success($t('custom.management.branding.saved'))
      await sysSettingStore.initSysSetting()
    }
  } finally {
    saving.value = false
  }
}

// 首次进入系统设置页就读取当前品牌配置，避免表单出现“空白后再闪现”的体验割裂。
onMounted(loadBrandingSetting)
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
      <NFormItem :label="$t('custom.management.branding.headerLogoUrl')">
        <NInput v-model:value="form.logo_background" maxlength="255" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.loadingLogoUrl')">
        <NInput v-model:value="form.logo_loading" maxlength="255" clearable />
      </NFormItem>
      <NFormItem :label="$t('custom.management.branding.homeBackgroundUrl')">
        <NInput v-model:value="form.home_background" maxlength="255" clearable />
      </NFormItem>
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

@media (max-width: 640px) {
  .branding-actions {
    padding-left: 0;
  }
}
</style>
