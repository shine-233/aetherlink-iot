# ThingsVis 宿主桥接说明

## 目录职责

该目录承载 AetherLink 前端嵌入 ThingsVis runtime 所需的宿主侧桥接代码，负责 iframe 生命周期、host/guest 消息通信、平台字段读写和历史数据补齐。

## 关键文件

- `ThingsVisWidget.vue`：主入口组件，连接 widget 初始化、保存回传、字段读写与 viewer/editor 双模式。
- `thingsvisWidgetPlatformWriteBridge.ts`：`ThingsVisWidget.vue` 的平台写入桥接模块，集中处理 `tv:platform-write` 的可信消息解析、目标设备约束、字段类型归一化后的发布和 `tv:platform-write-result` 回传。
- `thingsvisWidgetFieldHistoryBridge.ts`：`ThingsVisWidget.vue` 的历史字段桥接模块，集中处理 `__history` 字段需求扫描、buffer 预填判断与遥测历史接口结果归一化。
- `thingsvisWidgetFieldRequestBridge.ts`：`ThingsVisWidget.vue` 的字段请求桥接模块，集中处理 `thingsvis:requestFieldData` 的可信消息解析、目标设备约束、字段响应装配和回推前分流。
- `ThingsVisAppFrame.vue`：ThingsVis iframe 宿主入口，负责 iframe 生命周期、可信消息接入、viewer/editor 分流和桥接模块装配。
- `thingsvisAppFrameLifecycle.ts`：`ThingsVisAppFrame.vue` 的生命周期编排模块，集中处理 token/url 初始化、`tv:ready` 调度、`tv:init` 发送、viewer/editor 后置动作和卸载清理。
- `thingsvisFrameTransportBridge.ts`：`ThingsVisAppFrame.vue` 的 transport/message 壳模块，集中处理 `targetOrigin` 解析、可信消息判定、宿主 `postMessage` 出口和 `tv:platform-data` 双路回推。
- `hostBridge.ts`：`postMessage` 基础校验与监听封装。
- `thingsvisFrameBridge.ts`：iframe URL、`targetOrigin`、可信消息来源校验和宿主侧消息发送封装。
- `thingsvisFrameMessageDispatcher.ts`：可信 iframe 消息分发器，集中维护 `tv:*` / `thingsvis:*` 消息类型到宿主 handler 的映射，不承担来源校验或业务副作用。
- `thingsvisDashboardConfigBridge.ts`：dashboard 配置深拷贝归一化、canvas 背景兼容和 ThingsVis 401 重试处理。
- `thingsvisPlatformWriteBridge.ts`：平台写入请求解析、字段类型判定、命令参数构造和错误文案整理。
- `thingsvisPlatformWriteReplyBridge.ts`：平台写入结果回推编排模块，串联设备字段读取、平台发布接口、`tv:platform-write-result` 回传和错误日志策略。
- `thingsvisFieldRequestHydrationBridge.ts`：字段读取请求补水编排模块，串联 guest 请求解析、设备绑定解析、平台字段读取、告警/RDI 元数据补齐和 `tv:platform-data` 回推。
- `fieldReadBridge.ts`、`fieldReadRequestBridge.ts`、`widgetFieldDataBridge.ts`：字段请求拆分、设备解析、实时值/告警/历史数据响应拼装。
- `thingsvisFieldHydrationBridge.ts`：解析大屏节点中的 `{{ ds.xxx... }}` 字段绑定表达式，构建平台数据源补水描述并按设备分组回推。
- `thingsvisDeviceWsBridge.ts`：设备实时遥测与在线状态 WebSocket 桥接，集中处理 token、ping、重连、字段映射和 `tv:platform-data` 回推。
- `thingsvisDeviceCatalogBridge.ts`：设备筛选项、服务选项与分组树扁平化桥接，统一兼容后端多种字段命名。
- `thingsvisPlatformDeviceCatalogOrchestrator.ts`：平台设备目录编排模块，集中管理分组缓存、设备配置到物模型映射、物模型字段/预设缓存、按组加载、按 ID 查找、分页搜索和设备字段加载。
- `thingsvisDeviceConfigTemplateMapCacheBridge.ts`：设备配置到 ThingsVis 物模型 ID 的映射缓存模块，集中处理配置字段别名、并发加载去重和失败时空 Map 降级。
- `thingsvisTemplateAssetCacheBridge.ts`：ThingsVis 物模型字段与 widget 预设缓存模块，集中处理物模型 entry/preset 的 cache、in-flight 去重和物模型资产批量加载。
- `searchDevicesPagedBridge.ts`：设备分页搜索请求归一化、筛选参数拼装和响应结构生成。
- `thingsvisHostSaveBridge.ts`：宿主保存前的数据源清理和 dashboard 更新 payload 生成。
- `thingsvisHostActionsBridge.ts`：宿主动作编排模块，集中处理 `tv:save`、`tv:preview`、`tv:publish` 对应的保存接口、预览跳转、发布接口和用户提示。
- `thingsvisInitSchedulerBridge.ts`：iframe 初始化调度器，集中处理 `tv:ready` 防抖、重复签名跳过、初始化并发保护、指数退避重试和 iframe reload reset。
- `thingsvisViewerHydrationBridge.ts`：viewer 模式平台数据补水模块，集中管理补水 timer、in-flight/done 状态、dashboard 配置缓存、接口回退和按设备字段补水。
- `thingsvisEditorPrefetchOrchestrator.ts`：editor 模式初始化后的设备与字段预取编排器，集中处理 descriptor 收集、设备预加载、运行时注册和字段补水回推。

## 维护提示

- `tv:*` / `thingsvis:*` 消息类型、provider、saveTarget 都属于跨系统契约，修改前要先确认 guest runtime 兼容性。
- iframe 消息发送必须使用明确的 `targetOrigin`，不要把 ThingsVis 通信改成 wildcard origin。
- 历史字段、告警派生字段和实时字段走的是不同数据通道，排查问题时要分开看。
- `ThingsVisAppFrame.vue` 仍然是 iframe 宿主入口，但设备目录编排、初始化调度、viewer 补水、editor 预取、宿主动作、消息分发、生命周期编排和 transport/message 壳已经分别迁入独立模块；后续若继续拆分，优先评估运行时设备字段同步或平台写入结果回推的更细协作者，而不是把 transport 再塞回父组件。
- dashboard config 会同时影响初始化、保存、预览和 viewer 补水，修改 `nodes`、`dataSources`、`variables` 或 `canvas` 时要同步核对这些路径。
- 平台写入链路同时覆盖 telemetry、attribute 和 command，新增写入类型前要先确认 ThingsVis payload、设备字段模型和后端发布接口是否一致。
- `ThingsVisWidget.vue` 当前已把 widget 侧 `tv:platform-write` 下沉到 `thingsvisWidgetPlatformWriteBridge.ts`；后续若继续拆分，应优先考虑字段历史补齐或 viewer/editor 生命周期，而不是把写入细节重新塞回主组件。
- `ThingsVisWidget.vue` 当前已把历史字段扫描与历史接口读取下沉到 `thingsvisWidgetFieldHistoryBridge.ts`；后续若继续拆分，应优先考虑字段请求装配层或 viewer/editor 生命周期，而不是再把补水细节塞回主组件。
- `ThingsVisWidget.vue` 当前已把 `thingsvis:requestFieldData` 下沉到 `thingsvisWidgetFieldRequestBridge.ts`；后续若继续拆分，应优先考虑 viewer/editor 生命周期、client ready 装配或 runtime-device 同步壳。

## 当前重构记录

- 问题：`ThingsVisAppFrame.vue` 曾同时维护设备分组、设备配置物模型映射、物模型字段/预设缓存、分页搜索和 iframe 消息响应，导致平台 API 适配与宿主协议混在同一文件。
- 改进方案：新增 `thingsvisPlatformDeviceCatalogOrchestrator.ts`，以 `loadGroups`、`loadFilterOptions`、`loadDeviceById`、`loadDevicesByGroup`、`searchDevicesPaged`、`loadDeviceFields` 和 `reset` 作为小接口隐藏目录编排细节。
- 实施结果：AppFrame 只保留 postMessage 外壳、活跃设备注册和 WebSocket 运行时副作用；目录缓存、并发去重、物模型水合、group fallback 和搜索参数拼装都集中到新模块。
- 预期效果：继续降低 AppFrame 复杂度，让下一步收敛保存编排、viewer 补水或消息分发时不再被设备目录细节牵连。
- 问题：`tv:ready`、`READY`、iframe load 和初始化失败重试曾在 AppFrame 内部维护多组 timer 与状态变量，容易在后续修改中误伤重复 ready 去重或 backoff 行为。
- 改进方案：新增 `thingsvisInitSchedulerBridge.ts`，以 `schedule`、`resetAfterFrameLoad`、`dispose` 作为小接口隐藏初始化状态机，AppFrame 只注入 `canInit`、`getSignature` 和 `runInit`。
- 实施结果：初始化签名去重、并发保护、debounce、retry timer、退避次数和卸载清理都集中到调度器；dashboard 加载、`tv:init` 发送、viewer/editor 后置动作仍留在 AppFrame。
- 预期效果：继续压缩入口组件，同时降低重复 ready、iframe reload 和卸载清理路径的维护风险。
- 问题：viewer 模式补水曾在 AppFrame 内部维护 timer、in-flight/done、dashboard config cache 和按设备字段补水流程，和 iframe 消息处理混在一起。
- 改进方案：新增 `thingsvisViewerHydrationBridge.ts`，以 `schedule`、`reset`、`dispose` 作为小接口隐藏 viewer 补水状态机，AppFrame 只注入字段读取、WebSocket ensure 和 `postPlatformData`。
- 实施结果：viewer dashboard 配置缓存、schema 直传优先、接口回退、补水去重和卸载清理集中到新模块；editor 预取、实时 WebSocket 和 postMessage 副作用仍留在 AppFrame。
- 预期效果：进一步压缩入口组件，降低 viewer ready、dashboard 预加载和字段补水路径互相影响的风险。
- 问题：保存、预览和发布动作曾直接散落在 AppFrame 中，导致入口组件同时知道保存 payload、路由跳转、发布接口、提示文案和错误日志。
- 改进方案：新增 `thingsvisHostActionsBridge.ts`，以 `save`、`preview`、`publish` 作为小接口隐藏宿主动作编排，AppFrame 只负责把 `tv:*` 消息转交给对应动作。
- 实施结果：保存 payload 构造继续复用 `thingsvisHostSaveBridge.ts`，预览跳转、发布接口调用、成功/失败提示和保存成功事件都集中到 host actions 模块。
- 预期效果：继续压缩 AppFrame，并让后续消息分发拆分可以只关注消息类型到 handler 的映射，不再夹杂宿主动作细节。
- 问题：`tv:*`、`thingsvis:*`、`LOADED`、`READY` 等消息类型映射曾直接散落在 AppFrame 中，入口组件既维护协议表，又维护 handler 副作用。
- 改进方案：新增 `thingsvisFrameMessageDispatcher.ts`，以 `dispatch` 和 `hasHandler` 作为小接口隐藏消息分发表；AppFrame 只注入保存、平台写入、设备目录、字段读取和生命周期 handler。
- 实施结果：消息来源校验仍留在 `hostBridge.ts` / `thingsvisFrameBridge.ts` 与 AppFrame 的 iframe 上下文内，真实业务副作用仍留在对应 bridge 或 handler；新模块只负责可信消息的类型查找与调用。
- 预期效果：新增或调整 iframe 消息类型时可以先看 dispatcher 的协议表，减少在入口组件中穿插查找分发逻辑和业务逻辑的维护成本。
- 问题：editor 模式在 `tv:init` 后的设备预取、字段预取和回推曾由 AppFrame 直接维护，和 viewer 补水、初始化调度、消息分发混在一起。
- 改进方案：新增 `thingsvisEditorPrefetchOrchestrator.ts`，通过注入 `collectConfiguredDescriptors`、`loadDeviceById`、`registerDevices`、`loadRequestedFieldData`、`postDeviceById` 和 `postPlatformData`，集中编排 editor 预取。
- 问题：平台写入结果回推曾留在 AppFrame 内部，入口组件同时解析 payload、调用发布接口、读取设备字段、发送 `tv:platform-write-result` 和处理错误日志。
- 改进方案：新增 `thingsvisPlatformWriteReplyBridge.ts`，以 `handlePlatformWrite` 作为小接口隐藏写入编排；AppFrame 只注入设备 ID 解析、设备字段读取、发布 API、明确 targetOrigin 的 postMessage 回调和 logger。
- 实施结果：requestId 为空时不回推、成功 echo fallback、`PlatformWriteValidationError` 不写 error 日志等旧语义都集中在新 bridge 中；AppFrame 只保留 iframe/window 上下文和运行时依赖装配。
- 预期效果：继续压缩 AppFrame，并让平台写入链路的解析、发布和回推职责更容易单独审查。
- 问题：字段读取请求曾留在 AppFrame 内部，入口组件同时判断 iframe 是否可用、解析 payload、读取当前值/告警/RDI 元数据、补水并回推平台数据。
- 改进方案：新增 `thingsvisFieldRequestHydrationBridge.ts`，以 `loadRequestedFieldData` 和 `handleFieldDataRequest` 作为小接口隐藏字段请求补水；AppFrame 只注入 iframe ready 判断、运行时设备绑定、WebSocket ensure、平台 API 和 `postPlatformData`。
- 实施结果：`resolvePlatformFieldReadRequest` 的 `null` / `undefined` 语义、`dataSourceId || ''`、`deviceId || ''`、`new Set(fieldIds)` 和瞬时错误静默策略都保留在新 bridge 中。
- 预期效果：AppFrame 继续收敛为宿主壳，字段读取链路可单独审查，后续 lifecycle 清理不再被字段补水细节干扰。
- 问题：`ThingsVisAppFrame.vue` 在前几轮拆分后，仍同时保留 `tv:ready` 初始化编排和 iframe transport/message 壳，导致宿主入口还要同时关心生命周期顺序、安全边界和 `postMessage` 双向兼容。
- 改进方案：新增 `thingsvisAppFrameLifecycle.ts` 与 `thingsvisFrameTransportBridge.ts`，前者承接 token/url 初始化、dashboard preload、`tv:init`、viewer/editor 分流和卸载清理，后者承接 `targetOrigin`、可信消息解析、`postToThingsVis` 与 `postPlatformData`。
- 实施结果：`ThingsVisAppFrame.vue` 目前主要保留 bridge 装配与 iframe 宿主入口，`tv:ready` 去重、init retry、可信消息判定和 `tv:platform-data` 双路回推都迁入独立模块；当前实测父组件约 352 行，`thingsvisAppFrameLifecycle.ts` 约 220 行，`thingsvisFrameTransportBridge.ts` 约 85 行。
- 预期效果：后续继续调整 ThingsVis 宿主协议时，可以分别在 lifecycle 与 transport 层定位问题，不需要再在父组件里来回穿插追踪。
- 问题：`ThingsVisWidget.vue` 里的 `tv:platform-write` 曾同时混着可信消息解析、目标设备校验、字段类型判定、命令参数构造、平台发布和结果回推。
- 改进方案：新增 `thingsvisWidgetPlatformWriteBridge.ts`，以 `createThingsVisWidgetPlatformWriteHandler(...)` 形式集中 widget 侧平台写入桥接；主组件只注入当前 `client`、预览设备解析、字段类型图和三个发布 API。
- 实施结果：widget 侧写入链路现在统一走独立 bridge，`ThingsVisWidget.vue` 只保留 handler 装配；旧的 `tv:platform-write-result` 回包结构、command 单字段约束和缺少 `deviceId` 时的告警语义都保持不变。
- 预期效果：继续缩小 `ThingsVisWidget.vue`，后续补字段历史桥或 viewer/editor 生命周期时，不会再被平台写入细节牵连。
- 问题：`ThingsVisWidget.vue` 里的历史字段补水曾同时混着 `__history` 字段需求扫描、buffer 预填判断、节点字符串表达式解析和遥测历史接口结果归一化。
- 改进方案：新增 `thingsvisWidgetFieldHistoryBridge.ts`，以 `createThingsVisWidgetFieldHistoryBridge(...)` 形式集中 widget 侧历史字段桥接；主组件只注入当前配置、字段类型图、表达式解析器和历史 API。
- 实施结果：widget 侧历史补水链路现在统一走独立 bridge，`ThingsVisWidget.vue` 只保留给 `widgetFieldDataBridge.ts` 装配三个历史能力入口；旧的时间范围归一化、物模型字段占位设备跳过、运行时状态字段跳过和瞬时错误静默策略都保持不变。
- 预期效果：继续缩小 `ThingsVisWidget.vue`，后续收敛字段请求装配层或 viewer/editor 生命周期时，不会再被历史补水细节牵连。
- 问题：`ThingsVisWidget.vue` 里的 `thingsvis:requestFieldData` 曾同时混着可信消息解析、目标设备校验、字段响应装配和空响应短路。
- 改进方案：新增 `thingsvisWidgetFieldRequestBridge.ts`，以 `createThingsVisWidgetFieldRequestHandler(...)` 形式集中 widget 侧字段请求桥接；主组件只注入当前设备解析、历史桥能力、告警 API 和 `pushPlatformFieldData`。
- 实施结果：widget 侧字段请求现在统一走独立 bridge，`ThingsVisWidget.vue` 只保留 handler 装配；旧的 payload 兼容、设备 mismatch 拒绝和空字段结果不回推的语义都保持不变。
- 预期效果：继续缩小 `ThingsVisWidget.vue`，后续收敛 viewer/editor 生命周期或 runtime-device 同步时，不会再被字段请求装配细节牵连。
- 问题：`thingsvisPlatformDeviceCatalogOrchestrator.ts` 在目录编排之外还直接持有物模型字段、widget 预设、in-flight promise 和批量物模型资产加载状态，导致设备目录、物模型缓存和字段加载职责继续耦合。
- 改进方案：新增 `thingsvisTemplateAssetCacheBridge.ts`，以 `loadTemplateEntry`、`loadTemplatePresets`、`loadTemplateAssetsForDevices`、`reset` 作为小接口隐藏物模型资产缓存状态；目录编排器只保留设备组、设备配置映射和设备装配流程。
- 实施结果：物模型 entry/preset cache、并发去重、物模型字段读取和预设读取集中到新 bridge，`thingsvisPlatformDeviceCatalogOrchestrator.ts` 的 reset 只转调物模型缓存 reset 并清理目录相关缓存。
- 预期效果：后续继续拆设备目录时，可以分别审查“设备目录编排”和“物模型资产缓存”，减少分页搜索、按组加载、按 ID 查找之间互相牵连。
- 问题：`thingsvisPlatformDeviceCatalogOrchestrator.ts` 还直接维护 deviceConfigId/name 到 templateId 的缓存与并发 promise，和设备分组、搜索、物模型资产加载耦合在同一个编排器里。
- 改进方案：新增 `thingsvisDeviceConfigTemplateMapCacheBridge.ts`，以 `load` / `reset` 作为小接口隐藏设备配置到物模型映射缓存，保留 id/name alias、templateId alias、失败时空 Map 降级和 promise coalescing 语义。
- 实施结果：目录编排器通过 `deviceConfigTemplateMapCache.load()` 取映射，通过 `reset()` 统一清理；映射构建细节从按组加载和 `loadDeviceFields` 中剥离。
- 预期效果：进一步把 ThingsVis 设备目录拆成“分组/搜索编排”“物模型资产缓存”“设备配置到物模型映射缓存”三条可独立审查的路径。
- 实施结果：editor 预取继续保持 fire-and-forget、异常静默降级、不启动 WebSocket 的旧行为；设备成功预取后仍先注册运行时设备，再以 `__prefetch__<deviceId>` 回推 `tv:device-by-id`。
- 预期效果：AppFrame 当前进一步收敛到 iframe 生命周期和模块装配层，editor 与 viewer 的补水差异更清楚，后续排查预取问题时可以直接定位到独立编排器。
