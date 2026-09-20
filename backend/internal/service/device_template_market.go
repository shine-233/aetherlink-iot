package service

import (
	"context"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"
	"aetherlink-iot/backend/pkg/errcode"
)

// PHASE-D-D10 BEGIN 模板市场运营化服务

// MarketCatalog 行业分类目录（浏览页 tab）：租户内 distinct type_key + 计数。
func (*DeviceTemplate) MarketCatalog(claims *utils.UserClaims) ([]model.MarketCatalogEntry, error) {
	tenantID := claims.TenantID
	if tenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	rows, err := dal.ListMarketCatalog(context.Background(), tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return rows, nil
}

// ExportMarketBundle 按行业打包导出：逐模板复用既有导出契约（描述符可回放 import），
// 并对包含模板的 download_count 各 +1（typeKey 为空时导出该租户全量模板）。
func (*DeviceTemplate) ExportMarketBundle(typeKey string, claims *utils.UserClaims) (*model.MarketBundle, error) {
	tenantID := claims.TenantID
	if tenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	ids, err := dal.ListTemplateIDsByTypeKey(context.Background(), tenantID, typeKey)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if len(ids) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "no templates for this type_key")
	}
	templates := make([]*model.DeviceTemplateExport, 0, len(ids))
	for _, id := range ids {
		exported, err := GroupApp.DeviceTemplate.ExportDeviceTemplate(id, claims)
		if err != nil {
			return nil, err
		}
		templates = append(templates, exported)
	}
	// 打包计数：ExportDeviceTemplate 内部已按单模板计数（打包=逐模板各 +1），此处不再重复。
	bundle := &model.MarketBundle{
		TypeKey:    typeKey,
		ExportedAt: time.Now().UnixMilli(),
		Count:      len(templates),
		Templates:  templates,
	}
	// 出包即签名：未配置签名密钥一律拒绝出包，不允许无法验真的包在租户间流转。
	if err := SignMarketBundle(bundle); err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"sign_error": err.Error(),
		})
	}
	return bundle, nil
}

// PHASE-D-D10 END
