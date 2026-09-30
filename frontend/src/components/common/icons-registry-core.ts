/**
 * 文件用途：核心图标注册表（ionicons5 通用图标）。
 *
 * 核心逻辑：不再一次性 import 并注册全部 592 个图标组件，而是按图标名所属分组
 * 进行懒加载：访问 `coreIcons.Add` 时返回一个异步组件，真正渲染时才拉取对应分组 chunk。
 *
 * 关键注意事项：
 * - 图标清单、分组映射与分组 chunk 由 icons-registry-core-manifest.ts / icons-registry-core-buckets/
 *   承载（生成产物，勿手工编辑）。新增图标需同步更新清单与分组文件。
 * - 注册表名与图标外观必须保持与改造前一致，因此这里仍然暴露同名属性，
 *   只是解析时机从「模块加载」变为「首次访问」。
 * - Object.keys / 展开语义由 createLazyIconRegistry 的 ownKeys 陷阱保证。
 */
import { defineAsyncComponent } from 'vue'

import { coreBucketLoaders, coreIconBucketOf, coreIconNames } from './icons-registry-core-manifest'
import { createLazyIconRegistry } from './icons-registry-lazy'

export const coreIcons = createLazyIconRegistry({
  names: coreIconNames,
  resolve: (name) =>
    defineAsyncComponent(async () => {
      const bucket = coreIconBucketOf[name]
      const loadBucket = bucket ? coreBucketLoaders[bucket] : undefined

      if (!loadBucket) {
        throw new Error(`[icons-registry-core] 未注册的图标: ${name}`)
      }

      const bucketIcons = await loadBucket()
      return bucketIcons[name]
    })
})
