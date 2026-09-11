# PHASE-D-D4 交付说明(计算字段高级类型)

> 分支 `phase-d/d4`(基线 main@ac72114),主会话实现。

## 完成清单

1. **四种高级类型**(对齐 TB 4.3,70.sql 加列 type/config):
   - `timeseries_agg` 滚动窗口聚合(min/max/avg/count/sum,窗口 ≤24h,进程内 field+device 维度窗口,惰性清理);
   - `related_agg` 关联实体聚合(显式 device_ids + 聚合函数;资产树自动发现经 RelatedTargetsSource seam 留集成接线);
   - `geofence` 地理围栏(圆形 haversine + 多边形射线法,纯函数零依赖,输出 in/out 布尔);
   - `propagation` 传播(源值向显式目标设备派生同键副本,metadata 标记 propagated_from)。
   - simple 表达式路径完全不变(存量零回退)。
2. **历史重算**(70.sql 任务表):RecomputeRange 确定性回放(时序聚合用本地隔离窗口,不污染实时路径);
   异步任务服务(状态机 pending/running/done/failed + 进度回写);API `POST/GET /calcfield/recompute`(76.sql casbin 登记 SA/TA)。
3. **架构**:类型常量/配置解析/几何与聚合纯函数抽到 leaf 包 `internal/calcfield/types`
   (规避 service→calcfield→uplink→service 导入环,service 保存校验共用 ValidateFieldConfig)。
4. **前端**:计算字段表单新增"字段类型"选择器(simple/4 种高级类型),高级类型切换为 JSON 配置编辑器
   (提交前 JSON 校验),expression 按类型动态必填;i18n ×4。

## 证据
- 后端:`go build ./...` ✓、`go vet ./...` ✓、全包 sweep 无 FAIL;新增单测:配置校验 9 例、
  haversine/多边形、窗口聚合(滑动/count)、引擎路由(围栏)、重算确定性(两次重放逐 ts 相等+滚动 avg 语义)。
- 前端:typecheck 0 错误、`vitest run src/views/management/calculated-field` 全绿、build 成功。

## 预期冲突点
- `internal/service/calculated_field.go` Create 分支(类型校验段);`router/apps/calculated_field.go` 标记段;
  i18n custom.json ×4 追加键;model/dal 计算字段文件标记段。

## 未尽事项(留集成)
- related_agg 的资产树目标自动发现(RelatedTargetsSource 接 dal 资产树查询——需要资产↔设备绑定语义澄清);
- 重算任务运行期 E2E(隔离栈:建字段→灌历史→建任务→done+派生值落库断言);
- RecomputeStorageSink 的 app 装配注入(当前未注入时任务完成但 emitted=0,行为显式)。
