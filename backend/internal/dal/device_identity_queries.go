// 文件用途：集中设备编号与凭证的唯一性预检查询。
//
// 这些函数保持原有全局精确匹配语义，不自行加入 tenant 条件、trim、大小写
// 归一化、事务或锁；它们只是写入前预检，真正的并发唯一性仍由数据库约束负责。
//
// 批次一收敛（2026-08-24，见 references/gen-inheritance-audit.md）：预检位于
// 设备创建/激活高频路径，全部改走 raw global.DB 链（clone==1 根，每次链式起点
// 均为全新 Statement），杜绝高并发下 gen 继承链残留 Model/Dest 导致预检读到
// 旧快照（INSERT 后 SELECT 查重漏判/误判的 CI 实锤根因）。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// CheckDeviceNumberExists checks if a device number already exists in the database.
func CheckDeviceNumberExists(deviceNumber string) (bool, error) {
	var count int64
	err := global.DB.Model(&model.Device{}).
		Where("device_number = ?", deviceNumber).
		Count(&count).Error
	if err != nil {
		logrus.Error(err)
		return false, err
	}
	return count > 0, nil
}

// CheckDeviceNumbersExists returns the subset of exact device numbers already stored.
func CheckDeviceNumbersExists(deviceNumbers []string) (map[string]bool, error) {
	existing := make(map[string]bool, len(deviceNumbers))
	normalized := make([]string, 0, len(deviceNumbers))
	seen := make(map[string]struct{}, len(deviceNumbers))
	for _, deviceNumber := range deviceNumbers {
		if deviceNumber == "" {
			continue
		}
		if _, ok := seen[deviceNumber]; ok {
			continue
		}
		seen[deviceNumber] = struct{}{}
		normalized = append(normalized, deviceNumber)
	}
	if len(normalized) == 0 {
		return existing, nil
	}

	var devices []*model.Device
	err := global.DB.Model(&model.Device{}).
		Where("device_number IN ?", normalized).
		Select("device_number").
		Find(&devices).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	for _, device := range devices {
		existing[device.DeviceNumber] = true
	}
	return existing, nil
}

// CheckVoucherExists checks whether a voucher belongs to another device.
// 凭证哈希存储 Phase 1（references/backend-hardening-plan.md 车道1）：双模式预检——
// 先按 voucher_hash 计数（索引路径），未命中回落 voucher=? 明文计数；两列在写入侧
// 二段式与回填下保持同值，命中任一即判定冲突，Phase 2 停写明文后移除兜底分支。
//
// 键序兼容（与 GetDeviceByVoucher 同源）：唯一性预检必须与读取侧覆盖同一个匹配面，
// 否则两条路径写出的同义凭证（结构体序 / 字典序）互相看不见，会被判为"不冲突"而签发
// 重复凭证；而 broker 认证侧用 First() 取首条，重复凭证会让设备身份变得不确定。
// 故两轮计数同样按 DeviceVoucherLookupCandidates 展开候选。
func CheckVoucherExists(voucher string, excludeDeviceID string) (bool, error) {
	candidates := utils.DeviceVoucherLookupCandidates(voucher)

	hashes := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		hashes = append(hashes, utils.VoucherStorageHash(candidate))
	}

	// 第一轮：全部候选按 hash 一次 IN 探测（原逐候选 COUNT，最多 3 条往返 → 1 条）。
	// 只需判定存在性，LIMIT 1 让 PG 命中首行即停，不必数完整个匹配集。
	if exists, err := voucherColumnMatchExists("voucher_hash", hashes, excludeDeviceID); err != nil || exists {
		return exists, err
	}
	// 第二轮：全部候选按明文探测，覆盖尚未回填 voucher_hash 的存量行。
	return voucherColumnMatchExists("voucher", candidates, excludeDeviceID)
}

// voucherColumnMatchExists 判断 devices.<column> 是否有任一值落在 values 中（排除 excludeDeviceID）。
// column 仅由本文件以常量传入，不拼接外部输入。
func voucherColumnMatchExists(column string, values []string, excludeDeviceID string) (bool, error) {
	var ids []string
	err := global.DB.Model(&model.Device{}).
		Where(column+" IN ?", values).
		Where("id <> ?", excludeDeviceID).
		Limit(1).
		Pluck("id", &ids).Error
	if err != nil {
		logrus.Error(err)
		return false, err
	}
	return len(ids) > 0, nil
}
