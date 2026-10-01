// 本文件负责「告警历史」（model.AlarmHistory）的持久化访问。
//
// 从 alarm.go 按聚合拆出（wave5 be-dal-repositories 轨道），是原文件里最大的一块。
// 职责边界：历史的分页查询/作用域收窄、月度趋势、写入与描述更新、确认/重置动作、
// remark JSON 合并、各类计数，以及本文件用到的 SQL 片段常量。
//
// 关键约束：
// - 确认与重置必须在原 remark 基础上合并字段，不能整体覆盖（会丢既有备注）。
// - alarm_device_list 是 jsonb 数组；142.sql 已建 alarm_history_devices 关联表作为它的
//   关系型投影（触发器同步），集合过滤优先走关联表而非 jsonb 展开。
// - 所有查询先限定租户；跨租户（allTenants）只允许在 scope 收窄之后放开。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// alarmHistoryOwnerExistsSQL 告警历史的 owner 可见性过滤（TB-22 规范化后的形态）。
//
// 旧形态用 jsonb_array_elements_text 逐行展开 alarm_device_list 再 JOIN devices，
// 既无法走索引（每行一次函数扫描），也让 planner 无法估算集合大小。142.sql 建了
// alarm_history_devices 关联表（jsonb 列保留并由触发器同步），这里改成普通 btree join：
// 关联表主键 (alarm_history_id, device_id)、索引 (device_id, tenant_id) 均可命中。
// 语义不变：告警历史只要命中任一属于该 owner 的设备即对该 owner 可见。
const alarmHistoryOwnerExistsSQL = `EXISTS (
    SELECT 1
    FROM alarm_history_devices ahd
    INNER JOIN devices scoped_device
        ON scoped_device.id = ahd.device_id
       AND scoped_device.tenant_id = ah.tenant_id
    WHERE ahd.alarm_history_id = ah.id
      AND scoped_device.owner_user_id = ?
)`

// alarmHistoryDeviceExistsByIDSQL 告警历史"命中指定设备"过滤（142.sql 关联表形态）。
//
// 替换旧的两类写法：jsonb_exists(alarm_device_list, ?) 与 alarm_device_list::text LIKE '%id%'。
// 两者都需要逐行计算且无法走索引；LIKE 形态还有子串误命中（设备 id 是另一 id 的前缀时错配）。
// 关联表上的等值匹配既走索引又消除误命中。

// alarmHistoryDeviceExistsByIDSQL 告警历史"命中指定设备"过滤（142.sql 关联表形态）。
//
// 替换旧的两类写法：jsonb_exists(alarm_device_list, ?) 与 alarm_device_list::text LIKE '%id%'。
// 两者都需要逐行计算且无法走索引；LIKE 形态还有子串误命中（设备 id 是另一 id 的前缀时错配）。
// 关联表上的等值匹配既走索引又消除误命中。
const alarmHistoryDeviceExistsByIDSQL = `EXISTS (
    SELECT 1
    FROM alarm_history_devices ahd
    WHERE ahd.alarm_history_id = ah.id
      AND ahd.device_id = ?
)`

// alarmHistoryDeviceExistsByIDUnqualified 非别名形态（gen 链查询用，表名不带 ah 别名）。

// alarmHistoryDeviceExistsByIDUnqualified 非别名形态（gen 链查询用，表名不带 ah 别名）。
const alarmHistoryDeviceExistsByIDUnqualified = `EXISTS (
    SELECT 1
    FROM alarm_history_devices ahd
    WHERE ahd.alarm_history_id = alarm_history.id
      AND ahd.device_id = ?
)`

const alarmHistoryCurrentActiveExistsSQL = `EXISTS (
    SELECT 1
    FROM current_device_alarm_streams current_alarm
    INNER JOIN devices current_device
        ON current_device.id = current_alarm.device_id
       AND current_device.tenant_id = current_alarm.tenant_id
       AND current_device.activate_flag = 'active'
    WHERE current_alarm.id = ah.id
      AND current_alarm.tenant_id = ah.tenant_id
      AND current_alarm.alarm_status IN ('H', 'M', 'L')
      AND (? = '' OR current_alarm.device_id = ?)
      AND (? = '' OR current_device.owner_user_id = ?)
)`

// alarmHistoryScanBatchSize 控制 GetDeviceIdsByAlarmConfigId 的分批扫描窗口，
// 避免历史表无限增长时一次性把全表载入内存。
const alarmHistoryScanBatchSize = 1000

// GetDeviceIdsByAlarmConfigId 返回触发过指定告警配置的设备 ID 去重列表。
// tenant-scope: parent-owned?2026-08-26 ?????

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetAlarmHistoryByID(id string) (*model.AlarmHistory, error) {
	data, err := query.AlarmHistory.Where(query.AlarmHistory.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetAlarmHistoriesByIDs(ids []string) ([]*model.AlarmHistory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return query.AlarmHistory.Where(query.AlarmHistory.ID.In(ids...)).Find()
}

// 根据告警历史 ID 获取历史详情，并在存在时展开关联设备列表。
// tenant-scope: caller-enforced?2026-08-26 ?????

// GetAlarmHistoryListByPage 分页查询告警历史，并展开关联设备列表供上层直接展示。
// GetAlarmHistoryListByPageForScopes 层级作用域变体（ROADMAP C2）：alarm_history.tenant_id IN (scopes)。
// tenant-scope: caller-enforced (scopes 由 service 层展开并校验)。
func GetAlarmHistoryListByPageForScopes(d *model.GetAlarmHisttoryListByPage, scopes []string, ownerUserID *string) (int64, interface{}, error) {
	allTenants := d != nil && d.AllTenants
	if !allTenants && len(scopes) == 0 {
		logrus.Warn("dal: scoped alarm history query requires at least one tenant")
		return 0, nil, fmt.Errorf("tenant id is required")
	}
	queryBuilder := applyAlarmHistoryScopedFilters(newAlarmHistoryScopedDB("", ownerUserID, allTenants, scopes...), d, ownerUserID)
	var count int64
	if err := queryBuilder.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return count, nil, err
	}
	listBuilder := queryBuilder.Session(&gorm.Session{}).
		Select("ah.*, ac.name AS alarm_config_name, ac.alarm_level AS alarm_level").
		Joins("LEFT JOIN alarm_config ac ON ac.id = ah.alarm_config_id").
		Order("ah.create_at DESC")
	listBuilder = applyListPagination(listBuilder, d.Page, d.PageSize)
	list := make([]map[string]interface{}, 0)
	if err := listBuilder.Scan(&list).Error; err != nil {
		return 0, nil, err
	}
	if isAlarmHistoryActiveStatusFilter(d.AlarmStatus) {
		if err := expandCurrentActiveAlarmHistoryDeviceFields(list, ownerUserID); err != nil {
			return 0, nil, err
		}
	} else {
		expandAlarmHistoryListDeviceFields(list, ownerUserID)
	}
	for _, item := range list {
		expandMapRemarkFields(item)
	}
	return count, list, nil
}

// alarmHistoryOwnerExistsSQL 告警历史的 owner 可见性过滤（TB-22 规范化后的形态）。
//
// 旧形态用 jsonb_array_elements_text 逐行展开 alarm_device_list 再 JOIN devices，
// 既无法走索引（每行一次函数扫描），也让 planner 无法估算集合大小。142.sql 建了
// alarm_history_devices 关联表（jsonb 列保留并由触发器同步），这里改成普通 btree join：
// 关联表主键 (alarm_history_id, device_id)、索引 (device_id, tenant_id) 均可命中。
// 语义不变：告警历史只要命中任一属于该 owner 的设备即对该 owner 可见。

func newAlarmHistoryScopedDB(tenantID string, ownerUserID *string, allTenants bool, tenantScopes ...string) *gorm.DB {
	builder := global.DB.Table("alarm_history AS ah")
	if !allTenants {
		switch len(tenantScopes) {
		case 0:
			builder = builder.Where("ah.tenant_id = ?", tenantID)
		case 1:
			builder = builder.Where("ah.tenant_id = ?", tenantScopes[0])
		default:
			builder = builder.Where("ah.tenant_id IN ?", tenantScopes)
		}
	}
	if ownerUserID == nil || strings.TrimSpace(*ownerUserID) == "" {
		return builder
	}
	return builder.Where(alarmHistoryOwnerExistsSQL, strings.TrimSpace(*ownerUserID))
}

func applyAlarmHistoryScopedFilters(builder *gorm.DB, req *model.GetAlarmHisttoryListByPage, ownerUserID *string) *gorm.DB {
	if req == nil {
		return builder
	}
	if req.StartTime != nil && req.EndTime != nil && !req.StartTime.IsZero() && !req.EndTime.IsZero() {
		builder = builder.Where("ah.create_at BETWEEN ? AND ?", *req.StartTime, *req.EndTime)
	}
	if isAlarmHistoryActiveStatusFilter(req.AlarmStatus) {
		deviceID := ""
		if req.DeviceId != nil {
			deviceID = strings.TrimSpace(*req.DeviceId)
		}
		ownerID := ""
		if ownerUserID != nil {
			ownerID = strings.TrimSpace(*ownerUserID)
		}
		builder = builder.Where(alarmHistoryCurrentActiveExistsSQL, deviceID, deviceID, ownerID, ownerID)
	} else {
		statusValues := alarmHistoryStatusFilterValues(req.AlarmStatus)
		if len(statusValues) == 1 {
			builder = builder.Where("ah.alarm_status = ?", statusValues[0])
		}
	}
	if req.AlarmType != nil && strings.TrimSpace(*req.AlarmType) != "" {
		alarmType := strings.TrimSpace(*req.AlarmType)
		if alarmType == "PT" || alarmType == "pressure_alarm" {
			builder = builder.Where(
				"COALESCE(ah.remark::text, '') LIKE ? OR COALESCE(ah.remark::text, '') LIKE ?",
				`%"event_type":"PT"%`,
				`%"event_type":"pressure_alarm"%`,
			)
		} else {
			builder = builder.Where("COALESCE(ah.remark::text, '') LIKE ?", fmt.Sprintf(`%%"event_type":"%s"%%`, EscapeLikePattern(alarmType)))
		}
	}
	if !isAlarmHistoryActiveStatusFilter(req.AlarmStatus) && req.DeviceId != nil && strings.TrimSpace(*req.DeviceId) != "" {
		builder = builder.Where(
			alarmHistoryDeviceExistsByIDSQL,
			strings.TrimSpace(*req.DeviceId),
		)
	}
	return builder
}

// GetAlarmHistoryMonthlyTrend aggregates twelve calendar-month buckets in PostgreSQL.
// H/M/L rows count directly. Reset rows remain historical occurrences through reset_at,
// while ordinary N recovery rows are excluded. ownerUserID narrows TENANT_USER data to
// alarm rows that reference at least one device owned by that user.

// GetAlarmHistoryMonthlyTrend aggregates twelve calendar-month buckets in PostgreSQL.
// H/M/L rows count directly. Reset rows remain historical occurrences through reset_at,
// while ordinary N recovery rows are excluded. ownerUserID narrows TENANT_USER data to
// alarm rows that reference at least one device owned by that user.
func GetAlarmHistoryMonthlyTrend(tenantID string, ownerUserID *string, startTime, endTime time.Time, timezone string, allTenants bool) ([]model.AlarmHistoryMonthlyTrendPoint, error) {
	ownerFilter := ""
	if ownerUserID != nil {
		ownerFilter = strings.TrimSpace(*ownerUserID)
	}

	const sql = `
WITH params AS (
    SELECT
        ?::text AS tenant_id,
        ?::timestamptz AS start_time,
        ?::timestamptz AS end_time,
        ?::text AS owner_user_id,
        ?::text AS timezone,
        ?::boolean AS all_tenants
), months AS (
    SELECT generate_series(1, 12)::int AS month
), alarm_counts AS (
    SELECT
        EXTRACT(MONTH FROM ah.create_at AT TIME ZONE params.timezone)::int AS month,
        COUNT(*)::bigint AS count
    FROM alarm_history ah
    CROSS JOIN params
    WHERE (params.all_tenants OR ah.tenant_id = params.tenant_id)
      AND ah.create_at >= params.start_time
      AND ah.create_at < params.end_time
      AND (
          ah.alarm_status IN ('H', 'M', 'L')
          OR COALESCE(ah.remark::text, '') LIKE '%"reset_at"%'
      )
      AND (
          params.owner_user_id = ''
          OR EXISTS (
              SELECT 1
              FROM alarm_history_devices ahd
              INNER JOIN devices d ON d.id = ahd.device_id
              WHERE ahd.alarm_history_id = ah.id
                AND d.tenant_id = ah.tenant_id
                AND d.owner_user_id = params.owner_user_id
          )
      )
    GROUP BY EXTRACT(MONTH FROM ah.create_at AT TIME ZONE params.timezone)
)
SELECT
    months.month,
    COALESCE(alarm_counts.count, 0)::bigint AS count
FROM months
LEFT JOIN alarm_counts ON alarm_counts.month = months.month
ORDER BY months.month`

	points := make([]model.AlarmHistoryMonthlyTrendPoint, 0, 12)
	if err := global.DB.Raw(sql, tenantID, startTime, endTime, ownerFilter, timezone, allTenants).Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func AlarmHistorySave(history *model.AlarmHistory) error {
	return query.AlarmHistory.Save(history)
}

func AlarmHistoryDescUpdate(req *model.AlarmHistoryDescUpdateReq, tenantID string) error {
	result, err := query.AlarmHistory.Where(query.AlarmHistory.ID.Eq(req.AlarmHistoryId), query.AlarmHistory.TenantID.Eq(tenantID)).UpdateColumn(query.AlarmHistory.Description, req.Description)
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errors.New("set alarm history description failed")
	}
	return nil
}

func AcknowledgeAlarmHistory(id, tenantID, userID string) (*model.AlarmHistoryActionResp, error) {
	return acknowledgeAlarmHistory(id, tenantID, userID, "")
}

func AcknowledgeAlarmHistoryWithNote(id, tenantID, userID, note string) (*model.AlarmHistoryActionResp, error) {
	return acknowledgeAlarmHistory(id, tenantID, userID, note)
}

func ResetAlarmHistory(id, tenantID, userID string) (*model.AlarmHistoryActionResp, error) {
	return resetAlarmHistory(id, tenantID, userID, "")
}

func ResetAlarmHistoryWithNote(id, tenantID, userID, note string) (*model.AlarmHistoryActionResp, error) {
	return resetAlarmHistory(id, tenantID, userID, note)
}

// alarmHistoryDeviceListMaps 把历史记录里的设备 ID 列表展开成设备摘要。
// 这里保留原有的查询方式，只是把重复的 JSON 解析和设备查询收敛起来。

// alarmHistoryDeviceListMaps 把历史记录里的设备 ID 列表展开成设备摘要。
// 这里保留原有的查询方式，只是把重复的 JSON 解析和设备查询收敛起来。
func alarmHistoryDeviceListMaps(raw interface{}, ownerUserID *string) []map[string]interface{} {
	deviceIDs := alarmHistoryDeviceIDsFromValue(raw)
	return alarmHistoryDeviceRows(deviceIDs, loadAlarmHistoryDevicesByID(deviceIDs, ownerUserID))
}

func alarmHistoryDeviceIDsFromValue(raw interface{}) []string {
	switch value := raw.(type) {
	case string:
		return alarmHistoryDeviceIDs(value)
	case []byte:
		return alarmHistoryDeviceIDs(string(value))
	case json.RawMessage:
		return alarmHistoryDeviceIDs(string(value))
	default:
		return nil
	}
}

func mergeAlarmHistoryRemark(raw *string, fields map[string]interface{}) string {
	remark := make(map[string]interface{})
	if raw != nil && strings.TrimSpace(*raw) != "" {
		if err := json.Unmarshal([]byte(*raw), &remark); err != nil {
			remark["previous_remark"] = *raw
		}
	}
	for key, value := range fields {
		remark[key] = value
	}
	bytes, err := json.Marshal(remark)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func alarmHistoryDeviceIDs(raw string) []string {
	var ids []string
	if strings.TrimSpace(raw) == "" {
		return ids
	}
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		// 解析失败按空设备列表继续，不影响列表渲染；记录片段便于定位脏数据。
		logrus.Warnf("alarm history alarm_device_list 解析失败: err=%v raw_prefix=%q", err, alarmHistoryRawLogPreview(raw))
	}
	return ids
}

// alarmHistoryRawLogPreview 截取原始 JSON 的前 64 字节用于日志输出。

// alarmHistoryRawLogPreview 截取原始 JSON 的前 64 字节用于日志输出。
func alarmHistoryRawLogPreview(raw string) string {
	if len(raw) > 64 {
		return raw[:64]
	}
	return raw
}

// alarmHistoryDeviceConditions 拼装“租户 + 设备命中”的告警历史查询条件。
// 该条件在设备告警状态和设备关联配置查询中共用。
// 设备命中由 jsonb_exists(alarm_device_list, ?) 改为 142.sql 的关联表 EXISTS：
// 前者无法走索引，后者命中 alarm_history_devices 主键，且不再受 JSON 元素顺序影响。

// alarmHistoryDeviceConditions 拼装“租户 + 设备命中”的告警历史查询条件。
// 该条件在设备告警状态和设备关联配置查询中共用。
// 设备命中由 jsonb_exists(alarm_device_list, ?) 改为 142.sql 的关联表 EXISTS：
// 前者无法走索引，后者命中 alarm_history_devices 主键，且不再受 JSON 元素顺序影响。
func alarmHistoryDeviceConditions(tenantID, deviceID string) []gen.Condition {
	return append(
		[]gen.Condition{query.AlarmHistory.TenantID.Eq(tenantID)},
		gen.Cond(clause.Expr{SQL: alarmHistoryDeviceExistsByIDUnqualified, Vars: []interface{}{deviceID}})...,
	)
}

// DeleteAlarmHistoryByConfigId 删除指定告警配置对应的全部历史记录。

// alarmHistoryScanBatchSize 控制 GetDeviceIdsByAlarmConfigId 的分批扫描窗口，
// 避免历史表无限增长时一次性把全表载入内存。

func alarmHistoryStatusFilterValues(alarmStatus *string) []string {
	if alarmStatus == nil {
		return nil
	}
	status := strings.TrimSpace(*alarmStatus)
	if status == "" {
		return nil
	}
	if status == model.AlarmHistoryQueryStatusActive {
		return []string{"H", "M", "L"}
	}
	return []string{status}
}

func isAlarmHistoryActiveStatusFilter(alarmStatus *string) bool {
	return alarmStatus != nil && strings.TrimSpace(*alarmStatus) == model.AlarmHistoryQueryStatusActive
}

// tenant-scope: caller-enforced?2026-08-26 ?????

func CountActiveAlarmHistoryByScope(tenantID string, ownerUserID *string, allTenants bool) (int64, error) {
	var count int64
	builder := global.DB.Table("current_device_alarm_streams AS current_alarm").
		Joins("INNER JOIN devices current_device ON current_device.id = current_alarm.device_id AND current_device.tenant_id = current_alarm.tenant_id AND current_device.activate_flag = ?", "active").
		Where("current_alarm.alarm_status IN ?", []string{"H", "M", "L"})
	if !allTenants {
		builder = builder.Where("current_alarm.tenant_id = ?", tenantID)
	}
	if ownerUserID != nil && strings.TrimSpace(*ownerUserID) != "" {
		builder = builder.Where("current_device.owner_user_id = ?", strings.TrimSpace(*ownerUserID))
	}
	err := builder.Distinct("current_alarm.id").Count(&count).Error
	return count, err
}

func CountAlarmHistoryByScope(tenantID string, ownerUserID *string, allTenants bool) (int64, error) {
	var count int64
	err := newAlarmHistoryScopedDB(tenantID, ownerUserID, allTenants).Count(&count).Error
	return count, err
}
