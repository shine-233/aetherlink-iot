// File purpose: TB-19 解决方案模板引擎——「一键装一套行业方案」的编排层。
// Core logic: 方案 = 有序资源引用清单；安装 = 逐项调用资源中心已验证的
// ApplyResource 管道（export→import，含租户归属校验），逐项留安装流水。
// Key notes: 不建第二套打包/签名/冲突闸门——那是 TP-5 / P1.6 的既有链路。
// 物模型模板导入租户幂等，看板导入每次实例化新看板（与 TB 解决方案模板语义一致）。
package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// IndustrySolutionService TB-19 服务入口（无状态，零值可用）。
type IndustrySolutionService struct{}

// solutionResourceTypes 与资源中心 ApplyResource 的白名单保持同一份口径。
var solutionResourceTypes = map[string]bool{
	"device_template": true,
	"board_template":  true,
	"rule_chain":      true,
}

// CreateIndustrySolution 创建方案：逐项校验引用类型，并当场用资源中心的
// 导出路径验证「资源存在且属于本租户」——引用失效的方案不允许被创建。
func (*IndustrySolutionService) CreateIndustrySolution(_ context.Context, req *model.CreateIndustrySolutionReq, claims *utils.UserClaims) (*model.IndustrySolution, error) {
	if err := ensureTenantScopedWriteClaims(claims, "create industry solution"); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "name is required")
	}
	if len(req.Resources) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "resources must not be empty")
	}
	for i, ref := range req.Resources {
		if !solutionResourceTypes[strings.TrimSpace(ref.ResourceType)] {
			return nil, errcode.NewWithMessage(errcode.CodeParamError,
				"resources["+strconv.Itoa(i)+"].resource_type must be device_template, board_template or rule_chain")
		}
		if strings.TrimSpace(ref.ResourceID) == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError,
				"resources["+strconv.Itoa(i)+"].resource_id is required")
		}
	}

	// 创建即校验每一条引用，但**必须用只读的导出路径**：ApplyResource 是安装动作，
	// 会创建实例，用它探测会凭空多出一套资源。导出内部做归属检查，
	// 引用不存在或不属于本租户都会在这里被拒——不允许交付装不上的方案。
	for i, ref := range req.Resources {
		probe := strings.TrimSpace(ref.ResourceID)
		var probeErr error
		switch strings.TrimSpace(ref.ResourceType) {
		case "device_template":
			_, probeErr = GroupApp.DeviceTemplate.ExportDeviceTemplate(probe, claims)
		case "board_template":
			_, probeErr = GroupApp.Board.ExportBoard(probe, claims)
		case "rule_chain":
			_, probeErr = GroupApp.RuleChain.ExportChain(probe, claims)
		}
		if probeErr != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError,
				"resources["+strconv.Itoa(i)+"] is not a usable resource in your tenant")
		}
	}

	// 同租户同名拒绝（唯一索引兜底，服务层先查给可读错误）。
	if _, err := dal.GetIndustrySolutionByNameAndTenant(claims.TenantID, name); err == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			"solution name already exists in your tenant")
	}

	refs, err := json.Marshal(req.Resources)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "resources must be valid JSON")
	}
	now := time.Now()
	s := &model.IndustrySolution{
		ID:        uuid.NewString(),
		TenantID:  claims.TenantID,
		Name:      name,
		Resources: refs,
		Status:    model.IndustrySolutionStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		s.Description = &desc
	}
	if err := dal.InsertIndustrySolution(nil, s); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeDBError, "create solution failed")
	}

	logrus.WithFields(logrus.Fields{
		"module": "industry_solution", "action": "create",
		"tenant_id": claims.TenantID, "solution_id": s.ID,
		"resources": len(s.Resources),
		"audit_message": "industry solution created",
	}).Info("industry solution created")
	return s, nil
}

// ListIndustrySolutions 租户内方案分页列表。
func (*IndustrySolutionService) ListIndustrySolutions(_ context.Context, page, pageSize int, claims *utils.UserClaims) (map[string]interface{}, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.New(errcode.CodeUnauthorized)
	}
	total, list, err := dal.ListIndustrySolutionsByTenant(claims.TenantID, page, pageSize)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeDBError, "list solutions failed")
	}
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}

// GetIndustrySolution 方案详情（含安装流水回查）。
func (*IndustrySolutionService) GetIndustrySolution(_ context.Context, id string, claims *utils.UserClaims) (map[string]interface{}, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.New(errcode.CodeUnauthorized)
	}
	s, err := dal.GetIndustrySolutionByIDAndTenant(claims.TenantID, strings.TrimSpace(id))
	if err != nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	installs, err := dal.ListIndustrySolutionInstalls(claims.TenantID, s.ID, 50)
	if err != nil {
		installs = nil
	}
	return map[string]interface{}{
		"solution": s,
		"installs": installs,
	}, nil
}

// DeleteIndustrySolution 删除方案（引用的资源不动——方案只是引用清单）。
func (*IndustrySolutionService) DeleteIndustrySolution(_ context.Context, id string, claims *utils.UserClaims) error {
	if err := ensureTenantScopedWriteClaims(claims, "delete industry solution"); err != nil {
		return err
	}
	if err := dal.DeleteIndustrySolution(claims.TenantID, strings.TrimSpace(id)); err != nil {
		return errcode.New(errcode.CodeNotFound)
	}
	logrus.WithFields(logrus.Fields{
		"module": "industry_solution", "action": "delete",
		"tenant_id": claims.TenantID, "solution_id": id,
		"audit_message": "industry solution deleted",
	}).Info("industry solution deleted")
	return nil
}

// InstallIndustrySolution 一键安装：按清单顺序逐项应用，逐项留流水。
// ContinueOnError 缺省 true；单项失败不产生静默跳过——结果与流水都如实记录。
func (*IndustrySolutionService) InstallIndustrySolution(_ context.Context, id string, req *model.InstallIndustrySolutionReq, claims *utils.UserClaims) (*model.IndustrySolutionInstallRsp, error) {
	if err := ensureTenantScopedWriteClaims(claims, "install industry solution"); err != nil {
		return nil, err
	}
	s, err := dal.GetIndustrySolutionByIDAndTenant(claims.TenantID, strings.TrimSpace(id))
	if err != nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	var refs []model.IndustrySolutionResourceRef
	if err := json.Unmarshal(s.Resources, &refs); err != nil || len(refs) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			"solution resource list is empty or corrupted")
	}
	continueOnError := true
	if req != nil && req.ContinueOnError != nil {
		continueOnError = *req.ContinueOnError
	}

	rc := (*ResourceCenter)(nil)
	rsp := &model.IndustrySolutionInstallRsp{
		SolutionID:   s.ID,
		SolutionName: s.Name,
		Total:        len(refs),
		Items:        make([]model.IndustrySolutionInstallItemResult, 0, len(refs)),
	}
	auditRows := make([]model.IndustrySolutionInstall, 0, len(refs))

	for i, ref := range refs {
		item := model.IndustrySolutionInstallItemResult{
			ItemIndex:    i,
			ResourceType: ref.ResourceType,
			ResourceID:   ref.ResourceID,
		}
		apply := model.ResourceCenterApplyReq{
			ResourceType: ref.ResourceType,
			ResourceID:   ref.ResourceID,
			TargetName:   ref.TargetName,
		}
		applied, err := rc.ApplyResource(apply, claims)
		now := time.Now()
		row := model.IndustrySolutionInstall{
			ID:           uuid.NewString(),
			TenantID:     claims.TenantID,
			SolutionID:   s.ID,
			SolutionName: s.Name,
			ItemIndex:    i,
			ResourceType: ref.ResourceType,
			ResourceID:   ref.ResourceID,
			CreatedAt:    now,
		}
		if err != nil {
			item.Status = model.IndustryInstallStatusFailed
			item.Error = err.Error()
			row.Status = model.IndustryInstallStatusFailed
			errText := err.Error()
			row.Error = &errText
			rsp.Failed++
		} else {
			item.Status = model.IndustryInstallStatusApplied
			item.TargetID = applied.TargetID
			item.TargetName = applied.TargetName
			row.Status = model.IndustryInstallStatusApplied
			targetID := applied.TargetID
			row.TargetID = &targetID
			rsp.Applied++
		}
		rsp.Items = append(rsp.Items, item)
		auditRows = append(auditRows, row)

		if err != nil && !continueOnError {
			break
		}
	}

	// 流水与安装结果一致落库（append-only；失败不中断也不回滚已应用的项——
	// 逐项结果是「一键安装」的既定语义，写流水失败时只打日志不掩盖安装事实）。
	if err := dal.InsertIndustrySolutionInstalls(nil, auditRows); err != nil {
		logrus.WithFields(logrus.Fields{
			"module": "industry_solution", "action": "install",
			"tenant_id": claims.TenantID, "solution_id": s.ID,
			"audit_message": "install audit write failed",
		}).WithError(err).Warn("industry solution install audit persistence failed")
	}

	logrus.WithFields(logrus.Fields{
		"module": "industry_solution", "action": "install",
		"tenant_id": claims.TenantID, "solution_id": s.ID,
		"applied": rsp.Applied, "failed": rsp.Failed,
		"audit_message": "industry solution installed",
	}).Info("industry solution installed")
	return rsp, nil
}

// MarshalSolutionResources jsonb 反序列化兜底（保障 references 列损坏时给出可读错误）。
func MarshalSolutionResources(raw []byte) ([]model.IndustrySolutionResourceRef, error) {
	refs := make([]model.IndustrySolutionResourceRef, 0)
	err := json.Unmarshal(raw, &refs)
	return refs, err
}
