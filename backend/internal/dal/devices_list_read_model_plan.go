package dal

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"gorm.io/gen/field"
)

// deviceListPageReadModel keeps the GetDeviceListByPage planning and execution
// contract behind one internal seam.
type deviceListPageReadModel struct {
	req      *model.GetDeviceListByPageReq
	tenantID string
}

type deviceListPagePlan struct {
	req                   *model.GetDeviceListByPageReq
	builder               query.IDeviceDo
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

func (rm deviceListPageReadModel) baseQuery() query.IDeviceDo {
	q := query.Device
	c := query.DeviceConfig

	builder := q.WithContext(context.Background()).
		LeftJoin(c, c.ID.EqCol(q.DeviceConfigID))
	// 生命周期筛选默认保持历史行为：只返回已激活设备。
	// 只有显式传入 lifecycle_status 时才放宽，保证既有调用方响应逐字节不变。
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

func (plan *deviceListPagePlan) applySearchFilter(searchValue *string) {
	if !hasDeviceListValue(searchValue) {
		return
	}

	q := query.Device
	search := strings.ToLower(strings.TrimSpace(*searchValue))
	plan.builder = plan.builder.Where(
		query.Device.Where(q.Name.Lower().Like(fmt.Sprintf("%%%s%%", search))).
			Or(q.DeviceNumber.Lower().Like(fmt.Sprintf("%%%s%%", search))).
			Or(q.CurrentVersion.Lower().Like(fmt.Sprintf("%%%s%%", search))).
			Or(q.Description.Lower().Like(fmt.Sprintf("%%%s%%", search))),
	)
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

func (plan *deviceListPagePlan) applyWarnStatusFilter(warnStatus *string) error {
	if !hasDeviceListValue(warnStatus) {
		return nil
	}

	q := query.Device
	lda := query.LatestDeviceAlarm

	// Join the alarm table only when the request filters by alarm status.
	plan.builder = plan.builder.LeftJoin(
		lda,
		lda.DeviceID.EqCol(q.ID),
		lda.TenantID.EqCol(q.TenantID),
	)
	switch strings.ToUpper(strings.TrimSpace(*warnStatus)) {
	case "N":
		plan.builder = plan.builder.Where(
			query.Device.Where(lda.AlarmStatus.Eq("N")).Or(lda.AlarmStatus.IsNull()),
		)
		return nil
	case "Y":
		plan.builder = plan.builder.Where(lda.AlarmStatus.In("H", "M", "L"))
		return nil
	default:
		return fmt.Errorf("warn_status param error")
	}
}

func (plan deviceListPagePlan) count() (int64, error) {
	return plan.builder.Count()
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

	return plan.builder.Where(query.Device.ID.In(normalizedIDs...)).Count()
}

func (plan deviceListPagePlan) scanIDs(limit int) ([]string, error) {
	rows := []struct {
		ID string `gorm:"column:id"`
	}{}
	builder := plan.builder.Select(query.Device.ID).Order(query.Device.CreatedAt.Desc())
	if limit > 0 {
		builder = builder.Limit(limit)
	}
	if err := builder.Scan(&rows); err != nil {
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
	err := buildDeviceListPageScanQuery(builder).Scan(&deviceList)
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
	err := buildDeviceListPageScanQuery(plan.paginatedBuilder()).Scan(&deviceList)
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
	if err := plan.paginatedBuilder().Select(query.Device.ID).Order(query.Device.CreatedAt.Desc()).Scan(&rows); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (plan deviceListPagePlan) paginatedBuilder() query.IDeviceDo {
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
