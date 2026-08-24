// 文件用途: 设备列表读模型（GetDeviceListByPage 等）的投影定义、嵌套条件构造与遥测回填。
// 核心逻辑: 以 raw global.DB 链为唯一语句起点组装过滤/分页投影；条件一律由纯 field 表达式
// 或 clause 表达式组合，不再从包级 query 单例起 Do 链。
// 关键注意事项: 批次二收敛（2026-08-24，见 references/gen-inheritance-audit.md）——高并发下
// gorm/gen 包级表单例是继承式语句根，会跨请求残留 Statement.Model/Dest。本文件的读链起点、
// 嵌套条件构造器（deviceListServiceIdentifierCondition / deviceListDeviceTypeCondition /
// rdiDeviceSharedStatusCondition）与 TelemetryCurrentData 链均已去单例化：
//   - 读链起点改为 raw global.DB（clone==1 根，每次链式起点全新 Statement）；
//   - 条件构造器改用 field.Or / field.And 纯表达式组合（field.Expr 本身实现
//     clause.Expression，可直接被 raw 链 Where 消费），并发下互不播种；
//   - 产出的 SQL 条件语义与收敛前逐条对齐（JOIN 形态、WHERE 组合顺序、别名大小写不变）。
// 重构建议: 后续批次按同构模板继续收敛 device_query_reads.go 其余裸链起点。

package dal

import (
	"fmt"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/sirupsen/logrus"
)

// GetDeviceListByPage returns one filtered page of active tenant devices.
func GetDeviceListByPage(req *model.GetDeviceListByPageReq, tenantID string) (int64, []model.GetDeviceListByPageRsp, error) {
	return newDeviceListPageReadModel(req, tenantID).execute()
}

// CountDeviceListByFilter counts devices matching the same filters used by the
// device list endpoint without scanning display rows.
func CountDeviceListByFilter(req *model.GetDeviceListByPageReq, tenantID string) (int64, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return 0, logDeviceListPageError(err)
	}
	if plan.empty {
		return 0, nil
	}
	return plan.count()
}

// CountDeviceListFilteredIDs counts which provided ids are still inside the
// same filtered active-device scope.
func CountDeviceListFilteredIDs(req *model.GetDeviceListByPageReq, tenantID string, ids []string) (int64, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return 0, logDeviceListPageError(err)
	}
	if plan.empty {
		return 0, nil
	}
	return plan.countIDs(ids)
}

// ListDeviceIDsByFilter returns only ids for a filtered active-device scope.
func ListDeviceIDsByFilter(req *model.GetDeviceListByPageReq, tenantID string, limit int) ([]string, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return nil, logDeviceListPageError(err)
	}
	if plan.empty {
		return []string{}, nil
	}
	return plan.scanIDs(limit)
}

// GetDeviceListRowsByFilterAndIDs loads display rows for a small selected id
// set while preserving the same filters as the device list endpoint.
func GetDeviceListRowsByFilterAndIDs(req *model.GetDeviceListByPageReq, tenantID string, ids []string) ([]model.GetDeviceListByPageRsp, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return nil, logDeviceListPageError(err)
	}
	if plan.empty {
		return emptyDeviceListPage(), nil
	}
	return plan.scanByIDs(ids)
}

func deviceListGroupFilterIDs(groupID string) ([]string, error) {
	groupIDs, err := GetGroupChildrenIds(groupID)
	if err != nil {
		return nil, err
	}

	ids, err := GetDeviceIdsByGroupIds(groupIDs)
	if err != nil {
		return nil, err
	}

	// Preserve the existing ID set shape used by the list filter.
	return append(ids, groupID), nil
}

// deviceListServiceIdentifierCondition 把服务标识筛选翻译成纯 field 表达式组合。
// 批次二去单例化：不再从包级 query.Device 起 Do 链（Where(...).Or(...)），
// 改用 field.Or 直接组合，产出的布尔语义与旧 DO 条件逐字节等价。
func deviceListServiceIdentifierCondition(value string) field.Expr {
	c := query.DeviceConfig

	if value == "mqtt" {
		return field.Or(c.ProtocolType.Eq(value), query.Device.DeviceConfigID.IsNull())
	}
	return c.ProtocolType.Eq(value)
}

// deviceListDeviceTypeCondition 把设备类型筛选翻译成纯 field 表达式组合。
// 批次二去单例化：同上，不再经 query.Device 的 Do 链包装。
func deviceListDeviceTypeCondition(value string) field.Expr {
	c := query.DeviceConfig

	if value == "1" {
		return field.Or(query.Device.DeviceConfigID.IsNull(), c.DeviceType.Eq(value))
	}
	return c.DeviceType.Eq(value)
}

func emptyDeviceListPage() []model.GetDeviceListByPageRsp {
	return []model.GetDeviceListByPageRsp{}
}

func logDeviceListPageError(err error) error {
	logrus.Error(err)
	return err
}

// joinDeviceListLatestTelemetry 在设备列表链上挂最新遥测子查询（批次二起走 raw 链）。
// 子查询保持旧 gen 版语义：MAX(ts) AS ts 按 device_id 分组；可选 device_id 过滤留在子查询内部。
// 注意：Group 入参必须是不带引号的裸列名，gorm 会自行按方言加引号。
func joinDeviceListLatestTelemetry(builder *gorm.DB, deviceIDs []string) *gorm.DB {
	latestTelemetry := global.DB.Table("telemetry_current_datas").
		Select(`MAX("ts") AS "ts", device_id`).
		Group("device_id")
	if len(deviceIDs) > 0 {
		latestTelemetry = latestTelemetry.Where(query.TelemetryCurrentData.DeviceID.In(deviceIDs...))
	}
	return builder.Joins(`LEFT JOIN (?) AS t2 ON "t2"."device_id" = "devices"."id"`, latestTelemetry)
}

// buildDeviceListPageScanQuery 为展示行扫描附加固定投影与排序（批次二起作用于 raw 链）。
func buildDeviceListPageScanQuery(builder *gorm.DB) *gorm.DB {
	return builder.Select(strings.Join(deviceListPageSelectFields(), ", ")).
		Order(`"devices"."created_at" DESC`)
}

// deviceListPageSelectFields 返回设备列表读模型的固定投影。
// 批次二起以带引号的 SQL 片段表达（供 raw 链原样渲染），字段清单与别名和旧
// gen 字段版本一一对应；别名大小写（如 "DeviceConfigName"）必须保持，
// 否则响应结构体的列映射会因 Postgres 小写折叠而失配。
func deviceListPageSelectFields() []string {
	return []string{
		`"devices"."id"`,
		`"devices"."device_number"`,
		`"devices"."name"`,
		`"devices"."device_config_id"`,
		`"devices"."activate_flag"`,
		`"devices"."activate_at"`,
		`"devices"."batch_number"`,
		`"devices"."description"`,
		`"devices"."location"`,
		`"devices"."current_version"`,
		`"devices"."created_at"`,
		`"devices"."is_online"`,
		`"devices"."access_way"`,
		`"devices"."tenant_id"`,
		`"devices"."owner_user_id"`,
		`"devices"."parent_id"`,
		`"devices"."sub_device_addr"`,
		`"device_configs"."protocol_type"`,
		`"device_configs"."device_type"`,
		`"device_configs"."name" AS "DeviceConfigName"`,
		`"device_configs"."image_url"`,
		`"devices"."last_offline_time"`,
		`"devices"."additional_info"`,
	}
}

// hydrateDeviceListLatestTelemetry 回填每台设备的最新遥测时间戳。
// 批次二收敛：TelemetryCurrentData 链改走 raw global.DB 起点，不再继承包级单例语句根。
func hydrateDeviceListLatestTelemetry(deviceList *[]model.GetDeviceListByPageRsp) error {
	if deviceList == nil || len(*deviceList) == 0 {
		return nil
	}

	deviceIDs := make([]string, 0, len(*deviceList))
	for _, device := range *deviceList {
		deviceIDs = append(deviceIDs, device.ID)
	}

	t := query.TelemetryCurrentData
	rows := []*model.TelemetryCurrentData{}
	err := global.DB.Model(&model.TelemetryCurrentData{}).
		Where(t.DeviceID.In(deviceIDs...)).
		Order(`"telemetry_current_datas"."ts" DESC`).
		Find(&rows).Error
	if err != nil {
		return err
	}

	latestByDevice := make(map[string]*model.TelemetryCurrentData, len(deviceIDs))
	for _, row := range rows {
		if row == nil || row.DeviceID == "" || row.T.IsZero() || latestByDevice[row.DeviceID] != nil {
			continue
		}
		latestByDevice[row.DeviceID] = row
	}
	for index := range *deviceList {
		if latest := latestByDevice[(*deviceList)[index].ID]; latest != nil {
			ts := latest.T
			(*deviceList)[index].Ts = &ts
		}
	}
	return nil
}
func hasDeviceListValue(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}

func deviceListLike(f field.String, v string) field.Expr {
	return f.Like(fmt.Sprintf("%%%s%%", v))
}

// rdiDeviceSharedStatusCondition keeps the SQL-side list filter aligned with
// service.rdiDeviceSharedStatus: shared means the RDI recipient array exists
// and contains at least one non-empty user_id.
// 批次二去单例化：不再用 query.Device.Where(...).Or(...) 包装，改为 field.And / field.Or
// 纯表达式组合，布尔结构与旧版逐项对齐（shared=AND 两条 Like；unshared=IS NULL OR 两条 NotLike）。
func rdiDeviceSharedStatusCondition(additionalInfo field.String, shared bool) field.Expr {
	normalized := additionalInfo.
		Replace(" ", "").
		Replace("\n", "").
		Replace("\r", "").
		Replace("\t", "")
	nonEmptyUserIDs := normalized.Replace(`"user_id":""`, "")
	const (
		recipientsKeyPattern = `%"rdi_share_recipients":[%`
		nonEmptyUserPattern  = `%"user_id":"%`
	)

	if shared {
		return field.And(
			normalized.Like(recipientsKeyPattern),
			nonEmptyUserIDs.Like(nonEmptyUserPattern),
		)
	}

	return field.Or(
		additionalInfo.IsNull(),
		normalized.NotLike(recipientsKeyPattern),
		nonEmptyUserIDs.NotLike(nonEmptyUserPattern),
	)
}
