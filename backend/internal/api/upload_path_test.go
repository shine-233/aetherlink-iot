// 文件用途：覆盖上传路径测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFilePathRejectsTraversalFileTypes(t *testing.T) {
	for _, fileType := range []string{"..", "../upgradePackage", `..\upgradePackage`, "upgrade/Package", "upgrade.Package"} {
		t.Run(fileType, func(t *testing.T) {
			if _, _, err := generateFilePath(fileType, "firmware.bin"); err == nil {
				t.Fatalf("generateFilePath(%q) returned nil error, want traversal rejection", fileType)
			}
		})
	}
}

func TestGenerateFilePathCreatesPathUnderUploadBase(t *testing.T) {
	t.Chdir(t.TempDir())

	uploadDir, fileName, err := generateFilePath("upgradePackage", "firmware.bin")
	if err != nil {
		t.Fatalf("generateFilePath returned error: %v", err)
	}
	if filepath.Ext(fileName) != ".bin" {
		t.Fatalf("generated filename extension = %q, want .bin", filepath.Ext(fileName))
	}

	absBase, err := filepath.Abs(BaseUploadDir)
	if err != nil {
		t.Fatalf("resolve base upload dir: %v", err)
	}
	absUpload, err := filepath.Abs(uploadDir)
	if err != nil {
		t.Fatalf("resolve generated upload dir: %v", err)
	}
	rel, err := filepath.Rel(absBase, absUpload)
	if err != nil {
		t.Fatalf("relative generated upload dir: %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		t.Fatalf("generated upload dir %q escapes base %q", absUpload, absBase)
	}
}

func TestEnsureUploadPathContainedRejectsEscapes(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := ensureUploadPathContained(filepath.Join(BaseUploadDir, "upgradePackage", "file.bin")); err != nil {
		t.Fatalf("ensureUploadPathContained rejected safe path: %v", err)
	}

	for _, fullPath := range []string{
		filepath.Join(BaseUploadDir, "..", "secret.bin"),
		filepath.Join("..", "files", "secret.bin"),
		filepath.Join("outside", "secret.bin"),
	} {
		t.Run(fullPath, func(t *testing.T) {
			if err := ensureUploadPathContained(fullPath); err == nil {
				t.Fatalf("ensureUploadPathContained(%q) returned nil error, want escape rejection", fullPath)
			}
		})
	}
}
