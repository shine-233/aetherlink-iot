# grid/__tests__ 组件说明

## 目录职责

本目录保存网格布局工具和错误处理的单元测试，重点验证布局校验、工具函数和错误收集在边界输入下的行为。

## 文件关系

- `utils.test.ts` 覆盖网格工具函数和布局辅助逻辑。
- `errorHandler.test.ts` 覆盖 `GridError`、默认错误处理器和安全执行包装。
- 与根目录下 `GridLayoutPlus.test.ts`、`gridLayoutPlusUtils.test.ts` 一起构成网格组件的基础回归网。

## 重点文件

- `errorHandler.test.ts`：保障可恢复/不可恢复错误的行为边界。
- `utils.test.ts`：保障布局工具在异常输入、边界尺寸和转换场景下稳定。

## 审查建议

- 新增布局算法或错误类型时，应优先补充本目录测试。
- 测试应断言可观察结果，不要依赖 Vue 组件内部私有实现。
- 如果测试需要拦截 `console`，结束时必须恢复 mock，避免污染其他用例。
