// 文件用途：验证 pidfile 包的 PID 文件创建、冲突检测和删除行为。
// 核心逻辑：使用临时目录创建 PID 文件，并检查 Remove 清理和重复创建保护。
// 关键注意事项：测试依赖当前平台 processExists 实现，避免使用真实生产路径。
// 重构建议：后续可增加损坏 PID、stale PID 和权限失败的表驱动用例。
package pidfile

import (
	"path/filepath"
	"testing"
)

func TestNewAndRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	file, err := New(path)
	if err != nil {
		t.Fatal("Could not create test file", err)
	}

	_, err = New(path)
	if err == nil {
		t.Fatal("Test file creation not blocked")
	}

	if err := file.Remove(); err != nil {
		t.Fatal("Could not delete created test file")
	}
}

func TestRemoveInvalidPath(t *testing.T) {
	file := PIDFile{path: filepath.Join("foo", "bar")}

	if err := file.Remove(); err == nil {
		t.Fatal("Non-existing file doesn't give an error on delete")
	}
}
