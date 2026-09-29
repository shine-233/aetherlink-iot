# 2026-09-26 dev 活栈契约回归证据（契约测试 75~101 全量运行）

> 目的：批次一/二/三交付项的自动化契约测试此前因"本机无 docker"未运行，各行状态记 partial†。
> 本次按用户指示改用 **dev 环境**（本机原生 PostgreSQL 17 隔离集群 + Redis + 本地 MQTT 夹具）完成活栈回归。

## 1. 环境构成（dev 环境，无 docker）

| 组件 | 方式 | 说明 |
| --- | --- | --- |
| PostgreSQL 17 | `initdb` 新建隔离集群 `127.0.0.1:55433`（库 `aetherlink_go99`，用户 postgres/localdev-only） | 与 5432 生产库完全隔离；数据目录在 %TEMP% |
| Redis | 本机 6379 原生服务 | db1 供 broker 持久化 |
| MQTT Broker | `automation_tests/scripts/local_mqtt_broker.js`（conf-localdev.yml 注明的本地验证夹具） | 1883 端口 |
| 后端 | `backend-dev.exe -config ./configs/conf-localdev.yml` | 空库启动，**全新迁移链 1→138 一次通过**（151 张表建成） |

## 2. 运行结果

- 全量联跑（tests/75~101，24 个文件）：**207 passing / 0 failing / 11 pending**。
- pending 口径：88 号的 5 个看板可见性用例需先经角色-权限 API 给 TENANT_USER 授 /board 路由（全新库迁移不预设），已加显式 skip 守卫并注明；其余 pending 为套件固有条件用例。

## 3. 活栈回归抓到并已修复的真实缺陷

1. **TB-48 调度器路由未接线**：scheduler.go/enter.go 存在但 router_init.go 从未调用 InitScheduler → /scheduler/events 404。已补 `apps.Model.SchedulerRouter.InitScheduler(v1)`。
2. **TB-04 seed 幂等判定用文本比较**：CreateWidgetBundle 落库会规范化 JSON，二次 seed 永远误报"内容不一致"（100002）。改为语义比较（widgetBundleContentEqual）。
3. **TB-41 媒体路径 Windows 反斜杠**：saveFile 返回 `./files\media\...` 随登记落库，破坏 URL 语义。统一 `filepath.ToSlash`（OTA 分支同样修复）。
4. **TB-41 引用扫描对 json 列 LIKE**：boards.config / scada_documents.json_data 是 json 列，真 PG 下 `LIKE` 报"操作符不存在"。改为 `::text` 转型 + 方言感知（sqlite 单测自动去除转型）。
5. **TB-10 审计实体解析增强**：POST 集合级创建的 entity_id 从响应体 data.id 提取；请求体带 UUID 形态 id 的 POST 记为 action=update（此前全部记 create 且 entity_id 为空）。resolveOperationActionAndEntity 纯函数 + 6 例单测。
6. 测试侧修正：77 号改从创建响应取 product_key（列表接口不返回该字段，且正向对照此前空跑）；94 号写入越权期望改为 casbin 403 语义、跨租户删除断言更正为"只命中 B 自己的行"；99 号断言改语义解析（jsonb 规范化带空格）；88 号加 skip 守卫。

## 4. 状态影响

- 契约测试 75~101 全绿的交付行按 §2 四面一致口径由 **partial† 升级为 done**（OPENAPI/后端/前端/自动化测试四面齐，且测试已在活栈运行通过）。
- 88 号 5 个可见性用例对应的组共享逻辑已有 Go 单测覆盖，其活栈契约用例待角色授权前置补齐后补跑。
- Playwright E2E 仍未运行（需前端构建/预览服务）。
