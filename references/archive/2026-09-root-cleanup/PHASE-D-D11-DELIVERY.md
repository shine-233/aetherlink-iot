# PHASE-D-D11 交付说明(设备实时调试台)

> 分支 `phase-d/d11`(基线 main@ac72114),主会话实现。
> 范围裁决:复核发现平台已有 MQTT 调试工作台(`DeviceMqttDebugWorkbench`,内嵌接入向导页签,
> 后端 mqttdebug Manager 提供会话/订阅/发布/TTL/冷却)与 WS 实时遥测通道(`/telemetry/datas/current/ws`)。
> D11 不重复造后端,聚焦缺口:**实时上行原始帧时间线 + 命令下发与投递诊断面板**,作为新页签接入。

## 完成清单

1. **`device-debug-live.vue` 新页签**(device-tab-registry 注册 `device-debug-live`,标记段):
   - **实时上行帧**:直连平台遥测 WS 通道,设备鉴权帧 `{device_id, token}`,原始 JSON 帧时间线
     (最近 50 条,倒序),8s ping 保活、断线 3s 自动重连,连接状态徽标;与遥测图表互补——
     这里展示未加工帧,用于联调排查载荷结构问题。
   - **命令下发与投递诊断**:identify + params(JSON) → `POST /command/datas/pub`;
     `GET /command/datas/delivery/diagnostics/:id` 展示设备在线态与最近投递日志(ACK/超时可见)。
2. **i18n ×4**:custom.device_details.debugLive + page.deviceDebug.* 共 18 键。
3. 组件随页签壳层以 `id` prop 注入设备 ID(与既有页签契约一致)。

## 证据
- typecheck 0 错误;`vitest run src/views/device/details` **695 passed** 零回退;`pnpm build` 成功。

## 预期冲突点
- `device-tab-registry.ts` PHASE-D-D11 标记段;page.json/custom.json ×4 追加键。

## 未尽事项(留集成)
- 隔离栈运行期 E2E:真设备遥测 → WS 帧时间线可见;下发命令 → 投递诊断显示最新日志;
- 命令下行 ACK 的实时回显(当前依赖诊断接口,后续可接命令状态 WS)。
