// 文件用途：资源中心综合服务（TP-5）。
// 核心逻辑：
// 1. Catalog / List 统一编排物模型与大屏看板目录及分页结果；
// 2. ExportResourceBundle 支持统一打包导出物模型与大屏看板，并自动计算 HMAC-SHA256 签名；
// 3. ImportResourceBundle 执行验签 → 依赖检查 → 冲突预览 → 覆盖确认 → 统一安全落库；
// 4. ApplyResource 提供一键将资源库模板实例化到当前租户的便捷能力。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

type ResourceCenter struct{}

// ResourceCenterCatalog 获取综合分类目录。
func (*ResourceCenter) ResourceCenterCatalog(claims *utils.UserClaims) ([]model.ResourceCenterCatalogEntry, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	ctx := context.Background()
	return dal.ListResourceCenterCatalog(ctx, claims.TenantID)
}

// ResourceCenterList 综合分页检索。
func (*ResourceCenter) ResourceCenterList(req model.ResourceCenterListReq, claims *utils.UserClaims) (*model.ResourceCenterListRsp, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	ctx := context.Background()
	total, items, err := dal.ListResourceCenterItems(ctx, req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return &model.ResourceCenterListRsp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		List:     items,
	}, nil
}

// ExportResourceBundle 统一打包导出（支持同时打包物模型与大屏看板）。
func (*ResourceCenter) ExportResourceBundle(typeKey, resourceType string, claims *utils.UserClaims) (*model.MarketBundle, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	ctx := context.Background()
	tenantID := claims.TenantID

	rType := strings.TrimSpace(resourceType)
	if rType == "" {
		rType = "all"
	}

	includeTemplates := rType == "all" || rType == "device_template"
	includeBoards := rType == "all" || rType == "board_template"

	templates := make([]*model.DeviceTemplateExport, 0)
	if includeTemplates {
		tplIDs, err := dal.ListTemplateIDsByTypeKey(ctx, tenantID, typeKey)
		if err == nil && len(tplIDs) > 0 {
			for _, id := range tplIDs {
				exported, err := GroupApp.DeviceTemplate.ExportDeviceTemplate(id, claims)
				if err == nil && exported != nil {
					templates = append(templates, exported)
				}
			}
		}
	}

	boards := make([]*model.BoardTemplateExport, 0)
	if includeBoards {
		boardIDs, err := dal.ListBoardIDsByTypeKey(ctx, tenantID, typeKey)
		if err == nil && len(boardIDs) > 0 {
			for _, id := range boardIDs {
				exported, err := GroupApp.Board.ExportBoard(id, claims)
				if err == nil && exported != nil {
					boards = append(boards, exported)
				}
			}
		}
	}

	totalCount := len(templates) + len(boards)
	if totalCount == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "no resources found for export")
	}

	bundle := &model.MarketBundle{
		TypeKey:    typeKey,
		ExportedAt: time.Now().UnixMilli(),
		Count:      totalCount,
		Templates:  templates,
		Boards:     boards,
	}

	// 执行数字签名
	if err := SignMarketBundle(bundle); err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"sign_error": err.Error(),
		})
	}

	return bundle, nil
}

// ImportResourceBundle 导入综合资源包（验签 → 依赖检查 → 冲突预览 → 确认覆盖 → 统一安全落库）。
func (*ResourceCenter) ImportResourceBundle(req model.ImportMarketBundleReq, claims *utils.UserClaims) (*model.ImportMarketBundleRsp, error) {
	if err := ensureTenantScopedWriteClaims(claims, "import resource bundle"); err != nil {
		return nil, err
	}
	bundle := req.Bundle
	if bundle == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "bundle is required")
	}

	// 1) 验签 (Fail closed)
	if err := VerifyMarketBundle(bundle); err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "resource bundle verification failed: " + err.Error(),
			"stage": "verify",
		})
	}

	// 2) 依赖自洽与重名检查
	if issues := CheckMarketBundleDependencies(bundle); len(issues) > 0 {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":    "resource bundle has blocking dependency issues",
			"stage":    "dependencies",
			"blocking": issues,
		})
	}

	ctx := context.Background()
	tenantID := claims.TenantID

	// 3) 读取租户内现有物模型版本与看板版本
	existingTpls, err := dal.ListDeviceTemplateVersionsInTenant(tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	existingBoards, err := dal.ListBoardTemplateVersionsInTenant(ctx, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	// 4) 冲突预览
	preview := PreviewResourceBundleImport(bundle, existingTpls, existingBoards)
	rsp := &model.ImportMarketBundleRsp{Preview: preview, Applied: false}

	// 5) 预览模式直接返回
	if req.Preview {
		return rsp, nil
	}

	// 6) 覆盖确认闸门
	if len(preview.Overwrite) > 0 && !req.ConfirmOverwrite {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":                      "bundle overwrites existing resources; resubmit with confirm_overwrite=true",
			"stage":                      "confirm",
			"overwrite":                  preview.Overwrite,
			"confirm_overwrite_required": true,
		})
	}

	// 7) 逐项导入
	results := make([]model.MarketBundleTemplateImportResult, 0, len(bundle.Templates)+len(bundle.Boards))

	// 导入设备模板
	for _, tpl := range bundle.Templates {
		if tpl == nil {
			continue
		}
		ver := "1.0.0"
		if tpl.Version != nil && strings.TrimSpace(*tpl.Version) != "" {
			ver = strings.TrimSpace(*tpl.Version)
		}
		res := model.MarketBundleTemplateImportResult{
			Kind:    "device_template",
			Name:    strings.TrimSpace(tpl.Name),
			Version: ver,
		}
		savedTpl, created, ierr := (*DeviceTemplate)(nil).ImportDeviceTemplateWithTenant(
			model.ImportDeviceTemplateReq(*tpl), tenantID)
		switch {
		case ierr != nil:
			res.Outcome = string(MarketTemplateImportRejected)
			res.Reason = ierr.Error()
		case created:
			res.Outcome = string(MarketTemplateImportCreated)
			if savedTpl != nil {
				res.TemplateID = savedTpl.ID
			}
		default:
			res.Outcome = string(MarketTemplateImportIdempotent)
			if savedTpl != nil {
				res.TemplateID = savedTpl.ID
			}
		}
		EmitMarketTemplateImportAudit(BuildMarketTemplateImportAudit(
			claims.TenantID, claims.ID, res.Name, res.Version, created, ierr))
		results = append(results, res)
	}

	// 导入大屏看板
	for _, board := range bundle.Boards {
		if board == nil {
			continue
		}
		ver := "1.0.0"
		if board.Version != nil && strings.TrimSpace(*board.Version) != "" {
			ver = strings.TrimSpace(*board.Version)
		}
		res := model.MarketBundleTemplateImportResult{
			Kind:    "board_template",
			Name:    strings.TrimSpace(board.Name),
			Version: ver,
		}
		savedBoard, berr := GroupApp.Board.ImportBoard(board, claims)
		if berr != nil {
			res.Outcome = string(MarketTemplateImportRejected)
			res.Reason = berr.Error()
		} else {
			res.Outcome = string(MarketTemplateImportCreated)
			if savedBoard != nil {
				res.TemplateID = savedBoard.ID
			}
		}
		results = append(results, res)
	}

	rsp.Applied = true
	rsp.Results = results
	return rsp, nil
}

// ApplyResource 一键应用资源到当前租户。
func (*ResourceCenter) ApplyResource(req model.ResourceCenterApplyReq, claims *utils.UserClaims) (*model.ResourceCenterApplyRsp, error) {
	if err := ensureTenantScopedWriteClaims(claims, "apply resource"); err != nil {
		return nil, err
	}
	rType := strings.TrimSpace(req.ResourceType)
	resourceID := strings.TrimSpace(req.ResourceID)
	targetName := strings.TrimSpace(req.TargetName)

	switch rType {
	case "device_template":
		exported, err := GroupApp.DeviceTemplate.ExportDeviceTemplate(resourceID, claims)
		if err != nil {
			return nil, err
		}
		if targetName != "" {
			exported.Name = targetName
		}
		tpl, _, err := (*DeviceTemplate)(nil).ImportDeviceTemplateWithTenant(
			model.ImportDeviceTemplateReq(*exported), claims.TenantID)
		if err != nil {
			return nil, err
		}
		return &model.ResourceCenterApplyRsp{
			ResourceType: "device_template",
			TargetID:     tpl.ID,
			TargetName:   tpl.Name,
			Message:      "device template applied successfully",
			Resource:     tpl,
		}, nil

	case "board_template":
		exported, err := GroupApp.Board.ExportBoard(resourceID, claims)
		if err != nil {
			return nil, err
		}
		if targetName != "" {
			exported.Name = targetName
		}
		board, err := GroupApp.Board.ImportBoard(exported, claims)
		if err != nil {
			return nil, err
		}
		return &model.ResourceCenterApplyRsp{
			ResourceType: "board_template",
			TargetID:     board.ID,
			TargetName:   board.Name,
			Message:      "board template applied successfully",
			Resource:     board,
		}, nil

	case "rule_chain":
		// TB-19 剩余缺口闭环：规则链纳入方案可引用资源。导出走只读
		// ExportChain，导入复用 CreateChain 的全部校验（graph 规范化、
		// 名称校验、租户取自 claims）——每次安装实例化一条新链，与
		// 看板模板语义一致；源链只读不动。
		exported, err := GroupApp.RuleChain.ExportChain(resourceID, claims)
		if err != nil {
			return nil, err
		}
		name := exported.Name
		if targetName != "" {
			name = targetName
		}
		raw, err := json.Marshal(map[string]interface{}{
			"name":        name,
			"description": exported.Description,
			"enabled":     exported.Enabled,
			"graph":       exported.Graph,
		})
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "rule chain export is not valid json")
		}
		chain, err := GroupApp.RuleChain.CreateChain(raw, claims)
		if err != nil {
			return nil, err
		}
		return &model.ResourceCenterApplyRsp{
			ResourceType: "rule_chain",
			TargetID:     chain.ID,
			TargetName:   chain.Name,
			Message:      "rule chain applied successfully",
			Resource:     chain,
		}, nil

	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("unsupported resource_type: %s", rType))
	}
}
