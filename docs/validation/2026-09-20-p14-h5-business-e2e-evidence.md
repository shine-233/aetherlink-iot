# P1.4 H5 完整业务 E2E 证据（uni-app H5 真实构建产物 × 真实后端）（2026-09-20）

> 对应路线图：§1.2 P1.4 移动端控制与通知——门禁「至少 Android/H5 一条完整业务 E2E」的 H5 侧
> 被测产物：`active/mobile-app-uni/dist/build/h5`（uni-app Vue3 构建产物，非 dev server、非桩）
> 被测链路：H5 页面 → uni.request → http://127.0.0.1:9999/api/v1（真实后端活栈 + 真实 PG）
> 自动化用例：`automation_tests/e2e/35_p14_h5_mobile_business.spec.js`（Playwright 真实 Edge）
> 运行结果：**3/3 passed（8.8s）**

---

## 一、背景

P1.4 门禁要求「至少 Android/H5 一条完整业务 E2E」。此前移动端只有后端接口级
契约（`mobile_e2e_test.go` 9/9）与 H5/微信小程序构建通过，**没有一条从 UI 出发的
端到端业务证据**。本轮补齐：把 uni-app H5 构建产物经预览代理伺服（9725 端口，
后端 `cors.allowed_origins` 白名单内 origin，`x-token` 在 `cors.allowed_headers`），
真实 Edge 浏览器跑完整业务流。

## 二、被测业务流（全部真实数据，无 UI 桩）

1. **登录**——错误密码提交 → 页面渲染错误文案（`.login .err`）；正确凭证提交 →
   路由跳转 `#/pages/device/list`，设备列表渲染。
2. **设备列表**——`ensureDeviceWithTelemetry` 现场种子的设备按名称可见、在线
   徽章（`.badge`）渲染；列表为 `GET /device` 真实分页（最新优先排序落第一页）。
3. **遥测闭环**——点击设备行展开「最新遥测」面板，断言真实键值
   `temperature_1 = 25.5`（数据来自 `publishSimulatedTelemetryAndReadCurrent`
   的真实模拟遥测发布 + `GET /telemetry/datas/current/:id` 读取回，非写死文本）。

## 三、实测输出

```
ok 1 [msedge] › 35_p14_h5_mobile_business.spec.js:91 › 点击设备展开最新遥测面板并渲染真实键值 (878ms)
ok 2 [msedge] › 35_p14_h5_mobile_business.spec.js:71 › H5 登录：错误密码被拒展示错误，正确凭证进入设备列表 (886ms)
ok 3 [msedge] › 35_p14_h5_mobile_business.spec.js:82 › 设备列表展示种子设备与在线徽章 (529ms)
3 passed (8.8s)
```

运行方式（webServer 即既有预览代理，零新增服务代码）：

```
PLAYWRIGHT_USE_PREVIEW_PROXY=1 \
PREVIEW_DIST_DIR=C:/Users/Zz/Documents/projects/active/mobile-app-uni/dist/build/h5 \
npx playwright test e2e/35_p14_h5_mobile_business.spec.js
```

配套最小改动：`scripts/serve_preview_with_api_proxy.js` 支持
`PREVIEW_DIST_DIR` 环境变量把预览静态目录指向移动端 H5 产物（缺省仍为主前端
dist，向后兼容）。

## 四、踩坑记录

uni-app H5 把 `view/input` 编译为 `uni-view/uni-input` 自定义元素：`.field` 类落在
`uni-input` 上，可填充的原生 `<input>` 在其内部——Playwright 选择器必须下钻
`.login uni-input.field → input` 再 fill，直接 `.login input.field` 永远等不到元素。

## 五、结论与边界

- P1.4 门禁「至少 Android/H5 一条完整业务 E2E」的 **H5 侧已满足**；
- P1.4 剩余不变：FCM/APNs 真机联调、Android/iOS 原生商店上架（需签名证书与
  真机，环境阻塞）；进程内幂等存储重启即失（已知边界）。
