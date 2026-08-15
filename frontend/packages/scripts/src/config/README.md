# scripts CLI 配置加载

## 目录定位
本目录负责加载 `@aetherlink/scripts` 的默认配置和用户配置，是 CLI 命令读取项目级选项的入口。

## 主要文件
- `index.ts`：定义默认配置，并通过 `c12` 加载 `aetherlink.config` 配置文件。

## 依赖关系
依赖 `node:process`、`c12` 和上级 `types` 的 `CliOption` 类型。命令入口会通过这里获取清理路径、changelog 配置等选项。

## 审查发现
默认清理路径会影响文件删除范围，当前实现集中但缺少中文说明。配置加载失败或默认值变更会影响多个 CLI 子命令。

## 重构建议
后续可把默认配置导出为独立常量，并为配置合并结果增加最小测试，防止清理路径等高风险默认值误改。

## 验证建议
优先执行 `pnpm exec eslint packages/scripts/src/config --ext .ts`。配置行为变更时用临时配置文件验证默认值和覆盖值。
