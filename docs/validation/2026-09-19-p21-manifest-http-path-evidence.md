# P2.1 manifest 注册 HTTP 运行期路径证据（2026-09-19）

> 对应路线图：§1.2 P2.1 剩余子项「manifest 注册的 HTTP 运行期路径」
> （原注记：「服务层依赖无法离线编译，测试以源码交付」——模块缓存恢复后补上）
> 结论：**该子项闭环**。真实 Gin 引擎 + 真实 PostgreSQL，走生产同一条注册路径
> （POST /api/v1/plugins → PluginRegistryService.Create → pluginsdk 校验/验签 → 落库）。

## 一、测试（router/apps/plugin_registry_http_test.go）

签名用 **pluginsdk 本尊生成**（`SignManifest`，Ed25519，不在测试里复刻签名算法），
受信厂商公钥经 `plugin.trusted_vendor_keys`（viper，测试内注入并还原）。
缺 `AETHERLINK_TEST_PSQL_DSN` 一律 Skip（不当通过）。五个用例一次跑完：

| # | 场景 | 结果 |
| --- | --- | --- |
| 1 | 合法厂商签名 manifest | 注册成功（200），**原始 manifest JSON 持久化**（DB 直查核对） |
| 2 | 篡改签名（末 8 位替换） | 100002 拒绝——「坏的签名比没有更危险」 |
| 3 | 未受信厂商（vendor-rogue） | 100002 拒绝 |
| 4 | 非法 manifest（transport 白名单外 http） | 100002 拒绝 |
| 5 | 未签名 manifest | 注册成功（D9 兼容过渡路径保持） |

`ok aetherlink-iot/backend/router/apps`；整包回归（含 mobile e2e 等）全绿。

## 二、工程记录（诚实入档）

测试搭建过程踩的坑：① 首版漏加统一响应中间件（`response.Handler.Middleware()`），
`c.Set("data")` 无渲染出口、响应体为空——mobile_e2e 的同款装配照抄即好；
② gen 单例的 `query.SetDefault` 二次调用在 nil 场景会 panic，cleanup 只还原
`global.DB`（与 mobile_e2e 同款）；③ 名称生成函数在请求与断言处各调一次导致
两个不同名字——捕获成变量。三处均已修正。

## 三、P2.1 剩余（如实）

真实外部协议适配器（CAN/BACnet/BLE/LoRaWAN）仍按客户需求接入（路线图原注），
不在本地可闭环范围。
