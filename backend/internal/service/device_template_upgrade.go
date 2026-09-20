// 文件用途：P1.6 模板升级/回滚的服务层——版本推进与旧载荷重放。
// 核心逻辑：
//   - 升级：当前版本行 → 捕获其完整导出载荷存入历史 → 以新版本导入新行
//     （复用 ImportDeviceTemplateWithTenant 的租户幂等导入）；
//   - 回滚：按历史记录重放旧载荷（同租户同名同版本幂等命中即恢复），不删任何行。
//
// 关键注意事项：
//   - **回滚是重放不是删除**：删新版本行不可逆且会牵连引用它的设备配置；
//     重放旧版本行让两个版本共存，切换动作交给引用方。
//   - 目标版本必须**严格新于**当前版本（点分数字逐段比较）——"升级"允许降级
//     会让"哪个是当前"永远说不清；要回滚请走回滚通道，语义可审计。
//   - 历史必须在导入成功**之后**落库：先记历史再导入失败会留下"没升级却有回滚点"
//     的幽灵记录；导入成功后落库失败则如实报错（升级已生效，历史缺失可重发补记）。
package service

import (
	"encoding/json"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
)

// UpgradeDeviceTemplate 模板升级：导入新版本并记录可回滚的历史。
func (*DeviceTemplate) UpgradeDeviceTemplate(req model.UpgradeDeviceTemplateReq, claims *utils.UserClaims) (*model.UpgradeDeviceTemplateRsp, error) {
	if err := ensureTenantScopedWriteClaims(claims, "upgrade device template"); err != nil {
		return nil, err
	}
	payload := req.Payload
	if payload == nil || strings.TrimSpace(payload.Name) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "payload with template name is required")
	}
	name := strings.TrimSpace(payload.Name)
	newVersion := "1.0.0"
	if payload.Version != nil && strings.TrimSpace(*payload.Version) != "" {
		newVersion = strings.TrimSpace(*payload.Version)
	}
	if !isDottedNumericVersion(newVersion) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "target version must be dotted numeric (e.g. 1.2.0)")
	}

	// 当前版本 = 该名称在租户内最新的一行；不存在则没有"升级"可言。
	current, err := dal.GetLatestDeviceTemplateByName(claims.TenantID, name)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if current == nil || current.Version == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "template not found in tenant; import it before upgrading")
	}
	fromVersion := *current.Version
	if compareVersionSegments(mustSegments(fromVersion), mustSegments(newVersion)) >= 0 {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":        "target version must be strictly newer than the current one; use rollback to downgrade",
			"from_version": fromVersion,
			"to_version":   newVersion,
		})
	}

	// 捕获旧版本完整导出载荷（回滚凭据）。
	previous := buildTemplateExportPayload(current)
	previousRaw, mErr := json.Marshal(previous)
	if mErr != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "marshal previous payload failed: "+mErr.Error())
	}

	template, created, ierr := (*DeviceTemplate)(nil).ImportDeviceTemplateWithTenant(
		model.ImportDeviceTemplateReq(*payload), claims.TenantID)
	if ierr != nil {
		return nil, ierr
	}

	history := &model.TemplateUpgradeHistory{
		ID:              uuid.New().String(),
		TenantID:        claims.TenantID,
		TemplateName:    name,
		FromVersion:     fromVersion,
		ToVersion:       newVersion,
		PreviousPayload: string(previousRaw),
		ActorID:         claims.ID,
		CreatedAt:       time.Now().UTC(),
	}
	if herr := dal.InsertTemplateUpgradeHistory(history); herr != nil {
		// 升级已生效、历史未落：如实报错让调用方重试补记，不伪造成功。
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": herr.Error(),
			"note":      "upgrade applied but history write failed; retry to record",
		})
	}
	_ = created // 幂等命中说明该版本已存在，升级结果相同
	return &model.UpgradeDeviceTemplateRsp{HistoryID: history.ID, Template: template}, nil
}

// RollbackTemplateUpgrade 按历史记录重放旧版本载荷（幂等；不删除新版本行）。
func (*DeviceTemplate) RollbackTemplateUpgrade(historyID string, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	if err := ensureTenantScopedWriteClaims(claims, "rollback device template upgrade"); err != nil {
		return nil, err
	}
	history, err := dal.GetTemplateUpgradeHistoryInTenant(historyID, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "upgrade history not found")
	}
	var previous model.DeviceTemplateExport
	if uerr := json.Unmarshal([]byte(history.PreviousPayload), &previous); uerr != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "stored previous payload is corrupted: " + uerr.Error(),
		})
	}
	template, _, ierr := (*DeviceTemplate)(nil).ImportDeviceTemplateWithTenant(previous, claims.TenantID)
	if ierr != nil {
		return nil, ierr
	}
	return template, nil
}

// buildTemplateExportPayload 把当前行转成导出载荷（升级历史存的就是它）。
// 与导出端点同字段集，但**不依赖 claims**——历史记录要在服务端可重放。
func buildTemplateExportPayload(t *model.DeviceTemplate) model.DeviceTemplateExport {
	return model.DeviceTemplateExport{
		Kind:           "aetherlink-device-template",
		Name:           t.Name,
		Author:         t.Author,
		Version:        t.Version,
		Description:    t.Description,
		Remark:         t.Remark,
		Path:           t.Path,
		Label:          t.Label,
		Brand:          t.Brand,
		ModelNumber:    t.ModelNumber,
		TypeKey:        t.TypeKey,
		WebChartConfig: t.WebChartConfig,
		AppChartConfig: t.AppChartConfig,
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
	}
}

// mustSegments 点分数字版本 → 段数组（调用方已保证格式合法，解析失败按 0 段处理）。
func mustSegments(version string) []int {
	segments, _ := parseVersionSegments(version)
	return segments
}
