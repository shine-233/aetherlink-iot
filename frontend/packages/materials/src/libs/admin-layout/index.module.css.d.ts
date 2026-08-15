/**
 * 文件用途：声明 AdminLayout CSS Module 的可用类名。
 * 核心逻辑：为 index.module.css 中的布局类提供 TypeScript 类型约束。
 * 关键注意事项：该文件需要随 CSS 类名同步更新，否则组件导入样式会失去类型准确性。
 * 重构建议：可接入 CSS Module 类型生成流程，减少手工维护成本。
 */
declare const styles: {
  readonly 'layout-header': string
  readonly 'layout-header-placement': string
  readonly 'layout-tab': string
  readonly 'layout-tab-placement': string
  readonly 'layout-sider': string
  readonly 'layout-mobile-sider': string
  readonly 'layout-mobile-sider-mask': string
  readonly 'layout-sider_collapsed': string
  readonly 'layout-footer': string
  readonly 'layout-footer-placement': string
  readonly 'left-gap': string
  readonly 'left-gap_collapsed': string
  readonly 'sider-padding-top': string
  readonly 'sider-padding-bottom': string
}

export default styles
