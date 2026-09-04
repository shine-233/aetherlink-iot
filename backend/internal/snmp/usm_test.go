// 文件用途：USM 最小实现自洽性单测——密钥派生确定性、本地化密钥稳定、HMAC 摘要计算/
// 校验/篡改拒绝、USM 安全参数编解码往返。RFC 3414 规范向量与外网引擎互通属运行时边界。
package snmp

import (
	"bytes"
	"testing"
)

func TestPasswordToKeyDeterministic(t *testing.T) {
	engine := []byte{0x80, 0x00, 0x1f, 0x88, 0x80, 0x01, 0x02, 0x03}
	k1, err := PasswordToKey(AuthHMACMD5, "maplesyrup", engine)
	if err != nil {
		t.Fatal(err)
	}
	k2, _ := PasswordToKey(AuthHMACMD5, "maplesyrup", engine)
	if !bytes.Equal(k1, k2) || len(k1) != 16 {
		t.Fatalf("Ku 不稳定/长度错: %d", len(k1))
	}
	if _, err := PasswordToKey(AuthHMACMD5, "", engine); err == nil {
		t.Fatal("空口令应报错")
	}
	if _, err := PasswordToKey(AuthHMACMD5, "pw", nil); err == nil {
		t.Fatal("空 engineID 应报错")
	}
	ks, _ := PasswordToKey(AuthHMACSHA, "maplesyrup", engine)
	if len(ks) != 20 {
		t.Fatalf("SHA Ku 长度=%d", len(ks))
	}
}

func TestLocalizeKeyStableAndLength(t *testing.T) {
	engine := []byte{0x80, 0x00, 0x1f, 0x88, 0x00, 0x00, 0x00, 0x01}
	ku, _ := PasswordToKey(AuthHMACMD5, "authkey1", engine)
	kul := LocalizeKey(AuthHMACMD5, ku, engine)
	if len(kul) != 64 {
		t.Fatalf("Kul 长度=%d", len(kul))
	}
	if !bytes.Equal(kul, LocalizeKey(AuthHMACMD5, ku, engine)) {
		t.Fatal("Kul 应确定")
	}
	kulSHA := LocalizeKey(AuthHMACSHA, mustKulSHA(t, engine), engine)
	if len(kulSHA) != 64 {
		t.Fatalf("SHA Kul 长度=%d", len(kulSHA))
	}
}

func mustKulSHA(t *testing.T, engine []byte) []byte {
	t.Helper()
	ku, err := PasswordToKey(AuthHMACSHA, "authkey1", engine)
	if err != nil {
		t.Fatal(err)
	}
	return ku
}

func TestComputeAndVerifyAuthDigest(t *testing.T) {
	engine := []byte{0x80, 0x00, 0x1f, 0x88, 0x01, 0x02, 0x03, 0x04}
	ku, _ := PasswordToKey(AuthHMACMD5, "secret", engine)
	kul := LocalizeKey(AuthHMACMD5, ku, engine)
	msg := []byte("SNMPv3 whole message bytes")
	digest, err := ComputeAuthDigest(AuthHMACMD5, kul, msg)
	if err != nil || len(digest) != 12 {
		t.Fatalf("摘要 len=%d err=%v", len(digest), err)
	}
	if !VerifyAuthDigest(AuthHMACMD5, kul, msg, digest) {
		t.Fatal("合法摘要应通过校验")
	}
	tampered := append([]byte{}, msg...)
	tampered[3] ^= 0x01
	if VerifyAuthDigest(AuthHMACMD5, kul, tampered, digest) {
		t.Fatal("篡改报文应校验失败")
	}
	if VerifyAuthDigest(AuthHMACMD5, kul, msg, []byte("123456789012")) {
		t.Fatal("错误摘要应失败")
	}
	kuSHA, _ := PasswordToKey(AuthHMACSHA, "secret", engine)
	kulSHA := LocalizeKey(AuthHMACSHA, kuSHA, engine)
	dSHA, err := ComputeAuthDigest(AuthHMACSHA, kulSHA, msg)
	if err != nil || len(dSHA) != 12 {
		t.Fatalf("SHA 摘要 len=%d err=%v", len(dSHA), err)
	}
	if !VerifyAuthDigest(AuthHMACSHA, kulSHA, msg, dSHA) {
		t.Fatal("SHA 合法摘要应通过")
	}
}

func TestUSMParamsMarshalUnmarshal(t *testing.T) {
	p := &USMSecurityParams{
		EngineID:    []byte{0x80, 0x00, 0x1f, 0x88, 0x01},
		EngineBoots: 7,
		EngineTime:  12345,
		UserName:    "tester",
		AuthParams:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
	}
	raw := p.MarshalUSMParams()
	got, err := UnmarshalUSMParams(raw)
	if err != nil {
		t.Fatalf("解析: %v", err)
	}
	if got.EngineBoots != 7 || got.EngineTime != 12345 || got.UserName != "tester" {
		t.Fatalf("字段不符: %+v", got)
	}
	if !bytes.Equal(got.EngineID, p.EngineID) || !bytes.Equal(got.AuthParams, p.AuthParams) {
		t.Fatal("engineID/authParams 往返不一致")
	}
	if _, err := UnmarshalUSMParams([]byte{0x04, 0x01, 0x00}); err == nil {
		t.Fatal("截断参数应报错")
	}
}
