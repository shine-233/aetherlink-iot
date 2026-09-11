// 文件用途：预注册 CSV 导入校验的定向证据（ROADMAP P0.5）。
// 覆盖：路径穿越/绝对路径/非 csv 被拒、严格表头校验、单元格空白裁剪。
// 逐行坏行反馈依赖数据库批量插入，故此处不谎称已验证。
package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withImportBatchDir 在临时目录内造出 importBatch/ 并切换工作目录，供路径白名单校验使用。
func withImportBatchDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, preRegisterImportSegment), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	return filepath.Join(preRegisterImportSegment, "batch.csv")
}

func TestReadPreRegisterCSVRejectsUnsafePaths(t *testing.T) {
	withImportBatchDir(t)
	cases := map[string]string{
		"absolute path":     filepath.Join(os.TempDir(), "importBatch", "x.csv"),
		"parent traversal":  "importBatch/../../../etc/passwd.csv",
		"outside allowlist": "uploads/other/x.csv",
		"not a csv":         "importBatch/x.txt",
		"no extension":      "importBatch/x",
	}
	for name, path := range cases {
		if _, err := readPreRegisterImportCSV(path); err == nil {
			t.Fatalf("%s (%s) must be rejected", name, path)
		}
	}
}

func TestReadPreRegisterCSVRequiresStrictHeader(t *testing.T) {
	path := withImportBatchDir(t)

	// 表头必须严格为 device_number,name（允许单元格空白）。
	write := func(body string) error {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err := readPreRegisterImportCSV(path)
		return err
	}

	if err := write("name,device_number\r\nA,1\r\n"); err == nil {
		t.Fatal("reordered header must be rejected")
	}
	if err := write("device_number\r\nA\r\n"); err == nil {
		t.Fatal("missing name column must be rejected")
	}
	if err := write(""); err == nil {
		t.Fatal("empty file must be rejected")
	}

	// 合法表头：空白被裁剪后应通过，并返回数据行。
	if err := os.WriteFile(path, []byte(" device_number , name \r\nD1,N1\r\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	rows, err := readPreRegisterImportCSV(path)
	if err != nil {
		t.Fatalf("valid header must be accepted: %v", err)
	}
	if len(rows) != 1 || trimCSVCells(rows[0])[0] != "D1" {
		t.Fatalf("unexpected data rows: %v", rows)
	}
}

func TestTrimCSVCellsTrimsWhitespaceOnly(t *testing.T) {
	got := trimCSVCells([]string{"  a  ", "\tb\t", "c d", " "})
	want := []string{"a", "b", "c d", ""}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cell %d = %q, want %q", i, got[i], want[i])
		}
	}
	// 内部空格必须保留，不得把 "c d" 压成 "cd"。
	if strings.Contains(trimCSVCells([]string{"c d"})[0], "cd") {
		t.Fatal("internal whitespace must be preserved")
	}
}
