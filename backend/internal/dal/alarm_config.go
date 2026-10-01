// 本文件负责「告警配置」（model.AlarmConfig）的持久化访问。
//
// 从 alarm.go 按聚合拆出（wave5 be-dal-repositories 轨道）。职责边界：
// 配置的增删改查、按页查询与租户作用域收窄，以及"按设备反查生效配置"。
//
// 关键约束：
// - 所有列表查询都要先限定租户，再叠加名称/等级/状态/时间/设备过滤。
// - 租户作用域收窄集中在 newAlarmConfigListScopedDB，不要在调用点各写一遍。

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

// tenant-scope: caller-enforced?2026-08-26 ?????

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetAlarmByID(id string) (*model.AlarmConfig, error) {
	data, err := query.AlarmConfig.Where(query.AlarmConfig.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????

// GetAlarmConfigListByPage 分页查询告警配置，支持租户、名称、等级和启用状态过滤。
// allTenants 仅限 SYS_ADMIN 显式全租户视角；其余调用方必须携带非空租户，否则 fail-closed。
func GetAlarmConfigListByPage(d *model.GetAlarmConfigListByPageReq, allTenants bool) (int64, interface{}, error) {
	return alarmConfigListByPageScoped(d, allTenants, nil)
}

// GetAlarmConfigListByPageForScopes 层级作用域变体（ROADMAP C2）：alarm_config.tenant_id IN (scopes)。
// tenant-scope: caller-enforced (scopes 由 service 层展开并校验)。

// GetAlarmConfigListByPageForScopes 层级作用域变体（ROADMAP C2）：alarm_config.tenant_id IN (scopes)。
// tenant-scope: caller-enforced (scopes 由 service 层展开并校验)。
func GetAlarmConfigListByPageForScopes(d *model.GetAlarmConfigListByPageReq, allTenants bool, scopes []string) (int64, interface{}, error) {
	return alarmConfigListByPageScoped(d, allTenants, scopes)
}

func alarmConfigListByPageScoped(d *model.GetAlarmConfigListByPageReq, allTenants bool, scopes []string) (int64, interface{}, error) {
	base, err := newAlarmConfigListScopedDB(d, allTenants, scopes...)
	if err != nil {
		return 0, nil, err
	}
	var count int64
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return count, nil, err
	}

	listBuilder := base.Session(&gorm.Session{}).Select("ac.*, ng.name AS notification_group_name").
		Joins("LEFT JOIN notification_groups ng ON ng.id = ac.notification_group_id").
		Order("ac.created_at DESC")
	listBuilder = applyListPagination(listBuilder, d.Page, d.PageSize)
	list := make([]map[string]interface{}, 0)
	if err := listBuilder.Scan(&list).Error; err != nil {
		return 0, nil, err
	}
	return count, list, nil
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

// tenant-scope: caller-enforced?2026-08-26 ?????

// GetDeviceIdsByAlarmConfigId 返回触发过指定告警配置的设备 ID 去重列表。
// tenant-scope: parent-owned?2026-08-26 ?????
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

// newAlarmConfigListScopedDB 构造告警配置列表的 raw 语句根与过滤条件，
// 条件语义与收敛前的 applyAlarmConfigListFilters 逐条对齐。
// 空租户守卫（ROADMAP A1）：租户为空且未显式声明全租户视角时拒绝查询，
// 防止条件过滤被静默跳过后退化为跨租户全表扫描（与 board/users 收敛模式一致）。
func newAlarmConfigListScopedDB(req *model.GetAlarmConfigListByPageReq, allTenants bool, tenantScopes ...string) (*gorm.DB, error) {
	builder := global.DB.Table("alarm_config AS ac")
	if req == nil {
		return builder, nil
	}
	if len(tenantScopes) > 0 {
		switch len(tenantScopes) {
		case 1:
			builder = builder.Where("ac.tenant_id = ?", tenantScopes[0])
		default:
			builder = builder.Where("ac.tenant_id IN ?", tenantScopes)
		}
	} else if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		builder = builder.Where("ac.tenant_id = ?", tenantID)
	} else if !allTenants {
		logrus.Warn("dal: alarm config list query has empty TenantID without all-tenants scope; rejecting")
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.Name != nil && *req.Name != "" {
		builder = builder.Where("ac.name LIKE ?", ContainsLikePattern(*req.Name))
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
