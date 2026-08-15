// 文件用途：验证设备列表的 opt-in RDI 安装摘要字段与最小披露边界。
// 核心输入：devices.additional_info 中的 rdi_system_info，包含正式字段与旧 extra_fields。
// 主要副作用：无；测试只构造内存数据并序列化响应 DTO。
// 维护注意：Overview 新增安装字段时，先判断是否确属卡片必需信息，再同步摘要白名单。
package service

import (
	"encoding/json"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestRDISystemInfoSummaryUsesInstallationFieldsWithoutLeakingPrivateMetadata(t *testing.T) {
	raw := `{
		"rdi_system_info": {
			"installation_location": "Plant A",
			"maintenance_technician": "Tech A",
			"customer_name": "Private Customer",
			"contact_email": "customer@example.com",
			"contact_phone": "+1 555 0199",
			"warranty_status": "private warranty notes",
			"extra_fields": {
				"address": "1 Industrial Road",
				"installation_date": "2026-07-19",
				"installer_company": "Installer Co",
				"installer_contact": "Installer Desk",
				"installer_name": "Alex",
				"installer_phone": "+1 555 0100",
				"installer_email": "installer@example.com",
				"controller_serial_number": "RDI-SN-019",
				"private_note": "must not be returned"
			}
		}
	}`

	summary := rdiSystemInfoSummaryFromAdditionalInfo(&raw)
	if summary.InstallationLocation != "Plant A" ||
		summary.Address != "1 Industrial Road" ||
		summary.InstallationDate != "2026-07-19" ||
		summary.InstallerCompany != "Installer Co" ||
		summary.InstallerContact != "Installer Desk" ||
		summary.InstallerName != "Alex" ||
		summary.InstallerPhone != "+1 555 0100" ||
		summary.InstallerEmail != "installer@example.com" ||
		summary.ControllerSerialNumber != "RDI-SN-019" ||
		summary.MaintenanceTechnician != "Tech A" {
		t.Fatalf("unexpected RDI system info summary: %#v", summary)
	}

	encoded, err := json.Marshal(model.GetDeviceListByPageRsp{RDISystemInfoSummary: &summary})
	if err != nil {
		t.Fatalf("marshal device list row: %v", err)
	}
	body := string(encoded)
	if !strings.Contains(body, `"rdi_system_info_summary"`) {
		t.Fatalf("opt-in summary is missing from device list row: %s", body)
	}
	for _, excluded := range []string{
		"Private Customer",
		"customer@example.com",
		"+1 555 0199",
		"private warranty notes",
		"private_note",
		"extra_fields",
	} {
		if strings.Contains(body, excluded) {
			t.Fatalf("device list summary leaked %q: %s", excluded, body)
		}
	}

	defaultEncoded, err := json.Marshal(model.GetDeviceListByPageRsp{})
	if err != nil {
		t.Fatalf("marshal default device list row: %v", err)
	}
	if strings.Contains(string(defaultEncoded), `"rdi_system_info_summary"`) {
		t.Fatalf("default device list row must not expose the opt-in summary: %s", defaultEncoded)
	}
}
