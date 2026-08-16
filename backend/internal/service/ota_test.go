// 文件用途：验证 OTA 包管理和升级数据转换规则。
// 核心逻辑：构造 OTA 包、设备升级参数和 JSON 字段，断言版本、路径和状态映射。
// 关键注意事项：OTA 测试关系设备升级安全，需覆盖坏包、版本冲突和无权限设备。
// 重构建议：拆出文件存储和升级调度接口，补齐事务回滚、外部文件副作用和设备状态边界。
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/stretchr/testify/assert"
)

func otaTestStringPtr(value string) *string {
	return &value
}

// ---------- rdiOTANumber ----------

func TestOTA_rdiOTANumber(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		want   float64
		wantOk bool
	}{
		{"int", int(42), 42, true},
		{"int8", int8(8), 8, true},
		{"int16", int16(16), 16, true},
		{"int32", int32(32), 32, true},
		{"int64", int64(64), 64, true},
		{"uint", uint(42), 42, true},
		{"uint8", uint8(8), 8, true},
		{"uint16", uint16(16), 16, true},
		{"uint32", uint32(32), 32, true},
		{"uint64", uint64(64), 64, true},
		{"float32", float32(3.14), float64(float32(3.14)), true},
		{"float64", float64(2.718), 2.718, true},
		{"json.Number valid", json.Number("123"), 123, true},
		{"json.Number invalid", json.Number("abc"), 0, false},
		{"string number", "42", 42, true},
		{"string float", "3.14", 3.14, true},
		{"string with spaces", "  7  ", 7, true},
		{"string invalid", "abc", 0, false},
		{"bool (unsupported)", true, 0, false},
		{"nil (unsupported)", nil, 0, false},
		{"slice (unsupported)", []int{1}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := rdiOTANumber(tt.input)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.InDelta(t, tt.want, got, 0.001)
			}
		})
	}
}

// ---------- rdiOTAProgressFromParams ----------

func TestOTA_rdiOTAProgressFromParams(t *testing.T) {
	tests := []struct {
		name      string
		params    map[string]interface{}
		wantValue int16
		wantFound bool
	}{
		{"progress key", map[string]interface{}{"progress": 50}, 50, true},
		{"step key", map[string]interface{}{"step": 30}, 30, true},
		{"steps key", map[string]interface{}{"steps": 80}, 80, true},
		{"percent key", map[string]interface{}{"percent": 75}, 75, true},
		{"percentage key", map[string]interface{}{"percentage": 90}, 90, true},
		{"first key wins", map[string]interface{}{"progress": 50, "step": 30}, 50, true},
		{"clamp negative", map[string]interface{}{"progress": -10}, 0, true},
		{"clamp over 100", map[string]interface{}{"progress": 150}, 100, true},
		{"float rounded", map[string]interface{}{"progress": 49.6}, 50, true},
		{"string number", map[string]interface{}{"progress": "60"}, 60, true},
		{"invalid string ignored", map[string]interface{}{"progress": "abc"}, 0, false},
		{"no progress key", map[string]interface{}{"other": 1}, 0, false},
		{"nil params", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := rdiOTAProgressFromParams(tt.params)
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.Equal(t, tt.wantValue, got)
			}
		})
	}
}

// ---------- rdiOTAStatusValue ----------

func TestOTA_rdiOTAStatusValue(t *testing.T) {
	tests := []struct {
		name        string
		value       interface{}
		progress    int16
		hasProgress bool
		wantStatus  int16
		wantOk      bool
	}{
		// 数值状态码 1-6
		{"status 1", 1, 0, false, 1, true},
		{"status 2", 2, 0, false, 2, true},
		{"status 3", 3, 0, false, 3, true},
		{"status 4", 4, 0, false, 4, true},
		{"status 5", 5, 0, false, 5, true},
		{"status 6", 6, 0, false, 6, true},
		{"status 0 with progress >= 100", 0, 100, true, 4, true},
		{"status 0 with progress > 0", 0, 50, true, 3, true},
		{"status 0 without progress", 0, 0, false, 0, false},
		{"status 7 out of range", 7, 0, false, 0, false},
		{"status -1 out of range", -1, 0, false, 0, false},

		// 文本状态 - 排队
		{"queued", "queued", 0, false, 1, true},
		{"pending", "pending", 0, false, 1, true},
		{"waiting", "waiting", 0, false, 1, true},
		{"wait", "wait", 0, false, 1, true},

		// 文本状态 - 已推送
		{"pushed", "pushed", 0, false, 2, true},
		{"notified", "notified", 0, false, 2, true},
		{"issued", "issued", 0, false, 2, true},
		{"sent", "sent", 0, false, 2, true},

		// 文本状态 - 升级中
		{"upgrading", "upgrading", 0, false, 3, true},
		{"upgrade", "upgrade", 0, false, 3, true},
		{"in_progress", "in_progress", 0, false, 3, true},
		{"inprogress", "inprogress", 0, false, 3, true},
		{"progress", "progress", 0, false, 3, true},
		{"downloading", "downloading", 0, false, 3, true},
		{"downloaded", "downloaded", 0, false, 3, true},
		{"installing", "installing", 0, false, 3, true},
		{"running", "running", 0, false, 3, true},

		// 文本状态 - 成功
		{"success", "success", 0, false, 4, true},
		{"succeeded", "succeeded", 0, false, 4, true},
		{"done", "done", 0, false, 4, true},
		{"completed", "completed", 0, false, 4, true},
		{"complete", "complete", 0, false, 4, true},
		{"finished", "finished", 0, false, 4, true},
		{"finish", "finish", 0, false, 4, true},
		{"ok", "ok", 0, false, 4, true},

		// 文本状态 - 失败
		{"fail", "fail", 0, false, 5, true},
		{"failed", "failed", 0, false, 5, true},
		{"error", "error", 0, false, 5, true},
		{"timeout", "timeout", 0, false, 5, true},
		{"aborted", "aborted", 0, false, 5, true},

		// 文本状态 - 取消
		{"cancel", "cancel", 0, false, 6, true},
		{"canceled", "canceled", 0, false, 6, true},
		{"cancelled", "cancelled", 0, false, 6, true},

		// 文本格式化处理
		{"hyphen replaced", "in-progress", 0, false, 3, true},
		{"space replaced", "in progress", 0, false, 3, true},
		{"case insensitive", "SUCCESS", 0, false, 4, true},
		{"mixed case", "Failed", 0, false, 5, true},

		// 未知文本
		{"unknown text", "unknown", 0, false, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := rdiOTAStatusValue(tt.value, tt.progress, tt.hasProgress)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.wantStatus, got)
			}
		})
	}
}

// ---------- rdiOTAStatusFromParams ----------

func TestOTA_rdiOTAStatusFromParams(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		progress    int16
		hasProgress bool
		wantStatus  int16
		wantFound   bool
	}{
		{"status key numeric", map[string]interface{}{"status": 4}, 0, false, 4, true},
		{"state key text", map[string]interface{}{"state": "success"}, 0, false, 4, true},
		{"result key", map[string]interface{}{"result": "failed"}, 0, false, 5, true},
		{"code key", map[string]interface{}{"code": 3}, 0, false, 3, true},
		{"progress 100 implies success", nil, 100, true, 4, true},
		{"progress 50 implies upgrading", nil, 50, true, 3, true},
		{"progress 0 with hasProgress", nil, 0, true, 0, false},
		{"no status and no progress", map[string]interface{}{"other": 1}, 0, false, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := rdiOTAStatusFromParams(tt.params, tt.progress, tt.hasProgress)
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.Equal(t, tt.wantStatus, got)
			}
		})
	}
}

// ---------- rdiOTAStatusDescription ----------

func TestOTA_rdiOTAStatusDescription(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		status      int16
		hasStatus   bool
		progress    int16
		hasProgress bool
		want        string
	}{
		{"from status_description key", map[string]interface{}{"status_description": "自定义描述"}, 3, true, 0, false, "自定义描述"},
		{"from description key", map[string]interface{}{"description": "升级中"}, 3, true, 0, false, "升级中"},
		{"from message key", map[string]interface{}{"message": "下载完成"}, 3, true, 0, false, "下载完成"},
		{"from error key", map[string]interface{}{"error": "校验失败"}, 5, true, 0, false, "校验失败"},
		{"empty description key falls back", map[string]interface{}{"description": "  "}, 3, true, 0, false, "OTA upgrading"},
		{"status 1 label", nil, 1, true, 0, false, "OTA pending push"},
		{"status 2 label", nil, 2, true, 0, false, "OTA pushed"},
		{"status 3 label", nil, 3, true, 0, false, "OTA upgrading"},
		{"status 4 label", nil, 4, true, 0, false, "OTA upgrade succeeded"},
		{"status 5 label", nil, 5, true, 0, false, "OTA upgrade failed"},
		{"status 6 label", nil, 6, true, 0, false, "OTA canceled"},
		{"unknown status label", nil, 99, true, 0, false, "OTA status updated"},
		{"with progress", nil, 3, true, 50, true, "OTA upgrading, progress 50%"},
		{"no status no progress", nil, 0, false, 0, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rdiOTAStatusDescription(tt.params, tt.status, tt.hasStatus, tt.progress, tt.hasProgress)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------- rdiOTAVersionFromParams ----------

func TestOTA_rdiOTAVersionFromParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{"version key", map[string]interface{}{"version": "1.0.3"}, "1.0.3"},
		{"firmware_version key", map[string]interface{}{"firmware_version": "2.1.0"}, "2.1.0"},
		{"current_version key", map[string]interface{}{"current_version": "3.0.0"}, "3.0.0"},
		{"target_version key", map[string]interface{}{"target_version": "4.0.0"}, "4.0.0"},
		{"first key wins", map[string]interface{}{"version": "1.0", "firmware_version": "2.0"}, "1.0"},
		{"empty string ignored", map[string]interface{}{"version": "  "}, ""},
		{"no version key", map[string]interface{}{"other": "1.0"}, ""},
		{"nil params", nil, ""},
		{"non-string value", map[string]interface{}{"version": 123}, "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rdiOTAVersionFromParams(tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------- buildOTAUpgradeMessagePayload ----------

func TestOTA_buildOTAUpgradeMessagePayload(t *testing.T) {
	payload, err := buildOTAUpgradeMessagePayload("123456789", map[string]interface{}{
		"url": "https://example.com/fw.bin",
	})

	assert.NoError(t, err)

	var got map[string]interface{}
	assert.NoError(t, json.Unmarshal(payload, &got))
	assert.Equal(t, "123456789", got["id"])
	assert.Equal(t, "200", got["code"])
	assert.Equal(t, map[string]interface{}{
		"url": "https://example.com/fw.bin",
	}, got["params"])
}

// ---------- buildOTAUpgradeMessageParams ----------

func TestOTA_buildOTAUpgradeMessageParams_CompatibilityAliases(t *testing.T) {
	oldOtaAddress := global.OtaAddress
	global.OtaAddress = "https://ota.example"
	t.Cleanup(func() {
		global.OtaAddress = oldOtaAddress
	})

	firmware := []byte("signed firmware payload")
	firmwareDir := filepath.Join("files", "upgradePackage")
	assert.NoError(t, os.MkdirAll(firmwareDir, 0o755))
	firmwarePath := filepath.Join(firmwareDir, "ota-integrity-compatibility-test.bin")
	assert.NoError(t, os.WriteFile(firmwarePath, firmware, 0o600))
	t.Cleanup(func() { _ = os.Remove(firmwarePath) })
	digest := sha256.Sum256(firmware)
	signature := hex.EncodeToString(digest[:])

	pkg := &model.OtaUpgradePackage{
		Version:        "1.2.3",
		PackageURL:     otaTestStringPtr("./files/upgradePackage/ota-integrity-compatibility-test.bin"),
		SignatureType:  otaTestStringPtr("SHA256"),
		Signature:      &signature,
		AdditionalInfo: otaTestStringPtr(`{"retry":3}`),
		Module:         otaTestStringPtr("main"),
	}
	params, err := buildOTAUpgradeMessageParams(pkg)

	assert.NoError(t, err)
	assert.Equal(t, "1.2.3", params["version"])
	assert.Equal(t, "0", params["size"])
	assert.Equal(t, "https://ota.example/files/upgradePackage/ota-integrity-compatibility-test.bin", params["url"])
	assert.Equal(t, params["url"], params["firmware_url"])
	assert.Equal(t, params["signMethod"], params["sign_method"])
	assert.Equal(t, "SHA256", *params["signMethod"].(*string))
	assert.Equal(t, signature, params["sign"])
	assert.Equal(t, params["sign"], params["signature"])
	assert.Equal(t, map[string]interface{}{"retry": float64(3)}, params["extData"])
	assert.Equal(t, params["extData"], params["ext_data"])
	assert.Equal(t, "main", *params["module"].(*string))

	assert.NoError(t, os.WriteFile(firmwarePath, []byte("tampered firmware payload"), 0o600))
	_, err = buildOTAUpgradeMessageParams(pkg)
	assert.EqualError(t, err, "ota package integrity verification failed")
}

// ---------- otaPackageLocalPathFromURL ----------

func TestOTA_otaPackageLocalPathFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"path traversal ../", "/api/v1/ota/download/files/upgradePackage/../../../etc/passwd", true},
		{"path traversal .. segment", "/api/v1/ota/download/files/upgradePackage/../../secret", true},
		{"dot segment", "/api/v1/ota/download/files/upgradePackage/.", true},
		{"double dot segment", "/api/v1/ota/download/files/upgradePackage/..", true},
		{"valid simple filename", "/api/v1/ota/download/files/upgradePackage/firmware.bin", false},
		{"valid with subdirectory", "/api/v1/ota/download/files/upgradePackage/v1/firmware.bin", false},
		{"api prefix without leading slash", "api/v1/ota/download/files/upgradePackage/firmware.bin", false},
		{"files/upgradePackage marker", "files/upgradePackage/firmware.bin", false},
		{"/files/upgradePackage marker", "/files/upgradePackage/firmware.bin", false},
		{"backslash converted", `\api\v1\ota\download\files\upgradePackage\firmware.bin`, false},
		{"full URL with host", "http://example.com/api/v1/ota/download/files/upgradePackage/fw.bin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := otaPackageLocalPathFromURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOTA_otaPackageLocalPathFromURL_ValidPath(t *testing.T) {
	path, err := otaPackageLocalPathFromURL("/api/v1/ota/download/files/upgradePackage/v1/firmware.bin")
	assert.NoError(t, err)
	assert.Contains(t, path, filepath.Join("files", "upgradePackage"))
	assert.Contains(t, path, "v1")
	assert.Contains(t, path, "firmware.bin")
}
