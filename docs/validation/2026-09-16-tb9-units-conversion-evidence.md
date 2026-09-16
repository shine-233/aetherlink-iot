# TB-9 单位换算：内核 + 后端接线证据

> 日期：2026-09-16
> 仓库：`aetherlink-iot`，提交 `59621c2`（内核）→ `5fde940`（接线）
> 对应路线图：§7.1 TB-9（对标 ThingsBoard 4.1 头条能力 Units Conversion）
> 上游依据：`AetherLink-竞品全量核对与替代差距分析-20260916.md`（TB-9 此前**从未立项**）

---

## 一、缺陷/缺口回顾

全量核对时发现：ThingsBoard 4.1 的头条能力 **Units Conversion** 在本地**完全不存在**——
`device_model_telemetry.unit` / `device_model_attributes.unit` 只是自由文本 `varchar(50)`，
后端与前端 `0 命中`任何换算逻辑。

按 §1.0 缺口类型，这属于 `未实现`，且**路线图从未为它立项**（本轮补入 TB-9）。

---

## 二、交付物

### 2.1 `pkg/units` 纯逻辑内核（`59621c2`）

每单位声明「所属量纲 + 到基准单位的线性变换 `base = value*Scale + Offset`」，
换算一律经基准单位中转。这样**带偏移**的温度与**纯比例**的长度共用同一条通路。

覆盖：12 量纲、60+ 单位、别名索引、metric/imperial 代表单位。

**核心不变量是 fail closed，而不是"算得准"**：

| 情形 | 行为 |
| --- | --- |
| 未知单位符号 | 报 `ErrUnknownUnit`，**不**当成"原样返回" |
| 量纲不符（m → kg） | 报 `ErrDimensionMismatch` |
| 输入 NaN / ±Inf | 报 `ErrInvalidValue` |
| 同单位 | 逐位原样返回（避免 Offset 引入浮点抖动） |
| `ConvertSeries` | 全有或全无，避免产出混合单位的序列 |

理由：静默兜底会让"没换算"看起来像"换算成功"——在仪表盘上是**显示错数**，比报错危险得多。

### 2.2 接线到分析查询与导出（`5fde940`）

请求新增两个参数（`POST /api/v1/telemetry/analysis` 与 `.../analysis/export` 同步支持，
避免"界面看到的"与"导出的"单位不一致）：

- `unit`：遥测键的源单位符号（如 `°C` / `kPa` / `m3/h`）
- `unit_system`：目标制式 `metric` / `imperial`；**为空即不换算，行为与改动前逐位一致**

**不变量一：换算必须在聚合与对比之前完成。**
放在之后的话，`delta` 与百分比会停留在源单位，与已换算的当前/基线值不是同一把尺子——
数值换了单位、变化量没换，这种错误在界面上完全看不出来。

**不变量二：该拒绝时绝不静默返回未换算的值。**

| 情形 | 行为 |
| --- | --- |
| `count` 聚合 | 不换算 + `unit_reason`（计数量不是物理量） |
| 未提供 `unit` | 不换算 + `unit_reason` |
| 单位不可识别 | 不换算 + `unit_reason` |
| `sum` 遇带偏移单位（温度） | 不换算 + `unit_reason` |
| 当前/基线任一条换算失败 | 两条都保持原样 + `unit_reason` |

`sum` + 偏移单位为何要拒绝：`sum(convert(x)) ≠ convert(sum(x))`（3 个 0°C 之和换算成 °F 是 96，
而 0°C 换算成 °F 是 32）。温度求和在物理上本就没有意义，与其给一个取决于实现顺序的数，不如明确拒绝。

---

## 三、验证证据

### 3.1 编译

```
cd backend && GOTOOLCHAIN=local go build -p 1 ./...
BUILD_EXIT=0
```

### 3.2 内核

```
go test -p 1 ./pkg/units/ -count=1
ok  aetherlink-iot/backend/pkg/units  0.419s
```
39 个用例（12 顶层 + 27 子用例），含注册表不变量、7 组温度换算、13 组比例换算、
往返稳定性、同单位逐位相等、别名解析、以及 6 组 fail-closed 负向对照。

### 3.3 接线

```
go test -p 1 ./internal/service/ -run 'TelemetryUnit|RunTelemetryAnalysis|ConvertTelemetryAnalysisWindows|ResolveTelemetryUnitPlan' -count=1 -v
```
```
--- PASS: TestResolveTelemetryUnitPlan (13 子用例)
--- PASS: TestConvertTelemetryAnalysisWindows (5 子用例)
--- PASS: TestRunTelemetryAnalysisConvertsBeforeComparison
--- PASS: TestRunTelemetryAnalysisWithoutUnitSystemIsUnchanged
--- PASS: TestRunTelemetryAnalysisReportsUnitReasonInsteadOfSilentPassThrough (4 子用例)
ok  aetherlink-iot/backend/internal/service  0.239s
```

**最关键的一条是可判别断言**，而不是"确认换算被调用过"：
用摄氏→华氏（当前窗口均值 10°C、基线 20°C），
- 先换算再对比 → 50°F 对 68°F → `percent = -26.4705882%`
- 先对比再换算 → `percent` 会停在 `-50%`

两者数值不同，因此这条断言**能真正区分换算顺序对不对**。

### 3.4 回归

```
go test -p 1 ./internal/service/ ./internal/api/ -count=1
ok  aetherlink-iot/backend/internal/service  2.421s
ok  aetherlink-iot/backend/internal/api      0.295s
TEST_EXIT=0
```

---

## 四、仍未闭环（如实记录）

1. **源单位仍由调用方提供**，未从设备配置链服务端解析。
   完整链路是 `device → device_config → device_template_id → device_model_telemetry.unit`（两跳查询）。
   解析来源替换后，换算与拒绝语义不变（见 `telemetry_analysis_units.go` 的「重构建议」）。
2. **看板 / 前端消费方未接**——后端能力已通，但界面上还没有"切换单位制式"的入口，
   因此按 §1.0 仍记 `未接线`，**不宣称 TB-9 完成**。
3. `unit` 列仍是自由文本，建议改受控白名单（否则用户可写入任意字符串，
   换算时只能落到 `unit_reason`）。

---

## 五、已知取舍

- 摄氏↔华氏需经 `value*Scale + Offset` 往返，而 `5/9` 无法被二进制浮点精确表示：
  `0°C → °F` 得到 `31.999999999999936`（相对误差 2e-15）。这是该通用形式的固有代价，
  工程单位换算不需要逐位相等；测试用相对容差 `1e-9` 断言，而不是改实现去凑整数。
