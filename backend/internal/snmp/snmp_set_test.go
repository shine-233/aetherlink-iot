// 文件用途：SNMPv2c Set 扩展单测——整数/字符串/Counter32 值编码、SetRequest 结构、
// 空绑定拒绝，以及编码后能按 varbind 语义解析出正确 OID 与值类型。
package snmp

import (
	"bytes"
	"testing"
)

func decodeVarbinds(t *testing.T, raw []byte) (map[string]Value, error) {
	t.Helper()
	msg, err := parseTLV(raw)
	if err != nil || msg.tag != 0x30 {
		t.Fatalf("顶层 SEQUENCE 失败")
	}
	_, rest, err := consumeTLV(msg.body) // version
	if err != nil {
		t.Fatal(err)
	}
	_, rest, err = consumeTLV(rest) // community
	if err != nil {
		t.Fatal(err)
	}
	pdu, _, err := consumeTLV(rest)
	if err != nil || pdu.tag != pduSetRequest {
		t.Fatalf("PDU 非 set-request: %x", pdu.tag)
	}
	_, pduRest, err := consumeTLV(pdu.body) // request-id
	if err != nil {
		t.Fatal(err)
	}
	_, pduRest, err = consumeTLV(pduRest) // error-status
	if err != nil {
		t.Fatal(err)
	}
	_, pduRest, err = consumeTLV(pduRest) // error-index
	if err != nil {
		t.Fatal(err)
	}
	vbl, _, err := consumeTLV(pduRest)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Value{}
	vb := vbl.body
	for len(vb) > 0 {
		item, next, err := consumeTLV(vb)
		if err != nil {
			t.Fatal(err)
		}
		vb = next
		oidTLV, rest2, err := consumeTLV(item.body)
		if err != nil {
			t.Fatal(err)
		}
		valTLV, _, err := consumeTLV(rest2)
		if err != nil {
			t.Fatal(err)
		}
		out[decodeOIDString(oidTLV.body)] = Value{Type: valTLV.tag, Raw: valTLV.body}
	}
	return out, nil
}

func TestBuildSetRequestIntegerAndString(t *testing.T) {
	raw, err := BuildSetRequest("private", 9, []VarBind{
		{OID: "1.3.6.1.2.1.1.6.0", Value: OctetStringValue("Shanghai")},
		{OID: "1.3.6.1.2.1.1.7.0", Value: IntegerValue(100)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("Shanghai")) {
		t.Fatal("SetRequest 应含字符串值")
	}
	got, err := decodeVarbinds(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("varbind 数=%d", len(got))
	}
	if v := got["1.3.6.1.2.1.1.7.0"]; v.Type != 0x02 {
		t.Fatalf("整数类型错: %x", v.Type)
	}
	if iv, err := got["1.3.6.1.2.1.1.7.0"].AsInt(); err != nil || iv != 100 {
		t.Fatalf("整数值=%v err=%v", iv, err)
	}
	if s, ok := got["1.3.6.1.2.1.1.6.0"].AsString(); !ok || s != "Shanghai" {
		t.Fatalf("字符串值=%q ok=%v", s, ok)
	}
}

func TestBuildSetRequestCounter32(t *testing.T) {
	raw, err := BuildSetRequest("private", 2, []VarBind{
		{OID: "1.3.6.1.2.1.1.3.0", Value: Counter32Value(4294967295)},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeVarbinds(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	v := got["1.3.6.1.2.1.1.3.0"]
	if v.Type != 0x41 {
		t.Fatalf("Counter32 类型错: %x", v.Type)
	}
}

func TestBuildSetRequestRejectsEmpty(t *testing.T) {
	if _, err := BuildSetRequest("private", 1, nil); err == nil {
		t.Fatal("空绑定应报错")
	}
}
