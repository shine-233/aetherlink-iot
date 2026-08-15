/**
 * 文件用途：实现 Pinia 插件入口 中的 index.ts。
 * 核心逻辑：承载该目录的主要 UI、状态或配置逻辑，并通过项目别名与相邻模块协作。
 * 关键注意事项：修改时需要保持外部入参、事件、类型和测试契约稳定。
 * 重构建议：可按数据准备、业务判断和展示输出拆分，降低后续维护成本。
 */
import type { PiniaPluginContext } from 'pinia'
import { cloneDeep } from 'lodash-es'
import { SetupStoreId } from '@/enum'

/**
 * The plugin reset the state of the store which is written by setup syntax
 *
 * @param context
 */
export function resetSetupStore(context: PiniaPluginContext) {
  const setupSyntaxIds = Object.values(SetupStoreId) as string[]
  if (setupSyntaxIds.includes(context.store.$id)) {
    const { $state } = context.store

    const defaultStore = cloneDeep($state)

    context.store.$reset = () => {
      context.store.$patch(cloneDeep(defaultStore))
    }
  }
}
