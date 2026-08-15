// 文件用途：覆盖 validate lang jwt 工具函数的 Go 测试。
// 核心逻辑：通过表驱动或边界用例验证通用工具的输入校验、格式转换和错误返回，主要围绕 func TestValidateInputClassifiesAccountIdentifiers、func TestValidateEmailAcceptsOnlyCompleteEmailAddresses、func TestFormatLangCodeNormalizesAcceptLanguageHeader、func TestJWTGeneratesAndParsesUserClaims 等声明展开。
// 关键注意事项：工具包被多处业务代码复用，测试断言需保持跨调用方的兼容契约。
// 重构建议：后续可按工具类别拆分公共夹具，并补充失败路径和异常输入覆盖。

package utils

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestValidateInputClassifiesAccountIdentifiers(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		valid   bool
		kind    InputType
		message string
	}{
		{
			name:    "valid email trims whitespace",
			input:   " admin@example.com ",
			valid:   true,
			kind:    Email,
			message: "not a valid email",
		},
		{
			name:    "valid mainland phone",
			input:   "13800138000",
			valid:   true,
			kind:    Phone,
			message: "not a valid phone number",
		},
		{
			name:    "blank input",
			input:   "  ",
			valid:   false,
			kind:    "",
			message: "input cannot be empty",
		},
		{
			name:    "malformed email",
			input:   "admin@example",
			valid:   false,
			kind:    Email,
			message: "email format is incorrect",
		},
		{
			name:    "malformed numeric phone",
			input:   "12800138000",
			valid:   false,
			kind:    Phone,
			message: "phone number format is incorrect",
		},
		{
			name:    "unknown account input",
			input:   "operator",
			valid:   false,
			kind:    "",
			message: "input format is incorrect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateInput(tt.input)
			if got.IsValid != tt.valid || got.Type != tt.kind || got.Message != tt.message {
				t.Fatalf("ValidateInput(%q) = %+v, want valid=%v type=%q message=%q", tt.input, got, tt.valid, tt.kind, tt.message)
			}
		})
	}
}

func TestValidateEmailAcceptsOnlyCompleteEmailAddresses(t *testing.T) {
	valid := []string{"ops@example.com", "first.last+tag@example.co"}
	for _, email := range valid {
		if !ValidateEmail(email) {
			t.Fatalf("ValidateEmail(%q) = false, want true", email)
		}
	}

	invalid := []string{"", "ops@", "ops@example", "ops example@example.com"}
	for _, email := range invalid {
		if ValidateEmail(email) {
			t.Fatalf("ValidateEmail(%q) = true, want false", email)
		}
	}
}

func TestFormatLangCodeNormalizesAcceptLanguageHeader(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty header defaults to en_US", in: "", want: "en_US"},
		{name: "zh shorthand maps to zh_CN", in: "zh", want: "zh_CN"},
		{name: "en shorthand maps to en_US", in: "en", want: "en_US"},
		{name: "quality value is stripped", in: "zh-CN;q=0.9,en-US;q=0.8", want: "zh_CN"},
		{name: "five char locale keeps region", in: "fr-FR,zh-CN;q=0.8", want: "fr_FR"},
		{name: "unknown malformed language falls back to zh_CN", in: "malformed-language-code", want: "zh_CN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatLangCode(tt.in); got != tt.want {
				t.Fatalf("FormatLangCode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestJWTGeneratesAndParsesUserClaims(t *testing.T) {
	j := NewJWT([]byte("unit-test-secret"))
	createdAt := time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)
	claims := UserClaims{
		ID:         "user-1",
		Email:      "user@example.com",
		CreateTime: createdAt,
		Authority:  "SYS_ADMIN",
		TenantID:   "tenant-1",
	}

	token, err := j.GenerateToken(claims)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	parsed, err := j.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}

	if parsed.ID != claims.ID || parsed.Email != claims.Email || parsed.Authority != claims.Authority || parsed.TenantID != claims.TenantID {
		t.Fatalf("ParseToken claims = %+v, want %+v", parsed, claims)
	}
	if parsed.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("ParseToken ExpiresAt = %d, want a future expiration", parsed.ExpiresAt)
	}
	if !parsed.CreateTime.Equal(createdAt) {
		t.Fatalf("ParseToken CreateTime = %s, want %s", parsed.CreateTime, createdAt)
	}
}

func TestJWTRejectsWrongSecretMalformedAndExpiredTokens(t *testing.T) {
	j := NewJWT([]byte("correct-secret"))
	token, err := j.GenerateToken(UserClaims{ID: "user-1"})
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	if _, err = NewJWT([]byte("wrong-secret")).ParseToken(token); err == nil {
		t.Fatal("ParseToken with wrong secret expected error")
	}

	if _, err = j.ParseToken("not-a-token"); err == nil {
		t.Fatal("ParseToken with malformed token expected error")
	}

	hs384Token, err := jwt.NewWithClaims(jwt.SigningMethodHS384, UserClaims{
		ID: "wrong-algorithm-user",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Minute).Unix(),
		},
	}).SignedString([]byte("correct-secret"))
	if err != nil {
		t.Fatalf("SignedString HS384 token returned error: %v", err)
	}
	if _, err = j.ParseToken(hs384Token); err == nil {
		t.Fatal("ParseToken accepted HS384 token signed with the correct secret")
	}

	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		ID: "expired-user",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(-time.Minute).Unix(),
		},
	}).SignedString([]byte("correct-secret"))
	if err != nil {
		t.Fatalf("SignedString expired token returned error: %v", err)
	}

	_, err = j.ParseToken(expiredToken)
	if err == nil {
		t.Fatal("ParseToken expired token expected error")
	}
	if errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseToken expired token error = %v, want jwt validation error", err)
	}
}
