# PHASE-D-D10 交付说明(模板市场运营化)

> 分支 `phase-d/d10`(基线 main@ac72114),主会话实现。

## 完成清单

1. **行业分类目录 API** `GET /device/template/market/catalog`:租户内 distinct type_key + 模板数 + 累计导出数(浏览页 tab 数据源)。
2. **按行业打包导出** `GET /device/template/market/bundle?type_key=X`(空=全量):逐模板复用既有 ExportDeviceTemplate 契约(描述符可直接回放 import),JSON 信封 base64 载荷,前端解码为文件下载;curl 可 jq 解码。
3. **下载计数**:75.sql 新增 device_templates.download_count;ExportDeviceTemplate 单点计数(单个与打包共用,不双计);目录聚合 SUM 展示热度。
4. **路由/casbin**:market/catalog、market/bundle 登记(75.sql,SA/TA)。
5. **前端** `/market/browse`(elegant 路由自动生成):分类 tab(全部/各行业+计数)、模板卡片(版本/描述/分类/热度)、打包下载按钮、n-upload 导入(JSON → /device/template/import,幂等);market.ts 扩展 catalog/bundle/importDeviceTemplate/getLocalTemplateList + base64 下载助手;i18n ×4。

## 证据
- 后端:`go build ./...` ✓;dal/service/api/router 四包 `go test` ok(含既有 template market 257 用例零回退)。
- 前端:typecheck 0 错误;`pnpm build` 成功(路由生成 `/market/browse`)。

## 预期冲突点
- `router/apps/device.go` 模板路由组有 PHASE-D-D10 标记段;`internal/service/device_template.go` ExportDeviceTemplate 有计数标记段;i18n page.json ×4 追加键。

## 未尽事项(留集成)
- 运行期 E2E:建模板→打包含计数断言→另一租户导入幂等(纳入 36_template_market 套件扩展);
- 下载计数的真实 zip 二进制端点(当前 base64 信封足够 MVP;直接 zip 需处理前端鉴权下载);
- 远端市场(market_client)与本地目录的统一检索(远期)。
