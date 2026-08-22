// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

// device_config.go contains persistence helpers for device configuration.
//
// Purpose: create, update, delete, list, and convert device-config rows used
// by service-level configuration workflows. Core logic handles tenant-scoped
// filters, template relations, select-list queries, PO-to-VO conversion, and
// device/config binding updates. Important notes: query changes can affect
// broker connectivity and frontend config menus, so tenant, protocol, and
// template filters need focused DAL tests. Refactor suggestion: move large
// conversion logic out of the query helper once response-shape tests cover it.
package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gen"

	"github.com/sirupsen/logrus"
)

func CreateDeviceConfig(deviceconfig *model.DeviceConfig) error {
	return query.DeviceConfig.Create(deviceconfig)
}

// 修改配置物模型 id
func UpdateDeviceConfigTemplateID(id string, templateID *string) error {
	// nil值也要更新
	_, err := query.DeviceConfig.Where(query.DeviceConfig.ID.Eq(id)).Update(query.DeviceConfig.DeviceTemplateID, templateID)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func UpdateDeviceConfigPayloadSchemaID(id string, schemaID *string) error {
	return global.DB.Model(&model.DeviceConfig{}).
		Where("id = ?", id).
		Update("payload_schema_id", schemaID).Error
}

func UpdateDeviceConfig(id string, condsMap map[string]interface{}) error {
	p := query.DeviceConfig
	t := time.Now().UTC()
	condsMap["updated_at"] = &t
	// 主键不允许通过更新映射修改：剥离 id，避免 SET/WHERE 携带过期主键值
	// （历史事故：conds 中残留的旧 id 会叠加进 WHERE 导致 0 行受影响）。
	delete(condsMap, "id")
	info, err := p.WithContext(context.Background()).Where(p.ID.Eq(id)).Updates(condsMap)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("update deviceconfig failed, no rows affected")
	}
	return err
}

func DeleteDeviceConfig(id string) error {
	_, err := query.DeviceConfig.WithContext(context.Background()).Where(query.DeviceConfig.ID.Eq(id)).Delete()
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func GetDeviceConfigByID(id string) (*model.DeviceConfig, error) {
	// 1. 先从 Redis 缓存读取
	cacheKey := id + "_config"
	if global.REDIS != nil {
		result, err := global.REDIS.Get(context.Background(), cacheKey).Result()
		if err == nil {
			// 缓存命中
			var deviceconfig model.DeviceConfig
			if err := json.Unmarshal([]byte(result), &deviceconfig); err == nil {
				return &deviceconfig, nil
			}
			// JSON 反序列化失败，继续从数据库加载
		}
	}

	// 2. 缓存未命中，从数据库加载
	deviceconfig, err := query.DeviceConfig.Where(query.DeviceConfig.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	if deviceconfig == nil {
		return nil, fmt.Errorf("deviceconfig not found: %s", id)
	}

	// 3. 将结果写入缓存（永久有效）
	jsonData, err := json.Marshal(deviceconfig)
	if err == nil && global.REDIS != nil {
		err = global.REDIS.Set(context.Background(), cacheKey, jsonData, 0).Err()
		if err != nil {
			// 缓存写入失败不影响主流程，只记录日志
			logrus.Warn("failed to cache device config")
		}
	}

	return deviceconfig, nil
}

// GetDeviceConfigForTenant returns a device config only when it belongs to the
// requested tenant. Market publishing intentionally bypasses the legacy ID-only
// cache so a cached row can never widen the tenant boundary.
func GetDeviceConfigForTenant(id, tenantID string) (*model.DeviceConfig, error) {
	q := query.DeviceConfig
	deviceConfig, err := q.Where(q.ID.Eq(id), q.TenantID.Eq(tenantID)).First()
	if err != nil {
		return nil, err
	}
	if deviceConfig == nil {
		return nil, fmt.Errorf("device config not found: id=%s tenant_id=%s", id, tenantID)
	}
	return deviceConfig, nil
}

func GetDeviceConfigListByPage(deviceconfig *model.GetDeviceConfigListByPageReq, claims *utils.UserClaims) (int64, interface{}, error) {
	q := query.DeviceConfig
	var count int64
	var data []model.DeviceConfigRsp
	var deviceconfigList []*model.DeviceConfig
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(claims.TenantID))

	if deviceconfig.DeviceTemplateId != nil && *deviceconfig.DeviceTemplateId != "" {
		queryBuilder = queryBuilder.Where(q.DeviceTemplateID.Eq(*deviceconfig.DeviceTemplateId))
	}
	if deviceconfig.DeviceType != nil && *deviceconfig.DeviceType != "" {
		queryBuilder = queryBuilder.Where(q.DeviceType.Eq(*deviceconfig.DeviceType))
	}
	if deviceconfig.ProtocolType != nil && *deviceconfig.ProtocolType != "" {
		queryBuilder = queryBuilder.Where(q.ProtocolType.Eq(*deviceconfig.ProtocolType))
	}
	if deviceconfig.Name != nil && *deviceconfig.Name != "" {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *deviceconfig.Name)))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, deviceconfigList, err
	}

	if deviceconfig.Page != 0 && deviceconfig.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(deviceconfig.PageSize)
		queryBuilder = queryBuilder.Offset((deviceconfig.Page - 1) * deviceconfig.PageSize)
	}
	queryBuilder = queryBuilder.Order(q.CreatedAt.Desc())
	deviceconfigList, err = queryBuilder.Select().Find()
	if err != nil {
		logrus.Error(err)
		return count, deviceconfigList, err
	}
	deviceCounts, err := countActiveDevicesByConfigIDs(deviceConfigIDs(deviceconfigList))
	if err != nil {
		logrus.Error(err)
		return count, deviceconfigList, err
	}
	for i := range deviceconfigList {
		data = append(data, model.DeviceConfigRsp{
			DeviceConfig: deviceconfigList[i],
			DeviceCount:  deviceCounts[deviceconfigList[i].ID],
		})
	}

	return count, data, err
}

func deviceConfigIDs(deviceconfigList []*model.DeviceConfig) []string {
	ids := make([]string, 0, len(deviceconfigList))
	for _, deviceConfig := range deviceconfigList {
		if deviceConfig == nil || deviceConfig.ID == "" {
			continue
		}
		ids = append(ids, deviceConfig.ID)
	}
	return ids
}

func countActiveDevicesByConfigIDs(deviceConfigIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(deviceConfigIDs))
	if len(deviceConfigIDs) == 0 {
		return counts, nil
	}
	type row struct {
		DeviceConfigID string `gorm:"column:device_config_id"`
		Count          int64  `gorm:"column:count"`
	}
	rows := make([]row, 0, len(deviceConfigIDs))
	err := global.DB.Model(&model.Device{}).
		Select("device_config_id, count(*) as count").
		Where("activate_flag = ? AND device_config_id IN ?", "active", deviceConfigIDs).
		Group("device_config_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		counts[item.DeviceConfigID] = item.Count
	}
	return counts, nil
}

// 获取设备配置下拉菜单
func GetDeviceConfigSelectList(deviceConfigName *string, tenantID string, deviceType *string, protocolType *string) (any, error) {
	q := query.DeviceConfig
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))
	if deviceConfigName != nil {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *deviceConfigName)))
	}
	if deviceType != nil {
		queryBuilder = queryBuilder.Where(q.DeviceType.Eq(*deviceType))
	}
	if protocolType != nil {
		queryBuilder = queryBuilder.Where(q.ProtocolType.Eq(*protocolType))
	}
	var data []map[string]interface{}
	err := queryBuilder.Select(q.ID, q.Name).Order(q.CreatedAt.Desc()).Scan(&data)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return data, err
}

type DeviceConfigQuery struct{}

func (DeviceConfigQuery) First(ctx context.Context, option ...gen.Condition) (info *model.DeviceConfig, err error) {
	info, err = query.DeviceConfig.WithContext(ctx).Where(option...).First()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (DeviceConfigQuery) Find(ctx context.Context, option ...gen.Condition) (list []*model.DeviceConfig, err error) {
	list, err = query.DeviceConfig.WithContext(ctx).Where(option...).Find()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

type DeviceConfigVo struct{}

func (DeviceConfigVo) PoToVo(deviceConfigInfo *model.DeviceConfig) (info *model.DeviceConfigsRes) {
	info = &model.DeviceConfigsRes{
		ID:         deviceConfigInfo.ID,
		Name:       deviceConfigInfo.Name,
		DeviceType: deviceConfigInfo.DeviceType,
		CreatedAt:  deviceConfigInfo.CreatedAt,
		UpdatedAt:  deviceConfigInfo.UpdatedAt,
	}
	if deviceConfigInfo.DeviceTemplateID != nil {
		info.DeviceTemplateID = *deviceConfigInfo.DeviceTemplateID
	}
	if deviceConfigInfo.ProtocolType != nil {
		info.ProtocolType = *deviceConfigInfo.ProtocolType
	}
	if deviceConfigInfo.VoucherType != nil {
		info.VoucherType = *deviceConfigInfo.VoucherType
	}
	if deviceConfigInfo.ProtocolConfig != nil {
		info.ProtocolConfig = *deviceConfigInfo.ProtocolConfig
	}
	if deviceConfigInfo.DeviceConnType != nil {
		info.DeviceConnType = *deviceConfigInfo.DeviceConnType
	}
	if deviceConfigInfo.AdditionalInfo != nil {
		info.VoucherType = *deviceConfigInfo.AdditionalInfo
	}
	if deviceConfigInfo.Description != nil {
		info.Description = *deviceConfigInfo.Description
	}
	if deviceConfigInfo.Remark != nil {
		info.Remark = *deviceConfigInfo.Remark
	}
	return
}

// func GetDeviceOnline(ctx context.Context, deviceOnlines []model.DeviceOnline) (map[string]int, error) {
// 	var (
// 		result               = make(map[string]int, 0)
// 		deviceConfigIds      []string
// 		deviveConfigOtherMap = make(map[string]model.DeviceConfigOtherConfig, 0)
// 		deviceIds            []string
// 		deviceMap            = make(map[string]string, 0)
// 	)

// 	if len(deviceOnlines) == 0 {
// 		return result, nil
// 	}
// 	for _, v := range deviceOnlines {
// 		if v.DeviceConfigId == nil || *v.DeviceConfigId == "" {
// 			continue
// 		}
// 		deviceIds = append(deviceIds, v.DeviceId)
// 		deviceConfigIds = append(deviceConfigIds, *v.DeviceConfigId)
// 		deviceMap[v.DeviceId] = *v.DeviceConfigId
// 	}
// 	list, err := query.DeviceConfig.WithContext(ctx).Where(query.DeviceConfig.ID.In(deviceConfigIds...)).Find()
// 	if err != nil {
// 		return result, nil
// 	}
// 	for _, v := range list {
// 		if v.OtherConfig == nil || *v.OtherConfig == "" {
// 			continue
// 		}
// 		var config model.DeviceConfigOtherConfig
// 		err = json.Unmarshal([]byte(*v.OtherConfig), &config)
// 		if err != nil {
// 			continue
// 		}
// 		deviveConfigOtherMap[v.ID] = config
// 	}
// 	t := query.TelemetryCurrentData
// 	rows, err := t.WithContext(ctx).Where(t.DeviceID.In(deviceIds...)).Group(t.DeviceID).Select(t.DeviceID, t.T.Max().As("ts")).Find()
// 	if err != nil {
// 		return result, nil
// 	}
// 	now := time.Now().UTC()
// 	for _, v := range rows {
// 		logrus.Warning(v.DeviceID)
// 		var (
// 			deviceConfigId string
// 			ok             bool
// 		)
// 		if deviceConfigId, ok = deviceMap[v.DeviceID]; !ok {
// 			continue
// 		}
// 		if config, ok := deviveConfigOtherMap[deviceConfigId]; ok {

// 			if config.Heartbeat > 0 {
// 				//当前时间-最近一次遥测时间 大于心跳秒数  表示离线
// 				if now.Sub(v.T).Seconds() > float64(config.Heartbeat) {
// 					result[v.DeviceID] = 0
// 				} else {
// 					result[v.DeviceID] = 1
// 				}
// 				continue
// 			}
// 			//设置了超时时间 当前时间-最近一次遥测时间 大于超时时间（分）  表示离线
// 			if config.OnlineTimeout > 0 {
// 				if now.Sub(v.T).Minutes() > float64(config.OnlineTimeout) {
// 					result[v.DeviceID] = 0
// 				} else {
// 					result[v.DeviceID] = 1
// 				}

// 			}
// 		}
// 	}
// 	return result, nil
// }

// 修改凭证类型
func UpdateDeviceConfigVoucherType(id string, voucherType *string) error {
	// nil值也要更新
	_, err := query.DeviceConfig.Where(query.DeviceConfig.ID.Eq(id)).Update(query.DeviceConfig.VoucherType, voucherType)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func GetDeviceConfigIdByName(name string) *string {
	var configId string
	err := query.DeviceConfig.Where(query.DeviceConfig.Name.Eq(name)).Select(query.DeviceConfig.ID).Limit(1).Scan(&configId)
	if err != nil {
		return nil
	}
	return &configId
}

// 根据功能物模型 ID 查询关联的配置数量
func GetDeviceConfigCountByFuncTemplateId(id string) (int64, error) {
	count, err := query.DeviceConfig.Where(query.DeviceConfig.DeviceTemplateID.Eq(id)).Count()
	if err != nil {
		logrus.Error(err)
	}
	return count, err
}

// 给设备增加物模型
func UpdateDeviceDeviceConfigID(deviceID string, deviceConfigID *string) error {
	_, err := query.Device.Where(query.Device.ID.Eq(deviceID)).Update(query.Device.DeviceConfigID, deviceConfigID)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

const updateDeviceConfigBatchSize = 500

func UpdateDeviceDeviceConfigIDs(deviceIDs []string, deviceConfigID *string) error {
	normalizedIDs := normalizeDeviceIDs(deviceIDs)
	for start := 0; start < len(normalizedIDs); start += updateDeviceConfigBatchSize {
		end := start + updateDeviceConfigBatchSize
		if end > len(normalizedIDs) {
			end = len(normalizedIDs)
		}
		_, err := query.Device.
			Where(query.Device.ID.In(normalizedIDs[start:end]...)).
			Update(query.Device.DeviceConfigID, deviceConfigID)
		if err != nil {
			logrus.Error(err)
			return err
		}
	}
	return nil
}
