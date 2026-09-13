// 文件用途：模板市场打包载荷的导入与预览（ROADMAP P1.6）。
// 核心逻辑：把此前零调用方的三个完整性函数串成一条真实链路——
// 验签 → 依赖自洽 → 冲突预览 →（显式确认后）逐模板租户幂等导入 + 审计。
//
// 关键注意事项：
//   - 验签 fail closed：未签名、摘要不符、签名不符、密钥未配置一律拒绝。
//     打包载荷在租户间流转，无法验真的包一个都不能进。
//   - 阻断项（包内重名 / count 不符 / type_key 不自洽）非空即拒绝导入：
//     这些问题会让导入结果取决于处理顺序，属于不确定行为。
//   - 覆盖必须显式确认：预览列出租户内已存在的同名模板后，
//     调用方带 confirm_overwrite=true 重发才可继续。静默覆盖等于把
//     "最终谁说了算"交给请求到达顺序。
//   - 单模板导入复用 ImportDeviceTemplateWithTenant 的租户幂等语义
//     （同租户同名同版本返回既有模板）；每个模板独立审计，失败不中断整包，
//     逐条结果原样返回——只报总数不给明细的导入无法核对。
package service

import (
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// ImportMarketBundle 打包载荷导入/预览入口。
func (*DeviceTemplate) ImportMarketBundle(req model.ImportMarketBundleReq, claims *utils.UserClaims) (*model.ImportMarketBundleRsp, error) {
	if err := ensureTenantScopedWriteClaims(claims, "import market bundle"); err != nil {
		return nil, err
	}
	bundle := req.Bundle
	if bundle == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "bundle is required")
	}

	// 1) 验签。任何失败都拒绝，且不给"跳过验签"的口子。
	if err := VerifyMarketBundle(bundle); err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "market bundle verification failed: " + err.Error(),
			"stage": "verify",
		})
	}

	// 2) 包内自洽与依赖检查。阻断项非空即拒绝——不进入预览确认环节，
	//    因为这些问题确认多少遍也改变不了"结果取决于顺序"。
	if issues := CheckMarketBundleDependencies(bundle); len(issues) > 0 {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":    "market bundle has blocking dependency issues",
			"stage":    "dependencies",
			"blocking": issues,
		})
	}

	// 3) 冲突预览（只读）。查不到租户模板版本集合时同样失败：
	//    无法确认没有覆盖风险就不允许继续。
	existing, err := dal.ListDeviceTemplateVersionsInTenant(claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	preview := PreviewMarketBundleImport(bundle, existing)
	rsp := &model.ImportMarketBundleRsp{Preview: preview, Applied: false}

	// 4) 预览模式到此为止，不落库。
	if req.Preview {
		return rsp, nil
	}

	// 5) 覆盖需显式确认。这是"预览 → 确认 → 导入"的人工闸门。
	if len(preview.Overwrite) > 0 && !req.ConfirmOverwrite {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":                      "bundle overwrites existing templates; resubmit with confirm_overwrite=true",
			"stage":                      "confirm",
			"overwrite":                  preview.Overwrite,
			"confirm_overwrite_required": true,
		})
	}

	// 6) 逐模板导入 + 逐模板审计。单个模板失败不中断整包：
	//    中断会让"哪些进来了"取决于模板在包里的顺序，同样不可归因。
	results := make([]model.MarketBundleTemplateImportResult, 0, len(bundle.Templates))
	for _, template := range bundle.Templates {
		if template == nil {
			continue
		}
		version := "1.0.0"
		if template.Version != nil && strings.TrimSpace(*template.Version) != "" {
			version = strings.TrimSpace(*template.Version)
		}
		result := model.MarketBundleTemplateImportResult{
			Name:    strings.TrimSpace(template.Name),
			Version: version,
		}
		tpl, created, ierr := (*DeviceTemplate)(nil).ImportDeviceTemplateWithTenant(
			model.ImportDeviceTemplateReq(*template), claims.TenantID)
		switch {
		case ierr != nil:
			result.Outcome = string(MarketTemplateImportRejected)
			result.Reason = ierr.Error()
		case created:
			result.Outcome = string(MarketTemplateImportCreated)
			if tpl != nil {
				result.TemplateID = tpl.ID
			}
		default:
			result.Outcome = string(MarketTemplateImportIdempotent)
			if tpl != nil {
				result.TemplateID = tpl.ID
			}
		}
		// 每个模板独立留痕：幂等命中与拒绝同样要可归因。
		EmitMarketTemplateImportAudit(BuildMarketTemplateImportAudit(
			claims.TenantID, claims.ID, result.Name, result.Version, created, ierr))
		results = append(results, result)
	}
	rsp.Applied = true
	rsp.Results = results
	return rsp, nil
}
