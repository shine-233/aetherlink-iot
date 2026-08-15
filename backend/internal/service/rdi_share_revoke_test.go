// rdi_share_revoke_test.go 锁定“设备拥有者主动撤销分享”的权限口径与状态变更规则。
//
// Purpose: prove that only owners/admins may revoke, that a share recipient cannot revoke,
// and that a revoked recipient really loses the read gate used by shared-device access.
// Core logic: exercises the pure authorization helper and the additional_info share-state
// mutators with explicit fixtures, so no PostgreSQL/Redis instance is required.
// Important notes: the DB-touching entry points (RevokeShareToken / RevokeSharedDeviceRecipient)
// cannot run here; these tests cover the decision and state layers those entry points call.
// Refactor suggestion: extend the tables below when new revoke targets or roles appear.
package service

import (
	"encoding/json"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// rdiRevokeTestDevice 构造一台带分享状态的设备，owner 与 recipient 可分别指定。
func rdiRevokeTestDevice(t *testing.T, tenantID string, ownerUserID string, tokens []model.RDIShareTokenRecord, recipients []model.RDIShareRecipientRecord) *model.Device {
	t.Helper()
	additional := map[string]interface{}{}
	if tokens != nil {
		additional[rdiShareTokensKey] = tokens
	}
	if recipients != nil {
		additional[rdiShareRecipientsKey] = recipients
	}
	raw, err := json.Marshal(additional)
	if err != nil {
		t.Fatalf("marshal additional info: %v", err)
	}
	encoded := string(raw)
	device := &model.Device{ID: "device-a", TenantID: tenantID, AdditionalInfo: &encoded}
	if ownerUserID != "" {
		owner := ownerUserID
		device.OwnerUserID = &owner
	}
	return device
}

// TestAssertRDIShareRevokeAccessAllowsOwnerAndAdminsOnly 覆盖撤销入口的权限矩阵。
// 关键用例：仅仅是分享接收人的 TENANT_USER 必须被拒绝（fail closed）。
func TestAssertRDIShareRevokeAccessAllowsOwnerAndAdminsOnly(t *testing.T) {
	const (
		tenantID  = "tenant-a"
		ownerID   = "user-owner"
		otherID   = "user-other"
		recipient = "user-recipient"
	)
	recipients := []model.RDIShareRecipientRecord{
		{UserID: recipient, TenantID: tenantID, TokenHash: "hash-a", AcceptedAt: 100},
	}

	tests := []struct {
		name      string
		claims    *utils.UserClaims
		wantAllow bool
	}{
		{
			name:      "device owner may revoke",
			claims:    &utils.UserClaims{ID: ownerID, TenantID: tenantID, Authority: constant.TENANT_USER},
			wantAllow: true,
		},
		{
			name:      "same tenant admin may revoke",
			claims:    &utils.UserClaims{ID: otherID, TenantID: tenantID, Authority: constant.TENANT_ADMIN},
			wantAllow: true,
		},
		{
			name:      "system admin may revoke",
			claims:    &utils.UserClaims{ID: otherID, TenantID: "tenant-other", Authority: constant.SYS_ADMIN},
			wantAllow: true,
		},
		{
			name:      "share recipient must not revoke",
			claims:    &utils.UserClaims{ID: recipient, TenantID: tenantID, Authority: constant.TENANT_USER},
			wantAllow: false,
		},
		{
			name:      "same tenant non owner user must not revoke",
			claims:    &utils.UserClaims{ID: otherID, TenantID: tenantID, Authority: constant.TENANT_USER},
			wantAllow: false,
		},
		{
			name:      "foreign tenant admin must not revoke",
			claims:    &utils.UserClaims{ID: otherID, TenantID: "tenant-b", Authority: constant.TENANT_ADMIN},
			wantAllow: false,
		},
		{
			name:      "missing claims must not revoke",
			claims:    nil,
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := rdiRevokeTestDevice(t, tenantID, ownerID, nil, recipients)
			err := assertRDIShareRevokeAccess(device, tt.claims)
			if tt.wantAllow {
				if err != nil {
					t.Fatalf("assertRDIShareRevokeAccess() error = %v, want nil", err)
				}
				return
			}
			rdiTestRequireError(t, err, "assertRDIShareRevokeAccess()", errcode.CodeNoPermission, "")
		})
	}
}

// TestRDIShareStateRemoveTokenAlsoDropsRecipients 证明撤销 token 会连带清除其接收人，
// 避免出现“token 已撤销但接收人仍保有访问”的窗口。
func TestRDIShareStateRemoveTokenAlsoDropsRecipients(t *testing.T) {
	now := time.Now().UTC().Unix()
	state := newRDIShareState(map[string]interface{}{
		rdiShareTokensKey: []model.RDIShareTokenRecord{
			{TokenHash: "hash-a", ExpiresAt: now + 600},
			{TokenHash: "hash-b", ExpiresAt: now + 600},
			{TokenHash: "hash-expired", ExpiresAt: now - 1},
		},
		rdiShareRecipientsKey: []model.RDIShareRecipientRecord{
			{UserID: "user-a", TokenHash: "hash-a", AcceptedAt: now},
			{UserID: "user-b", TokenHash: "hash-b", AcceptedAt: now},
		},
	})

	removed, changed := state.RemoveToken("hash-a", now)
	if !removed || !changed {
		t.Fatalf("RemoveToken(hash-a) = (%t, %t), want (true, true)", removed, changed)
	}
	removedRecipients := state.RemoveRecipientsByToken("hash-a")
	if len(removedRecipients) != 1 || removedRecipients[0].UserID != "user-a" {
		t.Fatalf("RemoveRecipientsByToken(hash-a) = %#v, want only user-a", removedRecipients)
	}

	if state.HasActiveToken("hash-a", now) {
		t.Fatal("revoked token is still active")
	}
	if !state.HasActiveToken("hash-b", now) {
		t.Fatal("unrelated token must stay active after revocation")
	}
	// 过期 token 应被顺带裁剪，避免 additional_info 长期堆积。
	if tokens := state.Tokens(); len(tokens) != 1 || tokens[0].TokenHash != "hash-b" {
		t.Fatalf("Tokens() after revoke = %#v, want only hash-b", tokens)
	}
	if _, ok := state.FindRecipient("user-b"); !ok {
		t.Fatal("unrelated recipient must survive revocation")
	}
}

// TestRDIShareStateRemoveTokenMissingTokenReportsNoChange 确认撤销不存在的 token 不会误报成功。
func TestRDIShareStateRemoveTokenMissingTokenReportsNoChange(t *testing.T) {
	now := time.Now().UTC().Unix()
	state := newRDIShareState(map[string]interface{}{
		rdiShareTokensKey: []model.RDIShareTokenRecord{
			{TokenHash: "hash-a", ExpiresAt: now + 600},
		},
	})

	removed, changed := state.RemoveToken("hash-missing", now)
	if removed || changed {
		t.Fatalf("RemoveToken(missing) = (%t, %t), want (false, false)", removed, changed)
	}
	if got := state.RemoveRecipientsByToken("hash-missing"); got != nil {
		t.Fatalf("RemoveRecipientsByToken(missing) = %#v, want nil", got)
	}
}

// TestRDIShareStateRemoveRecipientKeepsTokenUsable 覆盖“只回收某个接收人”的语义：
// 该接收人被移除，但 token 仍然有效，拥有者可以继续分享给其他人。
// TestRDIShareStateRemoveTokenUnknownTargetDoesNotCountExpiredCleanupAsMatch
// keeps the revoke service from treating unrelated expiry cleanup as a hit for
// an unknown token. RemoveToken intentionally reports changed=true when it
// prunes stale records, but removed must remain false for the requested token.
func TestRDIShareStateRemoveTokenUnknownTargetDoesNotCountExpiredCleanupAsMatch(t *testing.T) {
	now := time.Now().UTC().Unix()
	state := newRDIShareState(map[string]interface{}{
		rdiShareTokensKey: []model.RDIShareTokenRecord{
			{TokenHash: "hash-expired", ExpiresAt: now - 1},
			{TokenHash: "hash-live", ExpiresAt: now + 600},
		},
	})

	removed, changed := state.RemoveToken("hash-missing", now)
	if removed {
		t.Fatal("RemoveToken(missing) reported a removal after pruning an unrelated expired token")
	}
	if !changed {
		t.Fatal("RemoveToken(missing) did not report the expected stale-record cleanup")
	}
	if tokens := state.Tokens(); len(tokens) != 1 || tokens[0].TokenHash != "hash-live" {
		t.Fatalf("Tokens() after unrelated cleanup = %#v, want only hash-live", tokens)
	}
}

func TestRDIShareStateRemoveRecipientKeepsTokenUsable(t *testing.T) {
	now := time.Now().UTC().Unix()
	state := newRDIShareState(map[string]interface{}{
		rdiShareTokensKey: []model.RDIShareTokenRecord{
			{TokenHash: "hash-a", ExpiresAt: now + 600},
		},
		rdiShareRecipientsKey: []model.RDIShareRecipientRecord{
			{UserID: "user-a", TokenHash: "hash-a", AcceptedAt: now},
			{UserID: "user-b", TokenHash: "hash-a", AcceptedAt: now},
		},
	})

	removedRecord, found := state.RemoveRecipient("user-a")
	if !found || removedRecord.UserID != "user-a" {
		t.Fatalf("RemoveRecipient(user-a) = (%#v, %t), want user-a found", removedRecord, found)
	}
	if _, ok := state.FindRecipient("user-a"); ok {
		t.Fatal("revoked recipient is still present")
	}
	if _, ok := state.FindRecipient("user-b"); !ok {
		t.Fatal("unrelated recipient must survive recipient revocation")
	}
	if !state.HasActiveToken("hash-a", now) {
		t.Fatal("recipient revocation must not invalidate the share token")
	}

	if _, found := state.RemoveRecipient("user-missing"); found {
		t.Fatal("RemoveRecipient(missing) reported a removal")
	}
	if _, found := state.RemoveRecipient(""); found {
		t.Fatal("RemoveRecipient(empty) reported a removal")
	}
}

// TestRevokedRecipientLosesSharedReadAccess 证明撤销后接收人真的失去读权限：
// rdiShareRecipientForUser 与 hasTelemetryTenantAccess(allowSharedRead=true) 都不再放行。
func TestRevokedRecipientLosesSharedReadAccess(t *testing.T) {
	const (
		tenantID    = "tenant-a"
		ownerID     = "user-owner"
		recipientID = "user-recipient"
	)
	now := time.Now().UTC().Unix()
	recipientClaims := &utils.UserClaims{ID: recipientID, TenantID: tenantID, Authority: constant.TENANT_USER}

	device := rdiRevokeTestDevice(t, tenantID, ownerID,
		[]model.RDIShareTokenRecord{{TokenHash: "hash-a", ExpiresAt: now + 600}},
		[]model.RDIShareRecipientRecord{{UserID: recipientID, TenantID: tenantID, TokenHash: "hash-a", AcceptedAt: now}},
	)

	// 撤销前：接收人可读（allowSharedRead=true），但不具备写/撤销权限。
	if _, ok := rdiShareRecipientForUser(device, recipientClaims); !ok {
		t.Fatal("recipient should be readable before revocation")
	}
	if !hasTelemetryTenantAccess(device, recipientClaims, true) {
		t.Fatal("recipient should have shared read access before revocation")
	}
	if hasTelemetryTenantAccess(device, recipientClaims, false) {
		t.Fatal("recipient must never gain write access")
	}

	// 模拟撤销：在设备行锁内执行的状态变更，这里直接对 additional_info 做同样的变更。
	state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
	if _, changed := state.RemoveToken("hash-a", now); !changed {
		t.Fatal("RemoveToken did not change share state")
	}
	if got := state.RemoveRecipientsByToken("hash-a"); len(got) != 1 {
		t.Fatalf("RemoveRecipientsByToken removed %d recipients, want 1", len(got))
	}
	revoked, err := json.Marshal(state.additional)
	if err != nil {
		t.Fatalf("marshal revoked additional info: %v", err)
	}
	revokedInfo := string(revoked)
	device.AdditionalInfo = &revokedInfo

	// 撤销后：接收人既不在接收人列表里，也拿不到共享读权限。
	if _, ok := rdiShareRecipientForUser(device, recipientClaims); ok {
		t.Fatal("revoked recipient is still resolvable")
	}
	if hasTelemetryTenantAccess(device, recipientClaims, true) {
		t.Fatal("revoked recipient still has shared read access")
	}
	if rdiShareTokenActive(parseAdditionalInfo(device.AdditionalInfo), "hash-a", now) {
		t.Fatal("revoked token is still active for anonymous share reads")
	}
}

func TestRevokeSharedDeviceRecipientLockedRejectsUnauthorizedCaller(t *testing.T) {
	device := rdiRevokeTestDevice(t, "tenant-a", "user-owner", nil, []model.RDIShareRecipientRecord{
		{UserID: "user-recipient", TenantID: "tenant-a", AcceptedAt: 100},
	})

	result, err := revokeSharedDeviceRecipientLocked(nil, device, "user-recipient", &utils.UserClaims{
		ID:        "user-other",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	})
	if result != nil {
		t.Fatalf("revokeSharedDeviceRecipientLocked() result = %#v, want nil", result)
	}
	rdiTestRequireError(t, err, "revokeSharedDeviceRecipientLocked()", errcode.CodeNoPermission, "")
}

func TestRevokeSharedDeviceRecipientLockedRejectsMissingRecipient(t *testing.T) {
	device := rdiRevokeTestDevice(t, "tenant-a", "user-owner", nil, []model.RDIShareRecipientRecord{
		{UserID: "user-recipient", TenantID: "tenant-a", AcceptedAt: 100},
	})

	result, err := revokeSharedDeviceRecipientLocked(nil, device, "user-missing", &utils.UserClaims{
		ID:        "user-owner",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	})
	if result != nil {
		t.Fatalf("revokeSharedDeviceRecipientLocked() result = %#v, want nil", result)
	}
	rdiTestRequireError(t, err, "revokeSharedDeviceRecipientLocked()", errcode.CodeNotFound, "share recipient not found")
}

// TestNormalizeRDIShareRevokeTargetRejectsMissingInputs 覆盖撤销入口的前置参数校验。
func TestNormalizeRDIShareRevokeTargetRejectsMissingInputs(t *testing.T) {
	validClaims := &utils.UserClaims{ID: "user-owner", TenantID: "tenant-a", Authority: constant.TENANT_USER}

	tests := []struct {
		name        string
		deviceID    string
		claims      *utils.UserClaims
		wantCode    int
		wantMessage string
	}{
		{name: "missing claims", deviceID: "device-a", claims: nil, wantCode: errcode.CodeNoPermission},
		{name: "blank claim id", deviceID: "device-a", claims: &utils.UserClaims{ID: "  "}, wantCode: errcode.CodeNoPermission},
		{name: "blank device id", deviceID: "   ", claims: validClaims, wantCode: errcode.CodeParamError, wantMessage: "device id is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeRDIShareRevokeTarget(tt.deviceID, tt.claims)
			rdiTestRequireError(t, err, "normalizeRDIShareRevokeTarget()", tt.wantCode, tt.wantMessage)
		})
	}

	got, err := normalizeRDIShareRevokeTarget("  device-a  ", validClaims)
	if err != nil {
		t.Fatalf("normalizeRDIShareRevokeTarget() error = %v, want nil", err)
	}
	if got != "device-a" {
		t.Fatalf("normalizeRDIShareRevokeTarget() = %q, want trimmed device-a", got)
	}
}
