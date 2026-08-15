// 文件用途：验证公开文件路径解析不会逃逸 `./files` 根目录。
// 核心逻辑：检查正常子路径的相对结果，并用空路径、`..`、绝对路径、反斜杠和盘符输入覆盖拒绝分支。
// 关键注意事项：测试关注解析函数本身，不覆盖 Gin 路由参数解码后的所有输入形态。
// 重构建议：后续可补充 URL 编码和符号链接场景，提升文件访问安全回归覆盖。
package publicfiles

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathStaysInsideFilesDirectory(t *testing.T) {
	got, err := ResolvePath("/avatars/user.png")
	if err != nil {
		t.Fatalf("resolve public file path: %v", err)
	}
	base, err := filepath.Abs("./files")
	if err != nil {
		t.Fatalf("resolve files base: %v", err)
	}
	rel, err := filepath.Rel(base, got)
	if err != nil {
		t.Fatalf("path should be relative to base: %v", err)
	}
	if rel != filepath.Join("avatars", "user.png") {
		t.Fatalf("unexpected relative path %q", rel)
	}
}

func TestResolvePathRejectsEscapes(t *testing.T) {
	for _, rawPath := range []string{
		"",
		"/",
		"/../secret.txt",
		"../secret.txt",
		filepath.Clean(filepath.Join(string(filepath.Separator), "tmp", "secret.txt")),
		`/tmp\secret.txt`,
		"/C:/secret.txt",
	} {
		t.Run(strings.ReplaceAll(rawPath, string(filepath.Separator), "_"), func(t *testing.T) {
			if _, err := ResolvePath(rawPath); err == nil {
				t.Fatalf("expected %q to be rejected", rawPath)
			}
		})
	}
}
