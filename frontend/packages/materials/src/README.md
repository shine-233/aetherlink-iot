# materials 源码入口

## 目录定位

本目录是 `frontend/packages/materials` 的源码入口，集中导出可复用布局、页签、滚动条组件和共享类型。

## 文件与子目录

- `index.ts`：包级导出入口，聚合 `AdminLayout`、`PageTab`、`SimpleScrollbar` 和公共类型。
- `libs/`：组件实现目录，包含管理后台布局、页签和滚动条等物料。
- `types/`：物料包共享类型定义。

## 审查记录与重构建议

- 问题描述：包入口承担公共 API 角色，但新增物料时容易只追加导出而缺少兼容说明。
- 改进方案：新增物料必须同时补子目录 README、文件头注释和入口导出说明。
- 实施步骤：先在子目录完成组件文档与测试，再从 `index.ts` 暴露稳定导出。
- 预期效果：减少跨包调用方受无意导出变更影响的风险。

## 验证建议

- 修改入口导出后运行 targeted ESLint：`pnpm exec eslint packages/materials/src`。
- 若变更公开导出名称，还需运行前端类型检查确认调用方同步完成。
