# Data Architecture Interfaces

## 目录职责

定义数据架构中编辑器级、组件配置级和组件运行时数据级管理接口，用来约束后续服务拆分和渐进式重构边界。

## 文件关系

- `IEditorDataManager.ts` 管理编辑器整体数据、组件树和画布配置。
- `IComponentConfigManager.ts` 管理单组件的基础、属性、数据源和交互配置层。
- `IComponentDataManager.ts` 管理组件运行时数据、数据源注册和更新。
- `index.ts` 仅聚合类型导出，供服务层或未来实现引用；它不会加载桥接器、适配器或运行时单例。

运行时实现应从 `SimpleDataBridge.ts`、`ConfigToSimpleDataAdapter.ts`、`VisualEditorBridge.ts` 等原模块显式导入。这样类型引用不会意外拉入编辑器依赖或触发模块初始化。

## 重点文件

- `IEditorDataManager.ts`：编辑器全局数据管理契约。
- `IComponentConfigManager.ts`：组件配置分层管理契约。
- `IComponentDataManager.ts`：组件运行时数据管理契约。

## 审查建议

- 接口字段代表跨模块约定，改名前需要确认服务、组件和测试引用。
- 新增接口方法前先判断是编辑态、运行态还是持久化契约，避免职责混杂。
- 接口扩展应配套至少一个调用方或测试 fixture，否则容易变成未验证设计。
