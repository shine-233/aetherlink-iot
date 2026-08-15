# scripts CLI 共享工具

## 目录定位
本目录保存 `@aetherlink/scripts` 内部共享工具，目前主要封装子进程命令执行。

## 主要文件
- `index.ts`：基于动态导入的 `execa` 执行命令，并返回标准输出。

## 依赖关系
依赖 `execa` 类型和运行时包，被多个 CLI 子命令间接使用。

## 审查发现
当前工具很薄，但它是提交、依赖更新等命令的执行边界。缺少说明时，容易忽略这里对 stdout 和错误传播方式的约定。

## 重构建议
后续可按需要增加命令日志、错误包装或 dry-run 支持，但应保持默认行为向后兼容。

## 验证建议
优先执行 `pnpm exec eslint packages/scripts/src/shared --ext .ts`。执行行为变更时用无副作用命令验证 stdout、stderr 和错误抛出。
