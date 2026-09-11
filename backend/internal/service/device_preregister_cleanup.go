// File purpose: P0.5 execution plane for pre-register batch cleanup.
// Core logic: query -> classify (tenant + activation split) -> delete, in one transaction.
// Key notes: classifyPreRegisterCleanup only sorts devices into deletable / blocked; on its own
// it never deletes anything, so without this file "cleanup" is a plan that is never executed.
//   - Cross-tenant device in the set is a security event: fail closed, never silently filtered.
//   - Activated devices are never deleted - cleaning a batch must not kill running devices.
//   - An already-empty batch returns 0, not an error (idempotent).

package service

import (
	"context"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"
	"aetherlink-iot/backend/pkg/errcode"
)

// PreRegisterCleanupResult 一次清理的结果。
type PreRegisterCleanupResult struct {
	Deleted           int      `json:"deleted"`
	BlockedActivated  []string `json:"blocked_activated"`
	Scanned           int      `json:"scanned"`
}

// preRegisterCleanupOperations 副作用集合，可注入以便无数据库验证清理语义。
type preRegisterCleanupOperations struct {
	load     func(req model.ExportPreRegisterReq, tenantID string) ([]*model.Device, error)
	classify func(devices []*model.Device, tenantID string) (*preRegisterCleanupPlan, error)
	remove   func(ids []string, tenantID string) (int64, error)
}

var preRegisterCleanupOps = preRegisterCleanupOperations{
	load:     loadPreRegisterCleanupCandidates,
	classify: classifyPreRegisterCleanup,
	remove:   deletePreRegisterDevices,
}

// CleanupDevicePreRegister 清理一个预注册批次中仍未激活的设备。
// 顺序固定为「查询 → 分流 → 删除」：反向执行会先删后判，
// 一旦分流阶段发现跨租户数据，设备已经被删掉了，无法回滚。
func (*Device) CleanupDevicePreRegister(req model.ExportPreRegisterReq, claims *utils.UserClaims) (*PreRegisterCleanupResult, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if req.ProductID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "product_id is required")
	}

	devices, err := preRegisterCleanupOps.load(req, claims.TenantID)
	if err != nil {
		return nil, err
	}
	plan, err := preRegisterCleanupOps.classify(devices, claims.TenantID)
	if err != nil {
		return nil, err
	}
	// 已清空：返回 0 而不是报错，重复清理同一批次是幂等的。
	if plan == nil || len(plan.deletable) == 0 {
		return &PreRegisterCleanupResult{
			BlockedActivated: plan.blockedActivatedValue(),
			Scanned:          len(devices),
		}, nil
	}

	deleted, err := preRegisterCleanupOps.remove(plan.deletable, claims.TenantID)
	if err != nil {
		return nil, err
	}
	return &PreRegisterCleanupResult{
		Deleted:          int(deleted),
		BlockedActivated: plan.blockedActivated,
		Scanned:          len(devices),
	}, nil
}

func loadPreRegisterCleanupCandidates(req model.ExportPreRegisterReq, tenantID string) ([]*model.Device, error) {
	qd := query.Device
	return buildPreRegisterExportQuery(req, tenantID).
		Select(qd.ID, qd.TenantID, qd.ActivateFlag, qd.DeviceNumber).
		Find()
}

// deletePreRegisterDevices 删除指定设备，租户条件与 id 列表同时生效。
// 带上 tenant_id 不是冗余：id 来自分流结果，多一层条件就把"越权 id 被误删"
// 从可能变成不可能。
func deletePreRegisterDevices(ids []string, tenantID string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	qd := query.Device
	result, err := qd.WithContext(context.Background()).
		Where(qd.ID.In(ids...)).
		Where(qd.TenantID.Eq(tenantID)).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// blockedActivatedValue 让 nil plan 也能安全取 blocked 列表。
func (p *preRegisterCleanupPlan) blockedActivatedValue() []string {
	if p == nil {
		return nil
	}
	return p.blockedActivated
}
