# Data Architecture Types

## 目录定位

`frontend/src/core/data-architecture/types/` 是数据架构类型层的集中出口目录。这里不是单一业务实现，而是一个面向全项目的类型聚合点，负责把基础类型、增强类型、兼容类型和辅助工具统一导出。

## 文件关系

- `index.ts` 是统一导出入口。
- `simple-types.ts` 提供简化数据源类型。
- `enhanced-types.ts` 提供增强版和兼容版类型。
- `http-config.ts`、`internal-api.ts`、`parameter-editor.ts`、`unified-types.ts` 分别承载不同类型分组。
- `enhanced-types.test.ts` 用于验证增强类型和适配逻辑。

## 这个目录的职责

这一层的目标是让上层页面、编辑器和执行链路共享同一套类型约定，减少重复定义和“每个模块一套字段名”的漂移风险。

## 统一出口

`index.ts` 会把下面几类内容重新导出：

- 执行器层类型
- 增强版配置类型
- 简化版数据源类型
- 类型守卫与常量
- 版本号和兼容性标记

## 典型结构

### 基础层

- `simple-types.ts`：面向常见场景的简化类型，适合表单、映射、触发器和视觉编辑器。
- `http-config.ts`：HTTP 数据源相关约束。
- `internal-api.ts`：内部 API 地址和接口结构约束。

### 增强层

- `enhanced-types.ts`：支持版本管理、动态参数、适配器和兼容字段。
- `parameter-editor.ts`：增强参数编辑相关类型。

### 聚合层

- `index.ts`：对外唯一推荐入口，避免上层直接依赖分散文件。

## 推荐导入方式

```ts
import type {
  DataSourceConfiguration,
  EnhancedDataSourceConfiguration,
  SimpleDataSourceConfig,
  TYPE_SYSTEM_VERSION
} from '@/core/data-architecture/types'
```

## 使用注意事项

1. 这不是“随便改一个字段就完事”的目录，很多类型会影响整个编辑器、执行器和适配器链路。
2. `index.ts` 同时承接旧版和新版导出，删除或改名任何符号都可能引发大范围联动。
3. `TYPE_SYSTEM_VERSION` 和 `SUPPORTED_CONFIG_VERSIONS` 属于兼容性信号，不要随意改动默认值。
4. 如果新增类型字段，务必同步考虑适配器、默认值和序列化路径。

## 静态审查建议

- 检查上游调用方是否依赖了过深的内部路径，尽量收敛到 `index.ts`。
- 检查新增字段是否同时补了默认值、转换器和测试。
- 检查旧版兼容导出是否仍然需要保留，避免无意破坏已存储配置。
- 检查枚举和联合类型改动是否会影响表单校验、接口适配和执行器映射。

## 后续改进方向

- 将“公共稳定导出”和“内部实验导出”拆成两个入口，降低误用概率。
- 为版本迁移和兼容策略补一份单独说明。
- 为类型层增加一张关系图，标清基础类型、增强类型和执行器层之间的依赖边界。
