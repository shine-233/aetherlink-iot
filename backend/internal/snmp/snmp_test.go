// 文件用途：SNMPv2c 最小实现单测——OID BER 编解码、GetRequest 构建、
// 自构造 GetResponse 解析（INTEGER/OCTET STRING/TIMETICKS/错误码）、坏输入拒绝。
package snmp

import (
	"bytes"
	"testing"
)

func TestOIDEncodeKnownVector(t *testing.T) {
	// 1.3.6.1.2.1.1.3.0 → 06 08 2b 06 01 02 01 01 03 00
	enc, err := EncodeOID("1.3.6.1.2.1.1.3.0")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x06, 0x08, 0x2b, 0x06, 0x01, 0x02, 0x01, 0x01, 0x03, 0x00}
	if !bytes.Equal(enc, want) {
		t.Fatalf("OID 编码不符: %x want %x", enc, want)
	}
	if got := decodeOIDString(enc[2:]); got != "1.3.6.1.2.1.1.3.0" {
		t.Fatalf("OID 解码=%q", got)
	}
}

func TestBuildGetRequestStructure(t *testing.T) {
	raw, err := BuildGetRequest("public", 12345, []string{"1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseTLV(raw); err != nil {
		t.Fatalf("请求应可解析: %v", err)
	}
	if _, err := BuildGetRequest("public", 1, nil); err == nil {
		t.Fatal("空 OID 应报错")
	}
}

func TestParseResponseValuesAndError(t *testing.T) {
	// 构造 GetResponse：sysUpTime TIMETICKS(12345) + sysDescr OCTET STRING "demo"
	vb1 := buildTLV(0x30, append(mustOID("1.3.6.1.2.1.1.3.0"), buildTLV(0x43, berIntBytes(12345))...))
	vb2 := buildTLV(0x30, append(mustOID("1.3.6.1.2.1.1.1.0"), buildTLV(0x04, []byte("demo"))...))
	vbl := buildTLV(0x30, append(vb1, vb2...))
	pdu := buildTLV(pduGetResponse,
		append(buildTLV(0x02, berIntBytes(7)),
			append(buildTLV(0x02, []byte{0}), // error-status=0
				append(buildTLV(0x02, []byte{0}), // error-index=0
					vbl...)...)...))
	msg := buildTLV(0x30,
		append(buildTLV(0x02, []byte{1}),
			append(buildTLV(0x04, []byte("public")), pdu...)...))

	resp, err := ParseResponse(msg)
	if err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if resp.ErrorStatus != 0 || len(resp.Varbinds) != 2 {
		t.Fatalf("状态/varbind 数不符: %+v", resp)
	}
	up := resp.Varbinds["1.3.6.1.2.1.1.3.0"]
	if v, err := up.AsInt(); err != nil || v != 12345 {
		t.Fatalf("TIMETICKS 解析=%v err=%v", v, err)
	}
	if s, ok := resp.Varbinds["1.3.6.1.2.1.1.1.0"].AsString(); !ok || s != "demo" {
		t.Fatalf("OCTET STRING 解析=%q ok=%v", s, ok)
	}
}

func TestParseResponseNoSuchObject(t *testing.T) {
	// error-status=2 (noSuchName in v1/v2c ≈ 2) 场景
	vb := buildTLV(0x30, append(mustOID("1.3.6.1.9.9.9.0"), 0x05, 0x00)) // OID + NULL
	vbl := buildTLV(0x30, vb)
	pdu := buildTLV(pduGetResponse,
		append(buildTLV(0x02, berIntBytes(2)),
			append(buildTLV(0x02, []byte{2}),
				append(buildTLV(0x02, []byte{1}), vbl...)...)...))
	msg := buildTLV(0x30,
		append(buildTLV(0x02, []byte{1}), append(buildTLV(0x04, []byte("public")), pdu...)...))
	resp, err := ParseResponse(msg)
	if err != nil {
		t.Fatalf("解析: %v", err)
	}
	if resp.ErrorStatus != 2 {
		t.Fatalf("error-status=%d", resp.ErrorStatus)
	}
	if v, err := resp.Varbinds["1.3.6.1.9.9.9.0"].AsInt(); err == nil {
		t.Fatalf("NULL 值不应转成整数: %d", v)
	}
}

func TestParseResponseRejectsGarbage(t *testing.T) {
	if _, err := ParseResponse([]byte{0x30, 0x05, 0xff, 0xff, 0xff, 0xff, 0xff}); err == nil {
		t.Fatal("垃圾报文应报错")
	}
	if _, err := ParseResponse([]byte{0x04, 0x01, 0x00}); err == nil {
		t.Fatal("非 SEQUENCE 应报错")
	}
}

func mustOID(oid string) []byte {
	b, err := EncodeOID(oid)
	if err != nil {
		panic(err)
	}
	return b
}
