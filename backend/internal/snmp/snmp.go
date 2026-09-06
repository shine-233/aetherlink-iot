// 文件用途：SNMPv2c 最小客户端核心（ROADMAP C6）——纯标准库 BER 编解码。
// 核心逻辑：构建 SNMPv2c GetRequest（version/community/request-id/OID varbind），
//   解析 GetResponse（error-status、error-index、value 类型：INTEGER/OCTET STRING/
//   OBJECT IDENTIFIER/Null/TIMETICKS），与一个无连接 UDP 收发器。
// 关键注意事项：
//   - 仅实现 v2c 明文 Get 必需子集（ASN.1 BER 标签：SEQUENCE=0x30/INTEGER=0x02/
//     OCTET STRING=0x04/Null=0x05/OID=0x06/TimeTicks=0x43/IPAddr=0x40），不含 Set/Walk/陷阱；
//   - community 以明文传输，仅限受信内网，勿用于公网（与平台 MQTT 明文门禁口径一致）；
//   - 解析输入以长度钳制 + 严格长度校验防越界。
package snmp

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

// PDU 类型。
const (
	pduGetRequest   = 0xA0
	pduGetResponse  = 0xA2
	pduSetRequest   = 0xA3
)

// VarBind 一条 OID→值 绑定（用于 Set）。
type VarBind struct {
	OID   string
	Value Value
}

// IntegerValue 构造 INTEGER 值。
func IntegerValue(v int64) Value { return Value{Type: 0x02, Raw: berIntBytes(v)} }

// OctetStringValue 构造 OCTET STRING 值。
func OctetStringValue(s string) Value { return Value{Type: 0x04, Raw: []byte(s)} }

// Counter32Value 构造 Counter32（0x41，无符号）。
func Counter32Value(v uint32) Value { return Value{Type: 0x41, Raw: berUintBytes(uint64(v))} }

func berUintBytes(v uint64) []byte {
	if v == 0 {
		return []byte{0}
	}
	var out []byte
	for i := 7; i >= 0; i-- {
		b := byte(v >> (i * 8))
		if b != 0 {
			out = append(out, b)
		}
	}
	return out
}

// Value 一个 varbind 值及其 ASN.1 类型。
type Value struct {
	Type uint8
	Raw  []byte
}

// AsInt 将值解释为 INTEGER/Counter/TIMETICKS（signed/unsigned 按最大长度判定）。
func (v Value) AsInt() (int64, error) {
	if v.Type == 0x05 { // NULL
		return 0, fmt.Errorf("value is NULL")
	}
	return decodeInteger(v.Raw)
}

// AsString 将值解释为 OCTET STRING。
func (v Value) AsString() (string, bool) {
	if v.Type != 0x04 {
		return "", false
	}
	return string(v.Raw), true
}

// Response SNMPv2c 响应。
type Response struct {
	ErrorStatus int
	ErrorIndex  int
	Varbinds    map[string]Value // OID → value
}

// EncodeOID 将点分 OID 编码为 BER。
func EncodeOID(oid string) ([]byte, error) {
	var ints []uint64
	for _, part := range splitDotted(oid) {
		var v uint64
		if _, err := fmt.Sscanf(part, "%d", &v); err != nil {
			return nil, fmt.Errorf("snmp: 非法 OID 段 %q", part)
		}
		ints = append(ints, v)
	}
	if len(ints) < 2 {
		return nil, fmt.Errorf("snmp: OID 至少两段")
	}
	// 首两段合并：40*first+second
	body := encodeBase128(40*ints[0] + ints[1])
	for _, v := range ints[2:] {
		body = append(body, encodeBase128(v)...)
	}
	return append([]byte{0x06}, appendBerLength(len(body), body)...), nil
}

func splitDotted(s string) []string {
	out := []string{}
	cur := ""
	for _, ch := range s {
		if ch == '.' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(ch)
		}
	}
	out = append(out, cur)
	return out
}

func encodeBase128(v uint64) []byte {
	if v == 0 {
		return []byte{0}
	}
	var rev []byte
	for v > 0 {
		rev = append(rev, byte(v&0x7f))
		v >>= 7
	}
	out := make([]byte, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		b := rev[i]
		if i != 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}

func berLength(n int) []byte {
	if n < 128 {
		return []byte{byte(n)}
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(n))
	// 取有效字节数
	i := 0
	for i < 7 && b[i] == 0 {
		i++
	}
	out := []byte{byte(0x80 | (8 - i))}
	out = append(out, b[i:]...)
	return out
}

func appendBerLength(n int, body []byte) []byte {
	return append(berLength(n), body...)
}

func tl(tag byte, body []byte) []byte {
	return append([]byte{tag}, appendBerLength(len(body), body)...)
}

func buildTLV(tag byte, body []byte) []byte { return tl(tag, body) }

// BuildGetRequest 构建 SNMPv2c GetRequest 报文。
func BuildGetRequest(community string, requestID int32, oids []string) ([]byte, error) {
	community = sanitizeCommunity(community)
	if len(oids) == 0 {
		return nil, fmt.Errorf("snmp: OID 列表为空")
	}
	reqID := buildTLV(0x02, berIntBytes(int64(requestID)))
	items := []byte{}
	for _, oid := range oids {
		oidBytes, err := EncodeOID(oid)
		if err != nil {
			return nil, err
		}
		items = append(items, buildTLV(0x30, append(oidBytes, 0x05))...) // OID + NULL
	}
	varbinds := buildTLV(0x30, items)
	pdu := buildTLV(pduGetRequest,
		append(reqID,
			append(buildTLV(0x02, []byte{0}), // error-status=0
				append(buildTLV(0x02, []byte{0}), // error-index=0
					varbinds...)...)...))
	msg := buildTLV(0x30,
		append(buildTLV(0x02, []byte{1}), // v2c
			append(buildTLV(0x04, []byte(community)),
				pdu...)...))
	return msg, nil
}

// BuildSetRequest 构建 SNMPv2c SetRequest 报文。
func BuildSetRequest(community string, requestID int32, binds []VarBind) ([]byte, error) {
	community = sanitizeCommunity(community)
	if len(binds) == 0 {
		return nil, fmt.Errorf("snmp: Set 至少一条 varbind")
	}
	reqID := buildTLV(0x02, berIntBytes(int64(requestID)))
	items := []byte{}
	for _, b := range binds {
		oidBytes, err := EncodeOID(b.OID)
		if err != nil {
			return nil, err
		}
		valBytes := buildTLV(b.Value.Type, b.Value.Raw)
		items = append(items, buildTLV(0x30, append(oidBytes, valBytes...))...)
	}
	varbinds := buildTLV(0x30, items) // varbind-list 外层 SEQUENCE（RFC 3416 PDU 结构）
	pdu := buildTLV(pduSetRequest,
		append(reqID,
			append(buildTLV(0x02, []byte{0}), // error-status=0
				append(buildTLV(0x02, []byte{0}), // error-index=0
					varbinds...)...)...))
	return buildTLV(0x30,
		append(buildTLV(0x02, []byte{1}),
			append(buildTLV(0x04, []byte(community)), pdu...)...)), nil
}

// BuildGetResponse 构建 SNMPv2c GetResponse 报文（测试内嵌 agent / 中继场景复用）。
// errorStatus 非 0 时按 errorIndex 报告，varbinds 仍按原样携带。
func BuildGetResponse(community string, requestID int32, errorStatus, errorIndex int, binds []VarBind) ([]byte, error) {
	community = sanitizeCommunity(community)
	if len(binds) == 0 {
		return nil, fmt.Errorf("snmp: GetResponse 至少一条 varbind")
	}
	reqID := buildTLV(0x02, berIntBytes(int64(requestID)))
	items := []byte{}
	for _, b := range binds {
		oidBytes, err := EncodeOID(b.OID)
		if err != nil {
			return nil, err
		}
		valBytes := buildTLV(b.Value.Type, b.Value.Raw)
		items = append(items, buildTLV(0x30, append(oidBytes, valBytes...))...)
	}
	varbinds := buildTLV(0x30, items)
	pdu := buildTLV(pduGetResponse,
		append(reqID,
			append(buildTLV(0x02, berIntBytes(int64(errorStatus))),
				append(buildTLV(0x02, berIntBytes(int64(errorIndex))),
					varbinds...)...)...))
	return buildTLV(0x30,
		append(buildTLV(0x02, []byte{1}),
			append(buildTLV(0x04, []byte(community)), pdu...)...)), nil
}

func sanitizeCommunity(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

func berIntBytes(v int64) []byte {
	if v == 0 {
		return []byte{0}
	}
	var out []byte
	for i := 7; i >= 0; i-- {
		b := byte(v >> (i * 8))
		if b != 0 {
			out = append(out, b)
		}
	}
	if out[0]&0x80 != 0 {
		out = append([]byte{0}, out...)
	}
	return out
}

func decodeInteger(raw []byte) (int64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("integer 为空")
	}
	var v int64
	if raw[0]&0x80 != 0 {
		v = -1
	}
	for _, b := range raw {
		v = v<<8 | int64(b)
	}
	return v, nil
}

// ParseResponse 解析 SNMPv2c GetResponse 报文。
func ParseResponse(raw []byte) (*Response, error) {
	msg, err := parseTLV(raw)
	if err != nil || msg.tag != 0x30 {
		return nil, fmt.Errorf("snmp: 响应非 SEQUENCE")
	}
	ver, rest, err := consumeTLV(msg.body)
	if err != nil || ver.tag != 0x02 {
		return nil, fmt.Errorf("snmp: 响应缺 version")
	}
	verInt, _ := decodeInteger(ver.body)
	if verInt != 1 {
		return nil, fmt.Errorf("snmp: 不支持版本 %d（需 v2c=1）", verInt)
	}
	comm, rest, err := consumeTLV(rest)
	if err != nil || comm.tag != 0x04 {
		return nil, fmt.Errorf("snmp: 响应缺 community")
	}
	_ = comm
	pdu, _, err := consumeTLV(rest)
	if err != nil {
		return nil, err
	}
	if pdu.tag != pduGetResponse {
		return nil, fmt.Errorf("snmp: 非 GetResponse PDU (0x%02x)", pdu.tag)
	}
	rid, pduRest, err := consumeTLV(pdu.body)
	if err != nil || rid.tag != 0x02 {
		return nil, fmt.Errorf("snmp: PDU 缺 request-id")
	}
	est, pduRest, err := consumeTLV(pduRest)
	if err != nil || est.tag != 0x02 {
		return nil, fmt.Errorf("snmp: PDU 缺 error-status")
	}
	status, _ := decodeInteger(est.body)
	eidx, pduRest, err := consumeTLV(pduRest)
	if err != nil || eidx.tag != 0x02 {
		return nil, fmt.Errorf("snmp: PDU 缺 error-index")
	}
	idx, _ := decodeInteger(eidx.body)

	vbl, _, err := consumeTLV(pduRest)
	if err != nil || vbl.tag != 0x30 {
		return nil, fmt.Errorf("snmp: PDU 缺 varbind-list")
	}
	resp := &Response{ErrorStatus: int(status), ErrorIndex: int(idx), Varbinds: map[string]Value{}}
	vb := vbl.body
	for len(vb) > 0 {
		item, next, err := consumeTLV(vb)
		if err != nil {
			return nil, err
		}
		vb = next
		if item.tag != 0x30 {
			continue
		}
		oidTLV, itemRest, err := consumeTLV(item.body)
		if err != nil || oidTLV.tag != 0x06 {
			continue
		}
		oidStr := decodeOIDString(oidTLV.body)
		valTLV, _, err := consumeTLV(itemRest)
		if err != nil {
			continue
		}
		resp.Varbinds[oidStr] = Value{Type: valTLV.tag, Raw: valTLV.body}
	}
	return resp, nil
}

func decodeOIDString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var ints []uint64
	ints = append(ints, decodeBase128Part(raw)...)
	if len(ints) == 0 {
		return ""
	}
	first := ints[0] / 40
	second := ints[0] % 40
	if first > 2 {
		first, second = 2, ints[0]-80
	}
	out := fmt.Sprintf("%d.%d", first, second)
	for _, v := range ints[1:] {
		out += fmt.Sprintf(".%d", v)
	}
	return out
}

func decodeBase128Part(raw []byte) []uint64 {
	var out []uint64
	var cur uint64
	for _, b := range raw {
		cur = cur<<7 | uint64(b&0x7f)
		if b&0x80 == 0 {
			out = append(out, cur)
			cur = 0
		}
	}
	return out
}

type tlv struct {
	tag  byte
	body []byte
}

func parseTLV(raw []byte) (tlv, error) {
	if len(raw) < 2 {
		return tlv{}, fmt.Errorf("tlv 过短")
	}
	tag := raw[0]
	ln := int(raw[1])
	pos := 2
	if ln&0x80 != 0 {
		n := ln & 0x7f
		if n > 4 || len(raw) < 2+n {
			return tlv{}, fmt.Errorf("tlv 长度字节非法")
		}
		ln = 0
		for i := 0; i < n; i++ {
			ln = ln<<8 | int(raw[2+i])
		}
		pos = 2 + n
	}
	if ln < 0 || pos+ln > len(raw) {
		return tlv{}, fmt.Errorf("tlv 长度越界")
	}
	return tlv{tag: tag, body: raw[pos : pos+ln]}, nil
}

func consumeTLV(raw []byte) (tlv, []byte, error) {
	t, err := parseTLV(raw)
	if err != nil {
		return t, nil, err
	}
	head := 2
	if len(raw) > 1 && raw[1]&0x80 != 0 {
		head = 2 + int(raw[1]&0x7f)
	}
	return t, raw[head+len(t.body):], nil
}

// Get 单次 UDP 请求（超时毫秒）。返回解析后的响应。
func Get(addr, community string, oids []string, timeout time.Duration) (*Response, error) {
	req, err := BuildGetRequest(community, 1, oids)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n > int(math.MaxUint16) {
		n = int(math.MaxUint16)
	}
	return ParseResponse(buf[:n])
}
