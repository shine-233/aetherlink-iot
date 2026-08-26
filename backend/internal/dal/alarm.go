// 本文件负责告警配置、告警信息、告警历史和告警名称缓存的持久化访问。
//
// 这里的职责是把 GORM Gen 查询、事务句柄、JSON 字段处理和模型转换
// 收敛成稳定的 DAL 边界，不承载业务权限判断，也不直接拼装 API 响应。
//
// 关键约束：
// - 所有告警查询都要先限定租户，再按需叠加名称、等级、状态、时间和设备过滤。
// - 告警历史的 remark 是 JSON，确认和重置时必须在原值基础上合并字段。
// - 设备列表需要把 alarm_device_list 从 ID 列表展开成设备摘要，避免上层重复处理。
// - 复杂过滤、分页和事务更新建议继续收敛为纯 helper，便于后续补充 focused DAL 测试。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func CreateAlarmConfig(d *model.AlarmConfig) error {
	return query.AlarmConfig.Create(d)
}

func UpdateAlarmConfig(d *model.AlarmConfig) error {
	info, err := query.AlarmConfig.Updates(d)
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data updated")
	}
	return nil
}

// UpdateAlarmConfigTriggerDuration 显式按列写入触发持续时长。
// 结构体形式的 Updates 会跳过零值，因此把 trigger_duration 改回 0 必须走这里。
func UpdateAlarmConfigTriggerDuration(id string, triggerDuration int32) error {
	info, err := query.AlarmConfig.Where(query.AlarmConfig.ID.Eq(id)).
		Update(query.AlarmConfig.TriggerDuration, triggerDuration)
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data updated")
	}
	return nil
}

func DeleteAlarmConfig(id string) error {
	info, err := query.AlarmConfig.Where(query.AlarmConfig.ID.Eq(id)).Delete()
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data deleted")
	}
	return nil
}

func GetAlarmByID(id string) (*model.AlarmConfig, error) {
	data, err := query.AlarmConfig.Where(query.AlarmConfig.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetAlarmHistoryByID(id string) (*model.AlarmHistory, error) {
	data, err := query.AlarmHistory.Where(query.AlarmHistory.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetAlarmHistoriesByIDs(ids []string) ([]*model.AlarmHistory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return query.AlarmHistory.Where(query.AlarmHistory.ID.In(ids...)).Find()
}

// 根据告警历史 ID 获取历史详情，并在存在时展开关联设备列表。
func GetAlarmInfoHistoryByID(id string, ownerUserID *string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := query.AlarmHistory.Where(query.AlarmHistory.ID.Eq(id)).Select(query.AlarmHistory.ALL).Scan(&result)
	if err != nil {
		return nil, err
	}
	if result["alarm_device_list"] == nil {
		return result, nil
	}
	result["alarm_device_list"] = alarmHistoryDeviceListMaps(result["alarm_device_list"], ownerUserID)
	return result, nil
}

// GetAlarmConfigListByPage 分页查询告警配置，支持租户、名称、等级和启用状态过滤。
// allTenants 仅限 SYS_ADMIN 显式全租户视角；其余调用方必须携带非空租户，否则 fail-closed。
func GetAlarmConfigListByPage(d *model.GetAlarmConfigListByPageReq, allTenants bool) (int64, interface{}, error) {
	// P1 修复（2026-08-24，见 VALIDATION.md）：告警配置列表改走 raw global.DB 链
	// （clone==1 根，每次链式起点均为全新 Statement），杜绝包级单例
	// LeftJoin(NotificationGroup)+ALL+Scan 在高并发下跨请求残留 Statement 读到空/旧数据；
	// JOIN 形态、投影列名、排序与分页语义与收敛前逐条一致。
	base, err := newAlarmConfigListScopedDB(d, allTenants)
	if err != nil {
		return 0, nil, err
	}
	var count int64
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	listBuilder := base.Session(&gorm.Session{}).
		Select("ac.*, ng.name AS notification_group_name").
		Joins("LEFT JOIN notification_groups ng ON ng.id = ac.notification_group_id").
		Order("ac.created_at DESC")
	listBuilder = applyListPagination(listBuilder, d.Page, d.PageSize)
	list := make([]map[string]interface{}, 0)
	if err := listBuilder.Scan(&list).Error; err != nil {
		return 0, nil, err
	}
	return count, list, nil
}

func CreateAlarmInfo(d *model.AlarmInfo) error {
	return query.AlarmInfo.Create(d)
}

func GetAlarmInfoByID(id string) (*model.AlarmInfo, error) {
	data, err := query.AlarmInfo.Where(query.AlarmInfo.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("no data found")
	}
	return data, nil
}

func UpdateAlarmInfo(d *model.AlarmInfo) error {
	info, err := query.AlarmInfo.Updates(d)
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data updated")
	}
	return nil
}

func UpdateAlarmInfoBatch(req *model.UpdateAlarmInfoBatchReq, userid string, tenantID string) error {
	info, err := query.AlarmInfo.Where(query.AlarmInfo.ID.In(req.Id...), query.AlarmInfo.TenantID.Eq(tenantID)).
		Updates(map[string]interface{}{
			"processing_result": req.ProcessingResult,
			"content":           req.ProcessingInstructions,
			"processor":         userid})
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data updated")
	}
	return nil
}

// GetAlarmInfoListByPage 分页查询告警信息，支持按租户、时间、处理结果和等级过滤。
// allTenants 仅限 SYS_ADMIN 显式全租户视角；其余调用方必须携带非空租户，否则 fail-closed。
func GetAlarmInfoListByPage(d *model.GetAlarmInfoListByPageReq, allTenants bool) (int64, interface{}, error) {
	// P1 修复（2026-08-24，见 VALIDATION.md）：告警信息列表改走 raw global.DB 链
	// （clone==1 根，每次链式起点均为全新 Statement），杜绝包级单例
	// LeftJoin(AlarmConfig,User)+ALL+Scan 在高并发下跨请求残留 Statement 读到空/旧数据；
	// JOIN 形态、投影列名、排序与分页语义与收敛前逐条一致。
	base, err := newAlarmInfoListScopedDB(d, allTenants)
	if err != nil {
		return 0, nil, err
	}
	var count int64
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	listBuilder := base.Session(&gorm.Session{}).
		Select("ai.*, ac.name AS alarm_config_name, ac.alarm_level AS alarm_level, u.name AS processor_name").
		Joins("LEFT JOIN alarm_config ac ON ac.id = ai.alarm_config_id").
		Joins("LEFT JOIN users u ON ai.processor = u.id").
		Order("ai.alarm_time DESC")
	listBuilder = applyListPagination(listBuilder, d.Page, d.PageSize)
	list := make([]map[string]interface{}, 0)
	if err := listBuilder.Scan(&list).Error; err != nil {
		return 0, nil, err
	}
	return count, list, nil
}

// GetAlarmHistoryListByPage 分页查询告警历史，并展开关联设备列表供上层直接展示。
func GetAlarmHistoryListByPage(d *model.GetAlarmHisttoryListByPage, tenantID string, ownerUserID *string) (int64, interface{}, error) {
	allTenants := d != nil && d.AllTenants
	// 空租户守卫（ROADMAP A1）：未声明全租户视角时，空租户必须显式拒绝，
	// 避免 WHERE tenant_id='' 静默匹配 0 行造成"偶发空列表"假象（users 收敛模式）。
	if !allTenants && strings.TrimSpace(tenantID) == "" {
		logrus.Warn("dal: alarm history list query has empty TenantID without all-tenants scope; rejecting")
		return 0, nil, fmt.Errorf("tenant id is required")
	}
	queryBuilder := applyAlarmHistoryScopedFilters(newAlarmHistoryScopedDB(tenantID, ownerUserID, allTenants), d, ownerUserID)
	var count int64
	if err := queryBuilder.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
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
	return count, list, nil
}

const alarmHistoryOwnerExistsSQL = `EXISTS (
    SELECT 1
    FROM jsonb_array_elements_text(
        CASE
            WHEN ah.alarm_device_list IS NULL THEN '[]'::jsonb
            WHEN jsonb_typeof(ah.alarm_device_list::jsonb) = 'array' THEN ah.alarm_device_list::jsonb
            ELSE '[]'::jsonb
        END
    ) AS scoped_alarm_device(device_id)
    INNER JOIN devices scoped_device ON scoped_device.id = scoped_alarm_device.device_id
    WHERE scoped_device.tenant_id = ah.tenant_id
      AND scoped_device.owner_user_id = ?
)`

func newAlarmHistoryScopedDB(tenantID string, ownerUserID *string, allTenants bool) *gorm.DB {
	builder := global.DB.Table("alarm_history AS ah")
	if !allTenants {
		builder = builder.Where("ah.tenant_id = ?", tenantID)
	}
	if ownerUserID == nil || strings.TrimSpace(*ownerUserID) == "" {
		return builder
	}
	return builder.Where(alarmHistoryOwnerExistsSQL, strings.TrimSpace(*ownerUserID))
}

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
			builder = builder.Where("COALESCE(ah.remark::text, '') LIKE ?", fmt.Sprintf(`%%"event_type":"%s"%%`, alarmType))
		}
	}
	if !isAlarmHistoryActiveStatusFilter(req.AlarmStatus) && req.DeviceId != nil && strings.TrimSpace(*req.DeviceId) != "" {
		builder = builder.Where(
			"jsonb_exists(COALESCE(ah.alarm_device_list::jsonb, '[]'::jsonb), ?)",
			strings.TrimSpace(*req.DeviceId),
		)
	}
	return builder
}

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
              FROM jsonb_array_elements_text(
                  CASE
                      WHEN ah.alarm_device_list IS NULL THEN '[]'::jsonb
                      WHEN jsonb_typeof(ah.alarm_device_list::jsonb) = 'array'
                          THEN ah.alarm_device_list::jsonb
                      ELSE '[]'::jsonb
                  END
              ) AS alarm_device(device_id)
              INNER JOIN devices d ON d.id = alarm_device.device_id
              WHERE d.tenant_id = ah.tenant_id
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

func alarmHistoryAcknowledgeRemark(raw *string, userID, ackAt string) string {
	return mergeAlarmHistoryRemark(raw, map[string]interface{}{
		"acknowledged":    true,
		"acknowledged_by": userID,
		"acknowledged_at": ackAt,
	})
}

func alarmHistoryResetRemark(raw *string, userID, resetAt string) string {
	return mergeAlarmHistoryRemark(raw, map[string]interface{}{
		"reset":    true,
		"reset_by": userID,
		"reset_at": resetAt,
	})
}

func alarmHistoryResetUpdates(remark string) map[string]interface{} {
	return map[string]interface{}{
		"alarm_status": "N",
		"remark":       remark,
	}
}

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
func alarmHistoryRawLogPreview(raw string) string {
	if len(raw) > 64 {
		return raw[:64]
	}
	return raw
}

// alarmHistoryDeviceConditions 拼装“租户 + 设备命中”的告警历史查询条件。
// 该条件在设备告警状态和设备关联配置查询中共用。
func alarmHistoryDeviceConditions(tenantID, deviceID string) []gen.Condition {
	return append(
		[]gen.Condition{query.AlarmHistory.TenantID.Eq(tenantID)},
		gen.Cond(datatypes.JSONQuery("alarm_device_list").HasKey(deviceID))...,
	)
}

func GetDeviceAlarmStatus(req *model.GetDeviceAlarmStatusReq, tenantID string) (bool, error) {
	latest := query.LatestDeviceAlarm
	result, err := latest.Where(
		latest.TenantID.Eq(tenantID),
		latest.DeviceID.Eq(req.DeviceId),
	).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if result.AlarmStatus == nil {
		return false, nil
	}
	switch strings.ToUpper(strings.TrimSpace(*result.AlarmStatus)) {
	case "H", "M", "L":
		return true, nil
	default:
		return false, nil
	}
}

func GetConfigByDevice(req *model.GetDeviceAlarmStatusReq, tenantID string) ([]model.AlarmConfig, error) {
	var result []map[string]interface{}
	err := query.AlarmHistory.Where(alarmHistoryDeviceConditions(tenantID, req.DeviceId)...).
		Select(query.AlarmHistory.AlarmConfigID, query.AlarmHistory.AlarmConfigID.Count()).Group(query.AlarmHistory.AlarmConfigID).Scan(&result)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}

	var (
		configIDs []string
		config    []model.AlarmConfig
	)
	for _, v := range result {
		configIDs = append(configIDs, v["alarm_config_id"].(string))
	}
	return config, query.AlarmConfig.Where(query.AlarmConfig.ID.In(configIDs...), query.AlarmConfig.TenantID.Eq(tenantID)).Scan(&config)
}

func GetAlarmNameWithCache(alarmId string) string {
	redis := global.REDIS
	cacheKey := fmt.Sprintf("GetAlarmNameWithCache:alarmId:%s", alarmId)
	var result string
	err := redis.Get(context.Background(), cacheKey).Scan(&result)
	if err == nil && result != "" {
		return result
	}
	alarmConfig, err := query.AlarmConfig.Where(query.AlarmConfig.ID.Eq(alarmId)).Select(query.AlarmConfig.Name).First()
	if err != nil {
		return ""
	}
	redis.Set(context.Background(), cacheKey, alarmConfig.Name, time.Hour)
	return alarmConfig.Name
}

func DeleteAlarmHistory(id string, tenantID string) error {
	info, err := query.AlarmHistory.Where(query.AlarmHistory.ID.Eq(id), query.AlarmHistory.TenantID.Eq(tenantID)).Delete()
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data deleted")
	}
	return nil
}

// DeleteAlarmHistoryByConfigId 删除指定告警配置对应的全部历史记录。
func DeleteAlarmHistoryByConfigId(alarmConfigId string) error {
	_, err := query.AlarmHistory.Where(query.AlarmHistory.AlarmConfigID.Eq(alarmConfigId)).Delete()
	return err
}

// alarmHistoryScanBatchSize 控制 GetDeviceIdsByAlarmConfigId 的分批扫描窗口，
// 避免历史表无限增长时一次性把全表载入内存。
const alarmHistoryScanBatchSize = 1000

// GetDeviceIdsByAlarmConfigId 返回触发过指定告警配置的设备 ID 去重列表。
func GetDeviceIdsByAlarmConfigId(alarmConfigId string) ([]string, error) {
	hq := query.AlarmHistory
	deviceSet := make(map[string]struct{})
	lastID := ""
	for {
		q := hq.Where(hq.AlarmConfigID.Eq(alarmConfigId))
		if lastID != "" {
			q = q.Where(hq.ID.Gt(lastID))
		}
		batch, err := q.Select(hq.ID, hq.AlarmDeviceList).Order(hq.ID).Limit(alarmHistoryScanBatchSize).Find()
		if err != nil {
			return nil, err
		}
		for _, h := range batch {
			var deviceIds []string
			if h.AlarmDeviceList != "" {
				if err := json.Unmarshal([]byte(h.AlarmDeviceList), &deviceIds); err != nil {
					// 解析失败按空列表继续去重流程；带上配置与记录 ID 便于定位脏数据行。
					logrus.Warnf("alarm history alarm_device_list 解析失败: err=%v alarm_config_id=%s history_id=%s",
						err, h.AlarmConfigID, h.ID)
				}
			}
			for _, did := range deviceIds {
				deviceSet[did] = struct{}{}
			}
		}
		if len(batch) < alarmHistoryScanBatchSize {
			break
		}
		lastID = batch[len(batch)-1].ID
	}
	result := make([]string, 0, len(deviceSet))
	for did := range deviceSet {
		result = append(result, did)
	}
	return result, nil
}

// DeleteAlarmNameCache 删除告警名称缓存。
func DeleteAlarmNameCache(alarmId string) error {
	redis := global.REDIS
	cacheKey := fmt.Sprintf("GetAlarmNameWithCache:alarmId:%s", alarmId)
	return redis.Del(context.Background(), cacheKey).Err()
}

// newAlarmConfigListScopedDB 构造告警配置列表的 raw 语句根与过滤条件，
// 条件语义与收敛前的 applyAlarmConfigListFilters 逐条对齐。
// 空租户守卫（ROADMAP A1）：租户为空且未显式声明全租户视角时拒绝查询，
// 防止条件过滤被静默跳过后退化为跨租户全表扫描（与 board/users 收敛模式一致）。
func newAlarmConfigListScopedDB(req *model.GetAlarmConfigListByPageReq, allTenants bool) (*gorm.DB, error) {
	builder := global.DB.Table("alarm_config AS ac")
	if req == nil {
		return builder, nil
	}
	if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		builder = builder.Where("ac.tenant_id = ?", tenantID)
	} else if !allTenants {
		logrus.Warn("dal: alarm config list query has empty TenantID without all-tenants scope; rejecting")
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.Name != nil && *req.Name != "" {
		builder = builder.Where("ac.name LIKE ?", fmt.Sprintf("%%%s%%", *req.Name))
	}
	if req.AlarmLevel != nil && *req.AlarmLevel != "" {
		builder = builder.Where("ac.alarm_level = ?", *req.AlarmLevel)
	}
	if req.Enabled != "" {
		builder = builder.Where("ac.enabled = ?", req.Enabled)
	}
	return builder, nil
}

// newAlarmInfoListScopedDB 构造告警信息列表的 raw 语句根与过滤条件，
// 条件语义与收敛前的 applyAlarmInfoListFilters 逐条对齐。
// 空租户守卫（ROADMAP A1）：租户为空且未显式声明全租户视角时拒绝查询，
// 防止条件过滤被静默跳过后退化为跨租户全表扫描（与 board/users 收敛模式一致）。
func newAlarmInfoListScopedDB(req *model.GetAlarmInfoListByPageReq, allTenants bool) (*gorm.DB, error) {
	builder := global.DB.Table("alarm_info AS ai")
	if req == nil {
		return builder, nil
	}
	if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		builder = builder.Where("ai.tenant_id = ?", tenantID)
	} else if !allTenants {
		logrus.Warn("dal: alarm info list query has empty TenantID without all-tenants scope; rejecting")
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.StartTime != nil && req.EndTime != nil {
		builder = builder.Where("ai.alarm_time BETWEEN ? AND ?", *req.StartTime, *req.EndTime)
	}
	if req.ProcessingResult != nil && *req.ProcessingResult != "" {
		builder = builder.Where("ai.processing_result = ?", *req.ProcessingResult)
	}
	if req.AlarmLevel != nil && *req.AlarmLevel != "" {
		builder = builder.Where("ai.alarm_level = ?", *req.AlarmLevel)
	}
	return builder, nil
}

// P1 修复（2026-08-24，见 VALIDATION.md）：告警历史列表的 gen LeftJoin 收敛完成。
// 原 applyAlarmHistoryListFilters/applyAlarmHistoryTimeFilter/applyAlarmHistoryStatusFilter/
// applyAlarmHistoryTypeFilter/applyAlarmHistoryDeviceFilter/withAlarmHistoryListJoins/
// applyAlarmHistoryListPage/scanAlarmHistoryList 为 gen 继承式语句根的遗留死代码
//（唯一调用方 GetAlarmHistoryListByPage 已于此前收敛为 raw global.DB 链，
// 见本文件 GetAlarmHistoryListByPage 的 Table("alarm_history AS ah")+
// Joins("LEFT JOIN alarm_config ac ...")+Select+Order 内联实现），
// 现整体删除以杜绝复用回退到 gen LeftJoin；过滤语义由 applyAlarmHistoryScopedFilters 承接。
func newAlarmHistoryTenantQuery(tenantID string) query.IAlarmHistoryDo {
	return query.AlarmHistory.WithContext(context.Background()).
		Where(query.AlarmHistory.TenantID.Eq(tenantID))
}

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

func CountActiveAlarmHistoryByTenant(tenantID string, ownerUserID *string) (int64, error) {
	return CountActiveAlarmHistoryByScope(tenantID, ownerUserID, false)
}

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

func getAlarmHistoryForAction(id, tenantID string) (*model.AlarmHistory, error) {
	return alarmHistoryRecordByID(id, tenantID).First()
}

func alarmHistoryRecordByID(id, tenantID string) query.IAlarmHistoryDo {
	return query.AlarmHistory.Where(
		query.AlarmHistory.ID.Eq(id),
		query.AlarmHistory.TenantID.Eq(tenantID),
	)
}

func updateAlarmHistoryRemark(id, tenantID, remark string) error {
	result, err := alarmHistoryRecordByID(id, tenantID).
		UpdateColumn(query.AlarmHistory.Remark, remark)
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errors.New("acknowledge alarm failed")
	}
	return nil
}

func applyAlarmHistoryReset(id, tenantID, remark string) error {
	result, err := alarmHistoryRecordByID(id, tenantID).
		Where(query.AlarmHistory.AlarmStatus.In("H", "M", "L")).
		Updates(alarmHistoryResetUpdates(remark))
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errors.New("reset alarm failed")
	}
	return nil
}
