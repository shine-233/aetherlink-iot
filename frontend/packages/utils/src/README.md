# utils 通用工具源码

## 目录定位
本目录提供 `@aetherlink/utils` 的通用工具，覆盖颜色处理、加密、存储适配和 ID 生成等跨包能力。

## 主要文件
- `index.ts`：统一导出工具模块。
- `color.ts`：封装颜色透明度、混合、转换和 HSV/RGB 工具。
- `crypto.ts`：基于 `crypto-js` 提供对象加解密。
- `storage.ts`：封装 local/session/localforage 和内存降级存储。
- `nanoid.ts`：重新导出 `nanoid`。

## 依赖关系
依赖 `colord`、`crypto-js`、`localforage` 和 `nanoid`。其他包如 `@aetherlink/axios` 会使用这里的 ID 能力。

## 审查发现
工具职责分散但规模较小，缺少中文目录说明和统一文件头。`storage.ts` 涉及浏览器能力降级，是后续验证重点。

## 重构建议
后续可为存储适配、加解密失败路径和颜色转换边界补充单测，避免通用工具在多处调用后才暴露问题。

## 验证建议
优先执行 `pnpm exec eslint packages/utils/src --ext .ts`。行为变更时补充纯函数和浏览器存储降级场景测试。
