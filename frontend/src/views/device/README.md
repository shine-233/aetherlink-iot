# 设备视图目录

## 目录定位

`frontend/src/views/device` 是 AetherLink IoT 前端设备域的核心页面集合，覆盖设备接入、设备列表、设备详情、设备配置、模板/物模型、分组、分享、服务接入、RDI 等主要用户流程。

如果把设备域按“生命周期”理解，这个目录基本覆盖了下面几条主线：

1. 发现与接入：
   - `manage/`
   - `service-access/`
   - `service-details/`
2. 配置与模板：
   - `config/`
   - `config-detail/`
   - `config-edit/`
   - `template/`
3. 运行与观测：
   - `details/`
   - `details-child/`
   - `device-details-app/`
4. 组织与协作：
   - `grouping/`
   - `grouping-details/`
   - `share/`
   - `shared-with-me/`
5. 扩展与 RDI：
   - `details/modules/rdi/`
   - `config/modules/`
   - `config-detail/modules/`

## 子目录与页面/API 关系

- `manage/`
  - 设备列表与接入总入口。
  - 主要依赖 `src/service/api/device.ts`、`src/service/api/rdi.ts`。
  - 高风险点：筛选参数、在线状态订阅、RDI 激活 payload、服务接入跳转参数。
- `details/`
  - 单设备详情壳层与多 tab 功能视图集合。
  - 主要依赖 `src/service/api/device.ts`、WebSocket 在线状态链路、ThingsVis 模板缓存工具。
  - 高风险点：路由 `d_id`、tab 显隐规划、在线状态兼容字段、刷新重挂载逻辑。
- `details-child/`
  - 子设备详情页壳层，和主详情页共享大量 tab 能力，但交互边界稍轻。
  - 主要依赖 `src/service/api/device.ts` 与设备 store。
- `config/`、`config-detail/`、`config-edit/`
  - 设备配置列表、配置详情、配置编辑。
  - 主要依赖 `src/service/api/device.ts`、`src/service/api/device-template-model.ts`。
  - 高风险点：配置字段和后端契约高度耦合，修改默认值或字段名时要同步核对后端。
- `template/`
  - 物模型流程总入口。
  - 主要依赖 `src/service/api/device-template-model.ts`、`device.ts`。
  - 高风险点：属性、命令、事件、图表配置的 payload 结构。
- `grouping/`、`grouping-details/`
  - 分组视图和分组详情。
  - 主要依赖 `device.ts` 中的分组相关接口。
- `share/`、`shared-with-me/`
  - 分享设备与被分享设备视图。
  - 高风险点：权限、跨租户可见性、分享状态刷新。
- `service-access/`、`service-details/`
  - 协议/服务接入入口与服务接入点详情。
  - 主要依赖 `device.ts`、插件和服务接入相关 API。

## 重点文件

- `manage/index.vue`
  - 设备域最高频入口之一，承担列表、筛选、在线状态和接入流程编排。
- `details/index.vue`
  - 设备详情壳层，负责设备上下文装配和 tab 规划。
- `details-child/index.vue`
  - 子设备详情壳层，负责轻量详情编排和状态同步。
- `template/index.vue`
  - 模板与物模型流程总入口。
- `config-detail/modules/data-handle.vue`
  - 配置详情中契约较重的处理模块之一。

## 当前静态审查结论

### 发现的问题

- 设备域大型页面较多，`manage/index.vue`、`details/index.vue`、`template/index.vue` 这类壳层容易同时承接路由、请求、状态、弹窗与子模块编排。
- 同一类字段兼容逻辑经常在多个页面重复出现，例如设备号、在线状态、分享状态、模板能力、RDI 能力等。
- 目录 README 虽然已经存在，但此前更偏总览，尚不足以帮助维护者快速定位“哪个页面负责哪条设备生命周期链路”。

### 改进方案

- 按生命周期梳理页面职责，并把重复的设备状态映射、查询参数拼装、payload 生成逐步下沉到 composable 或 service helper。
- 对高复杂度壳层页面，优先拆“纯数据归一化”“tab/步骤规划”“副作用订阅”三类逻辑，而不是一上来就拆 UI。
- 继续为重点页面补中文文件头与静态审查说明，让 README 与源码注释互相校验。

### 建议实施步骤

1. 继续优先处理 `manage/index.vue`、`details/index.vue`、`details-child/index.vue`、`template/index.vue` 这些真实业务壳层。
2. 再梳理设备配置、模板和 RDI 子目录里的重复字段映射与表单 payload 逻辑。
3. 最后统一补强设备域目录下仍偏薄弱的 README 和相邻模块说明。

### 预期效果

- GitHub 浏览者可以先从 README 快速理解设备生命周期地图，再进入具体源码。
- 维护者更容易判断“该改页面壳层、子模块、composable 还是 API wrapper”。
- 设备域后续继续重构时，影响面和风险点会更容易被提前识别。
