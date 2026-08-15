# 设备详情业务模块

## 目录职责

`frontend/src/views/device/details/modules` 保存设备详情页下的业务 tab 和子视图，是设备详情壳层之下最重要的一层功能实现目录。

这里的模块覆盖遥测、状态历史、命令下发、公共信息、RDI 扩展等用户可见功能，属于设备域里同时兼具“业务密度高”和“页面联动多”的区域。

## 文件关系

- `../index.vue`
  - 设备详情页壳层，按 tab 挂载本目录下的模块，并向它们下发设备 ID、详情对象和刷新信号。
- `telemetry/`
  - 遥测总视图，负责当前值、实时订阅、历史、趋势、模拟上报和日志。
- `public/`
  - 公共展示或共享子模块。
- `RdiDeviceOperationsView.vue`
  - 组合 `rdi/composables` 的 RDI 相关能力。
- `RdiDeviceHistoryView.vue`
  - RDI 顶层历史数据深模块，只接收设备 ID，内部封装默认首刷、时间范围、多序列、统计与 CSV/Excel 导出。
- `RdiDeviceDetailsView.vue`
  - RDI 客户化只读详细信息入口，读取设备配置快照并集中展示设备基础信息、安装资料、联系人、客户、维护和保修信息，同时复用 `useRdiTelemetry` / `RdiTelemetrySummary` 展示 T1/T2、输入节点、输出节点、LED 和电量当前值；不提供保存或命令下发动作。
- `rdi/composables/useRdiTemperatureUnit.ts`
  - RDI 实时状态与历史模块共享的 C/F 温标状态，首次读取并持续写回本地偏好，避免跨 tab 温标漂移。

## 重点文件

- `RdiDeviceOperationsView.vue`
  - RDI 高风险业务视图，通常包含较强的兼容和展示逻辑。
- `RdiDeviceHistoryView.vue`
  - RDI 客户化 `History Data` tab 的专用入口；不依赖 ThingsVis 模板图表，变更时应优先验证默认最近 1 小时、设备切换重载和导出动作。
- `RdiDeviceDetailsView.vue`
  - RDI 客户化 `Detailed Information` tab 的专用入口；顶层 `system_info` 优先、旧 `extra_fields` 兼容，在线状态优先使用父详情实时值，遥测轮询复用公共 composable。变更时应确认四语种字段标签、设备切换旧响应隔离、轮询清理与只读边界。
- `command-delivery.vue`
  - 设备命令下发入口，直接影响设备控制链路。当前明确分开普通异步下发、单设备在线“等待设备响应”和 Expected Message；在线模式显示 message ID、设备/下发/超时结果、响应、错误和耗时，但真实设备正确性仍以运行证据为准。
- `device-status.vue`
  - 状态历史相关视图。
- `device-diagnosis.vue`
  - 设备诊断与调试日志视图，承接诊断统计、失败记录和调试报文开关。
- `give-an-alarm.vue`
  - 设备告警历史与告警规则双区域，包含确认、复位、备注维护等运维动作。
- `stats.vue`
  - 当前实现是“属性下发配置 + 下发日志”双区域，文件名与“统计摘要”语义并不完全一致。
- `device-analysis.vue`
  - 当前实现是“子设备关系管理”视图，保留了历史 `analysis` 命名，维护时要按父子设备管理理解其职责。
- `telemetry-chart.vue`
  - 遥测图表入口，与数据查询和展示性能都有关。
- `telemetry/telemetry.vue`
  - 遥测总视图，已在本轮继续补中文注释和静态审查建议。

## RDI 详细页的“已保存参数设定”边界（本轮）

`RdiDeviceDetailsView.vue` 现在在只读 `Detailed Information` tab 中增加了
`Configured Parameter Settings / 已保存参数设定` 摘要卡。该卡的语义和数据边界必须与实时遥测区分开：

- 数据来源是 `rdiDeviceConfig(deviceId)` 返回的平台保存配置快照（包括后端物化的默认值）中的 `config`，不是设备上报的当前配置、命令 ACK，也不能证明现场设备已经应用这些值。
- 摘要覆盖上报间隔、T1/T2 开关/温度范围/触发延时、Input Node 1/2 模式与延时、Output Node 01 告警/正常电平与延时，以及各类告警通知开关。
- 温度阈值跟随当前 C/F 选择；非法或不可解析的数字统一显示 `--`；Output Node 01 的两个延时相等时合并为单一 `Trigger Effective Time`，避免重复展示。
- 参数摘要不读取或输出告警收件邮箱、`send_target` 或任意原始 `field_setting`，也不提供保存、下发或其他命令入口。独立的基础资料区仍按既有 `system_info` 约定展示安装和联系人字段，不能把它与告警收件人混同。
- `RdiTelemetrySummary` 仍展示实时 T1/T2、输入/输出节点、LED 和电量等当前值；它与保存参数摘要是两套不同证据，维护时不要用实时值替换快照语义。

本轮只完成源码、测试源码和文档的静态对齐；没有运行构建、编译、浏览器、真实设备或后端接口验收，因此上述内容不等同于运行级验收结论。

## 当前静态审查结论

### 发现的问题

- 多个 tab 同时依赖设备 ID、详情对象、在线状态和刷新信号，若 props/emit 契约不清，容易出现联动回归。
- 遥测、RDI、命令下发等模块各自包含较强业务逻辑，后续继续叠加功能时容易变成大文件。
- 诊断、告警等运维视图已经开始承载真实副作用操作，但对应的状态、错误反馈和权限边界仍偏分散。
- 父级 README 如果过薄，维护者很难快速判断哪个模块属于高风险设备行为面。
- `stats.vue` 与 `device-analysis.vue` 的文件名和真实职责存在语义偏差，阅读入口容易误导。
- `RdiDeviceOperationsView.vue` 已经演进成“配置 + 遥测 + 历史 + 命令 + 分享”的复合操作视图，设备切换与轮询链路的状态一致性风险明显高于普通 tab。

### 改进方案

- 继续为高价值模块补文件头和目录 README，明确 props、emit、接口依赖和副作用边界。
- 将模块内的业务转换逻辑逐步下沉到 composable 或 service helper，避免页面脚本持续膨胀。
- 对遥测、命令下发和 RDI 这些高风险模块，优先做小步、可解释的静态收敛。
- 对诊断、告警这类运维视图，逐步把查询、轮询、动作回调与展示层拆开。
- 对历史命名与真实职责不一致的模块，优先补说明文档，再评估是否值得做低风险重命名。
- 对纯占位模块，尽快在文档中标注“预留/未实现”状态，避免使用者形成错误预期。
- 对 `RdiDeviceOperationsView.vue` 这类复合操作视图，优先维持“页面只编排、能力下沉 composable”的边界，避免回流成超大脚本文件。

### 建议实施步骤

1. 先完成高热点模块的中文化和静态审查建议补齐。
2. 再识别重复的 props/emit、数据转换和刷新逻辑，逐步提炼 helper。
3. 静态批次结束后，再统一验证设备详情页的 tab 切换、实时状态和指令链路。

### 预期效果

- 设备详情各 tab 的职责边界更清晰。
- 后续新增或下线模块时更容易定位落点和影响面。
- GitHub 浏览者能更快建立设备详情模块树的整体认知。
