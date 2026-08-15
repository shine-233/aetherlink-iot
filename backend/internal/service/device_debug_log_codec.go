// device_debug_log_codec.go 负责把设备调试日志统一整理成对外稳定结构，
// 兼容旧字段命名并清理不再暴露给接口层的历史载荷字段。
package service

import (
	"encoding/json"

	model "aetherlink-iot/backend/internal/model"
)

func invalidDeviceDebugLogEntry(raw string) model.DeviceDebugLogEntry {
	return model.DeviceDebugLogEntry{
		Action:  "error",
		Outcome: "error",
		Error:   "invalid log json",
		Meta: map[string]interface{}{
			"raw_size": len(raw),
		},
	}
}

func decodeDeviceDebugLogRow(raw string) (model.DeviceDebugLogEntry, bool) {
	var item model.DeviceDebugLogEntry
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return model.DeviceDebugLogEntry{}, false
	}
	return item, true
}

func normalizeDecodedDeviceDebugLogRow(raw string, item model.DeviceDebugLogEntry, ok bool) model.DeviceDebugLogEntry {
	if !ok {
		return invalidDeviceDebugLogEntry(raw)
	}
	return normalizeDeviceDebugLogEntry(item)
}

// upgradeHistoricalDeviceDebugLogEntry 把历史日志行中的 event/result/extra
// 映射到当前 action/outcome/meta 结构，保证老数据也能被前端稳定消费。
func upgradeHistoricalDeviceDebugLogEntry(item model.DeviceDebugLogEntry) model.DeviceDebugLogEntry {
	if item.Action == "" {
		item.Action = item.Event
	}
	if item.Outcome == "" {
		item.Outcome = normalizeDeviceDebugOutcome(item.Result)
	}
	if item.Meta == nil && item.Extra != nil {
		item.Meta = item.Extra
	}
	return item
}

func sanitizeDeviceDebugLogEntry(item model.DeviceDebugLogEntry) model.DeviceDebugLogEntry {
	item.Event = ""
	item.Result = ""
	item.Extra = nil
	return item
}

func normalizeDeviceDebugLogRow(raw string) model.DeviceDebugLogEntry {
	item, ok := decodeDeviceDebugLogRow(raw)
	return normalizeDecodedDeviceDebugLogRow(raw, item, ok)
}

func normalizeDeviceDebugLogRows(rows []string) []model.DeviceDebugLogEntry {
	list := make([]model.DeviceDebugLogEntry, 0, len(rows))
	for _, raw := range rows {
		list = append(list, normalizeDeviceDebugLogRow(raw))
	}
	return list
}

func normalizeDeviceDebugLogEntry(item model.DeviceDebugLogEntry) model.DeviceDebugLogEntry {
	// 先做历史字段升级，再执行脱敏/裁剪，避免旧数据绕过统一清洗逻辑。
	return sanitizeDeviceDebugLogEntry(upgradeHistoricalDeviceDebugLogEntry(item))
}
