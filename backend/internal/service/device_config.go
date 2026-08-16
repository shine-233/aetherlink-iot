// device_config.go 负责设备配置领域的核心服务编排。
// 这里承接的主链路包括：设备配置的创建、修改、删除、批量绑定、凭证表单生成，以及自动化动作/条件元数据装配。
// 核心约束是：先完成租户与权限校验，再执行配置 CRUD、物模型解绑、缓存失效和协议插件侧联动。
// 设备配置会同时影响设备接入、物模型映射、凭证类型和自动化规则，因此修改时必须特别关注缓存一致性与物模型关系同步。
// 静态审查建议：
// 1. 当前问题：注释中夹杂英文、重复说明和过长解释，维护成本高，容易与实现漂移。
// 2. 改进方案：统一为简洁中文注释，抽取可复用的小 helper，保留权限与缓存流程不变。
// 3. 实施步骤：先清理死注释和重复文案，再收敛重复标签生成逻辑，最后继续按职责拆分协议边界。
// 4. 预期效果：降低阅读成本，减少协议/物模型改动时的注释失真风险。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"aetherlink-iot/backend/initialize"
	protocolplugin "aetherlink-iot/backend/internal/service/protocol_plugin"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// DeviceConfig 是设备配置服务的门面对象。
// 当前文件中的公开方法大多挂在该类型上，供 API 层统一调用。
type DeviceConfig struct{}

func validatePayloadSchemaBinding(schemaID *string, tenantID string) error {
	if schemaID == nil || *schemaID == "" {
		return nil
	}
	if _, err := dal.GetPayloadSchemaByID(*schemaID, tenantID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.NewWithMessage(errcode.CodeParamError, "payload_schema_id is not available for this tenant")
		}
		return wrapDeviceConfigDBError(err)
	}
	return nil
}

// ensureDeviceConfigReadAccess 校验设备配置读取权限并返回配置实体。
// configID 是待访问的设备配置 ID；claims 用于校验系统管理员或同租户访问资格。
// 返回值中的 DeviceConfig 会作为后续物模型、协议和自动化数据装配的基础输入。
func ensureDeviceConfigReadAccess(configID string, claims *utils.UserClaims) (*model.DeviceConfig, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device config")
	}
	deviceConfig, err := dal.GetDeviceConfigByID(configID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if claims.Authority != constant.SYS_ADMIN && deviceConfig.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device config")
	}
	return deviceConfig, nil
}

// ensureDeviceConfigWriteAccess 在读取权限基础上再次确认写权限。
// 这里与读取校验分开保留，便于调用方显式表达“只读”和“可修改”两种业务意图。
func ensureDeviceConfigWriteAccess(configID string, claims *utils.UserClaims) (*model.DeviceConfig, error) {
	deviceConfig, err := ensureDeviceConfigReadAccess(configID, claims)
	if err != nil {
		return nil, err
	}
	if claims.Authority != constant.SYS_ADMIN && deviceConfig.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify device config")
	}
	return deviceConfig, nil
}

// CreateDeviceConfig 创建新的设备配置。
// req 提供名称、接入方式、物模型绑定、协议参数和凭证类型等输入；claims 决定配置所属租户。
// 返回新建后的配置实体。注意：空物模型 ID 会被视为未绑定，协议类型和凭证类型会在缺省时回落到 MQTT/ACCESSTOKEN。
func (*DeviceConfig) CreateDeviceConfig(req *model.CreateDeviceConfigReq, claims *utils.UserClaims) (deviceconfig model.DeviceConfig, err error) {
	if err := ensureTenantScopedWriteClaims(claims, "create device config"); err != nil {
		return deviceconfig, err
	}

	deviceconfig.ID = uuid.New()
	deviceconfig.Name = req.Name
	deviceconfig.Description = req.Description
	deviceconfig.DeviceConnType = req.DeviceConnType
	// 将空字符串视为未指定物模型，避免写入无效外键。
	if req.DeviceTemplateId != nil && *req.DeviceTemplateId == "" {
		req.DeviceTemplateId = nil
	}
	deviceconfig.DeviceTemplateID = req.DeviceTemplateId
	if req.PayloadSchemaId != nil && *req.PayloadSchemaId == "" {
		req.PayloadSchemaId = nil
	}
	if err := validatePayloadSchemaBinding(req.PayloadSchemaId, claims.TenantID); err != nil {
		return deviceconfig, err
	}
	deviceconfig.PayloadSchemaID = req.PayloadSchemaId
	deviceconfig.DeviceType = req.DeviceType
	if req.AdditionalInfo != nil && !IsJSON(*req.AdditionalInfo) {
		return deviceconfig, errcode.NewWithMessage(errcode.CodeParamError, "additional_info is not a valid JSON")
	}
	deviceconfig.AdditionalInfo = req.AdditionalInfo
	if req.ProtocolConfig != nil && !IsJSON(*req.ProtocolConfig) {
		return deviceconfig, errcode.NewWithMessage(errcode.CodeParamError, "protocol_config is not a valid JSON")
	}
	deviceconfig.ProtocolConfig = req.ProtocolConfig
	// 未显式指定协议类型时，沿用 MQTT 作为默认值。
	if req.ProtocolType == nil {
		deviceconfig.ProtocolType = StringPtr("MQTT")
	} else {
		deviceconfig.ProtocolType = req.ProtocolType
	}
	if req.VoucherType == nil {
		deviceconfig.VoucherType = StringPtr("ACCESSTOKEN")
	} else {
		deviceconfig.VoucherType = req.VoucherType
	}
	deviceconfig.Remark = req.Remark
	t := time.Now().UTC()
	deviceconfig.CreatedAt = t
	deviceconfig.UpdatedAt = t
	deviceconfig.TenantID = claims.TenantID
	deviceconfig.TemplateSecret = StringPtr(uuid.New())

	err = dal.CreateDeviceConfig(&deviceconfig)
	if err != nil {
		logrus.Error(err)
		return deviceconfig, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return deviceconfig, err
}

// UpdateDeviceConfig 更新设备配置并处理配置变更后的副作用。
// 核心流程依次为：读取旧配置、处理物模型解绑、校验 JSON 字段、检查 other_config 约束、落库并清缓存、最后执行协议联动。
// 返回值为更新后的最新配置实体，供上层直接回显。
func (*DeviceConfig) UpdateDeviceConfig(req model.UpdateDeviceConfigReq, claims *utils.UserClaims) (any, error) {
	oldConfig, err := loadDeviceConfigForUpdate(req.Id, claims)
	if err != nil {
		return nil, err
	}
	if err := validatePayloadSchemaBinding(req.PayloadSchemaId, oldConfig.TenantID); err != nil {
		return nil, err
	}
	if err := clearBlankDeviceTemplateID(&req); err != nil {
		return nil, err
	}
	if err := clearBlankPayloadSchemaID(&req); err != nil {
		return nil, err
	}
	condsMap, err := prepareDeviceConfigUpdate(req, oldConfig.OtherConfig)
	if err != nil {
		return nil, err
	}

	data, err := updateDeviceConfigAndRefreshCache(req.Id, condsMap)
	if err != nil {
		return data, err
	}
	if err := applyDeviceConfigProtocolSideEffects(req.Id, oldConfig, data); err != nil {
		return nil, err
	}

	return data, nil
}

// loadDeviceConfigForUpdate 抽出更新前的配置加载逻辑，统一复用写权限校验。
func loadDeviceConfigForUpdate(configID string, claims *utils.UserClaims) (*model.DeviceConfig, error) {
	return ensureDeviceConfigWriteAccess(configID, claims)
}

// clearBlankDeviceTemplateID 处理“物模型解绑”的前端传参约定。
// 当前端将 DeviceTemplateId 传为空字符串时，表示用户希望解除物模型关联；这里会先落库清空，再把请求体同步改为 nil。
// 这样后续统一的 StructToMap 更新流程就不会把空字符串重新写回数据库。
func clearBlankDeviceTemplateID(req *model.UpdateDeviceConfigReq) error {
	if req.DeviceTemplateId == nil || *req.DeviceTemplateId != "" {
		return nil
	}

	// 前端会用空字符串表示“解绑物模型”，这里直接清掉关联并同步请求体。
	if err := dal.UpdateDeviceConfigTemplateID(req.Id, nil); err != nil {
		return wrapDeviceConfigDBError(err)
	}
	req.DeviceTemplateId = nil
	return nil
}

func clearBlankPayloadSchemaID(req *model.UpdateDeviceConfigReq) error {
	if req.PayloadSchemaId == nil || *req.PayloadSchemaId != "" {
		return nil
	}
	if err := dal.UpdateDeviceConfigPayloadSchemaID(req.Id, nil); err != nil {
		return wrapDeviceConfigDBError(err)
	}
	initialize.DelDeviceConfigCache(req.Id)
	req.PayloadSchemaId = nil
	return nil
}

// prepareDeviceConfigUpdate builds the validated update map after any template
// unlink has already been persisted. It intentionally does not perform writes.
func prepareDeviceConfigUpdate(req model.UpdateDeviceConfigReq, oldOtherConfig *string) (map[string]interface{}, error) {
	condsMap, err := buildDeviceConfigUpdateMap(req)
	if err != nil {
		return nil, err
	}
	if err := validateDeviceConfigOtherConfigChange(req.OtherConfig, oldOtherConfig); err != nil {
		return nil, err
	}
	return condsMap, nil
}

// buildDeviceConfigUpdateMap 将更新请求转换为 DAL 可消费的字段映射。
// 同时校验 additional_info、protocol_config、other_config 是否为合法 JSON，避免半结构化字段写入脏数据。
func buildDeviceConfigUpdateMap(req model.UpdateDeviceConfigReq) (map[string]interface{}, error) {
	condsMap, err := StructToMapAndVerifyJson(req, "additional_info", "protocol_config", "other_config")
	if err != nil {
		return nil, wrapDeviceConfigParamError(err)
	}
	return condsMap, nil
}

// validateDeviceConfigOtherConfigChange 校验 other_config 中互斥的在线判定参数。
// 当前约束要求 heartbeat 与 online_timeout 不能同时为非零，否则会造成设备在线状态判定语义冲突。
func validateDeviceConfigOtherConfigChange(reqOtherConfig, oldOtherConfig *string) error {
	if !deviceConfigOtherConfigChanged(reqOtherConfig, oldOtherConfig) {
		return nil
	}

	var otherConfig model.DeviceConfigOtherConfig
	if err := json.Unmarshal([]byte(*reqOtherConfig), &otherConfig); err != nil {
		return wrapDeviceConfigParamError(err)
	}
	if otherConfig.OnlineTimeout != 0 && otherConfig.Heartbeat != 0 {
		return errcode.New(210001)
	}
	return nil
}

// deviceConfigOtherConfigChanged 判断 other_config 是否真的发生了变更。
// 这里只在请求值非空且与旧指针不同的场景触发深度校验，避免无效反序列化。
func deviceConfigOtherConfigChanged(reqOtherConfig, oldOtherConfig *string) bool {
	return reqOtherConfig != nil && reqOtherConfig != oldOtherConfig
}

// updateDeviceConfigAndRefreshCache 执行设备配置更新并刷新配置缓存。
// condsMap 是经过 JSON 校验后的更新字段集合。更新成功后会先删除设备配置缓存，再重新查询最新实体返回给调用方。
// 这里故意采用“删缓存后回读”的顺序，避免上层继续拿到旧的物模型、协议或凭证信息。
func updateDeviceConfigAndRefreshCache(configID string, condsMap map[string]interface{}) (*model.DeviceConfig, error) {
	logrus.Debug("device config update requested")
	if err := dal.UpdateDeviceConfig(configID, condsMap); err != nil {
		logrus.Error(err)
		return nil, wrapDeviceConfigDBError(err)
	}

	initialize.DelDeviceConfigCache(configID)

	data, err := dal.GetDeviceConfigByID(configID)
	if err != nil {
		return data, wrapDeviceConfigDBError(err)
	}
	return data, nil
}

// applyDeviceConfigProtocolSideEffects 处理配置更新后与协议相关的附带动作。
// 当前包括两类副作用：协议插件配置变化后的断链处理，以及协议类型变化后的凭证类型重置。
func applyDeviceConfigProtocolSideEffects(configID string, oldConfig, data *model.DeviceConfig) error {
	if err := disconnectChangedPluginProtocolConfig(configID, oldConfig, data); err != nil {
		return err
	}
	return resetVoucherTypeForChangedProtocol(configID, oldConfig, data)
}

// disconnectChangedPluginProtocolConfig 在非 MQTT 协议的插件配置发生变化时主动断开旧连接。
// 这样可以强制设备按新协议参数重新建立会话，避免缓存的旧连接继续使用过期配置。
func disconnectChangedPluginProtocolConfig(configID string, oldConfig, data *model.DeviceConfig) error {
	if data.ProtocolType == nil || *data.ProtocolType == "MQTT" {
		return nil
	}
	if !deviceConfigProtocolConfigChanged(oldConfig.ProtocolConfig, data.ProtocolConfig) {
		return nil
	}

	err := protocolplugin.DeviceConfigUpdateAndDisconnect(configID, *data.ProtocolType, data.DeviceType)
	if err != nil {
		return wrapDeviceConfigDBError(err)
	}
	return nil
}

// deviceConfigProtocolConfigChanged 判断协议配置 JSON 文本是否发生变更。
// 这里只比较非空场景下的原始字符串内容，适合作为是否触发协议插件断链的快速开关。
func deviceConfigProtocolConfigChanged(oldProtocolConfig, newProtocolConfig *string) bool {
	return oldProtocolConfig != nil && newProtocolConfig != nil && *oldProtocolConfig != *newProtocolConfig
}

// resetVoucherTypeForChangedProtocol 在协议类型切换后同步调整凭证类型。
// MQTT 会默认回落到 ACCESSTOKEN，其他协议暂不强制赋默认值，由插件侧表单或上层业务继续决定。
func resetVoucherTypeForChangedProtocol(configID string, oldConfig, data *model.DeviceConfig) error {
	if oldConfig.ProtocolType == nil || data.ProtocolType == nil || *oldConfig.ProtocolType == *data.ProtocolType {
		return nil
	}

	voucherType := voucherTypeForProtocol(*data.ProtocolType)
	if err := dal.UpdateDeviceConfigVoucherType(configID, voucherType); err != nil {
		return wrapDeviceConfigDBError(err)
	}
	return nil
}

// voucherTypeForProtocol 根据协议类型推导默认凭证类型。
// 这是协议切换联动中的兜底逻辑，不负责完整的插件表单推导。
func voucherTypeForProtocol(protocolType string) *string {
	if protocolType == "MQTT" {
		return StringPtr("ACCESSTOKEN")
	}
	return nil
}

// wrapDeviceConfigDBError 统一包装设备配置领域的数据库异常，保留底层 SQL 错误文本供审计排查。
func wrapDeviceConfigDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

// wrapDeviceConfigParamError 统一包装设备配置领域的参数异常。
// 主要用于 JSON 反序列化和结构化字段校验失败的场景。
func wrapDeviceConfigParamError(err error) error {
	return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
		"err": err.Error(),
	})
}

// DeleteDeviceConfig 删除设备配置。
// 删除前会先校验写权限，并确认没有任何设备仍引用该配置；删除后还会清理配置缓存与设备数据脚本缓存。
// 这样可以避免物模型或接入脚本仍从旧缓存中读取已删除配置。
func (*DeviceConfig) DeleteDeviceConfig(id string, claims *utils.UserClaims) error {
	if _, err := ensureDeviceConfigWriteAccess(id, claims); err != nil {
		return err
	}
	// 删除前先确认是否仍有设备引用该配置。
	devices, err := dal.GetDevicesByDeviceConfigID(id)
	if err != nil {
		return err
	}
	if len(devices) > 0 {
		return errcode.WithVars(200051, map[string]interface{}{
			"count": len(devices),
		})
	}

	err = dal.DeleteDeviceConfig(id)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	// 删除后清理配置与设备侧缓存，避免旧数据继续被读取。
	initialize.DelDeviceConfigCache(id)
	initialize.DelDeviceDataScriptCache(id)

	return nil
}

// GetDeviceConfigByID 按 ID 查询单个设备配置，主要供详情页或编辑页回显使用。
func (*DeviceConfig) GetDeviceConfigByID(ctx context.Context, id string, claims *utils.UserClaims) (any, error) {
	info, err := ensureDeviceConfigReadAccess(id, claims)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// GetDeviceConfigListByPage 返回分页设备配置列表。
// 返回结构固定包含 total 和 list，便于前端表格组件直接消费；当总数为 0 时，会显式返回空数组而不是 nil。
func (*DeviceConfig) GetDeviceConfigListByPage(req *model.GetDeviceConfigListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetDeviceConfigListByPage(req, claims)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	deviceconfigListRsp := make(map[string]interface{})
	deviceconfigListRsp["total"] = total
	if total == int64(0) {
		list = make([]*map[string]interface{}, 0)
	}
	deviceconfigListRsp["list"] = list

	return deviceconfigListRsp, err
}

// GetDeviceConfigListMenu 返回设备配置下拉菜单数据。
// 典型使用场景是设备创建、批量绑定或筛选视图按设备类型和协议快速选择配置。
func (*DeviceConfig) GetDeviceConfigListMenu(req *model.GetDeviceConfigListMenuReq, claims *utils.UserClaims) (any, error) {
	data, err := dal.GetDeviceConfigSelectList(req.DeviceConfigName, claims.TenantID, req.DeviceType, req.ProtocolType)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}
