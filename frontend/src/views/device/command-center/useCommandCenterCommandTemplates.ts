import { computed, ref } from 'vue'

export interface CommandCenterSavedCommandTemplate {
  id: string
  name: string
  identify: string
  value: string
  timeoutSeconds: number
  updatedAt: string
}

const TEMPLATE_STORAGE_KEY = 'aetherlink_command_center_templates_v1'
const MAX_TEMPLATES = 12
const TEMPLATE_EXPORT_KIND = 'aetherlink.commandTemplates'

export interface CommandCenterCommandTemplateImportResult {
  imported: number
  skipped: number
}

const getStorage = () => (typeof window === 'undefined' ? null : window.localStorage)

const normalizeTemplate = (input: unknown): CommandCenterSavedCommandTemplate | null => {
  const source = input as Partial<CommandCenterSavedCommandTemplate>
  const identify = typeof source?.identify === 'string' ? source.identify.trim() : ''
  if (!identify) return null

  const name = typeof source?.name === 'string' && source.name.trim() ? source.name.trim() : identify
  const timeoutSeconds = Number(source?.timeoutSeconds)

  return {
    id: typeof source?.id === 'string' && source.id ? source.id : `command-template-${Date.now()}`,
    name: name.slice(0, 64),
    identify: identify.slice(0, 128),
    value: typeof source?.value === 'string' ? source.value : '',
    timeoutSeconds: Number.isFinite(timeoutSeconds) ? Math.min(Math.max(Math.round(timeoutSeconds), 1), 3600) : 60,
    updatedAt: typeof source?.updatedAt === 'string' && source.updatedAt ? source.updatedAt : new Date().toISOString()
  }
}

const loadTemplates = () => {
  const storage = getStorage()
  if (!storage) return []

  try {
    const raw = storage.getItem(TEMPLATE_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.map(normalizeTemplate).filter(Boolean).slice(0, MAX_TEMPLATES) as CommandCenterSavedCommandTemplate[]
  } catch {
    storage.removeItem(TEMPLATE_STORAGE_KEY)
    return []
  }
}

const saveTemplates = (templates: CommandCenterSavedCommandTemplate[]) => {
  const storage = getStorage()
  if (!storage) return
  storage.setItem(TEMPLATE_STORAGE_KEY, JSON.stringify(templates.slice(0, MAX_TEMPLATES)))
}

export function serializeCommandTemplatesForExport(templates: CommandCenterSavedCommandTemplate[]) {
  return JSON.stringify(
    {
      kind: TEMPLATE_EXPORT_KIND,
      version: 1,
      exportedAt: new Date().toISOString(),
      templates: templates.map((template) => ({
        name: template.name,
        identify: template.identify,
        value: template.value,
        timeoutSeconds: template.timeoutSeconds,
        updatedAt: template.updatedAt
      }))
    },
    null,
    2
  )
}

export function parseCommandTemplatesForImport(raw: string) {
  const parsed = JSON.parse(raw)
  const candidates = Array.isArray(parsed)
    ? parsed
    : typeof parsed === 'object' &&
        parsed !== null &&
        parsed.kind === TEMPLATE_EXPORT_KIND &&
        parsed.version === 1 &&
        Array.isArray(parsed.templates)
      ? parsed.templates
      : []
  const now = Date.now()

  return candidates
    .map((candidate, index) => {
      const template = normalizeTemplate(candidate)
      if (!template) return null

      return {
        ...template,
        id: `command-template-import-${now}-${index}`
      }
    })
    .filter(Boolean) as CommandCenterSavedCommandTemplate[]
}

export function useCommandCenterCommandTemplates() {
  const savedCommandTemplates = ref<CommandCenterSavedCommandTemplate[]>(loadTemplates())
  const commandTemplateName = ref('')

  const hasSavedCommandTemplates = computed(() => savedCommandTemplates.value.length > 0)

  const saveCommandTemplate = (draft: {
    identify: string
    value: string
    timeoutSeconds: number
    name?: string
    syncName?: boolean
  }) => {
    const identify = draft.identify.trim()
    if (!identify) return false

    const now = new Date().toISOString()
    const name = draft.name?.trim() || commandTemplateName.value.trim() || identify
    const existing = savedCommandTemplates.value.find(
      (template) => template.name.toLowerCase() === name.toLowerCase()
    )
    const nextTemplate: CommandCenterSavedCommandTemplate = {
      id: existing?.id || `command-template-${Date.now()}`,
      name: name.slice(0, 64),
      identify: identify.slice(0, 128),
      value: draft.value || '',
      timeoutSeconds: Number.isFinite(draft.timeoutSeconds)
        ? Math.min(Math.max(Math.round(draft.timeoutSeconds), 1), 3600)
        : 60,
      updatedAt: now
    }
    const nextTemplates = [
      nextTemplate,
      ...savedCommandTemplates.value.filter((template) => template.id !== nextTemplate.id)
    ].slice(0, MAX_TEMPLATES)
    savedCommandTemplates.value = nextTemplates
    saveTemplates(nextTemplates)
    if (draft.syncName !== false) {
      commandTemplateName.value = nextTemplate.name
    }
    return true
  }

  const deleteCommandTemplate = (templateId: string) => {
    savedCommandTemplates.value = savedCommandTemplates.value.filter((template) => template.id !== templateId)
    saveTemplates(savedCommandTemplates.value)
  }

  const importCommandTemplates = (raw: string): CommandCenterCommandTemplateImportResult => {
    const importedTemplates = parseCommandTemplatesForImport(raw)
    const dedupedImports: CommandCenterSavedCommandTemplate[] = []
    const seenImportNames = new Set<string>()
    for (const template of importedTemplates) {
      const nameKey = template.name.toLowerCase()
      if (seenImportNames.has(nameKey)) continue
      seenImportNames.add(nameKey)
      dedupedImports.push(template)
    }

    const importedNames = new Set(dedupedImports.map((template) => template.name.toLowerCase()))
    const nextTemplates = [
      ...dedupedImports,
      ...savedCommandTemplates.value.filter((template) => !importedNames.has(template.name.toLowerCase()))
    ].slice(0, MAX_TEMPLATES)

    savedCommandTemplates.value = nextTemplates
    saveTemplates(nextTemplates)

    return {
      imported: dedupedImports.length,
      skipped: Math.max(0, importedTemplates.length - dedupedImports.length)
    }
  }

  return {
    commandTemplateName,
    deleteCommandTemplate,
    hasSavedCommandTemplates,
    importCommandTemplates,
    saveCommandTemplate,
    savedCommandTemplates
  }
}
