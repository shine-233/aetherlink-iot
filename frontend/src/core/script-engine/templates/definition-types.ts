import type { ScriptTemplate } from '@/core/script-engine/types'

export type BuiltInTemplateDefinition = Omit<ScriptTemplate, 'id' | 'createdAt' | 'updatedAt'>
