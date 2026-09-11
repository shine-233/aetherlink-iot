// 文件用途：预注册批次的脱敏导出与清理范围判定（ROADMAP P0.5 后半段）。
// 核心逻辑：导出面一律经 utils.MaskVoucher 掩码，明文凭证只存在于创建响应；
// 清理先按租户与激活态分流，再交给调用方执行删除，绝不在未分流的情况下批量删除。
// 关键注意事项：
//  1. 导出表头刻意区别于导入表头（导入为 device_number,name，导出多列且含 voucher_masked），
//     避免导出文件被直接当导入文件回灌——那会用掩码串覆盖真实凭证。
//  2. 导出顺序按 device_number 排序，保证同一集合多次导出字节一致（幂等可比较）。
//  3. 跨租户设备出现在待清理集合里是安全事件而非普通跳过：一律 fail closed 返回错误，
//     不静默过滤——静默过滤会掩盖越权查询缺陷。
//  4. 已激活（ActivateFlag != inactive）设备永不进入可删除集合，避免清理批次时误删在运设备。
//
// 重构建议：接入 API 层时，导出应返回 CSV 字节与 Content-Disposition；清理应在同一事务内
// 完成「查询→分流→删除」，并对 deletable 为空的情况返回 0 而不是报错。
package service

import (
	"bytes"
	"encoding/csv"
	"sort"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// preRegisterExportColumns 导出表头。刻意与导入表头（device_number,name）不同，
// 防止导出文件被当作导入文件回灌。
func preRegisterExportColumns() []string {
	return []string{"device_number", "name", "voucher_masked", "activate_flag", "created_at"}
}

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

// buildPreRegisterExportRows 把预注册设备转为导出数据行（不含表头）。
// 顺序按 device_number 升序，保证同一输入的导出结果字节一致。
func buildPreRegisterExportRows(devices []*model.Device) [][]string {
	ordered := make([]*model.Device, 0, len(devices))
	for _, device := range devices {
		if device == nil {
			continue
		}
		ordered = append(ordered, device)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].DeviceNumber < ordered[j].DeviceNumber
	})

	rows := make([][]string, 0, len(ordered))
	for _, device := range ordered {
		rows = append(rows, []string{
			device.DeviceNumber,
			preRegisterStringValue(device.Name),
			utils.MaskVoucher(device.Voucher),
			device.ActivateFlag,
			preRegisterExportTimestamp(device.CreatedAt),
		})
	}
	return rows
}

// preRegisterExportTimestamp 统一按 UTC RFC3339 输出；空时间输出空串而不是零值时间，
// 避免把"没有时间"伪装成 1970-01-01。
func preRegisterExportTimestamp(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// encodePreRegisterExportCSV 把数据行编码为带表头的 CSV 字节。
func encodePreRegisterExportCSV(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(preRegisterExportColumns()); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
