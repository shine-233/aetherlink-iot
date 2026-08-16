# Dependabot PR 逐项处置记录（2026-08-17）

这是一份基于 GitHub REST/CLI 当前回读和 `origin/main=57f3c5ccf6cb6dd550951d72c4be6b39e1309fe5` 当前源码/锁文件的处置记录。这里的“已覆盖”只表示当前依赖文件存在等价或更高版本；只有 `merged_at` 非空才计为该 PR 本身已经合并。关闭 PR 不能自动算作修复。

## 当前总数

- Dependabot PR 总数：**38**。
- `closed + merged`：**14**。
- `closed + 未 merged`：**24**。
- `open`：**0**。
- 标题不是 grouped update 的单依赖 PR：**25**；grouped PR：**13**。
- 因此，“28 个普通 Dependabot PR”不是当前 GitHub API 可复现的数字；当前可复核的事实是 38 个历史 Dependabot PR，且没有待合并的 open PR。
- 当前 open alerts：Dependabot **0**、CodeQL **0**、Secret Scanning **0**。

## 已合并（14 个）

| PR | 更新 | 当前证据 | 处置 |
|---:|---|---|---|
| [#1](https://github.com/shine-233/aetherlink-iot/pull/1) | autotest `lib/pq 1.10.9 -> 1.12.3` | `backend/cmd/aetherlink-device-autotest/go.mod` 为 `1.12.3` | 已完成，不重开 |
| [#2](https://github.com/shine-233/aetherlink-iot/pull/2) | autotest `testify 1.8.4 -> 1.11.1` | 当前 `go.mod` 为 `1.11.1` | 已完成 |
| [#3](https://github.com/shine-233/aetherlink-iot/pull/3) | frontend nginx `1.27-alpine -> 1.31-alpine` | `frontend/Dockerfile` 为 `nginx:1.31-alpine` | 已完成 |
| [#4](https://github.com/shine-233/aetherlink-iot/pull/4) | autotest `zap 1.26.0 -> 1.28.0` | 当前 `go.mod` 为 `1.28.0` | 已完成 |
| [#5](https://github.com/shine-233/aetherlink-iot/pull/5) | backend Alpine `3.20 -> 3.24` | `backend/Dockerfile` 为 `alpine:3.24` | 已完成 |
| [#9](https://github.com/shine-233/aetherlink-iot/pull/9) | automation minor/patch group | 当前 automation lock 已含对应 axios/Playwright 更新 | 已完成 |
| [#11](https://github.com/shine-233/aetherlink-iot/pull/11) | automation `mocha 10.8.2 -> 11.8.0` | `automation_tests/package.json`/lock 为 `11.8.0` | 已完成 |
| [#13](https://github.com/shine-233/aetherlink-iot/pull/13) | broker Go group，16 项 | 当前 `mqtt-broker/go.mod` 已落地该组结果 | 已完成 |
| [#16](https://github.com/shine-233/aetherlink-iot/pull/16) | broker Alpine `3.20 -> 3.24` | `mqtt-broker/Dockerfile` 为 `alpine:3.24` | 已完成 |
| [#17](https://github.com/shine-233/aetherlink-iot/pull/17) | GitHub Actions group，8 项 | 当前 actions 已达到或超过该组版本，CodeQL 为 `v4.37.7` | 已完成 |
| [#21](https://github.com/shine-233/aetherlink-iot/pull/21) | `vue-router 4.5.1 -> 4.6.4` | frontend manifest/lock 为 `4.6.4` | 已完成 |
| [#23](https://github.com/shine-233/aetherlink-iot/pull/23) | `vue-eslint-parser 9.4.2 -> 10.4.1` | manifest/lock 为 `10.4.1` | 已完成 |
| [#24](https://github.com/shine-233/aetherlink-iot/pull/24) | `rimraf 5.0.5 -> 6.1.3` | scripts package/lock 为 `6.1.3` | 已完成 |
| [#48](https://github.com/shine-233/aetherlink-iot/pull/48) | automation 移除 `uuid` | automation lock 已不再包含该依赖 | 已完成；后续只需复核残留 override |

## 已关闭但未合并（24 个）

| PR | 更新 | 当前证据 | 逐项处置 |
|---:|---|---|---|
| [#6](https://github.com/shine-233/aetherlink-iot/pull/6) | autotest `paho 1.4.3 -> 1.5.1` | 当前 `go.mod` 已是 `1.5.1`，但不是该 PR 的 merge 结果；Dependabot 也回报 up-to-date | 已被当前状态覆盖，不重开 |
| [#7](https://github.com/shine-233/aetherlink-iot/pull/7) | backend Go `1.26.4-alpine -> 1.27rc2-alpine` | 当前仍为 Go `1.26.4-alpine`；RC 版本曾导致 preflight/automation 失败 | 不合并 RC；等稳定版并重新验证 |
| [#8](https://github.com/shine-233/aetherlink-iot/pull/8) | frontend Node `22-alpine -> 26-alpine` | 当前仍为 `node:22-alpine` | 不直接采用大版本；先做 Node 兼容性与容器验证 |
| [#10](https://github.com/shine-233/aetherlink-iot/pull/10) | `mochawesome 7.1.4 -> 8.0.1` | 当前已是 `8.0.1`，被后续 #48/当前结果覆盖 | 不重开 |
| [#12](https://github.com/shine-233/aetherlink-iot/pull/12) | automation Chai `4 -> 6` | automation 仍为 Chai 4；Chai 6 是主版本/ESM 兼容性变化 | 单独做 ESM/API 迁移评估，不重开原 PR |
| [#14](https://github.com/shine-233/aetherlink-iot/pull/14) | backend Go group，22 项 | 当前 backend 仍保留旧版本，且该 PR 的 CodeQL Go、Backend tests 曾失败 | 不整体重开，拆成小批次逐批验证 |
| [#15](https://github.com/shine-233/aetherlink-iot/pull/15) | broker Go `1.26.4-alpine -> 1.27rc2-alpine` | 当前仍为 Go `1.26.4-alpine`；RC 版本未通过门禁 | 不合并 RC；等稳定版 |
| [#18](https://github.com/shine-233/aetherlink-iot/pull/18) | frontend minor/patch group，29 项 | 只有部分目标出现在当前 manifest/lock，不能把 group 视为整体完成 | 拆小批次；每批跑 typecheck/build/test |
| [#19](https://github.com/shine-233/aetherlink-iot/pull/19) | `@iconify/vue 4.1.1 -> 5.0.1` | 当前 manifest/lock 为 `5.0.1`，且代码已有 v5 适配 | 等价修复已存在，不重开 |
| [#20](https://github.com/shine-233/aetherlink-iot/pull/20) | `happy-dom 14.12.3 -> 20.11.2` | 当前 manifest/lock 为 `20.11.2` | 已覆盖，不重开 |
| [#22](https://github.com/shine-233/aetherlink-iot/pull/22) | `npm-check-updates 16.14.15 -> 23.0.2` | `frontend/packages/scripts/package.json` 已是 `23.0.2` | 已覆盖，不重开 |
| [#25](https://github.com/shine-233/aetherlink-iot/pull/25) | Pinia `2.1.7 -> 3.0.4` | 当前仍为 `2.1.7` | 主版本迁移，单独评估，不重开原 PR |
| [#26](https://github.com/shine-233/aetherlink-iot/pull/26) | `@unocss/vite 0.58.5 -> 66.7.5` | 当前 manifest/lock 已是 `66.7.5`；Dependabot 回报 up-to-date | 已覆盖，不重开 |
| [#27](https://github.com/shine-233/aetherlink-iot/pull/27) | `unplugin-vue-components 0.26.0 -> 32.1.0` | 当前仍为 `0.26.0` | 大版本插件迁移，先审查配置/API，再单独重建候选 |
| [#28](https://github.com/shine-233/aetherlink-iot/pull/28) | broker `logrus 1.6.0 -> 1.8.3` | 当前 broker 直接依赖中已没有 logrus；没有需要升级的当前直接依赖 | 视为被依赖移除覆盖，不重开 |
| [#29](https://github.com/shine-233/aetherlink-iot/pull/29) | broker grpc `1.79.3 -> 1.82.1` | 当前 broker 为 grpc `1.83.0`，高于目标 | 被更高版本覆盖，不重开 |
| [#30](https://github.com/shine-233/aetherlink-iot/pull/30) | frontend `nanoid 3.3.15 -> 6.0.1` | 当前使用 `5.1.16`，lock 没有 `6.0.1` | 单独评估 API/构建兼容性，不直接重开 |
| [#31](https://github.com/shine-233/aetherlink-iot/pull/31) | automation security group，3 项 | 当前 lock 已含安全更新/移除 uuid；仍需后续复核 `uuid` override | 当前结果覆盖，关闭原 PR；另开小清理 PR 不与本次混做 |
| [#32](https://github.com/shine-233/aetherlink-iot/pull/32) | frontend security group，5 项 | ECharts/happy-dom/Vite/Vitest 已覆盖，但 nanoid 仍不是 6.x | 标记为部分覆盖，不把整个 group 记为完成 |
| [#37](https://github.com/shine-233/aetherlink-iot/pull/37) | backend Go group，21 项 | 当前没有该 group 的整体落地结果；该 PR 的 backend/CodeQL/dependency checks 曾失败 | 与 #14 合并分析，拆小批次，不整体重开 |
| [#38](https://github.com/shine-233/aetherlink-iot/pull/38) | frontend group，27 项 | 当前没有该 group 的完整落地结果 | 拆成可独立构建/测试的小批次 |
| [#44](https://github.com/shine-233/aetherlink-iot/pull/44) | root Compose Redis `7-alpine -> 8-alpine` | 当前仍为 `redis:7-alpine` | 先做 Redis 8 Compose、持久化和应用兼容性验证 |
| [#45](https://github.com/shine-233/aetherlink-iot/pull/45) | optional Compose group，3 项 | 当前仍为 adapter/ThingsVis 旧镜像；没有真实 optional integration 环境 | 保持 pending；有真实环境和协议证据后再拆分 |
| [#47](https://github.com/shine-233/aetherlink-iot/pull/47) | multidb Compose group，2 项 | 当前仍为 MySQL `8.4`、Adminer `4.8.1`；候选 MySQL `26.7` 标签未被验证 | 不合并未知标签；确认支持版本后拆分 |

## 这次实际采取的动作

1. 没有重开或盲目合并历史 PR；当前 GitHub 已经没有 open Dependabot PR，重开旧分支会把已解决/冲突/主版本风险重新混在一起。
2. 补上 device-autotest 的普通 `minor/patch` Dependabot group，并用 `automation_tests/tests/00_dependabot_config_contract.test.js` 覆盖三套 Go module、两套 Node、三套 Dockerfile、三套 Compose 和 Actions 配置。
3. 新增 `.github/workflows/container-ci.yml`：PR、main push 和正式 tag 都对 backend/frontend/broker 做 `linux/amd64` build-only；不登录 GHCR、不 push、不申请 `packages: write`。
4. 将三个 container build check 接入两个 tag release workflow 的 required-check 轮询，并为未来 release 补充合同测试。
5. integration workflow 新增 `Integration result` 汇总 job；配置缺失、下游 skipped 或 live 测试失败都会明确以 fail-closed 结果结束。

## 尚未关闭的依赖工作

- backend #14/#37、frontend #18/#38 仍需要拆分后的新 PR；不能用“旧 group PR 已 closed”冒充完成。
- Chai 6、Pinia 3、unplugin-vue-components 32、nanoid 6、Node 26、Go 1.27 RC、Redis 8 和 optional image 更新都需要兼容性或运行时证据，不能仅凭静态 Dependabot check 合并。
- 本次没有创建新 release tag；最新正式版仍是 `v0.1.2`。新的 declared-and-locked source SBOM 需要下一版 tag 才会生成，不能回写不可变的 v0.1.2 资产。
