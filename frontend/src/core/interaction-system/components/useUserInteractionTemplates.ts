import { ref } from 'vue'

import type { InteractionTemplate } from './interactionTemplateTypes'

/**
 * 文件用途：用户自定义交互模板的本地持久化。
 * 核心逻辑：以 localStorage 为存储介质，提供读写与自动反序列化，读取失败时降级为空列表。
 * 关键注意事项：存储键与反序列化结构变更时需考虑旧数据兼容，失败一律静默降级。
 */

const STORAGE_KEY = 'interaction-user-templates'

export function useUserInteractionTemplates() {
  const userTemplates = ref<InteractionTemplate[]>([])

  const saveUserTemplates = () => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(userTemplates.value))
    } catch (error) {
      /* intentionally empty */
    }
  }

  const loadUserTemplates = () => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY)
      if (saved) {
        userTemplates.value = JSON.parse(saved)
      }
    } catch (error) {
      /* intentionally empty */
    }
  }

  return { userTemplates, saveUserTemplates, loadUserTemplates }
}
