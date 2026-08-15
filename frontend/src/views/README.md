# 前端视图目录

`frontend/src/views` 是 AetherLink IoT 控制台的路由页面根目录。这里的 Vue 文件通常由
`frontend/src/router` 挂载，通过 `frontend/src/service` 调用后端 API，并和 store、
composable、局部组件一起完成具体业务流程。

## 当前页面域

- `home/`：首次接入、租户上下文提示和客户引导入口。
- `device/`：设备列表、详情、物模型、配置、命令、接入、分享、分组和服务接入。
- `automation/`：场景、联动、规则编辑和自动化入口。
- `visualization/`：ThingsVis 可视化、仪表盘、编辑器、菜单嵌入和预览。
- `alarm/`：告警配置、当前告警、历史告警、通知组和通知记录。
- `dashboard/`：工作台、导航工作区、RDI 概览等首页/看板类页面。
- `management/`、`system-management-user/`、`personal-center/`：用户、角色、权限、通知、系统设置、API 凭据、系统日志、设备地图和个人资料。
- `apply/`、`product/`：插件、服务申请、产品和 OTA 相关页面。
- `device-details-app/`：独立设备详情应用入口，和常规布局下的设备详情分开维护。
- `_builtin/`：登录、异常页等内置页面。

已删除的旧壳目录不要再写入当前页面清单；例如根级 `about/`、`data-service/`、
`rule-engine/`、`manage/` 现在都不是当前路由页面。这里的根级 `manage/` 不等同于
仍被 `frontend/src/router/elegant/imports.ts` 导入的现行
`frontend/src/views/device/manage/`，后者仍属于设备管理页面域。

## 维护约定

- 新增、移动或删除页面时，同步检查 router、权限守卫、国际化 key、coverage contract 和相关测试元数据。
- 页面组件应优先通过已有 `service/api/*` 封装访问接口，不要在视图里散落请求地址、鉴权逻辑或字段转换。
- 大页面重构先拆数据映射、表单校验、payload 构造和状态清理 helper，再拆 UI 组件。
- 删除旧代码前按三证据判断：内容是否旧、引用是否只剩孤儿、是否没有真实业务入口。
- 不要把导航壳、mock 包装测试或 README 说明当成业务功能已经完成的证据。
