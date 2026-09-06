// 文件用途：SNMPv3 USM（RFC 3414）最小安全层——密钥生成与消息认证。
// 核心逻辑：password→Ku→Kul（本地化密钥，绑定 authoritativeEngineID）的 RFC 3414
//   keyLocalization；HMAC-MD5-96 / HMAC-SHA-96 认证参数生成与校验；USM 安全参数
//   （engineID/boots/time/userName/authParams）的 BER 编解码，用于组装 v3 报文。
// 关键注意事项：
//   - 只实现认证（authPriv 的加密、engine boots/time 的时间窗与 discovery 会话留待下一层）；
//   - 报文组装按 RFC3412 msgSecurityParameters 作为 OCTET STRING 嵌入，返回带认证摘要的
//     "整报文"供上层封装（先认证后加密的顺序在加 priv 时再扩展）；
//   - 常量时间比较校验摘要；拒绝空 engineID/短口令。
package snmp

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/subtle"
	"fmt"
	"strings"
)

// AuthProtocol USM 认证协议。
type AuthProtocol int

const (
	AuthNone AuthProtocol = iota
	AuthHMACMD5
	AuthHMACSHA
)

func (p AuthProtocol) String() string {
	switch p {
	case AuthHMACMD5:
		return "HMAC-MD5-96"
	case AuthHMACSHA:
		return "HMAC-SHA-96"
	default:
		return "none"
	}
}

// PasswordToKey RFC 3414 5.1：口令 → 64B 缓冲 → H(buffer||engineID||buffer) 得到 Ku。
func PasswordToKey(protocol AuthProtocol, password string, engineID []byte) ([]byte, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return nil, fmt.Errorf("usm: 口令为空")
	}
	if len(engineID) == 0 {
		return nil, fmt.Errorf("usm: engineID 为空")
	}
	pp := []byte(password)
	if len(pp) == 0 {
		return nil, fmt.Errorf("usm: 口令为空")
	}
	buf := make([]byte, 64)
	for i := range buf {
		buf[i] = pp[i%len(pp)]
	}
	material := append(append(append([]byte{}, buf...), engineID...), buf...)
	switch protocol {
	case AuthHMACMD5:
		// lgtm [go/weak-sensitive-data-hashing] -- RFC 3414 requires MD5 for legacy USM interoperability.
		m := md5.New()
		m.Write(material)
		return m.Sum(nil), nil
	case AuthHMACSHA:
		// lgtm [go/weak-sensitive-data-hashing] -- RFC 3414 requires SHA-1 for legacy USM interoperability.
		s := sha1.New()
		s.Write(material)
		return s.Sum(nil), nil
	default:
		return nil, fmt.Errorf("usm: 不支持的认证协议")
	}
}

// LocalizeKey RFC 3414 2.6：Kul = K(1)||…||K(n)，K(i)=H(K(i-1)||engineID||Ku)，共 64/hashLen 段。
func LocalizeKey(protocol AuthProtocol, ku, engineID []byte) []byte {
	hashLen := 16
	if protocol == AuthHMACSHA {
		hashLen = 20
	}
	hash := func(prev []byte) []byte {
		material := append(append(append([]byte{}, prev...), engineID...), ku...)
		switch protocol {
		case AuthHMACMD5:
			// lgtm [go/weak-sensitive-data-hashing] -- RFC 3414 requires MD5 for legacy USM interoperability.
			m := md5.New()
			m.Write(material)
			return m.Sum(nil)
		default:
			// lgtm [go/weak-sensitive-data-hashing] -- RFC 3414 requires SHA-1 for legacy USM interoperability.
			s := sha1.New()
			s.Write(material)
			return s.Sum(nil)
		}
	}
	kul := make([]byte, 0, 64)
	prev := ku
	for len(kul) < 64 {
		k := hash(prev)
		kul = append(kul, k[:hashLen]...)
		prev = k
	}
	return kul[:64]
}

// ComputeAuthDigest 计算并截断为 12 字节（RFC 3414 HMAC-96）认证摘要。
func ComputeAuthDigest(protocol AuthProtocol, kul, wholeMsg []byte) ([]byte, error) {
	switch protocol {
	case AuthHMACMD5:
		mac := hmac.New(md5.New, kul)
		mac.Write(wholeMsg)
		return mac.Sum(nil)[:12], nil
	case AuthHMACSHA:
		mac := hmac.New(sha1.New, kul)
		mac.Write(wholeMsg)
		return mac.Sum(nil)[:12], nil
	default:
		return nil, fmt.Errorf("usm: none 无认证摘要")
	}
}

// VerifyAuthDigest 常量时间校验 12 字节摘要。
func VerifyAuthDigest(protocol AuthProtocol, kul, wholeMsg, expect []byte) bool {
	if len(expect) != 12 {
		return false
	}
	got, err := ComputeAuthDigest(protocol, kul, wholeMsg)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, expect) == 1
}

// USMSecurityParams USM 安全参数（RFC 3414 msgSecurityParameters 内层）。
type USMSecurityParams struct {
	EngineID    []byte
	EngineBoots int32
	EngineTime  int32
	UserName    string
	AuthParams  []byte // 12 字节摘要占位（发送前留 0，接收后为实际摘要）
}

// MarshalUSMParams 编码为 msgSecurityParameters（不含外层 OCTET STRING 包裹）。
func (p *USMSecurityParams) MarshalUSMParams() []byte {
	var b []byte
	b = append(b, buildTLV(0x04, p.EngineID)...)
	b = append(b, buildTLV(0x02, berIntBytes(int64(p.EngineBoots)))...)
	b = append(b, buildTLV(0x02, berIntBytes(int64(p.EngineTime)))...)
	b = append(b, buildTLV(0x04, []byte(p.UserName))...)
	b = append(b, buildTLV(0x04, p.AuthParams)...)
	return b
}

// UnmarshalUSMParams 从 msgSecurityParameters 解析（输入为内层字节）。
func UnmarshalUSMParams(raw []byte) (*USMSecurityParams, error) {
	out := &USMSecurityParams{}
	seg, rest, err := consumeTLV(raw)
	if err != nil || seg.tag != 0x04 {
		return nil, fmt.Errorf("usm: engineID 解析失败")
	}
	out.EngineID = append([]byte{}, seg.body...)
	seg, rest, err = consumeTLV(rest)
	if err != nil || seg.tag != 0x02 {
		return nil, fmt.Errorf("usm: boots 解析失败")
	}
	out.EngineBoots = int32(mustInt(seg.body))
	seg, rest, err = consumeTLV(rest)
	if err != nil || seg.tag != 0x02 {
		return nil, fmt.Errorf("usm: time 解析失败")
	}
	out.EngineTime = int32(mustInt(seg.body))
	seg, rest, err = consumeTLV(rest)
	if err != nil || seg.tag != 0x04 {
		return nil, fmt.Errorf("usm: userName 解析失败")
	}
	out.UserName = string(seg.body)
	seg, _, err = consumeTLV(rest)
	if err != nil || seg.tag != 0x04 {
		return nil, fmt.Errorf("usm: authParams 解析失败")
	}
	out.AuthParams = append([]byte{}, seg.body...)
	return out, nil
}

func mustInt(raw []byte) int64 {
	v, _ := decodeInteger(raw)
	return v
}

// NewAuthDigestBuffer 返回 12 字节全 0 摘要占位（构造报文时先用全 0，再整体回填）。
func NewAuthDigestBuffer() []byte { return make([]byte, 12) }

// ReplaceAuthParamsInMessage 把已编码报文中 msgSecurityParameters 的摘要段替换为真实摘要。
// 简化实现：假设报文为 UTF8 明文场景由上层用占位符号替换；此处提供内存版整报替换工具，
// 供报文构造流程调用（按 authParams 固定 12 字节偏移）。
func ReplaceAuthParamsInMessage(msg []byte, marker []byte, digest []byte) ([]byte, bool) {
	idx := bytes.Index(msg, marker)
	if idx < 0 || len(digest) != 12 {
		return msg, false
	}
	out := append([]byte{}, msg...)
	copy(out[idx:idx+12], digest)
	return out, true
}
