// 文件用途：2FA（ROADMAP C7）服务层——TOTP 绑定/解绑/登录第二因子/恢复码。
// 核心链路：登录密码正确且用户已启用 2FA 时签发短期挑战 ticket；第二因子端点
//   校验 TOTP（含 30s 步内防重放）或一次性恢复码（用后即废），通过后走既有 UserLoginAfter
//   签发正式会话。secret 以 AES-GCM 密文落库，密钥由 jwt.key 派生（未配置则拒绝启用）。
package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/totp"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/golang-jwt/jwt/v4"
	"github.com/go-basic/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// UserTotp 2FA 服务聚合入口。
type UserTotp struct{}

// TOTPSetupRsp 绑定准备响应（明文 secret 只出现这一次）。
type TOTPSetupRsp struct {
	Secret   string `json:"secret"`
	URI      string `json:"uri"`
	Account  string `json:"account"`
	Issuer   string `json:"issuer"`
	Enabled  bool   `json:"enabled"`
}

// RecoveryCodesRsp 激活后一次性返回恢复码明文。
type RecoveryCodesRsp struct {
	Codes []string `json:"codes"`
}

const (
	totpChallengeTTLMinutes = 5
	totpRecoveryCodeCount   = 8
)

// totpKeyMaterial 由 jwt.key 派生 AES-GCM 密钥；jwt.key 为空视为未配置。
func totpKeyMaterial() ([]byte, error) {
	key := strings.TrimSpace(viper.GetString("jwt.key"))
	if key == "" {
		return nil, errors.New("jwt.key not configured; 2FA activation unavailable")
	}
	sum := sha256.Sum256([]byte("aetherlink-totp-v1:" + key))
	return sum[:], nil
}

func aesGCMEncrypt(plain string) (string, error) {
	key, err := totpKeyMaterial()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func aesGCMDecrypt(cipherB64 string) (string, error) {
	key, err := totpKeyMaterial()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("bad ciphertext")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func hashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte("aetherlink-totp-recovery:" + code))
	return hex.EncodeToString(sum[:])
}

func generateRecoveryCodes(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		raw := make([]byte, 5)
		_, _ = rand.Read(raw)
		out = append(out, strings.ToUpper(fmt.Sprintf("%s-%s", hex.EncodeToString(raw[:2]), hex.EncodeToString(raw[2:]))))
	}
	return out
}

// totpEnabled 判断用户是否已启用 2FA。
func totpEnabled(userID string) (bool, error) {
	row, err := dal.GetUserTOTP(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return row.Enabled, nil
}

// Setup 生成一次性绑定材料（不落库）。
func (*UserTotp) Setup(claimsUserID, email string) (*TOTPSetupRsp, error) {
	enabled, err := totpEnabled(claimsUserID)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	if enabled {
		return nil, errcode.New(errcode.CodeTotpAlreadyEnabled)
	}
	secret, err := totp.GenerateSecret(20)
	if err != nil {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	issuer := "AetherLink"
	return &TOTPSetupRsp{
		Secret:  secret,
		URI:     totp.ProvisioningURI(issuer, email, secret),
		Account: email,
		Issuer:  issuer,
		Enabled: false,
	}, nil
}

// Activate 校验验证码后落库密文并发放一次性恢复码。
func (*UserTotp) Activate(claimsUserID, email, code string) (*RecoveryCodesRsp, error) {
	secret, err := totp.GenerateSecret(20)
	if err != nil {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	// 激活前先在内存里验证一遍，避免把无效码落库。
	if !totp.Validate(strings.TrimSpace(code), secret, 1, time.Now().UTC()) {
		return nil, errcode.New(errcode.CodeTotpInvalid)
	}
	cipherText, err := aesGCMEncrypt(secret)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "2FA key not configured")
	}
	if err := dal.SaveUserTOTPSecret(claimsUserID, cipherText); err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	codes := generateRecoveryCodes(totpRecoveryCodeCount)
	rows := make([]*model.UserTOTPRecoveryCode, 0, len(codes))
	for _, c := range codes {
		rows = append(rows, &model.UserTOTPRecoveryCode{
			ID:       uuid.New(),
			UserID:   claimsUserID,
			CodeHash: hashRecoveryCode(c),
		})
	}
	if err := dal.CreateUserTOTPRecoveryCodes(rows); err != nil {
		_ = dal.DisableUserTOTP(claimsUserID)
		return nil, errcode.New(errcode.CodeDBError)
	}
	_ = email
	return &RecoveryCodesRsp{Codes: codes}, nil
}

// Disable 解绑：需提供当前有效 TOTP 或一个未用恢复码。
func (*UserTotp) Disable(claimsUserID, code string) error {
	row, err := dal.GetUserTOTP(claimsUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.CodeTotpNotEnabled)
		}
		return errcode.New(errcode.CodeDBError)
	}
	if !row.Enabled {
		return errcode.New(errcode.CodeTotpNotEnabled)
	}
	plain, err := aesGCMDecrypt(row.SecretCipher)
	if err != nil {
		return errcode.New(errcode.CodeTotpInvalid)
	}
	if !totp.Validate(strings.TrimSpace(code), plain, 1, time.Now().UTC()) {
		used, err2 := dal.ConsumeUserTOTPRecoveryCode(claimsUserID, hashRecoveryCode(strings.TrimSpace(code)))
		if err2 != nil || !used {
			return errcode.New(errcode.CodeTotpInvalid)
		}
	}
	if err := dal.DisableUserTOTP(claimsUserID); err != nil {
		return errcode.New(errcode.CodeDBError)
	}
	return nil
}

// Status 查询绑定状态。
func (*UserTotp) Status(claimsUserID string) (bool, error) {
	return totpEnabled(claimsUserID)
}

// IssueChallenge 签发短期第二因子挑战（用户已通过密码校验）。
func (*UserTotp) IssueChallenge(userID string) (string, error) {
	key := strings.TrimSpace(viper.GetString("jwt.key"))
	if key == "" {
		return "", errcode.New(errcode.CodeTokenGenerateError)
	}
	claims := jwt.MapClaims{
		"uid":     userID,
		"purpose": "totp_challenge",
		"exp":     time.Now().UTC().Add(totpChallengeTTLMinutes * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(key))
}

func parseChallenge(ticket string) (string, error) {
	key := strings.TrimSpace(viper.GetString("jwt.key"))
	if key == "" {
		return "", errcode.New(errcode.CodeInvalidAuth)
	}
	parsed, err := jwt.Parse(ticket, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(key), nil
	})
	if err != nil || !parsed.Valid {
		return "", errcode.New(errcode.CodeInvalidAuth)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", errcode.New(errcode.CodeInvalidAuth)
	}
	if purpose, _ := claims["purpose"].(string); purpose != "totp_challenge" {
		return "", errcode.New(errcode.CodeInvalidAuth)
	}
	uid, _ := claims["uid"].(string)
	if uid == "" {
		return "", errcode.New(errcode.CodeInvalidAuth)
	}
	return uid, nil
}

// LoginWithSecondFactor 第二因子登录：TOTP 或恢复码任一通过即签发正式会话。
func (*UserTotp) LoginWithSecondFactor(ticket, code string) (*model.LoginRsp, error) {
	userID, err := parseChallenge(ticket)
	if err != nil {
		return nil, err
	}
	user, err := GroupApp.User.GetUserById(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.CodeInvalidAuth)
		}
		return nil, errcode.New(errcode.CodeDBError)
	}
	row, err := dal.GetUserTOTP(userID)
	if err != nil {
		return nil, errcode.New(errcode.CodeTotpNotEnabled)
	}
	if !row.Enabled {
		return nil, errcode.New(errcode.CodeTotpNotEnabled)
	}
	code = strings.TrimSpace(code)
	plain, err := aesGCMDecrypt(row.SecretCipher)
	if err != nil {
		return nil, errcode.New(errcode.CodeTotpInvalid)
	}
	now := time.Now().UTC()
	step := int64(totp.CounterAt(now))
	if row.LastUsedStep >= step {
		// 本窗口验证码已消费过（防重放）；仍放行恢复码路径。
	} else if totp.Validate(code, plain, 1, now) {
		if err := dal.SetTOTPLastUsedStep(userID, step); err != nil {
			return nil, errcode.New(errcode.CodeDBError)
		}
		return GroupApp.User.UserLoginAfter(user)
	}
	used, err := dal.ConsumeUserTOTPRecoveryCode(userID, hashRecoveryCode(code))
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	if !used {
		return nil, errcode.New(errcode.CodeTotpInvalid)
	}
	return GroupApp.User.UserLoginAfter(user)
}
