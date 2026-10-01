// 本文件负责「当前告警」（model.AlarmInfo）的持久化访问。
//
// 从 alarm.go 按聚合拆出（wave5 be-dal-repositories 轨道）。职责边界：
// 当前告警的增删改查、按页查询与租户作用域收窄，以及 remark 为 JSON 时的
// 展开与生命周期状态推导。
//
// 关键约束：
// - remark 是 JSON 列，读写两侧都必须容错（脏数据不能 panic，按空值继续）。
// - 租户作用域收窄集中在 newAlarmInfoListScopedDB。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// 根据告警历史 ID 获取历史详情，并在存在时展开关联设备列表。
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetAlarmInfoHistoryByID(id string, ownerUserID *string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := query.AlarmHistory.Where(query.AlarmHistory.ID.Eq(id)).Select(query.AlarmHistory.ALL).Scan(&result)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	expandMapRemarkFields(result)
	if result["alarm_device_list"] == nil {
		return result, nil
	}
	result["alarm_device_list"] = alarmHistoryDeviceListMaps(result["alarm_device_list"], ownerUserID)
	return result, nil
}

func expandMapRemarkFields(item map[string]interface{}) {
	if item == nil {
		return
	}
	var rawRemark string
	switch v := item["remark"].(type) {
	case string:
		rawRemark = v
	case *string:
		if v != nil {
			rawRemark = *v
		}
	}
	statusStr := ""
	if s, ok := item["alarm_status"].(string); ok {
		statusStr = s
	}
	item["lifecycle_status"] = computeAlarmLifecycleStatus(statusStr, &rawRemark)
	if strings.TrimSpace(rawRemark) != "" {
		var r map[string]interface{}
		if err := json.Unmarshal([]byte(rawRemark), &r); err == nil {
			for _, k := range []string{"acknowledged", "acknowledged_by", "acknowledged_at", "reset", "reset_by", "reset_at", "cleared_by", "cleared_at", "action_note", "sla_escalation"} {
				if val, exists := r[k]; exists && val != nil {
					item[k] = val
				}
			}
		}
	}
}

func computeMapLifecycleStatus(item map[string]interface{}) string {
	if item == nil {
		return "ACTIVE_UNACK"
	}
	var rawRemark string
	switch v := item["remark"].(type) {
	case string:
		rawRemark = v
	case *string:
		if v != nil {
			rawRemark = *v
		}
	}
	statusStr := ""
	if s, ok := item["alarm_status"].(string); ok {
		statusStr = s
	}
	return computeAlarmLifecycleStatus(statusStr, &rawRemark)
}

// GetAlarmConfigListByPage 分页查询告警配置，支持租户、名称、等级和启用状态过滤。
// allTenants 仅限 SYS_ADMIN 显式全租户视角；其余调用方必须携带非空租户，否则 fail-closed。

func CreateAlarmInfo(d *model.AlarmInfo) error {
	return query.AlarmInfo.Create(d)
}

// tenant-scope: caller-enforced?2026-08-26 ?????

// tenant-scope: caller-enforced?2026-08-26 ?????
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

// GetAlarmInfoListByPageForScopes 层级作用域变体（ROADMAP C2）：alarm_info.tenant_id IN (scopes)。
// tenant-scope: caller-enforced (scopes 由 service 层展开并校验)。

// GetAlarmInfoListByPageForScopes 层级作用域变体（ROADMAP C2）：alarm_info.tenant_id IN (scopes)。
// tenant-scope: caller-enforced (scopes 由 service 层展开并校验)。
func GetAlarmInfoListByPageForScopes(d *model.GetAlarmInfoListByPageReq, allTenants bool, scopes []string) (int64, interface{}, error) {
	return alarmInfoListByPageScoped(d, allTenants, scopes)
}

func GetAlarmInfoListByPage(d *model.GetAlarmInfoListByPageReq, allTenants bool) (int64, interface{}, error) {
	return alarmInfoListByPageScoped(d, allTenants, nil)
}

func alarmInfoListByPageScoped(d *model.GetAlarmInfoListByPageReq, allTenants bool, scopes []string) (int64, interface{}, error) {
	base, err := newAlarmInfoListScopedDB(d, allTenants, scopes...)
	if err != nil {
		return 0, nil, err
	}
	var count int64
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
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
// GetAlarmHistoryListByPageForScopes 层级作用域变体（ROADMAP C2）：alarm_history.tenant_id IN (scopes)。
// tenant-scope: caller-enforced (scopes 由 service 层展开并校验)。

// newAlarmInfoListScopedDB 构造告警信息列表的 raw 语句根与过滤条件，
// 条件语义与收敛前的 applyAlarmInfoListFilters 逐条对齐。
// 空租户守卫（ROADMAP A1）：租户为空且未显式声明全租户视角时拒绝查询，
// 防止条件过滤被静默跳过后退化为跨租户全表扫描（与 board/users 收敛模式一致）。
func newAlarmInfoListScopedDB(req *model.GetAlarmInfoListByPageReq, allTenants bool, tenantScopes ...string) (*gorm.DB, error) {
	builder := global.DB.Table("alarm_info AS ai")
	if req == nil {
		return builder, nil
	}
	if len(tenantScopes) > 0 {
		switch len(tenantScopes) {
		case 1:
			builder = builder.Where("ai.tenant_id = ?", tenantScopes[0])
		default:
			builder = builder.Where("ai.tenant_id IN ?", tenantScopes)
		}
	} else if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
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
// （唯一调用方 GetAlarmHistoryListByPage 已于此前收敛为 raw global.DB 链，
// 见本文件 GetAlarmHistoryListByPage 的 Table("alarm_history AS ah")+
// Joins("LEFT JOIN alarm_config ac ...")+Select+Order 内联实现），
// 现整体删除以杜绝复用回退到 gen LeftJoin；过滤语义由 applyAlarmHistoryScopedFilters 承接。
