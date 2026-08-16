// 文件用途：覆盖 file 工具函数的 Go 测试。
// 核心逻辑：通过表驱动或边界用例验证通用工具的输入校验、格式转换和错误返回，主要围绕 func TestFileExistReportsExistingAndMissingPaths、func TestCheckPathRejectsTraversalSeparatorsAndDotSegments、func TestCheckFilenameAllowsSingleExtensionAndRejectsTraversalFilenames、func TestValidateFileTypeAppliesDefaultUpgradeImportAndPluginPolicies 等声明展开。
// 关键注意事项：工具包被多处业务代码复用，测试断言需保持跨调用方的兼容契约。
// 重构建议：后续可按工具类别拆分公共夹具，并补充失败路径和异常输入覆盖。

package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileExistReportsExistingAndMissingPaths(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "upload.csv")
	if err := os.WriteFile(filePath, []byte("device_id,name\n1,pump\n"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if !FileExistInRoot(dir, "upload.csv") {
		t.Fatalf("FileExistInRoot(%q, %q) = false, want true", dir, "upload.csv")
	}
	if FileExistInRoot(dir, "missing.csv") {
		t.Fatal("FileExistInRoot returned true for a missing file")
	}
	if FileExistInRoot(dir, "../upload.csv") {
		t.Fatal("FileExistInRoot accepted a traversal path")
	}
}

func TestCheckPathRejectsTraversalSeparatorsAndDotSegments(t *testing.T) {
	accepted := []string{"tenant_uploads", "device123", "safe-name_001"}
	for _, input := range accepted {
		if err := CheckPath(input); err != nil {
			t.Fatalf("CheckPath(%q) returned error: %v", input, err)
		}
	}

	rejected := []string{"../secret", "tenant/uploads", `tenant\uploads`, "tenant.uploads", "."}
	for _, input := range rejected {
		if err := CheckPath(input); err == nil {
			t.Fatalf("CheckPath(%q) returned nil, want rejection", input)
		}
	}
}

func TestCheckFilenameAllowsSingleExtensionAndRejectsTraversalFilenames(t *testing.T) {
	accepted := []string{"logo.png", "firmware", "device_upload_001.csv"}
	for _, input := range accepted {
		if err := CheckFilename(input); err != nil {
			t.Fatalf("CheckFilename(%q) returned error: %v", input, err)
		}
	}

	rejected := []string{"archive.tar.gz", "../logo.png", `folder\logo.png`, "a.b.c"}
	for _, input := range rejected {
		if err := CheckFilename(input); err == nil {
			t.Fatalf("CheckFilename(%q) returned nil, want rejection", input)
		}
	}
}

func TestValidateFileTypeAppliesDefaultUpgradeImportAndPluginPolicies(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		fileType string
		want     bool
	}{
		{name: "default image uppercase extension", filename: "LOGO.PNG", fileType: "", want: true},
		{name: "default office upload", filename: "devices.xlsx", fileType: "", want: true},
		{name: "default 3d model upload", filename: "pump-model.GLB", fileType: "", want: true},
		{name: "default rejects executable", filename: "payload.exe", fileType: "", want: false},
		{name: "upgrade package accepts bin", filename: "firmware.BIN", fileType: "upgradePackage", want: true},
		{name: "upgrade package accepts apk", filename: "app.apk", fileType: "upgradePackage", want: true},
		{name: "upgrade package rejects csv", filename: "devices.csv", fileType: "upgradePackage", want: false},
		{name: "import batch accepts csv", filename: "devices.CSV", fileType: "importBatch", want: true},
		{name: "import batch rejects firmware", filename: "firmware.bin", fileType: "importBatch", want: false},
		{name: "device plugin bypasses extension restriction", filename: "collector.plugin", fileType: "d_plugin", want: true},
		{name: "missing extension is rejected by default", filename: "README", fileType: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidateFileType(tc.filename, tc.fileType); got != tc.want {
				t.Fatalf("ValidateFileType(%q, %q) = %v, want %v", tc.filename, tc.fileType, got, tc.want)
			}
		})
	}
}

func TestValidateFileExtensionMatchesCaseInsensitiveExtensionsWithoutDot(t *testing.T) {
	if !ValidateFileExtension("devices.CSV", []string{"csv", "xlsx"}) {
		t.Fatal("ValidateFileExtension rejected uppercase csv extension")
	}
	if !ValidateFileExtension("firmware.bin", []string{"BIN"}) {
		t.Fatal("ValidateFileExtension rejected allowed extension with different case")
	}
	if ValidateFileExtension("archive.tar.gz", []string{"tar"}) {
		t.Fatal("ValidateFileExtension accepted the wrong final extension")
	}
	if ValidateFileExtension("README", []string{"txt"}) {
		t.Fatal("ValidateFileExtension accepted a filename without extension")
	}
}

func TestSanitizeFilenameRemovesPathsDangerousCharactersHiddenPrefixAndReservedNames(t *testing.T) {
	if got := SanitizeFilename(`..\..\CON.PNG`); got != "_CON.png" {
		t.Fatalf("SanitizeFilename reserved path = %q, want _CON.png", got)
	}
	if got := SanitizeFilename(".profile.txt"); got != "_.profile.txt" {
		t.Fatalf("SanitizeFilename hidden file = %q, want _.profile.txt", got)
	}
	if got := SanitizeFilename("report 2026/06.csv"); got != "06.csv" {
		t.Fatalf("SanitizeFilename path basename = %q, want 06.csv", got)
	}

	longName := strings.Repeat("a", 250) + ".CSV"
	got := SanitizeFilename(longName)
	if len(strings.TrimSuffix(got, ".csv")) != 200 {
		t.Fatalf("SanitizeFilename long base length = %d, want 200", len(strings.TrimSuffix(got, ".csv")))
	}
	if !strings.HasSuffix(got, ".csv") {
		t.Fatalf("SanitizeFilename long extension = %q, want .csv suffix", got)
	}
}

func TestFileSignReturnsMD5AndSHA256Digests(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "payload.txt")
	content := []byte("AetherLink IoT file signing payload")
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	md5Sum := md5.Sum(content)
	gotMD5, err := FileSignInRoot(dir, "payload.txt", "MD5")
	if err != nil {
		t.Fatalf("FileSign MD5 returned error: %v", err)
	}
	if gotMD5 != hex.EncodeToString(md5Sum[:]) {
		t.Fatalf("FileSign MD5 = %q, want %q", gotMD5, hex.EncodeToString(md5Sum[:]))
	}

	shaSum := sha256.Sum256(content)
	gotSHA, err := FileSignInRoot(dir, "payload.txt", "SHA256")
	if err != nil {
		t.Fatalf("FileSign SHA256 returned error: %v", err)
	}
	if gotSHA != hex.EncodeToString(shaSum[:]) {
		t.Fatalf("FileSign SHA256 = %q, want %q", gotSHA, hex.EncodeToString(shaSum[:]))
	}
	if len(gotSHA) != 64 || gotSHA != strings.ToLower(gotSHA) {
		t.Fatalf("FileSign SHA256 format = %q, want 64-character lowercase hex", gotSHA)
	}
	if gotSHA == gotMD5 {
		t.Fatal("FileSign SHA256 matched MD5 digest, want distinct SHA256 digest")
	}

	if _, err := FileSignInRoot(dir, "missing.txt", "MD5"); err == nil {
		t.Fatal("FileSign returned nil error for missing file")
	}
	if _, err := FileSignInRoot(dir, "../payload.txt", "MD5"); err == nil {
		t.Fatal("FileSignInRoot accepted a traversal path")
	}
}
