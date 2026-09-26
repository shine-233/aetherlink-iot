// 文件用途：实现统一集成实体（Integration，TB-45）服务层与管线绑定校验。
// 核心逻辑：Integration CRUD（创建/更新/详情/删除/分页），转换器绑定 ID 做租户内存在性校验
//
//	（fail-closed：绑一个不存在或跨租户的转换器直接拒绝），config 仅要求合法 JSON。
//
// 关键注意事项：租户边界完全取自 claims.TenantID，绝不信任请求体；转换器引用校验
// 复用 dal.GetDataConverterByID 的租户过滤，跨租户绑定即 NotFound。
// 重构建议：SNMP/插件连接器同构接入后，把 connector_type 专属配置校验下沉到独立策略函数。
package service

import (
	"context"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// IntegrationService 统一集成实体服务（TB-45）。
type IntegrationService struct{}

// validateConverterBinding 校验转换器绑定 ID 属于本租户（fail-closed）。
// 空串/nil 视为未绑定；查不到即返回参数错误，避免绑定悬空外键。
func validateConverterBinding(tenantID string, converterID *string, label string) error {
	if converterID == nil || strings.TrimSpace(*converterID) == "" {
		return nil
	}
	if _, err := dal.GetDataConverterByID(strings.TrimSpace(*converterID), tenantID); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, label+" not found in tenant")
	}
	return nil
}

// normalizeBindingID 去除绑定 ID 首尾空白，空串归一为 nil（未绑定）。
func normalizeBindingID(id *string) *string {
	if id == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*id)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// CreateIntegration 创建集成实例
func (*IntegrationService) CreateIntegration(ctx context.Context, req *model.CreateIntegrationReq, claims *utils.UserClaims) (*model.Integration, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	tenantID := claims.TenantID

	uplinkID := normalizeBindingID(req.ConverterUplinkID)
	if err := validateConverterBinding(tenantID, uplinkID, "converter_uplink_id"); err != nil {
		return nil, err
	}
	downlinkID := normalizeBindingID(req.ConverterDownlinkID)
	if err := validateConverterBinding(tenantID, downlinkID, "converter_downlink_id"); err != nil {
		return nil, err
	}

	configStr := "{}"
	if req.Config != nil && strings.TrimSpace(*req.Config) != "" {
		if !IsJSON(*req.Config) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "config must be valid JSON")
		}
		configStr = *req.Config
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now().UTC()
	integration := &model.Integration{
		ID:                  uuid.New(),
		Name:                req.Name,
		TenantID:            tenantID,
		ConnectorType:       req.ConnectorType,
		ConverterUplinkID:   uplinkID,
		ConverterDownlinkID: downlinkID,
		Config:              configStr,
		Enabled:             enabled,
		CreatedAt:           &now,
		UpdatedAt:           &now,
	}

	if err := dal.CreateIntegration(integration); err != nil {
		logrus.Errorf("failed to create integration: %v", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return integration, nil
}

// UpdateIntegration 更新集成实例（部分字段语义与 DataConverterService.UpdateDataConverter 一致）
func (*IntegrationService) UpdateIntegration(ctx context.Context, req *model.UpdateIntegrationReq, claims *utils.UserClaims) (*model.Integration, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}

	record, err := dal.GetIntegrationByID(req.ID, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "integration not found")
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		record.Name = *req.Name
	}
	if req.ConnectorType != nil && *req.ConnectorType != "" {
		record.ConnectorType = *req.ConnectorType
	}
	if req.ConverterUplinkID != nil {
		binding := normalizeBindingID(req.ConverterUplinkID)
		if err := validateConverterBinding(claims.TenantID, binding, "converter_uplink_id"); err != nil {
			return nil, err
		}
		record.ConverterUplinkID = binding
	}
	if req.ConverterDownlinkID != nil {
		binding := normalizeBindingID(req.ConverterDownlinkID)
		if err := validateConverterBinding(claims.TenantID, binding, "converter_downlink_id"); err != nil {
			return nil, err
		}
		record.ConverterDownlinkID = binding
	}
	if req.Config != nil {
		if strings.TrimSpace(*req.Config) == "" {
			record.Config = "{}"
		} else {
			if !IsJSON(*req.Config) {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "config must be valid JSON")
			}
			record.Config = *req.Config
		}
	}
	if req.Enabled != nil {
		record.Enabled = *req.Enabled
	}

	now := time.Now().UTC()
	record.UpdatedAt = &now

	if err := dal.UpdateIntegration(record); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return record, nil
}

// GetIntegrationByID 查询单个集成实例详情
func (*IntegrationService) GetIntegrationByID(ctx context.Context, id string, claims *utils.UserClaims) (*model.Integration, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	record, err := dal.GetIntegrationByID(id, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "integration not found")
	}
	return record, nil
}

// DeleteIntegration 删除集成实例
func (*IntegrationService) DeleteIntegration(ctx context.Context, id string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	if _, err := dal.GetIntegrationByID(id, claims.TenantID); err != nil {
		return errcode.NewWithMessage(errcode.CodeNotFound, "integration not found")
	}
	return dal.DeleteIntegration(id, claims.TenantID)
}

// ListIntegrations 分页查询列表
func (*IntegrationService) ListIntegrations(ctx context.Context, req *model.GetIntegrationListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	total, list, err := dal.ListIntegrations(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	res := make(map[string]interface{})
	res["total"] = total
	res["list"] = list
	return res, nil
}
