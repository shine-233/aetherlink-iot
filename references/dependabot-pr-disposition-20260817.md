# Dependabot PR 逐项处置记录（2026-08-17）

这是一份基于 GitHub REST/CLI **2026-08-17 目前回读**和 `origin/main=d1d90a9cdff271c2358005a0fdbfefc3a573df08` 当前源码/锁文件的处置记录。这里的“已覆盖”只表示当前依赖文件存在等价或更高版本；只有 `merged_at` 非空才计为该 PR 本身已经合并。关闭 PR 不能自动算作修复。

## 当前总数

- Dependabot PR 总数：**55**。
- `closed + merged`：**21**。
- `closed + 未 merged`：**24**。
- `open`：**10**（#55、#57、#59、#60、#61、#63、#65、#67、#69、#70）。
- 因此，“28 个普通 Dependabot PR”不是当前 GitHub API 可复现的数字；当前可复核的事实是 55 个历史/当前 PR，其中 10 个仍开放。#64 已在本轮合并。
- 当前 open alerts：Dependabot **0**、CodeQL **0**、Secret Scanning **0**。

## 本轮当前开放 PR 的逐项处置

| PR | 当前证据 | 处置 |
|---:|---|---|
| [#55](https://github.com/shine-233/aetherlink-iot/pull/55) | PostgreSQL 16→18、Redis 7→8 的运行时大版本组；已有持有评论 | 保留；等待迁移、持久化和应用兼容性证据 |
| [#57](https://github.com/shine-233/aetherlink-iot/pull/57) | MySQL 8.4→26.7、PostgreSQL 16→18、Adminer 4.8.1→5.5.1；已有持有评论 | 保留；未知镜像标签和多数据库兼容性不能盲合 |
| [#59](https://github.com/shine-233/aetherlink-iot/pull/59) | ThingsVis/HTTP adapter 外部镜像组；没有 optional runtime 环境 | 保留；等待 optional profile/protocol 证据 |
| [#60](https://github.com/shine-233/aetherlink-iot/pull/60) | 21 项 Go 组引入 `quic-go@0.59.0` moderate advisory `GHSA-vvgj-x9jq-8cj9`，且历史检查失败 | 保留；拆小批次并先消除依赖审查阻断 |
| [#61](https://github.com/shine-233/aetherlink-iot/pull/61) | 27 项前端组；与已合并 #64 在 `frontend/package.json`/`pnpm-lock.yaml` 冲突，GitHub 无法自动 rebase/merge | 保留；从当前 main 重建并保留 #64，再重新检查 |
| [#63](https://github.com/shine-233/aetherlink-iot/pull/63) | `vue-tsc 3.3.9` 检查出现 TS2440、string/number narrowing 等多处错误 | 保留；修复类型错误后再跑全门禁 |
| [#65](https://github.com/shine-233/aetherlink-iot/pull/65) | 已更新到当前 main；`@vueuse/core` 10.x→14.4.0，托管检查当时仍在运行 | 保留；即使 CI 绿也需要浏览器/运行时回归 |
| [#67](https://github.com/shine-233/aetherlink-iot/pull/67) | `eslint-plugin-vue 10.10.0` 检查在前端失败：`Cannot read properties of undefined (reading 'rules')` | 保留；修复 ESLint 配置/API 兼容性 |
| [#69](https://github.com/shine-233/aetherlink-iot/pull/69) | `bumpp 12.2.1` 与当前 lock 冲突；要求 Node `>=22.18`，影响 release/version/tag 行为 | 保留；隔离 dry-run 后再决定 |
| [#70](https://github.com/shine-233/aetherlink-iot/pull/70) | `execa 10.0.1` 使 `stdout` 变为联合类型，`packages/scripts/src/shared/index.ts` 出现 TS2339 | 保留；先补类型收窄 |

本轮已为 #61、#63、#65、#67、#69、#70 写入 GitHub 处置评论；#55、#57、#59、#60 已有此前的持有评论。没有为了把开放数量变成 0 而关闭或盲合这些 PR。

## 已合并的早期 PR（历史记录，14 个）

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

## 本轮新增合并（7 个）

| PR | 更新 | 合并提交 | 处置 |
|---:|---|---|---|
| [#54](https://github.com/shine-233/aetherlink-iot/pull/54) | MQTT broker Go builder `1.26.4-alpine -> 1.26.6-alpine` | 已合并 | 完成 |
| [#56](https://github.com/shine-233/aetherlink-iot/pull/56) | backend Go builder `1.26.4-alpine -> 1.26.6-alpine` | 已合并 | 完成 |
| [#58](https://github.com/shine-233/aetherlink-iot/pull/58) | broker Go minor/patch group | 已合并 | 完成 |
| [#62](https://github.com/shine-233/aetherlink-iot/pull/62) | `@eslint/js 9.27.0 -> 9.39.5` | `032ab8c6e506e0fcf2cbdf2c9348dfa8b60daa78` | 完成 |
| [#64](https://github.com/shine-233/aetherlink-iot/pull/64) | `rollup-plugin-visualizer 5.9.2 -> 7.0.1` | `d1d90a9cdff271c2358005a0fdbfefc3a573df08` | 重新基于当前 main 验证；VITE_BUNDLE_REPORT build 与全部 hosted checks 通过后合并 |
| [#66](https://github.com/shine-233/aetherlink-iot/pull/66) | `cross-env 7.0.3 -> 10.1.0` | `3c2bbe4f0a5dbdd759ac8594454c92c98e95ef86` | 完成 |
| [#68](https://github.com/shine-233/aetherlink-iot/pull/68) | `@types/node 20.11.24 -> 26.2.0` | `17572ac576defe69468a94a573661b2de5314d93` | 完成 |

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

1. 没有重开或盲目合并历史 PR；本轮只合并 #64 这一项，并把仍有冲突、失败或运行时风险的 PR 逐一写明保留原因。
2. 补上 device-autotest 的普通 `minor/patch` Dependabot group，并用 `automation_tests/tests/00_dependabot_config_contract.test.js` 覆盖三套 Go module、两套 Node、三套 Dockerfile、三套 Compose 和 Actions 配置。
3. 新增 `.github/workflows/container-ci.yml`：PR、main push 和正式 tag 都对 backend/frontend/broker 做 `linux/amd64` build-only；不登录 GHCR、不 push、不申请 `packages: write`。
4. 将三个 container build check 接入两个 tag release workflow 的 required-check 轮询，并为未来 release 补充合同测试。
5. integration workflow 新增 `Integration result` 汇总 job；配置缺失、下游 skipped 或 live 测试失败都会明确以 fail-closed 结果结束。
6. integration workflow 现在额外要求显式的 MQTT endpoint、`generic-emulator` 模式、Ready Check device id 和 auto-start，并在 hosted job 中构建固定的通用设备 emulator；`real-rdi` 不会被这条 lane 冒充。

## 尚未关闭的依赖工作

- backend #14/#37、frontend #18/#38 仍需要拆分后的新 PR；不能用“旧 group PR 已 closed”冒充完成。
- Chai 6、Pinia 3、unplugin-vue-components 32、nanoid 6、Node 26、Go 1.27 RC、Redis 8 和 optional image 更新都需要兼容性或运行时证据，不能仅凭静态 Dependabot check 合并。
- 本次没有创建新 release tag；最新正式版仍是 `v0.1.2`。新的 declared-and-locked source SBOM 需要下一版 tag 才会生成，不能回写不可变的 v0.1.2 资产。
