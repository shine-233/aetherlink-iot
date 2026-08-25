// 文件用途：验证密码重置验证码、一次性链接和外部副作用失败边界。
// 核心逻辑：通过内存 Redis adapter 与 SQLite 用户表调用公开 service 方法，覆盖二选一、邮箱匹配和重复消费。
// 关键注意事项：测试不依赖真实 SMTP/Redis，且不得放宽密码重置的 fail-closed 语义。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type memoryPasswordResetStore struct {
	values      map[string]string
	increments  map[string]int64
	setKeys     []string
	deletedKeys []string
	getDelKeys  []string
}

func newMemoryPasswordResetStore() *memoryPasswordResetStore {
	return &memoryPasswordResetStore{
		values:     map[string]string{},
		increments: map[string]int64{},
	}
}

func (store *memoryPasswordResetStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	store.values[key] = value
	store.setKeys = append(store.setKeys, key)
	return nil
}

func (store *memoryPasswordResetStore) Delete(_ context.Context, keys ...string) error {
	for _, key := range keys {
		delete(store.values, key)
		delete(store.increments, key)
		store.deletedKeys = append(store.deletedKeys, key)
	}
	return nil
}

func (store *memoryPasswordResetStore) Get(_ context.Context, key string) (string, error) {
	value, ok := store.values[key]
	if !ok {
		return "", redis.Nil
	}
	return value, nil
}

func (store *memoryPasswordResetStore) GetDel(_ context.Context, key string) (string, error) {
	store.getDelKeys = append(store.getDelKeys, key)
	value, ok := store.values[key]
	if !ok {
		return "", redis.Nil
	}
	delete(store.values, key)
	return value, nil
}

func (store *memoryPasswordResetStore) Increment(_ context.Context, key string) (int64, error) {
	store.increments[key]++
	return store.increments[key], nil
}

func (*memoryPasswordResetStore) Expire(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func TestResetPasswordRequiresExactlyOneVerificationMethod(t *testing.T) {
	store := newMemoryPasswordResetStore()
	user := &User{passwordResetStore: store}

	err := user.ResetPassword(context.Background(), &model.ResetPasswordReq{
		Email:      "user@example.com",
		Password:   "NewPassword1!",
		VerifyCode: "123456",
		ResetToken: "reset-token",
	})

	assertPasswordResetError(t, err, errcode.CodeParamError, "验证码和重置链接不能同时使用")
	if len(store.getDelKeys) != 0 {
		t.Fatalf("ambiguous request consumed reset token: %#v", store.getDelKeys)
	}
}

func TestResetPasswordSupportsVerificationCode(t *testing.T) {
	db := setupPasswordResetTestDB(t)
	seedPasswordResetUser(t, db, "user-code", "user@example.com")
	store := newMemoryPasswordResetStore()
	store.values[passwordResetCodeKey("user@example.com")] = "123456"
	user := &User{passwordResetStore: store}

	err := user.ResetPassword(context.Background(), &model.ResetPasswordReq{
		Email:      " USER@example.com ",
		Password:   "NewPassword1!",
		VerifyCode: "123456",
	})
	if err != nil {
		t.Fatalf("ResetPassword with verify_code failed: %v", err)
	}

	assertPasswordResetUpdated(t, db, "user@example.com", "NewPassword1!")
	if _, ok := store.values[passwordResetCodeKey("user@example.com")]; ok {
		t.Fatal("verification code was not cleared after password reset")
	}
}

func TestResetPasswordTokenEmailMismatchDoesNotConsumeToken(t *testing.T) {
	store := newMemoryPasswordResetStore()
	store.values[resetPasswordTokenKey("reset-token")] = "owner@example.com"
	user := &User{passwordResetStore: store}

	err := user.ResetPassword(context.Background(), &model.ResetPasswordReq{
		Email:      "other@example.com",
		Password:   "NewPassword1!",
		ResetToken: "reset-token",
	})

	assertPasswordResetError(t, err, errcode.CodeParamError, "重置链接与邮箱不匹配")
	if got := store.values[resetPasswordTokenKey("reset-token")]; got != "owner@example.com" {
		t.Fatalf("mismatched email consumed or changed reset token: %q", got)
	}
	if len(store.getDelKeys) != 0 {
		t.Fatalf("mismatched email reached token consumption: %#v", store.getDelKeys)
	}
}

func TestResetPasswordTokenCanOnlyBeConsumedOnce(t *testing.T) {
	db := setupPasswordResetTestDB(t)
	seedPasswordResetUser(t, db, "user-token", "user@example.com")
	store := newMemoryPasswordResetStore()
	store.values[resetPasswordTokenKey("reset-token")] = "user@example.com"
	user := &User{passwordResetStore: store}
	req := &model.ResetPasswordReq{
		Email:      "user@example.com",
		Password:   "NewPassword1!",
		ResetToken: "reset-token",
	}

	if err := user.ResetPassword(context.Background(), req); err != nil {
		t.Fatalf("first reset token consumption failed: %v", err)
	}
	assertPasswordResetUpdated(t, db, "user@example.com", "NewPassword1!")

	err := user.ResetPassword(context.Background(), req)
	assertPasswordResetError(t, err, errcode.CodeParamError, "重置链接已过期或无效")
	if len(store.getDelKeys) != 1 {
		t.Fatalf("reset token get-del calls = %d, want 1 successful consumption", len(store.getDelKeys))
	}
}

func TestRequestPasswordResetLinkKeepsVerificationCodeWhenEmailFails(t *testing.T) {
	store := newMemoryPasswordResetStore()
	codeKey := passwordResetCodeKey("user@example.com")
	store.values[codeKey] = "123456"
	user := &User{
		passwordResetStore: store,
		passwordResetEmailSender: func(string, string, string, ...string) error {
			return errors.New("smtp unavailable")
		},
	}

	_, err := user.RequestPasswordResetLink(context.Background(), &model.ResetPasswordLinkReq{
		Email:      "user@example.com",
		VerifyCode: "123456",
	})
	assertPasswordResetError(t, err, 200010, "")

	if got := store.values[codeKey]; got != "123456" {
		t.Fatalf("verification code after email failure = %q, want preserved", got)
	}
	for key := range store.values {
		if strings.HasPrefix(key, resetPasswordTokenKeyPrefix) {
			t.Fatalf("email failure left a usable reset token: %q", key)
		}
	}
	if len(store.setKeys) != 1 || !strings.HasPrefix(store.setKeys[0], resetPasswordTokenKeyPrefix) {
		t.Fatalf("reset token storage calls = %#v, want one generated token", store.setKeys)
	}
}

func setupPasswordResetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open password reset sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate password reset users: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

func seedPasswordResetUser(t *testing.T, db *gorm.DB, id, email string) {
	t.Helper()
	now := time.Now().UTC()
	name := id
	status := "N"
	authority := "TENANT_USER"
	tenantID := "tenant-a"
	oldHash, oldHashErr := utils.BcryptHash("OldPassword1!")
	if oldHashErr != nil {
		t.Fatalf("seed password reset user hash: %v", oldHashErr)
	}
	if err := db.Create(&model.User{
		ID:                  id,
		Name:                &name,
		PhoneNumber:         id + "-phone",
		Email:               email,
		Status:              &status,
		Authority:           &authority,
		Password:            oldHash,
		TenantID:            &tenantID,
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}).Error; err != nil {
		t.Fatalf("seed password reset user: %v", err)
	}
}

func assertPasswordResetUpdated(t *testing.T, db *gorm.DB, email, password string) {
	t.Helper()
	var user model.User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		t.Fatalf("read reset user: %v", err)
	}
	if !utils.BcryptCheck(password, user.Password) {
		t.Fatal("password hash was not updated to the requested password")
	}
	if user.PasswordLastUpdated == nil {
		t.Fatal("password_last_updated was not recorded")
	}
}

func assertPasswordResetError(t *testing.T, err error, code int, message string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected password reset error, got nil")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("password reset error type = %T, want *errcode.Error", err)
	}
	if appErr.Code != code {
		t.Fatalf("password reset error code = %d, want %d", appErr.Code, code)
	}
	if message != "" && appErr.CustomMsg != message {
		t.Fatalf("password reset error message = %q, want %q", appErr.CustomMsg, message)
	}
}
