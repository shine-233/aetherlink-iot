// 文件用途：预注册脱敏导出与清理分流的定向证据（ROADMAP P0.5）。
// 覆盖：导出绝不泄漏明文凭证、表头与导入表头区分、导出顺序幂等、
// 跨租户 fail closed、已激活设备不可删、空批次清理幂等不报错。
// 说明：本文件为纯逻辑证据，不依赖数据库；服务层与 API 接线尚未落地，故不谎称端到端已验证。
package service

import (
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

// newPreRegisterExportFixture 造一台预注册设备；激活态与租户可按用例调整。
func newPreRegisterExportFixture(id, number, tenantID, activateFlag, voucher string) *model.Device {
	name := "fixture-" + number
	createdAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	return &model.Device{
		ID:           id,
		DeviceNumber: number,
		Name:         &name,
		Voucher:      voucher,
		TenantID:     tenantID,
		ActivateFlag: activateFlag,
		CreatedAt:    &createdAt,
	}
}

// plaintextVoucher 造一个真实的 voucher 形态，用于断言导出面不含其敏感片段。
func plaintextVoucher(secret string) string {
	return `{"username":"` + secret + `"}`
}

func TestPreRegisterExportNeverLeaksPlaintextVoucher(t *testing.T) {
	secret := "SUPERSECRETUSERNAME0001"
	devices := []*model.Device{
		newPreRegisterExportFixture("d1", "PR-0002", "t1", preRegisterActivateFlag, plaintextVoucher(secret)),
	}
	payload, err := encodePreRegisterExportCSV(buildPreRegisterExportRows(devices))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(string(payload), secret) {
		t.Fatalf("export leaked plaintext voucher secret: %s", string(payload))
	}
	if !strings.Contains(string(payload), "voucher_masked") {
		t.Fatalf("export missing voucher_masked column: %s", string(payload))
	}
}

func TestPreRegisterExportHeaderDiffersFromImportHeader(t *testing.T) {
	// 导入表头是严格的 device_number,name；导出若与之相同，导出文件会被误当导入回灌。
	header := strings.Join(preRegisterExportColumns(), ",")
	if header == "device_number,name" {
		t.Fatalf("export header must not equal import header: %s", header)
	}
	if len(preRegisterExportColumns()) < 3 {
		t.Fatalf("export header too short: %s", header)
	}
}

func TestBuildPreRegisterExportRowsIsDeterministic(t *testing.T) {
	first := []*model.Device{
		newPreRegisterExportFixture("d1", "PR-B", "t1", preRegisterActivateFlag, plaintextVoucher("bbb")),
		newPreRegisterExportFixture("d2", "PR-A", "t1", preRegisterActivateFlag, plaintextVoucher("aaa")),
	}
	second := []*model.Device{
		newPreRegisterExportFixture("d2", "PR-A", "t1", preRegisterActivateFlag, plaintextVoucher("aaa")),
		newPreRegisterExportFixture("d1", "PR-B", "t1", preRegisterActivateFlag, plaintextVoucher("bbb")),
	}
	left, err := encodePreRegisterExportCSV(buildPreRegisterExportRows(first))
	if err != nil {
		t.Fatalf("encode left: %v", err)
	}
	right, err := encodePreRegisterExportCSV(buildPreRegisterExportRows(second))
	if err != nil {
		t.Fatalf("encode right: %v", err)
	}
	if string(left) != string(right) {
		t.Fatalf("export not deterministic:\n%s\nvs\n%s", left, right)
	}
	if !strings.Contains(string(left), "PR-A,fixture-PR-A") {
		t.Fatalf("expected sorted output starting with PR-A: %s", left)
	}
}

func TestEncodePreRegisterExportCSVWithNoRows(t *testing.T) {
	payload, err := encodePreRegisterExportCSV(buildPreRegisterExportRows(nil))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(payload), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("empty export should contain header only, got %d lines: %s", len(lines), payload)
	}
}

func TestPreRegisterExportTimestampKeepsMissingTimeEmpty(t *testing.T) {
	device := newPreRegisterExportFixture("d1", "PR-A", "t1", preRegisterActivateFlag, plaintextVoucher("aaa"))
	device.CreatedAt = nil
	rows := buildPreRegisterExportRows([]*model.Device{device})
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0][4] != "" {
		t.Fatalf("missing created_at should export empty string, got %q", rows[0][4])
	}
}

func TestClassifyPreRegisterCleanupRejectsCrossTenant(t *testing.T) {
	devices := []*model.Device{
		newPreRegisterExportFixture("d1", "PR-A", "tenant-other", preRegisterActivateFlag, plaintextVoucher("aaa")),
	}
	plan, err := classifyPreRegisterCleanup(devices, "tenant-1")
	if err == nil {
		t.Fatalf("cross-tenant device must fail closed, got plan %+v", plan)
	}
	if plan != nil {
		t.Fatalf("plan must be nil on cross-tenant rejection, got %+v", plan)
	}
}

func TestClassifyPreRegisterCleanupBlocksActivatedDevices(t *testing.T) {
	devices := []*model.Device{
		newPreRegisterExportFixture("d1", "PR-A", "t1", preRegisterActivateFlag, plaintextVoucher("aaa")),
		newPreRegisterExportFixture("d2", "PR-B", "t1", "active", plaintextVoucher("bbb")),
	}
	plan, err := classifyPreRegisterCleanup(devices, "t1")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(plan.deletable) != 1 || plan.deletable[0] != "d1" {
		t.Fatalf("deletable = %v, want [d1]", plan.deletable)
	}
	if len(plan.blockedActivated) != 1 || plan.blockedActivated[0] != "PR-B" {
		t.Fatalf("blockedActivated = %v, want [PR-B]", plan.blockedActivated)
	}
}

func TestClassifyPreRegisterCleanupEmptyIsIdempotent(t *testing.T) {
	plan, err := classifyPreRegisterCleanup(nil, "t1")
	if err != nil {
		t.Fatalf("cleaning an empty batch must not error: %v", err)
	}
	if !plan.isEmpty() {
		t.Fatalf("empty batch plan should be empty: %+v", plan)
	}
}

func TestClassifyPreRegisterCleanupAllActivatedDeletesNothing(t *testing.T) {
	devices := []*model.Device{
		newPreRegisterExportFixture("d1", "PR-A", "t1", "active", plaintextVoucher("aaa")),
		newPreRegisterExportFixture("d2", "PR-B", "t1", "active", plaintextVoucher("bbb")),
	}
	plan, err := classifyPreRegisterCleanup(devices, "t1")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(plan.deletable) != 0 {
		t.Fatalf("activated devices must never be deletable, got %v", plan.deletable)
	}
	if len(plan.blockedActivated) != 2 {
		t.Fatalf("blockedActivated = %v, want 2 entries", plan.blockedActivated)
	}
}
