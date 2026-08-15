# Data Architecture Modals

## 目录职责

维护数据架构配置流程中的弹窗级组件，当前重点是 HTTP 数据源配置表单的完整编排。

## 文件关系

- `HttpConfigForm.vue` 编排 `components/common/HttpConfigStep1.vue` 到 `HttpConfigStep4.vue`。
- 弹窗提交结果需要满足 `types/http-config.ts`、`templates/http-templates.ts` 和执行器的配置契约。
- 测试文件覆盖表单交互、步骤状态和提交行为。

## 重点文件

- `HttpConfigForm.vue`：HTTP 配置弹窗主入口，负责步骤导航、完整性提示和最终提交。
- `HttpConfigForm.test.ts`：弹窗级交互和提交契约回归测试。

## 审查建议

- 表单默认值、禁用条件和提交 payload 变化都可能影响已保存配置。
- 弹窗层应编排步骤，不应复制步骤内部校验或执行器请求构造逻辑。
- 修改步骤顺序或锁定条件时，同步补充用户可见行为测试。
