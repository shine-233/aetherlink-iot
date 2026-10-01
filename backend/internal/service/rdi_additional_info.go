// 文件用途：RDI additional_info 解析与 system_info 归一化子聚合，从 rdi.go 拆出。
// 核心逻辑：把设备 additional_info 的 JSON 文本解析成 map，再投影为 RDISystemInfo，
// 并对入库前/出库后的字段做别名归一与 extra 字段提升。
// 关键注意事项：additional_info 是外部协议边界，坏 JSON 一律 fail-safe 返回空结构，
// 绝不把解析错误上抛成 500。
package service

import (
	"encoding/json"
	"strings"

	"aetherlink-iot/backend/internal/model"
)

func parseAdditionalInfo(info *string) map[string]interface{} {
	additional := map[string]interface{}{}
	if info == nil || strings.TrimSpace(*info) == "" {
		return additional
	}
	if err := json.Unmarshal([]byte(*info), &additional); err != nil {
		return map[string]interface{}{}
	}
	return additional
}

func systemInfoFromAdditionalInfo(additional map[string]interface{}) model.RDISystemInfo {
	info := model.RDISystemInfo{}
	if val, ok := additional[rdiSystemInfoKey]; ok {
		if bytes, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(bytes, &info)
		}
	}
	if info.ExtraFields == nil {
		info.ExtraFields = map[string]interface{}{}
	}
	promoteRDISystemInfoExtraFields(&info)
	return info
}

func normalizeRDISystemInfoForStorage(info model.RDISystemInfo) model.RDISystemInfo {
	if info.ExtraFields == nil {
		info.ExtraFields = map[string]interface{}{}
	}
	promoteRDISystemInfoExtraFields(&info)
	for _, key := range promotedRDISystemInfoExtraKeys {
		delete(info.ExtraFields, key)
	}
	return info
}

func promoteRDISystemInfoExtraFields(info *model.RDISystemInfo) {
	if info == nil || info.ExtraFields == nil {
		return
	}
	if info.Address == "" {
		info.Address = stringFromExtraField(info.ExtraFields, "address")
	}
	if info.InstallationDate == "" {
		info.InstallationDate = stringFromExtraField(info.ExtraFields, "installation_date")
	}
	if info.InstallerCompany == "" {
		info.InstallerCompany = stringFromExtraField(info.ExtraFields, "installer_company")
	}
	if info.InstallerContact == "" {
		info.InstallerContact = stringFromExtraField(info.ExtraFields, "installer_contact")
	}
	if info.InstallerName == "" {
		info.InstallerName = stringFromExtraField(info.ExtraFields, "installer_name")
	}
	if info.InstallerPhone == "" {
		info.InstallerPhone = stringFromExtraField(info.ExtraFields, "installer_phone")
	}
	if info.InstallerEmail == "" {
		info.InstallerEmail = stringFromExtraField(info.ExtraFields, "installer_email")
	}
	if info.ControllerSerialNumber == "" {
		info.ControllerSerialNumber = stringFromExtraField(info.ExtraFields, "controller_serial_number")
	}
}

func stringFromExtraField(fields map[string]interface{}, key string) string {
	if raw, ok := fields[key]; ok {
		if value, ok := raw.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readString(values map[string]interface{}, key string, fallback string) string {
	if raw, ok := values[key]; ok {
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return fallback
}
