// Package license 实现离线商业许可证的解析与验证（ROADMAP P3：商业许可证边界）。
//
// 许可证是一段 base64 编码的签名 JSON：Document 的规范 JSON + Ed25519 签名。
// 验证完全离线：不需要联网、不依赖硬件指纹（那是另一层，刻意不做——
// 硬件指纹把迁移和虚拟化变成客服灾难，边界应先用法律条款+密钥控制划清）。
//
// 设计要点：
//   - 公钥由部署方配置（license.public_keys，支持多 key_id 轮换）；
//     未配置公钥 = 无法验证任何许可证 = 商业边界未启用。
//   - 时间窗（not_before / not_after）在签名**之内**：改窗口必然破坏签名。
//   - 配额与特性在签名之内：伪造更大配额必然破坏签名。
package license

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 验证失败的各级哨兵，便于调用方区分原因。
var (
	ErrLicenseMalformed   = errors.New("license: material is malformed")
	ErrLicenseUnsigned    = errors.New("license: material is not signed")
	ErrLicenseBadSig      = errors.New("license: signature is invalid")
	ErrLicenseUnknownKey  = errors.New("license: signing key is not trusted")
	ErrLicenseNotYetValid = errors.New("license: not valid yet")
	ErrLicenseExpired     = errors.New("license: expired")
)

// Document 许可证正文。所有字段都在签名覆盖范围内。
type Document struct {
	Edition    string   `json:"edition"`               // 例如 community / professional / enterprise
	IssuedTo   string   `json:"issued_to,omitempty"`   // 授予对象（客户/合同标识）
	Features   []string `json:"features,omitempty"`    // 启用的特性标识
	MaxDevices int64    `json:"max_devices,omitempty"` // 设备配额；0 = 不限量
	MaxTenants int64    `json:"max_tenants,omitempty"` // 租户配额；0 = 不限量
	NotBefore  int64    `json:"not_before"`            // unix 毫秒
	NotAfter   int64    `json:"not_after"`             // unix 毫秒
	IssuedAt   int64    `json:"issued_at"`             // unix 毫秒
}

// Material 签名载荷：document 的规范 JSON + Ed25519 签名 + 密钥标识。
type Material struct {
	Document  json.RawMessage `json:"document"`  // 保持原字节，签什么验什么
	Signature string          `json:"signature"` // Ed25519(document)，base64 (std)
	KeyID     string          `json:"key_id"`
}

// Verifier 持有受信公钥集的验证器。零值不可用，须由 NewVerifier 构造。
type Verifier struct {
	keys map[string]ed25519.PublicKey
}

// NewVerifier 用 base64 (std) 编码的 Ed25519 公钥表构造验证器。
// 空表返回错误——没有公钥的验证器只会制造"已验证"的假象。
func NewVerifier(publicKeys map[string]string) (*Verifier, error) {
	keys := make(map[string]ed25519.PublicKey, len(publicKeys))
	for id, encoded := range publicKeys {
		id = strings.TrimSpace(id)
		encoded = strings.TrimSpace(encoded)
		if id == "" || encoded == "" {
			return nil, fmt.Errorf("license: public key entry %q is empty", id)
		}
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("license: public key %q is not valid base64: %w", id, err)
		}
		if len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("license: public key %q must be %d bytes, got %d", id, ed25519.PublicKeySize, len(raw))
		}
		keys[id] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, errors.New("license: no public keys configured")
	}
	return &Verifier{keys: keys}, nil
}

// Parse 解码并验证一段许可证材料。通过则返回正文与摘要（供审计引用）。
func (v *Verifier) Parse(materialBase64 string, now time.Time) (*Document, string, error) {
	if v == nil || len(v.keys) == 0 {
		return nil, "", errors.New("license: verifier is not configured")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(materialBase64))
	if err != nil {
		return nil, "", fmt.Errorf("%w: material is not valid base64: %v", ErrLicenseMalformed, err)
	}
	var material Material
	if err := json.Unmarshal(raw, &material); err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrLicenseMalformed, err)
	}
	if strings.TrimSpace(material.Signature) == "" || len(material.Document) == 0 {
		return nil, "", ErrLicenseUnsigned
	}
	publicKey, ok := v.keys[strings.TrimSpace(material.KeyID)]
	if !ok {
		return nil, "", fmt.Errorf("%w: key %q", ErrLicenseUnknownKey, material.KeyID)
	}
	signature, err := base64.StdEncoding.DecodeString(material.Signature)
	if err != nil {
		return nil, "", fmt.Errorf("%w: signature is not valid base64", ErrLicenseBadSig)
	}
	if !ed25519.Verify(publicKey, material.Document, signature) {
		return nil, "", ErrLicenseBadSig
	}
	var doc Document
	if err := json.Unmarshal(material.Document, &doc); err != nil {
		return nil, "", fmt.Errorf("%w: document is not valid JSON: %v", ErrLicenseMalformed, err)
	}
	if doc.NotAfter > 0 && now.UnixMilli() > doc.NotAfter {
		return nil, "", ErrLicenseExpired
	}
	if doc.NotBefore > 0 && now.UnixMilli() < doc.NotBefore {
		return nil, "", ErrLicenseNotYetValid
	}
	sum := sha256.Sum256(material.Document)
	return &doc, hex.EncodeToString(sum[:]), nil
}

// Allows 特性是否被许可证启用。空 Features 表示不按特性限制。
func (d *Document) Allows(feature string) bool {
	if d == nil {
		return false
	}
	if len(d.Features) == 0 {
		return true
	}
	for _, f := range d.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// GenerateKeyPair 生成一对用于离线商业许可证签发的 Ed25519 密钥对。
// 返回 base64 (std) 编码的公钥与私钥字符串。
func GenerateKeyPair() (string, string, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", fmt.Errorf("license: key generation failed: %w", err)
	}
	pubBase64 := base64.StdEncoding.EncodeToString(pub)
	privBase64 := base64.StdEncoding.EncodeToString(priv)
	return pubBase64, privBase64, nil
}

// SignDocument 使用 Ed25519 私钥对许可证 Document 进行规范序列化与数字签名，
// 返回 base64 编码的许可证材料字符串，可直接部署在平台的 license.material 配置中。
func SignDocument(doc *Document, keyID string, privateKey ed25519.PrivateKey) (string, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return "", errors.New("license: key_id cannot be empty")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("license: invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}
	if doc == nil {
		return "", errors.New("license: document cannot be nil")
	}
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("license: marshal document failed: %w", err)
	}
	sig := ed25519.Sign(privateKey, docBytes)
	material := Material{
		Document:  json.RawMessage(docBytes),
		Signature: base64.StdEncoding.EncodeToString(sig),
		KeyID:     keyID,
	}
	materialBytes, err := json.Marshal(material)
	if err != nil {
		return "", fmt.Errorf("license: marshal material failed: %w", err)
	}
	return base64.StdEncoding.EncodeToString(materialBytes), nil
}

// SignDocumentWithBase64Key 便捷函数：使用 base64 编码的私钥字符串进行签名。
func SignDocumentWithBase64Key(doc *Document, keyID string, privateKeyBase64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKeyBase64))
	if err != nil {
		return "", fmt.Errorf("license: invalid base64 private key: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("license: private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(raw))
	}
	return SignDocument(doc, keyID, ed25519.PrivateKey(raw))
}

