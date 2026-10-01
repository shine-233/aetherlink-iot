// 文件用途：覆盖邮箱验证码 / 换绑邮箱 / 告警邮箱拆分后抽出的纯函数与 Redis 失败计数路径。
// 核心逻辑：miniredis 驱动验证码 key 契约、失败上限作废、候选邮箱回退；纯函数覆盖 owner 解析回退规则。
package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
)

func useEmailTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	oldRedis := global.REDIS
	global.REDIS = client
	t.Cleanup(func() {
		global.REDIS = oldRedis
		_ = client.Close()
	})
	return server
}

func TestVerificationCodeRedisKeyContract(t *testing.T) {
	// 这些 key 形状是线上 Redis 数据契约（注册/找回密码/换绑共用），拆分后不得漂移。
	cases := map[string]string{
		verificationCodeKey("a@example.com"):            "a@example.com_code",
		changeEmailVerificationCodeKey("a@example.com"): "a@example.com_code",
		verificationCodeSendLimitKey("a@example.com"):   "email:a@example.com:code_send_limit",
		verificationCodeAttemptsKey("a@example.com"):    "email:a@example.com:code_attempts",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("redis key = %q, want %q", got, want)
		}
	}
}

func TestVerificationCodeRecipientStateError(t *testing.T) {
	if err := verificationCodeRecipientStateError(true, false); err != nil {
		t.Fatalf("existing user, non-register: err = %v, want nil", err)
	}
	if err := verificationCodeRecipientStateError(false, true); err != nil {
		t.Fatalf("new user, register: err = %v, want nil", err)
	}
	assertErrcodeError(t, verificationCodeRecipientStateError(false, false), "missing user non-register", errcode.CodeEmailNotFound, "")
	assertErrcodeError(t, verificationCodeRecipientStateError(true, true), "registered user register", 200008, "")
}

func TestVerificationCodesEqual(t *testing.T) {
	if !verificationCodesEqual("123456", "123456") {
		t.Fatal("identical codes should match")
	}
	for _, provided := range []string{"123457", "12345", "", "1234567"} {
		if verificationCodesEqual("123456", provided) {
			t.Fatalf("code %q should not match", provided)
		}
	}
}

func TestAcquireVerificationCodeSendSlotRateLimits(t *testing.T) {
	server := useEmailTestRedis(t)
	ctx := context.Background()
	if err := acquireVerificationCodeSendSlot(ctx, "a@example.com"); err != nil {
		t.Fatalf("first send slot: %v", err)
	}
	assertErrcodeError(t, acquireVerificationCodeSendSlot(ctx, "a@example.com"), "second send within interval", errcode.CodeRateLimit, "")
	if err := acquireVerificationCodeSendSlot(ctx, "b@example.com"); err != nil {
		t.Fatalf("other email must not share the limit: %v", err)
	}
	server.FastForward(verificationCodeSendInterval + time.Second)
	if err := acquireVerificationCodeSendSlot(ctx, "a@example.com"); err != nil {
		t.Fatalf("send slot after interval: %v", err)
	}
}

func TestIssueVerificationCodeStoresCodeAndResetsAttempts(t *testing.T) {
	server := useEmailTestRedis(t)
	ctx := context.Background()
	_ = server.Set(verificationCodeAttemptsKey("a@example.com"), "4")

	code, err := issueVerificationCode(ctx, "a@example.com")
	if err != nil {
		t.Fatalf("issueVerificationCode: %v", err)
	}
	if len(code) != verificationCodeLength {
		t.Fatalf("code length = %d, want %d", len(code), verificationCodeLength)
	}
	stored, err := server.Get(verificationCodeKey("a@example.com"))
	if err != nil || stored != code {
		t.Fatalf("stored code = %q (%v), want %q", stored, err, code)
	}
	if ttl := server.TTL(verificationCodeKey("a@example.com")); ttl != verificationCodeValidityTTL {
		t.Fatalf("code ttl = %v, want %v", ttl, verificationCodeValidityTTL)
	}
	if server.Exists(verificationCodeAttemptsKey("a@example.com")) {
		t.Fatal("attempt counter should be reset when a new code is issued")
	}
}

func TestRegisterVerificationCodeFailureInvalidatesCodeAtLimit(t *testing.T) {
	server := useEmailTestRedis(t)
	ctx := context.Background()
	email := "a@example.com"
	_ = server.Set(verificationCodeKey(email), "123456")

	for i := 1; i < verificationCodeMaxAttempts; i++ {
		registerVerificationCodeFailure(ctx, email)
		if err := ensureVerificationCodeAttemptsAllowed(ctx, email); err != nil {
			t.Fatalf("attempt %d should still be allowed: %v", i, err)
		}
		if !server.Exists(verificationCodeKey(email)) {
			t.Fatalf("code invalidated too early at attempt %d", i)
		}
	}
	if ttl := server.TTL(verificationCodeAttemptsKey(email)); ttl != verificationCodeValidityTTL {
		t.Fatalf("attempt counter ttl = %v, want %v", ttl, verificationCodeValidityTTL)
	}

	registerVerificationCodeFailure(ctx, email)
	if server.Exists(verificationCodeKey(email)) {
		t.Fatal("code should be invalidated once the attempt limit is reached")
	}
	assertErrcodeError(t, ensureVerificationCodeAttemptsAllowed(ctx, email), "attempts exhausted", 200011, "")
}

func TestVerifyChangeEmailCodeCandidates(t *testing.T) {
	server := useEmailTestRedis(t)
	ctx := context.Background()

	// 没有任何验证码 → 200011
	_, err := verifyChangeEmailCode(ctx, "new@example.com", "Old@Example.com", "123456")
	assertErrcodeError(t, err, "no code stored", 200011, "")

	// 兼容旧流程：验证码发到当前邮箱（按小写归一化查找）
	_ = server.Set(verificationCodeKey("old@example.com"), "654321")
	matched, err := verifyChangeEmailCode(ctx, "new@example.com", " Old@Example.com ", " 654321 ")
	if err != nil || matched != "old@example.com" {
		t.Fatalf("current-email code: matched=%q err=%v", matched, err)
	}

	// 新邮箱验证码优先；错误码计入新邮箱的失败次数
	_ = server.Set(verificationCodeKey("new@example.com"), "111111")
	_, err = verifyChangeEmailCode(ctx, "new@example.com", "", "999999")
	assertErrcodeError(t, err, "wrong code", 200012, "")
	if got, _ := server.Get(verificationCodeAttemptsKey("new@example.com")); got != "1" {
		t.Fatalf("new email attempts = %q, want 1", got)
	}
	matched, err = verifyChangeEmailCode(ctx, "new@example.com", "old@example.com", "111111")
	if err != nil || matched != "new@example.com" {
		t.Fatalf("new-email code: matched=%q err=%v", matched, err)
	}

	// 失败次数超限的候选直接拒绝，不再比对
	_ = server.Set(verificationCodeAttemptsKey("new@example.com"), "5")
	_, err = verifyChangeEmailCode(ctx, "new@example.com", "old@example.com", "111111")
	assertErrcodeError(t, err, "attempts exhausted", 200011, "")
}

func TestClearChangeEmailVerificationCodeAndMigrateLoginToken(t *testing.T) {
	server := useEmailTestRedis(t)
	ctx := context.Background()
	_ = server.Set(verificationCodeKey("new@example.com"), "111111")
	_ = server.Set(loginEmailTokenKey("old@example.com"), "token-1")

	clearChangeEmailVerificationCode(ctx, "")
	if !server.Exists(verificationCodeKey("new@example.com")) {
		t.Fatal("blank matched email must not delete anything")
	}
	clearChangeEmailVerificationCode(ctx, "new@example.com")
	if server.Exists(verificationCodeKey("new@example.com")) {
		t.Fatal("matched code should be cleared")
	}

	migrateChangeEmailLoginToken(ctx, "old@example.com", "new@example.com")
	if server.Exists(loginEmailTokenKey("old@example.com")) {
		t.Fatal("old login token should be removed")
	}
	if got, _ := server.Get(loginEmailTokenKey("new@example.com")); got != "token-1" {
		t.Fatalf("new login token = %q, want token-1", got)
	}
	if ttl := server.TTL(loginEmailTokenKey("new@example.com")); ttl != changeEmailLoginTokenTTL {
		t.Fatalf("new login token ttl = %v, want %v", ttl, changeEmailLoginTokenTTL)
	}
}

func TestValidateChangeEmailRequest(t *testing.T) {
	_, err := validateChangeEmailRequest(nil, "a@example.com")
	assertErrcodeError(t, err, "nil request", errcode.CodeParamError, "change email request is required")
	_, err = validateChangeEmailRequest(&model.ChangeEmailReq{NewEmail: "  "}, "a@example.com")
	assertErrcodeError(t, err, "blank new email", errcode.CodeParamError, "new_email is required")
	_, err = validateChangeEmailRequest(&model.ChangeEmailReq{NewEmail: " A@Example.com "}, "a@example.com")
	assertErrcodeError(t, err, "same email", errcode.CodeParamError, "new email must be different from current email")
	got, err := validateChangeEmailRequest(&model.ChangeEmailReq{NewEmail: " New@Example.com "}, "a@example.com")
	if err != nil || got != "new@example.com" {
		t.Fatalf("normalized new email = %q (%v)", got, err)
	}
}

func TestChangeEmailUserUpdatesRenamesOnlyEmailDerivedNames(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cases := []struct {
		name     *string
		wantName bool
	}{
		{name: nil, wantName: true},
		{name: StringPtr(" old@example.com "), wantName: true},
		{name: StringPtr("OLD@example.com"), wantName: true},
		{name: StringPtr("Alice"), wantName: false},
	}
	for _, tc := range cases {
		updates := changeEmailUserUpdates(&model.User{Email: "old@example.com", Name: tc.name}, "new@example.com", now)
		if updates["email"] != "new@example.com" || updates["updated_at"] != now {
			t.Fatalf("updates = %#v", updates)
		}
		_, hasName := updates["name"]
		if hasName != tc.wantName {
			t.Fatalf("name %v: has name update = %v, want %v", tc.name, hasName, tc.wantName)
		}
	}
}

func TestHasUserClaimsIdentity(t *testing.T) {
	if hasUserClaimsIdentity(nil) || hasUserClaimsIdentity(&utils.UserClaims{ID: "  "}) {
		t.Fatal("nil/blank claims must not carry identity")
	}
	if !hasUserClaimsIdentity(&utils.UserClaims{ID: "user-1"}) {
		t.Fatal("claims with id should carry identity")
	}
}

func TestNormalizeWarningEmailDeviceIDs(t *testing.T) {
	got := normalizeWarningEmailDeviceIDs([]string{" d1 ", "", "d2", "d1", "  "})
	if want := []string{"d1", "d2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized ids = %#v, want %#v", got, want)
	}
}

func TestResolveSingleDeviceOwnerUserID(t *testing.T) {
	device := func(tenant string, owner *string) *model.Device {
		return &model.Device{TenantID: tenant, OwnerUserID: owner}
	}
	cases := []struct {
		name    string
		ids     []string
		devices map[string]*model.Device
		want    string
		wantOK  bool
	}{
		{"single owner", []string{"d1", "d2"}, map[string]*model.Device{"d1": device("t1", StringPtr("u1")), "d2": device("t1", StringPtr(" u1 "))}, "u1", true},
		{"no ids", nil, map[string]*model.Device{}, "", false},
		{"missing device", []string{"d1", "d2"}, map[string]*model.Device{"d1": device("t1", StringPtr("u1"))}, "", false},
		{"nil device", []string{"d1"}, map[string]*model.Device{"d1": nil}, "", false},
		{"cross tenant", []string{"d1"}, map[string]*model.Device{"d1": device("t2", StringPtr("u1"))}, "", false},
		{"no owner", []string{"d1"}, map[string]*model.Device{"d1": device("t1", nil)}, "", false},
		{"blank owner", []string{"d1"}, map[string]*model.Device{"d1": device("t1", StringPtr("  "))}, "", false},
		{"multiple owners", []string{"d1", "d2"}, map[string]*model.Device{"d1": device("t1", StringPtr("u1")), "d2": device("t1", StringPtr("u2"))}, "", false},
	}
	for _, tc := range cases {
		got, ok := resolveSingleDeviceOwnerUserID("t1", tc.ids, tc.devices)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("%s: got (%q, %v), want (%q, %v)", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestMergeWarningEmailsIntoAdditionalInfoPreservesOtherKeys(t *testing.T) {
	raw := StringPtr(`{"theme":"dark","warning_emails":["old@example.com"]}`)
	merged, err := mergeWarningEmailsIntoAdditionalInfo(raw, []string{"a@example.com"})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(merged), &decoded); err != nil {
		t.Fatalf("decode merged: %v", err)
	}
	if decoded["theme"] != "dark" {
		t.Fatalf("other keys lost: %#v", decoded)
	}
	if got := warningEmailsFromAdditionalInfo(&merged); !reflect.DeepEqual(got, []string{"a@example.com"}) {
		t.Fatalf("round-trip warning emails = %#v", got)
	}

	merged, err = mergeWarningEmailsIntoAdditionalInfo(StringPtr("not-json"), []string{})
	if err != nil || merged != `{"warning_emails":[]}` {
		t.Fatalf("invalid json base: merged=%q err=%v", merged, err)
	}
}
