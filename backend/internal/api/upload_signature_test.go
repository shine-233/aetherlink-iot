// 文件用途：覆盖上传内容签名校验，防止仅修改扩展名绕过可识别格式的上传边界。
// 边界说明：CSV、裸固件、模型和插件等没有稳定通用魔数的格式继续交由后续业务解析器校验。
package api

import "testing"

func TestValidateContentSignatureAcceptsKnownFormats(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		header   []byte
	}{
		{name: "png", filename: "image.png", header: []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}},
		{name: "jpeg", filename: "image.jpg", header: []byte{0xFF, 0xD8, 0xFF, 0xE0}},
		{name: "zip", filename: "archive.zip", header: []byte("PK\x03\x04")},
		{name: "xlsx", filename: "sheet.xlsx", header: []byte("PK\x03\x04")},
		{name: "xls", filename: "sheet.xls", header: []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}},
		{name: "gzip", filename: "firmware.gz", header: []byte{0x1F, 0x8B, 0x08}},
		{name: "svg with bom", filename: "vector.svg", header: append([]byte{0xEF, 0xBB, 0xBF}, []byte("  <svg xmlns=\"http://www.w3.org/2000/svg\">")...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateContentSignature(tt.filename, "resource", tt.header); err != nil {
				t.Fatalf("validateContentSignature rejected valid %s content: %v", tt.name, err)
			}
		})
	}
}

func TestValidateContentSignatureRejectsDisguisedAndEmptyKnownFormats(t *testing.T) {
	for _, tt := range []struct {
		name     string
		filename string
		header   []byte
	}{
		{name: "text disguised as png", filename: "payload.png", header: []byte("plain text")},
		{name: "jpeg disguised as zip", filename: "payload.zip", header: []byte{0xFF, 0xD8, 0xFF}},
		{name: "empty png", filename: "empty.png", header: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateContentSignature(tt.filename, "resource", tt.header); err == nil {
				t.Fatalf("validateContentSignature accepted invalid content for %s", tt.filename)
			}
		})
	}
}

func TestValidateContentSignaturePreservesFormatsWithoutReliableMagic(t *testing.T) {
	for _, filename := range []string{"devices.csv", "firmware.bin", "model.json"} {
		t.Run(filename, func(t *testing.T) {
			if err := validateContentSignature(filename, "resource", nil); err != nil {
				t.Fatalf("validateContentSignature changed compatibility contract for %s: %v", filename, err)
			}
		})
	}

	if err := validateContentSignature("plugin.zip", "d_plugin", []byte("plugin-specific payload")); err != nil {
		t.Fatalf("plugin payload must remain delegated to the plugin parser: %v", err)
	}
}
