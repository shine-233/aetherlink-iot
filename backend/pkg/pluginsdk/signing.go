// Manifest 签名与验签（ROADMAP P3：第三方插件签名）。
//
// 设计要点：
//   - 用 **Ed25519 非对称签名**（与 pkg/license 同族）：插件由**厂商私钥**签名、
//     平台用厂商公钥验签——市场分发场景里平台不该持有厂商私钥，HMAC 对称密钥做不到这点
//     （那是 P1.6 模板市场包的场景：同一方签发并验签）。
//   - 签名覆盖**除 Signature/SignedBy 两字段外的规范 JSON**（字段序由结构体声明序保证），
//     否则签名三字段进不去摘要、验签死循环。
//   - 验签按"未签名 → 未知密钥 → 签名不符"逐级拒绝；与注册流程的组合语义：
//     未签名允许（D9 兼容过渡），**带了签名就必须验过**——坏的签名比没有签名更危险。
package pluginsdk

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Manifest 新增的两个签名字段（omitempty：老 manifest 解析不受影响）。
// 在 manifest.go 的 Manifest 结构体上声明会更好，但为保持签名 shadow 与结构体
// 同源，这里只做文档说明——实际字段见 manifest.go。
var (
	// ErrManifestUnsigned manifest 不携带签名。
	ErrManifestUnsigned = errors.New("pluginsdk: manifest is not signed")
	// ErrManifestBadSignature 签名与内容不符。
	ErrManifestBadSignature = errors.New("pluginsdk: manifest signature is invalid")
	// ErrManifestUnknownSigner 签名密钥不在受信表内。
	ErrManifestUnknownSigner = errors.New("pluginsdk: manifest signer is not trusted")
)

// canonicalManifest 序列化"待签内容"：排除 Signature / SignedBy 两字段。
func canonicalManifest(m *Manifest) ([]byte, error) {
	if m == nil {
		return nil, errors.New("pluginsdk: manifest is nil")
	}
	shadow := struct {
		Name             string            `json:"name"`
		Version          string            `json:"version"`
		Title            string            `json:"title,omitempty"`
		Description      string            `json:"description,omitempty"`
		Transport        string            `json:"transport,omitempty"`
		MinHostVersion   string            `json:"min_host_version,omitempty"`
		ConfigSchema     map[string]any    `json:"config_schema,omitempty"`
		PointTable       []PointDefinition `json:"point_table,omitempty"`
		CredentialFields []CredentialField `json:"credential_fields,omitempty"`
	}{}
	shadow.Name = m.Name
	shadow.Version = m.Version
	shadow.Title = m.Title
	shadow.Description = m.Description
	shadow.Transport = m.Transport
	shadow.MinHostVersion = m.MinHostVersion
	shadow.ConfigSchema = m.ConfigSchema
	shadow.PointTable = m.PointTable
	shadow.CredentialFields = m.CredentialFields
	return json.Marshal(&shadow)
}

// SignManifest 用厂商 Ed25519 私钥签名 manifest，填充 Signature / SignedBy。
// 签名前先做 Validate——给一个本身就非法的 manifest 签名等于给废纸盖章。
func SignManifest(m *Manifest, keyID string, privateKey ed25519.PrivateKey) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("pluginsdk: private key size is invalid")
	}
	if err := m.Validate(); err != nil {
		return err
	}
	canonical, err := canonicalManifest(m)
	if err != nil {
		return err
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, canonical))
	m.SignedBy = keyID
	return nil
}

// VerifyManifestSignature 用受信厂商公钥表验签。
// keys 为空时返回 UnknownSigner——没有受信密钥的验签只会制造"已验证"假象。
func VerifyManifestSignature(m *Manifest, keys map[string]ed25519.PublicKey) error {
	if m == nil {
		return errors.New("pluginsdk: manifest is nil")
	}
	if m.Signature == "" || m.SignedBy == "" {
		return ErrManifestUnsigned
	}
	if len(keys) == 0 {
		return fmt.Errorf("%w: no trusted vendor keys configured", ErrManifestUnknownSigner)
	}
	publicKey, ok := keys[m.SignedBy]
	if !ok {
		return fmt.Errorf("%w: key %q", ErrManifestUnknownSigner, m.SignedBy)
	}
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("%w: signature is not valid base64", ErrManifestBadSignature)
	}
	canonical, err := canonicalManifest(m)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, canonical, signature) {
		return ErrManifestBadSignature
	}
	return nil
}
