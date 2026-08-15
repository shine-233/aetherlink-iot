# color-palette 内置 JSON 数据

## 目录定位
本目录保存颜色名称和默认调色板的静态数据，是 `@aetherlink/color-palette` 计算颜色名称和色板族的基础输入。

## 主要文件
- `color-name.json`：颜色名称数据，供 `name.ts` 计算最近颜色名。
- `palette.json`：默认色板族数据，供 `palette.ts` 匹配和派生色板。

## 依赖关系
被同级 `name.ts`、`palette.ts` 和 `index.ts` 读取。数据格式需要与 `type.ts` 中的色板和颜色项类型保持一致。

## 审查发现
当前数据文件没有独立说明，后续维护者容易把数据修订和算法修订混在一起评审。

## 重构建议
如果后续频繁调整数据，可增加数据来源、生成方式和格式校验脚本说明，避免手工编辑造成结构漂移。

## 验证建议
数据变更后至少运行 color-palette 的 targeted eslint 和类型检查；若新增数据校验脚本，应验证 JSON 能被 `name.ts` 与 `palette.ts` 正常导入。
