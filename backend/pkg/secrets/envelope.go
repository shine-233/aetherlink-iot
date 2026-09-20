// 文件用途：AI provider 凭证的信封加密（ROADMAP P0.7）。
// 核心逻辑：明文密钥永不落库；密文自带算法版本与密钥 ID，轮换窗口内可读取旧密文并可重新加密。
// 关键注意事项：
//  1. 主密钥缺失、非法或长度错误一律 fail closed，绝不降级为明文落库。
//  2. AAD 绑定租户，禁止把 A 租户密文搬到 B 租户行上冒充可用凭证。
//  3. 本包只处理“静态加密”，不参与任何网络出口判定（safe-egress 逻辑不变）。
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// 信封格式：aenv1.<keyID>.<base64(nonce||ciphertext)>
const (
	EnvelopePrefix = "aenv1"
	keyBytes       = 32 // AES-256
	envelopeParts  = 3
)

var (
	ErrMasterKeyUnavailable = errors.New("secrets: master key unavailable")
	ErrMalformedEnvelope    = errors.New("secrets: malformed envelope")
	ErrDecryptFailed        = errors.New("secrets: decrypt failed")
)

// masterKey 取指定 keyID 的 32 字节主密钥。任何不可用情况都视为配置错误。
func masterKey(keyID string) ([]byte, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, ErrMasterKeyUnavailable
	}
	raw := strings.TrimSpace(viper.GetString("secrets.master_keys." + keyID))
	if raw == "" {
		return nil, fmt.Errorf("%w: key id %q not configured", ErrMasterKeyUnavailable, keyID)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: key id %q is not base64: %v", ErrMasterKeyUnavailable, keyID, err)
	}
	if len(key) != keyBytes {
		return nil, fmt.Errorf("%w: key id %q must decode to %d bytes, got %d",
			ErrMasterKeyUnavailable, keyID, keyBytes, len(key))
	}
	return key, nil
}

// activeKeyID 返回当前写入使用的密钥 ID；主密钥不可用时返回错误（fail closed）。
func activeKeyID() (string, error) {
	id := strings.TrimSpace(viper.GetString("secrets.active_key_id"))
	if id == "" {
		return "", fmt.Errorf("%w: secrets.active_key_id not configured", ErrMasterKeyUnavailable)
	}
	if _, err := masterKey(id); err != nil {
		return "", err
	}
	return id, nil
}

// ActiveKeyID 暴露当前主密钥 ID，供运维核对轮换状态。
func ActiveKeyID() (string, error) { return activeKeyID() }

// IsEnvelope 判断给定值是否为本包产出的信封密文。
func IsEnvelope(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != envelopeParts || parts[0] != EnvelopePrefix {
		return false
	}
	return parts[1] != "" && parts[2] != ""
}

// KeyIDOf 返回信封使用的密钥 ID，用于判断是否需要重加密。
func KeyIDOf(value string) (string, error) {
	if !IsEnvelope(value) {
		return "", ErrMalformedEnvelope
	}
	return strings.Split(value, ".")[1], nil
}

// Seal 使用当前主密钥加密明文。aad 作为附加认证数据（建议传租户 ID）。
func Seal(plain string, aad ...string) (string, error) {
	keyID, err := activeKeyID()
	if err != nil {
		return "", err
	}
	return SealWithKey(keyID, plain, aad...)
}

// SealWithKey 使用指定密钥 ID 加密，供轮换时按目标版本重写。
func SealWithKey(keyID, plain string, aad ...string) (string, error) {
	key, err := masterKey(keyID)
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
	ct := gcm.Seal(nonce, nonce, []byte(plain), aadBytes(aad))
	return fmt.Sprintf("%s.%s.%s", EnvelopePrefix, keyID, base64.StdEncoding.EncodeToString(ct)), nil
}

// Open 解密信封。aad 必须与 Seal 时一致，否则认证失败（防止跨租户搬运）。
func Open(value string, aad ...string) (string, error) {
	if !IsEnvelope(value) {
		return "", ErrMalformedEnvelope
	}
	parts := strings.Split(value, ".")
	key, err := masterKey(parts[1])
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", ErrMalformedEnvelope
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
		return "", ErrMalformedEnvelope
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, aadBytes(aad))
	if err != nil {
		// 不回显明文或密文内容，避免密钥材料/凭证泄漏进日志。
		return "", fmt.Errorf("%w (key id %s)", ErrDecryptFailed, parts[1])
	}
	return string(plain), nil
}

// NeedsReseal 判断密文是否需要用当前主密钥重新加密。
// 遗留明文（非信封格式）同样需要重新封装。
func NeedsReseal(value string) bool {
	if !IsEnvelope(value) {
		return true
	}
	current, err := activeKeyID()
	if err != nil {
		// 主密钥不可用时不做判断；写入路径会由 Seal 直接 fail closed。
		return false
	}
	id, err := KeyIDOf(value)
	if err != nil {
		return true
	}
	return id != current
}

// aadBytes 把 AAD 片段拼接成确定性字节串（以 0 分隔，避免串联歧义）。
func aadBytes(aad []string) []byte {
	var out []byte
	for _, part := range aad {
		out = append(out, []byte(part)...)
		out = append(out, 0)
	}
	return out
}

// Mask 返回不可逆展示掩码：仅保留前 head 位，短于 head 时全掩码（不泄漏长度）。
func Mask(plain string, head int) string {
	if head <= 0 {
		return "****"
	}
	runes := []rune(plain)
	if len(runes) > head {
		return string(runes[:head]) + "****"
	}
	return "****"
}
