// 文件用途：预注册 CSV 导入校验的定向证据（ROADMAP P0.5）。
// 覆盖：路径穿越/绝对路径/非 csv 被拒、严格表头校验、单元格空白裁剪、逐行坏行反馈的错误码与行号。
// 逐行坏行反馈的行构造在触库之前完成，故此处可以真实断言，不谎称已验证。
package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
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

// ---------------------------------------------------------------------------
// 逐行坏行反馈：错误码与行号（ROADMAP P0.5 剩余任务 1）
//
// 背景：坏行错误此前复用全站共享的 100005，其模板只插值 ${field}，
// 把调用方给出的 message 与 csv_row 一起丢掉，渲染成「batch_file不能为空」，
// 把排查者系统性地指向错误方向。现改用能承载行号与原因的 100006 / 100007。
// ---------------------------------------------------------------------------

func writeCSV(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}
}

// requireErrcode 断言 err 是指定错误码的 *errcode.Error，并回传以便继续断言变量。
func requireErrcode(t *testing.T, err error, wantCode int) *errcode.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected errcode %d, got nil", wantCode)
	}
	var coded *errcode.Error
	if !errors.As(err, &coded) {
		t.Fatalf("expected *errcode.Error, got %T: %v", err, err)
	}
	if coded.Code != wantCode {
		t.Fatalf("code = %d, want %d (variables=%v)", coded.Code, wantCode, coded.Variables)
	}
	return coded
}

func TestBuildFilePreRegisterRowsReportsCsvRowOnMissingField(t *testing.T) {
	path := withImportBatchDir(t)
	writeCSV(t, path, "device_number,name\nbad-row-1,\n")
	batchFile := path

	_, _, err := buildFilePreRegisterRows(model.CreateDevicePreRegisterReq{BatchFile: &batchFile}, "tenant-1")

	// 表头算第 1 行，因此第一条数据行是第 2 行。
	coded := requireErrcode(t, err, 100006)
	if got := coded.Variables["csv_row"]; got != 2 {
		t.Fatalf("csv_row = %v, want 2", got)
	}
	msg, _ := coded.Variables["message"].(string)
	if !strings.Contains(msg, "required") {
		t.Fatalf("message = %q, want it to carry the real reason", msg)
	}
}

func TestBuildFilePreRegisterRowsCsvRowFollowsDataIndex(t *testing.T) {
	path := withImportBatchDir(t)
	writeCSV(t, path, "device_number,name\nok-1,OK One\nbad-2,\n")
	batchFile := path

	_, _, err := buildFilePreRegisterRows(model.CreateDevicePreRegisterReq{BatchFile: &batchFile}, "tenant-1")

	coded := requireErrcode(t, err, 100006)
	if got := coded.Variables["csv_row"]; got != 3 {
		t.Fatalf("csv_row = %v, want 3", got)
	}
}

func TestBuildFilePreRegisterRowsReportsHeaderMismatchWithActualHeader(t *testing.T) {
	path := withImportBatchDir(t)
	writeCSV(t, path, "sn,name\nD1,N1\n")
	batchFile := path

	_, _, err := buildFilePreRegisterRows(model.CreateDevicePreRegisterReq{BatchFile: &batchFile}, "tenant-1")

	coded := requireErrcode(t, err, 100007)
	msg, _ := coded.Variables["message"].(string)
	if !strings.Contains(msg, "sn,name") {
		t.Fatalf("message = %q, want it to echo the actual header", msg)
	}
}

// 回归护栏：坏行错误绝不能退回 100005——它的模板只插值 ${field}，
// 会把行号与真实原因一起吞掉。这条断言是本次修复的防回退锁。
func TestBuildFilePreRegisterRowsNeverUsesGenericEmptyFieldCode(t *testing.T) {
	path := withImportBatchDir(t)
	writeCSV(t, path, "device_number,name\nbad-row-1,\n")
	batchFile := path

	_, _, err := buildFilePreRegisterRows(model.CreateDevicePreRegisterReq{BatchFile: &batchFile}, "tenant-1")

	var coded *errcode.Error
	if !errors.As(err, &coded) {
		t.Fatalf("expected *errcode.Error, got %T", err)
	}
	if coded.Code == 100005 {
		t.Fatal("row-level CSV error must not use 100005: its template only interpolates ${field} and drops csv_row/message")
	}
}
