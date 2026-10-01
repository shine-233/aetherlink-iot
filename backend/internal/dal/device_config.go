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
// template filters need focused DAL tests.
//
// 9-28 DAL 拆分：路由缓存子聚合迁至 device_config_routing.go；devices 表绑定
// 写入与活跃设备计数迁至 device_config_device_binding.go；顺带清理了本文件
// 早已注释停用的 GetDeviceOnline 死代码块。导出符号与签名不变。
package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	constant "aetherlink-iot/backend/pkg/constant"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gen"
	"gorm.io/gorm"

	"github.com/sirupsen/logrus"
)

// isolatedDeviceConfig 返回从全新 gorm Statement 出发的 device_config 链起点。
// P1 修复（2026-08-23，见 VALIDATION.md）：高负载下 gorm 会依据执行语句上残留的
// Statement.Model 注入陈旧主键 WHERE（间歇 record-not-found / 假成功删除）。
// Session{NewDB:true} 强制每次操作都使用零起点的全新语句，切断跨请求状态继承。
func isolatedDeviceConfig() query.IDeviceConfigDo {
	return query.DeviceConfig.Session(&gorm.Session{NewDB: true})
}

// 写路径统一走 raw global.DB 链（gorm.Open 根 clone==1，每次链式起点都是全新
// Statement，无跨请求 Model/Dest 继承通道），从根上消除高负载下被注入陈旧主键
// WHERE 的间歇性读写不一致（P1 修复，见 VALIDATION.md）。
func CreateDeviceConfig(deviceconfig *model.DeviceConfig) error {
	err := global.DB.Create(deviceconfig).Error
	if err == nil {
		InvalidateDeviceConfigRouting(deviceconfig.ID)
	}
	return err
}

// 修改配置物模型 id
func UpdateDeviceConfigTemplateID(id string, templateID *string) error {
	// nil值也要更新
	err := global.DB.Model(&model.DeviceConfig{}).
		Where("id = ?", id).
		Update("device_template_id", templateID).Error
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

// UpdateDeviceConfigDefaultRuleChainID 按主键写档案级默认规则链（TB-18）。
// nil 也写入（解绑语义）。与 UpdateDeviceConfigPayloadSchemaID 同口径：主键单列更新，
// 租户校验由 service 层 ensureDeviceConfigWriteAccess 前置完成。
// tenant-scope: caller-enforced（主键更新路径，写入值已经过 validateDefaultRuleChainBinding 租户归属校验）
func UpdateDeviceConfigDefaultRuleChainID(id string, chainID *string) error {
	defer InvalidateDeviceConfigRouting(id)
	return global.DB.Model(&model.DeviceConfig{}).
		Where("id = ?", id).
		Update("default_rule_chain_id", chainID).Error
}

// CountDeviceConfigsByDefaultRuleChainID 统计仍把 default_rule_chain_id 指向指定链的档案数
// （TB-18 删除守卫）。按 id+tenant 双条件过滤，防止跨租户计数。
// tenant-scope: caller-enforced（chainID 来自已通过租户归属校验的 rule chain，tenantID 为调用方 claims 租户）
func CountDeviceConfigsByDefaultRuleChainID(chainID, tenantID string) (int64, error) {
	var count int64
	err := global.DB.Model(&model.DeviceConfig{}).
		Where("default_rule_chain_id = ? AND tenant_id = ?", chainID, tenantID).
		Count(&count).Error
	return count, err
}

func UpdateDeviceConfig(id string, condsMap map[string]interface{}) error {
	defer InvalidateDeviceConfigRouting(id)
	t := time.Now().UTC()
	condsMap["updated_at"] = &t
	delete(condsMap, "id")
	info := global.DB.Model(&model.DeviceConfig{}).
		Where("id = ?", id).
		Updates(condsMap)
	err := info.Error
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("update deviceconfig failed, no rows affected")
	}
	return err
}

// DeleteDeviceConfigForTenant 按 id+tenant 双条件删除设备配置，DAL 层强制租户隔离（安全审计 F4：
// 消除对 service 层 check-then-act 的单一依赖）。P1 修复（2026-08-23，见 VALIDATION.md）：
// RowsAffected 不得忽略——删除未命中行时显式报错，杜绝"API 返回成功但行仍在"的假成功删除。
func DeleteDeviceConfigForTenant(id, tenantID string) error {
	defer InvalidateDeviceConfigRouting(id)
	info := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.DeviceConfig{})
	err := info.Error
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("delete deviceconfig failed, no rows affected: id=%s", id)
	}
	return nil
}

// tenant-scope: no-tenant-column?2026-08-26 ?????
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
	deviceconfig, err := isolatedDeviceConfig().Where(query.DeviceConfig.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	if deviceconfig == nil {
		return nil, fmt.Errorf("deviceconfig not found: %s", id)
	}

	// 3. 将结果写入缓存（带兜底 TTL）
	//
	// 口径修正（对齐 GetDeviceCacheById 的 P2 修复）：此前这里写的是永久键（TTL=0），
	// 是当时全后端唯一还在用永久缓存的地方。写路径主动失效（service/device_config.go
	// 的四处 initialize.DelDeviceConfigCache）仍是主机制，但只要有**任何一条**写路径
	// 漏了失效，脏数据就会**永久**留在 Redis 里——而设备/脚本/processor 三个缓存
	// 早已改用 constant.CacheFallbackTTL 兜底自愈，本处漏掉了。
	// 兜底过期不改变正常路径行为（写后即删，键本来就不该长期存在），
	// 只保证遗漏失效时最多脏 30 分钟。
	jsonData, err := json.Marshal(deviceconfig)
	if err == nil && global.REDIS != nil {
		err = global.REDIS.Set(context.Background(), cacheKey, jsonData, constant.CacheFallbackTTL).Err()
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
	deviceConfig, err := isolatedDeviceConfig().Where(q.ID.Eq(id), q.TenantID.Eq(tenantID)).First()
	if err != nil {
		return nil, err
	}
	if deviceConfig == nil {
		return nil, fmt.Errorf("device config not found: id=%s tenant_id=%s", id, tenantID)
	}
	return deviceConfig, nil
}

func GetDeviceConfigListByPage(deviceconfig *model.GetDeviceConfigListByPageReq, claims *utils.UserClaims) (int64, interface{}, error) {
	// 空租户守卫（ROADMAP A1）：claims.TenantID 运行期可能因 token 边界条件变为空串，
	// WHERE tenant_id='' 会静默匹配 0 行——显式拒绝而非返回"偶发空列表"（users 收敛模式，
	// 守卫收敛到 requireClaimsTenantID，2026-09-28）。
	tenantID, err := requireClaimsTenantID(claims)
	if err != nil {
		return 0, nil, err
	}
	q := query.DeviceConfig
	var count int64
	var data []model.DeviceConfigRsp
	var deviceconfigList []*model.DeviceConfig
	queryBuilder := isolatedDeviceConfig().WithContext(context.Background())
	if tenantID != "" {
		queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))
	}

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
		queryBuilder = queryBuilder.Where(q.Name.Like(ContainsLikePattern(*deviceconfig.Name)))
	}

	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, deviceconfigList, err
	}

	queryBuilder = applyListPagination(queryBuilder, deviceconfig.Page, deviceconfig.PageSize)
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

// 获取设备配置下拉菜单
func GetDeviceConfigSelectList(deviceConfigName *string, tenantID string, deviceType *string, protocolType *string) (any, error) {
	q := query.DeviceConfig
	queryBuilder := isolatedDeviceConfig().WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))
	if deviceConfigName != nil {
		queryBuilder = queryBuilder.Where(q.Name.Like(ContainsLikePattern(*deviceConfigName)))
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
	info, err = isolatedDeviceConfig().WithContext(ctx).Where(option...).First()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

type DeviceConfigVo struct{}

// 修改凭证类型
func UpdateDeviceConfigVoucherType(id string, voucherType *string) error {
	// nil值也要更新
	err := global.DB.Model(&model.DeviceConfig{}).
		Where("id = ?", id).
		Update("voucher_type", voucherType).Error
	if err != nil {
		logrus.Error(err)
	}
	return err
}

// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetDeviceConfigIdByName(name string) *string {
	var configId string
	err := global.DB.Model(&model.DeviceConfig{}).
		Where("name = ?", name).
		Select("id").Limit(1).Scan(&configId).Error
	if err != nil {
		return nil
	}
	return &configId
}

// 根据功能物模型 ID 查询关联的配置数量
func countBy(q *gorm.DB) (int64, error) {
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetDeviceConfigCountByFuncTemplateId(id string) (int64, error) {
	count, err := countBy(global.DB.Model(&model.DeviceConfig{}).Where("device_template_id = ?", id))
	if err != nil {
		logrus.Error(err)
	}
	return count, err
}

// GetDeviceConfigByNameAndTenant 查询指定租户下指定名称的设备配置（TB-15）。
func GetDeviceConfigByNameAndTenant(tenantID, name string) (*model.DeviceConfig, error) {
	var dc model.DeviceConfig
	err := global.DB.Where("tenant_id = ? AND name = ?", tenantID, name).First(&dc).Error
	if err != nil {
		return nil, err
	}
	return &dc, nil
}

// GetDeviceConfigNamesMatchingBase 查询指定租户下 baseName 或 baseName (N) 形式的配置名称列表（TB-15）。
func GetDeviceConfigNamesMatchingBase(tenantID, baseName string) ([]string, error) {
	var names []string
	err := global.DB.Model(&model.DeviceConfig{}).
		Where("tenant_id = ? AND (name = ? OR name LIKE ?)", tenantID, baseName, baseName+" (%)").
		Pluck("name", &names).Error
	return names, err
}
