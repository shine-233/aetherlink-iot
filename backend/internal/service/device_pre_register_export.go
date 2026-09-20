// 文件用途：预注册批次清理的范围判定（ROADMAP P0.5）。
// 核心逻辑：按租户与激活态把待清理设备分流为「可删除 / 已激活被拒」，再交给调用方执行删除，
// 绝不在未分流的情况下批量删除。执行面见 device_preregister_cleanup.go，
// 本文件的 classifyPreRegisterCleanup 由它以注入方式复用（classify 字段）。
// 关键注意事项：
//  1. 跨租户设备出现在待清理集合里是安全事件而非普通噪声：一律 fail closed 返回错误，
//     不静默过滤——静默过滤会掩盖越权查询缺陷。
//  2. 已激活（ActivateFlag != inactive）设备永不进入可删除集合，避免清理批次时误删在运设备。
//  3. 空批次返回空计划且不报错，保证重复调用幂等。
//  4. 导出面（Excel，含 utils.MaskVoucher 脱敏）在 device_preregister_export.go，不在本文件。
package service

import (
	"strings"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

// preRegisterCleanupPlan 清理分流结果：deletable 为可安全删除的设备 ID，
// blockedActivated 为因已激活而被拒绝删除的设备编号，供调用方回传给用户。
type preRegisterCleanupPlan struct {
	deletable          []string
	blockedActivated   []string
	blockedCrossTenant []string
}

// isEmpty 是否已无可清理项。清理已清空的批次返回空计划而非错误，保证幂等。
func (p *preRegisterCleanupPlan) isEmpty() bool {
	return p == nil || (len(p.deletable) == 0 && len(p.blockedActivated) == 0)
}

// classifyPreRegisterCleanup 按租户与激活态分流待清理设备。
// 跨租户设备一律返回错误（fail closed）；已激活设备进入 blockedActivated 而非可删除集。
func classifyPreRegisterCleanup(devices []*model.Device, tenantID string) (*preRegisterCleanupPlan, error) {
	plan := &preRegisterCleanupPlan{}
	for _, device := range devices {
		if device == nil {
			continue
		}
		if device.TenantID != tenantID {
			// 越权数据进入清理集合说明上游查询缺少租户条件，属于缺陷而非普通数据噪声。
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission,
				"pre-register cleanup refuses cross-tenant device")
		}
		if !strings.EqualFold(strings.TrimSpace(device.ActivateFlag), preRegisterActivateFlag) {
			plan.blockedActivated = append(plan.blockedActivated, device.DeviceNumber)
			continue
		}
		plan.deletable = append(plan.deletable, device.ID)
	}
	return plan, nil
}
