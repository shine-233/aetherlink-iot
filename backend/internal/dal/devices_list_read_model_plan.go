// 文件用途: 设备列表读模型（GetDeviceListByPage）的执行计划：基础查询、过滤装配与 count/scan 两段执行。
// 核心逻辑: plan.builder 全程持有 raw global.DB 链（clone==1 根，每次请求全新 Statement），
// 各过滤器以 clause.Expression / 纯 field 表达式挂载条件；count 与 scan 在同一链上顺序执行，
// 与 users.go GetUserListByPageWithAddress 的权威写法一致。
// 关键注意事项: 批次二收敛（2026-08-24，见 references/gen-inheritance-audit.md）——
//   - baseQuery 从 query.Device.WithContext 改为 global.DB.Model(&model.Device{}) + 显式
//     LEFT JOIN device_configs 的 raw 链重建；activate_flag='active' + 租户条件保持等价；
//   - applySearchFilter / applyWarnStatusFilter 两个嵌套条件构造器去单例化：
//     不再经 query.Device.Where(...).Or(...) 的 Do 链包装，改用 field.Or 纯表达式组合，
//     并发下互不播种；
//   - JOIN 形态（含 latest_device_alarms 复合 ON）、WHERE 组合顺序与投影别名均与收敛前对齐。
// 重构建议: 若后续引入读库分离或查询超时控制，在 baseQuery 单点接入即可覆盖全部列表读路径。

package dal

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gen/field"
	"gorm.io/gorm"
)

// deviceListPageReadModel keeps the GetDeviceListByPage planning and execution
// contract behind one internal seam.
type deviceListPageReadModel struct {
	req      *model.GetDeviceListByPageReq
	tenantID string
}

type deviceListPagePlan struct {
	req                   *model.GetDeviceListByPageReq
	builder               *gorm.DB
	empty                 bool
	latestTelemetryJoined bool
}

func newDeviceListPageReadModel(req *model.GetDeviceListByPageReq, tenantID string) deviceListPageReadModel {
	return deviceListPageReadModel{
		req:      req,
		tenantID: tenantID,
	}
}

func (rm deviceListPageReadModel) execute() (int64, []model.GetDeviceListByPageRsp, error) {
	plan, err := rm.plan()
	if err != nil {
		return 0, emptyDeviceListPage(), logDeviceListPageError(err)
	}
	if plan.empty {
		return 0, emptyDeviceListPage(), nil
	}

	count, deviceList, err := plan.execute()
	if err != nil {
		return count, deviceList, logDeviceListPageError(err)
	}

	return count, deviceList, nil
}

func (rm deviceListPageReadModel) plan() (deviceListPagePlan, error) {
	plan := deviceListPagePlan{
		req:     rm.req,
		builder: rm.baseQuery(),
	}
	plan.applyLifecycleTelemetryFilter()

	if err := plan.applyGroupFilter(); err != nil || plan.empty {
		return plan, err
	}

	plan.applyFieldFilters()
	plan.applyProtocolTypeFilters()
	if err := plan.applyStatusAlarmFilters(); err != nil {
		return plan, err
	}

	return plan, nil
}

// baseQuery 返回设备列表读模型的 raw 链起点。
// 批次二收敛（见 references/gen-inheritance-audit.md）：从 query.Device.WithContext 改为
// global.DB.Model(&model.Device{}) + 显式 LEFT JOIN device_configs；clone==1 根保证每次
// 链式起点都是全新 Statement。JOIN 条件与旧 gen 版 LeftJoin(c, c.ID.EqCol(q.DeviceConfigID))
// 渲染结果一致；生命周期筛选默认只返回已激活设备，仅当显式传入 lifecycle_status 时放宽，
// 保证既有调用方响应逐字节不变。
func (rm deviceListPageReadModel) baseQuery() *gorm.DB {
	q := query.Device

	builder := global.DB.WithContext(context.Background()).Model(&model.Device{}).
		Joins("LEFT JOIN device_configs ON device_configs.id = devices.device_config_id")
	if cond, applies := deviceListLifecycleCondition(rm.req); applies {
		if cond != nil {
			builder = builder.Where(cond)
		}
	} else {
		builder = builder.Where(q.ActivateFlag.Eq("active"))
	}
	if rm.req == nil || !rm.req.AllTenants {
		builder = builder.Where(q.TenantID.Eq(rm.tenantID))
	}
	return builder
}

// deviceListLifecycleCondition 把 opt-in 的 lifecycle_status 翻译成设备表条件（REQ-05b）。
// 返回的 applies=false 表示调用方未传该参数，应沿用历史的 active-only 行为；
// applies=true 且 cond=nil 表示 "all"，即不施加任何 activate_flag 约束。
func deviceListLifecycleCondition(req *model.GetDeviceListByPageReq) (field.Expr, bool) {
	if req == nil || req.LifecycleStatus == nil {
		return nil, false
	}
	// DTO 的 oneof 校验是精确匹配；DAL 保持同一口径，不在这里静默接受
	// 带空格或大小写不同的非法值。
	switch *req.LifecycleStatus {
	case "activated":
		return query.Device.ActivateFlag.Eq("active"), true
	case "inactive":
		return query.Device.ActivateFlag.Neq("active"), true
	case "transmitted", "all":
		return nil, true
	default:
		return nil, false
	}
}

// applyLifecycleTelemetryFilter 实现客户 REQ-05b 的“传输完成”：只要设备至少
// 有一条 current telemetry，就说明平台曾成功接收并持久化过上报。该状态从
// 现有事实表派生，不新增容易与真实数据漂移的生命周期列。
func (plan *deviceListPagePlan) applyLifecycleTelemetryFilter() {
	if plan == nil || plan.req == nil || plan.req.LifecycleStatus == nil || *plan.req.LifecycleStatus != "transmitted" {
		return
	}

	plan.ensureLatestTelemetryJoined()
	t2 := query.TelemetryCurrentData.As("t2")
	plan.builder = plan.builder.Where(t2.T.IsNotNull())
}

func (plan *deviceListPagePlan) ensureLatestTelemetryJoined() {
	if plan == nil || plan.latestTelemetryJoined {
		return
	}
	plan.builder = joinDeviceListLatestTelemetry(plan.builder, nil)
	plan.latestTelemetryJoined = true
}

func (plan *deviceListPagePlan) applyGroupFilter() error {
	if !hasDeviceListValue(plan.req.GroupId) {
		return nil
	}

	groupID := strings.TrimSpace(*plan.req.GroupId)
	ids, err := deviceListGroupFilterIDs(groupID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		plan.empty = true
		return nil
	}

	plan.builder = plan.builder.Where(query.Device.ID.In(ids...))
	return nil
}

func (plan *deviceListPagePlan) applyFieldFilters() {
	plan.applyCoreFieldFilters()
	plan.applySearchFilter(plan.req.Search)
	plan.applyDescriptorFieldFilters()
	plan.applyConfigFieldFilters()
}

func (plan *deviceListPagePlan) applyCoreFieldFilters() {
	q := query.Device

	plan.applyTextFilter(plan.req.IsEnabled, q.IsEnabled.Eq)
	plan.applyTextFilter(plan.req.OwnerUserID, q.OwnerUserID.Eq)
	plan.applyTextFilter(plan.req.ProductID, q.ProductID.Eq)
	plan.applyTextFilter(plan.req.ServiceAccessID, q.ServiceAccessID.Eq)
	plan.applyTextFilter(plan.req.AccessWay, q.AccessWay.Eq)
	plan.applyTextFilter(plan.req.Label, func(value string) field.Expr {
		return deviceListLike(q.Label, value)
	})
}

// applySearchFilter 对名称/编号/固件/描述做小写模糊匹配。
// 批次二去单例化：不再用 query.Device.Where(...).Or(...) 的 Do 链包装四条 LIKE，
// 改为 field.Or 纯表达式组合，产出的 OR 组与旧版逐项等价。
func (plan *deviceListPagePlan) applySearchFilter(searchValue *string) {
	if !hasDeviceListValue(searchValue) {
		return
	}

	q := query.Device
	search := strings.ToLower(strings.TrimSpace(*searchValue))
	plan.builder = plan.builder.Where(field.Or(
		q.Name.Lower().Like(ContainsLikePattern(search)),
		q.DeviceNumber.Lower().Like(ContainsLikePattern(search)),
		q.CurrentVersion.Lower().Like(ContainsLikePattern(search)),
		q.Description.Lower().Like(ContainsLikePattern(search)),
	))
}

func (plan *deviceListPagePlan) applyDescriptorFieldFilters() {
	q := query.Device

	plan.applyTextFilter(plan.req.Name, func(value string) field.Expr {
		return deviceListLike(q.Name, value)
	})
	plan.applyTextFilter(plan.req.CurrentVersion, q.CurrentVersion.Eq)
	plan.applyTextFilter(plan.req.PIDNumber, func(value string) field.Expr {
		return deviceListLike(q.DeviceNumber, value)
	})
	plan.applyTextFilter(plan.req.FirmwareVersion, func(value string) field.Expr {
		return deviceListLike(q.CurrentVersion, value)
	})
	plan.applyTextFilter(plan.req.Description, func(value string) field.Expr {
		return deviceListLike(q.Description, value)
	})
	plan.applyTextFilter(plan.req.BatchNumber, func(value string) field.Expr {
		return deviceListLike(q.BatchNumber, value)
	})
	plan.applyTextFilter(plan.req.DeviceNumber, func(value string) field.Expr {
		return deviceListLike(q.DeviceNumber, value)
	})
}

func (plan *deviceListPagePlan) applyConfigFieldFilters() {
	q := query.Device
	c := query.DeviceConfig

	plan.applyTextFilter(plan.req.DeviceConfigId, q.DeviceConfigID.Eq)
	plan.applyTextFilter(plan.req.DeviceTemplateID, c.DeviceTemplateID.Eq)
}

func (plan *deviceListPagePlan) applyProtocolTypeFilters() {
	plan.applyServiceIdentifierFilter(plan.req.ServiceIdentifier)
	plan.applyDeviceTypeFilter(plan.req.DeviceType)
}

func (plan *deviceListPagePlan) applyServiceIdentifierFilter(serviceIdentifier *string) {
	if !hasDeviceListValue(serviceIdentifier) {
		return
	}

	plan.builder = plan.builder.Where(deviceListServiceIdentifierCondition(strings.TrimSpace(*serviceIdentifier)))
}

func (plan *deviceListPagePlan) applyDeviceTypeFilter(deviceType *string) {
	if !hasDeviceListValue(deviceType) {
		return
	}

	plan.builder = plan.builder.Where(deviceListDeviceTypeCondition(strings.TrimSpace(*deviceType)))
}

func (plan *deviceListPagePlan) applyStatusAlarmFilters() error {
	plan.applySharedStatusFilter(plan.req.SharedStatus)
	if err := plan.applyOnlineStatusFilter(plan.req.IsOnline); err != nil {
		return err
	}
	if err := plan.applyLastReportedFilters(); err != nil {
		return err
	}
	return plan.applyWarnStatusFilter(plan.req.WarnStatus)
}

func (plan *deviceListPagePlan) applySharedStatusFilter(sharedStatus *string) {
	q := query.Device

	if hasDeviceListValue(sharedStatus) {
		switch strings.ToLower(strings.TrimSpace(*sharedStatus)) {
		case "shared":
			plan.builder = plan.builder.Where(rdiDeviceSharedStatusCondition(q.AdditionalInfo, true))
		case "unshared":
			plan.builder = plan.builder.Where(rdiDeviceSharedStatusCondition(q.AdditionalInfo, false))
		}
	}
}

func (plan *deviceListPagePlan) applyOnlineStatusFilter(isOnline *int) error {
	if isOnline == nil {
		return nil
	}

	q := query.Device
	switch *isOnline {
	case 1:
		plan.builder = plan.builder.Where(q.IsOnline.Eq(1))
		return nil
	case 0:
		plan.builder = plan.builder.Where(q.IsOnline.Neq(1))
		return nil
	default:
		return fmt.Errorf("is_online param error")
	}
}

func (plan *deviceListPagePlan) applyLastReportedFilters() error {
	after := plan.req.LastReportedAfter
	before := plan.req.LastReportedBefore
	neverReported := plan.req.NeverReported
	if after == nil && before == nil && neverReported == nil {
		return nil
	}
	if after != nil && *after <= 0 {
		return fmt.Errorf("last_reported_after param error")
	}
	if before != nil && *before <= 0 {
		return fmt.Errorf("last_reported_before param error")
	}
	if after != nil && before != nil && *after >= *before {
		return fmt.Errorf("last reported range param error")
	}
	if neverReported != nil && *neverReported && (after != nil || before != nil) {
		return fmt.Errorf("never_reported cannot be combined with a last reported range")
	}

	plan.ensureLatestTelemetryJoined()
	t2 := query.TelemetryCurrentData.As("t2")
	if neverReported != nil {
		if *neverReported {
			plan.builder = plan.builder.Where(t2.T.IsNull())
		} else {
			plan.builder = plan.builder.Where(t2.T.IsNotNull())
		}
	}
	if after != nil {
		plan.builder = plan.builder.Where(t2.T.Gte(time.UnixMilli(*after)))
	}
	if before != nil {
		plan.builder = plan.builder.Where(t2.T.Lt(time.UnixMilli(*before)))
	}
	return nil
}

// applyWarnStatusFilter 按最新告警状态过滤设备。
// 批次二收敛：告警表联接改为显式 raw JOIN 字符串（复合 ON：device_id + tenant_id，
// 与旧 gen 版 LeftJoin(lda, DeviceID.EqCol(ID), TenantID.EqCol(TenantID)) 渲染等价）；
// "N" 分支的去单例化同 applySearchFilter——field.Or 替代 query.Device.Where(...).Or(...)。
func (plan *deviceListPagePlan) applyWarnStatusFilter(warnStatus *string) error {
	if !hasDeviceListValue(warnStatus) {
		return nil
	}

	lda := query.LatestDeviceAlarm

	// Join the alarm table only when the request filters by alarm status.
	plan.builder = plan.builder.Joins(
		"LEFT JOIN latest_device_alarms ON latest_device_alarms.device_id = devices.id AND latest_device_alarms.tenant_id = devices.tenant_id",
	)
	switch strings.ToUpper(strings.TrimSpace(*warnStatus)) {
	case "N":
		plan.builder = plan.builder.Where(field.Or(lda.AlarmStatus.Eq("N"), lda.AlarmStatus.IsNull()))
		return nil
	case "Y":
		plan.builder = plan.builder.Where(lda.AlarmStatus.In("H", "M", "L"))
		return nil
	default:
		return fmt.Errorf("warn_status param error")
	}
}

// isolatedBuilder 返回链语句的克隆会话：count/scan 多段执行各自从快照出发，
// 执行期子句写回不再污染共享链（与旧 gen 终结操作的 Session 隔离语义对齐）。
func (plan deviceListPagePlan) isolatedBuilder() *gorm.DB {
	return plan.builder.Session(&gorm.Session{})
}

func (plan deviceListPagePlan) count() (int64, error) {
	var total int64
	err := plan.isolatedBuilder().Count(&total).Error
	return total, err
}

func (plan deviceListPagePlan) execute() (int64, []model.GetDeviceListByPageRsp, error) {
	count, err := plan.count()
	if err != nil {
		return count, emptyDeviceListPage(), err
	}

	deviceList, err := plan.scan()
	if err != nil {
		return count, deviceList, err
	}

	return count, deviceList, nil
}

func (plan deviceListPagePlan) countIDs(ids []string) (int64, error) {
	normalizedIDs := normalizeDeviceIDs(ids)
	if len(normalizedIDs) == 0 {
		return 0, nil
	}

	var total int64
	err := plan.builder.Where(query.Device.ID.In(normalizedIDs...)).Session(&gorm.Session{}).Count(&total).Error
	return total, err
}

func (plan deviceListPagePlan) scanIDs(limit int) ([]string, error) {
	rows := []struct {
		ID string `gorm:"column:id"`
	}{}
	builder := plan.isolatedBuilder().Select(`"devices"."id"`).Order(`"devices"."created_at" DESC`)
	if limit > 0 {
		builder = builder.Limit(limit)
	}
	if err := builder.Scan(&rows).Error; err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (plan deviceListPagePlan) scanByIDs(ids []string) ([]model.GetDeviceListByPageRsp, error) {
	normalizedIDs := normalizeDeviceIDs(ids)
	if len(normalizedIDs) == 0 {
		return emptyDeviceListPage(), nil
	}

	deviceList := emptyDeviceListPage()
	builder := plan.builder.Where(query.Device.ID.In(normalizedIDs...))
	err := buildDeviceListPageScanQuery(builder).Session(&gorm.Session{}).Scan(&deviceList).Error
	if err != nil {
		return deviceList, err
	}
	if err := hydrateDeviceListLatestTelemetry(&deviceList); err != nil {
		return deviceList, err
	}
	return reorderDeviceListRowsByID(deviceList, normalizedIDs), nil
}

func (plan deviceListPagePlan) scan() ([]model.GetDeviceListByPageRsp, error) {
	if plan.req.Page != 0 && plan.req.PageSize != 0 {
		return plan.scanCurrentPageByIDs()
	}

	deviceList := emptyDeviceListPage()
	err := buildDeviceListPageScanQuery(plan.paginatedBuilder()).Session(&gorm.Session{}).Scan(&deviceList).Error
	if err != nil {
		return deviceList, err
	}
	if err := hydrateDeviceListLatestTelemetry(&deviceList); err != nil {
		return deviceList, err
	}
	return deviceList, nil
}

func (plan deviceListPagePlan) scanCurrentPageByIDs() ([]model.GetDeviceListByPageRsp, error) {
	ids, err := plan.scanPageIDs()
	if err != nil {
		return emptyDeviceListPage(), err
	}
	return plan.scanByIDs(ids)
}

func (plan deviceListPagePlan) scanPageIDs() ([]string, error) {
	rows := []struct {
		ID string `gorm:"column:id"`
	}{}
	builder := plan.paginatedBuilder().Session(&gorm.Session{}).
		Select(`"devices"."id"`).Order(`"devices"."created_at" DESC`)
	err := builder.Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (plan deviceListPagePlan) paginatedBuilder() *gorm.DB {
	if plan.req.Page == 0 || plan.req.PageSize == 0 {
		return plan.builder
	}

	return plan.builder.Limit(plan.req.PageSize).
		Offset((plan.req.Page - 1) * plan.req.PageSize)
}

func reorderDeviceListRowsByID(rows []model.GetDeviceListByPageRsp, ids []string) []model.GetDeviceListByPageRsp {
	if len(rows) <= 1 {
		return rows
	}

	rowsByID := make(map[string]model.GetDeviceListByPageRsp, len(rows))
	for _, row := range rows {
		rowsByID[row.ID] = row
	}

	ordered := make([]model.GetDeviceListByPageRsp, 0, len(rows))
	for _, id := range ids {
		row, ok := rowsByID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, row)
	}
	return ordered
}

func (plan *deviceListPagePlan) applyTextFilter(value *string, buildCondition func(string) field.Expr) {
	if !hasDeviceListValue(value) {
		return
	}

	plan.builder = plan.builder.Where(buildCondition(strings.TrimSpace(*value)))
}
