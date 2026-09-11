# PHASE-D-D1 交付说明(规则引擎 2.0)

> 分支 `phase-d/d1`(基线 main@ac72114)。实现过程:后台 agent 完成 ~60% 后两次被平台模型容量中断,
> 主会话接管续写(agent 的注册表/富化/转换 handler 直接保留,flow/analytics/external/trace/前端为接管后补齐)。

## 完成清单

### 后端(节点 8 种 → 28 种)
- **注册表** `rule_chain_nodes.go`(agent):28 种节点按 8 类(trigger/filter/transform/enrichment/flow/action/analytics/external),每类配置校验器;`RuleChainNodeTypeMeta` 收敛到单一来源。
- **消息架构升级**(agent + 收尾):节点间消息升级为 `{payload, metadata, originator}` 三元组,支持富化合入、分叉输出(split)、originator 切换;执行总超时/子链栈/trace 归属收敛到 `ruleChainExecution`。
- **Enrichment 富化 ×4**(agent):originator 属性/最新遥测/关联设备属性/租户元数据 → 合入 metadata(可配 prefix);设备与租户维度 fail-closed。
- **Transformation ×5 新**(agent):script(挂 data_script 引擎,script_id/code 二选一)、rename_keys、split_array(上限 100 分支)、dedup(时间窗,进程内注册表惰性清理)、change_originator(device_id/from_key)。
- **Filter ×3 新**(agent):exists / string_match(contains/equals/prefix/suffix) / in_range。
- **Flow ×3**(主会话):subchain(租户守卫子链加载+栈防环+深度≤5+子链图专用校验 ParseRuleChainSubgraph)、delay(≤5s,响应执行超时取消)、checkpoint(落库锚点,失败即分支失败)。
- **Analytics ×3**(主会话):generator(模拟数据源,标量/区间随机)、latest 聚合(最新遥测合入载荷)、message_count(滑窗计数,上限防洪泛)。
- **External ×2**(主会话):mqtt_forward(信封 {payload,metadata},发布器未装配 fail-fast;app 装配注入点已留 `ruleChainMQTTPublisher`)、kafka(骨架:生产者注入点,未注入 noop 放行,配置门控)。
- **节点级调试 trace**(主会话):`recordRuleChainNodeTrace` 挂引擎执行路径;开关 `rule-chain.trace-enabled`(viper,默认关,懒加载缓存热路径零开销);异步落库+recover,错误摘要截断 500;新表 `rule_chain_node_traces`(67.sql)。
- **trace 查询 API**(主会话):`GET /rule-chains/:id/nodes/:nodeId/traces?limit=10`(租户守卫,链不存在按 404 语义);68.sql 登记 casbin g2+授权(SA/TA)。
- **契约前移**:节点配置校验统一挂到 ParseRuleChainGraph(坏配置进不了库);存量测试按新契约更新(TestExecuteRuleChainAlarmActionValidation,意图不变)。
- **迁移**:67.sql 两表(trace/checkpoint)、68.sql 路由登记授权,均幂等。

### 前端
- 画布 palette 7 → 28 节点(i18n ×4,21 新标签键 + 面板 5 键);
- 丢节点 VueFlow 类型修正:触发器 input,其余 default(修复非触发节点落画布后无法接收入边 + nodeType 缺失导致属性面板不显示的既有缺陷);
- 非 special 类型节点通用 JSON 配置编辑器(blur 解析,坏 JSON 报错不脏写);
- 节点点击"最近调试记录"面板(pass/耗时/错误/时间,手动刷新);
- `rule_chain.ts` 新增 `ruleChainNodeTraces`;API 导出契约快照已按新增导出更新。

## 证据
- 后端:`go build ./...` ✓、`go vet ./...` ✓、`go test -p 2 ./...` 全仓 **0 FAIL**(32 internal 包 ok);新增 D1 单测 13 例(富化×2/去重/延时/检查点/子链含环检测/generator/计数/latest/MQTT 外发含 fail-fast/kafka 含 noop/trace/注册表)全绿。
- 前端:typecheck 0 错误;`vitest run src/views/automation src/service/api` **497 passed**(含更新后的导出契约快照)。

## 预期冲突点(集成注意)
- `router/apps/rule_chain.go`、`internal/api/rule_chain.go` 有 PHASE-D-D1 标记段;
- `rule_chain_graph.go` Validate 拆分为 validateGraph(requireTrigger)+ParseRuleChainSubgraph——与其它分支改动该文件的合并需人工核对;
- API 导出快照 `index.exports.test.ts` 已更新(D2 分支如也动过需合并后再 -u 一次)。

## 未尽事项(留集成)
- external.mqtt_forward 的 app 装配注入(`ruleChainMQTTPublisher` 接 broker 客户端)与 kafka 真实生产者(配置门控);
- 运行期 E2E:隔离栈建链→SNMP 模拟遥测→富化/去重/子链/checkpoint 全链路 + trace 面板实数据验证;
- 67/68.sql 在迁移链上的版本号如与其它 D 分支冲突,集成时统一重排。
