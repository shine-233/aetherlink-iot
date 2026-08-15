# Product Service APIs

## 目录职责

`frontend/src/service/product` 封装产品、OTA 和升级包相关接口，是产品页面与后端产品管理能力之间的前端契约层。

## 文件关系

- `list.ts` 提供产品/配置列表查询入口。
- `update-ota.ts` 和 `update-package.ts` 分别服务 OTA 流程与升级包管理页面。
- 页面和 store 应通过这些 wrapper 访问后端，不应绕过 `src/service/request` 新建客户端。

## 重点文件

- `update-ota.ts`: OTA 任务、状态和详情查询的主要入口。
- `update-package.ts`: 升级包上传、列表和维护入口。
- `list.ts`: 产品选择和配置选择的共享数据源。

## 审查建议

变更时优先核对后端路由、请求参数名和响应 envelope，尤其是 OTA 状态字段和文件上传参数。若接口继续增多，建议按“产品列表、OTA、升级包”拆分类型定义并补充契约测试。
