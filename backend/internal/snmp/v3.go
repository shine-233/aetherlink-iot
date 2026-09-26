// 文件用途：SNMPv3 authNoPriv 客户端报文层（TB-22）——把 usm.go 安全参数封装为 RFC 3412
// v3 报文，并提供 engine discovery 与可内嵌测试 agent 的构建/解析入口。
// 核心逻辑：buildV3Frame 统一组装（version=3/msgGlobalData/msgSecurityParameters/scopedPDU）；
//   discovery：noAuthNoPriv 可上报探测 → 响应 msgSecurityParameters 回带权威 engineID/boots/time；
//   认证 Get：口令→Ku→Kul 本地化 → HMAC-96 摘要回填 → 响应摘要常量时间校验。
// 关键注意事项：
//   - 仅实现 authNoPriv（authPriv 加密未实现，与 usm.go 同一边界口径），上层点表校验拒绝 priv 请求；
//   - 密钥派生沿用同包 usm.go 自洽口径（Ku=PasswordToKey 含 engineID，Kul=LocalizeKey），
//     与外网标准 agent 的互操作属运行时边界（见 usm_test.go 头注），测试内嵌 agent 两侧同源；
//   - 摘要占位为 12 字节唯一明文标记：发送按 RFC 3414 先零化占位计算摘要、再回填真实摘要；
//     响应/agent 侧解析时零化摘要段后常量时间验签；
//   - 客户端内存持有口令用于 discovery 后派生 Kul（v3 客户端常规做法，不可序列化外泄）；
//   - v2c 路径（snmp.Get）完全不受影响，v3 全部经本文件入口。
package snmp

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// RFC 3412 常量。
const (
	snmpVersion3       = 3
	v3SecurityModelUSM = 3
	v3MsgMaxSize       = 65507
	// msgFlags（RFC 3412 §6.5）：0x01 reportable、0x02 authenticated、0x04 encrypted。
	v3FlagReportable = 0x01
	v3FlagAuth       = 0x02
	// Report PDU（RFC 3412 §4.3）：discovery 响应/引擎统计错误的载体。
	pduReport = 0xA8
)

// usmStatsUnknownEngineIDs 引擎统计 OID：discovery 探测请求按规范携带该 OID。
const usmStatsUnknownEngineIDs = "1.3.6.1.6.3.15.1.1.4.0"

// authDigestMarker 12 字节摘要占位（报文内唯一明文标记，发送前经 ReplaceAuthParamsInMessage 回填）。
var authDigestMarker = []byte("AUTH*PARAMS!")

// v3MsgCounter 进程级报文号计数器（RFC 3412 要求请求间 msgID 可区分）。
var v3MsgCounter atomic.Int32

func nextV3MsgID() int32 { return v3MsgCounter.Add(1) & 0x7fffffff }

// V3SecurityParams 单个 v3 报文的安全参数（对齐 RFC 3412 报文头字段）。
type V3SecurityParams struct {
	UserName    string
	Auth        AuthProtocol
	AuthKey     []byte // 本地化密钥 Kul（发送前回填摘要用）
	EngineID    []byte
	EngineBoots int32
	EngineTime  int32
	MsgID       int32
	RequestID   int32
}

// buildV3Frame 组装 v3 报文（version/msgGlobalData/msgSecurityParameters/scopedPDU）；
// authenticate=true 时以 AuthKey 计算 HMAC-96 摘要并回填占位。
func buildV3Frame(p V3SecurityParams, msgFlags byte, scopedPDU []byte, authenticate bool) ([]byte, error) {
	globalData := buildTLV(0x30,
		append(buildTLV(0x02, berIntBytes(int64(p.MsgID))),
			append(buildTLV(0x02, berUintBytes(v3MsgMaxSize)),
				append(buildTLV(0x04, []byte{msgFlags}),
					buildTLV(0x02, berIntBytes(v3SecurityModelUSM))...)...)...))
	usm := &USMSecurityParams{
		EngineID:    p.EngineID,
		EngineBoots: p.EngineBoots,
		EngineTime:  p.EngineTime,
		UserName:    p.UserName,
		AuthParams:  append([]byte{}, authDigestMarker...),
	}
	securityParams := buildTLV(0x04, usm.MarshalUSMParams())
	msg := buildTLV(0x30,
		append(buildTLV(0x02, []byte{snmpVersion3}),
			append(globalData,
				append(securityParams, scopedPDU...)...)...))
	if authenticate {
		// RFC 3414：摘要按「摘要段全零」的整报文计算——先零化占位再哈希，
		// 最后回填真实摘要（与 parseV3Frame 的零化验签口径一致）。
		zeroed, ok := ReplaceAuthParamsInMessage(msg, authDigestMarker, make([]byte, 12))
		if !ok {
			return nil, fmt.Errorf("snmp: v3 摘要占位零化失败")
		}
		digest, err := ComputeAuthDigest(p.Auth, p.AuthKey, zeroed)
		if err != nil {
			return nil, err
		}
		out, ok := ReplaceAuthParamsInMessage(msg, authDigestMarker, digest)
		if !ok {
			return nil, fmt.Errorf("snmp: v3 摘要占位回填失败")
		}
		msg = out
	}
	return msg, nil
}

// rebuildScopedPDU 以给定 PDU 重组 scopedPDU（contextEngineID 取权威侧，contextName 空）。
func rebuildScopedPDU(p V3SecurityParams, pdu []byte) []byte {
	return buildTLV(0x30,
		append(buildTLV(0x04, p.EngineID),
			append(buildTLV(0x04, nil), pdu...)...))
}

// BuildV3GetRequest 构建 SNMPv3 authNoPriv GetRequest（已回填认证摘要）。
func BuildV3GetRequest(p V3SecurityParams, oids []string) ([]byte, error) {
	if len(oids) == 0 {
		return nil, fmt.Errorf("snmp: OID 列表为空")
	}
	varbinds, err := buildVarbindListForGet(oids)
	if err != nil {
		return nil, err
	}
	pdu := buildPDU(pduGetRequest, p.RequestID, 0, 0, varbinds)
	return buildV3Frame(p, v3FlagReportable|v3FlagAuth, rebuildScopedPDU(p, pdu), true)
}

// BuildV3GetResponse 构建带认证的 v3 GetResponse（内嵌测试 agent 用）。
func BuildV3GetResponse(p V3SecurityParams, errorStatus, errorIndex int, binds []VarBind) ([]byte, error) {
	if len(binds) == 0 {
		return nil, fmt.Errorf("snmp: v3 GetResponse 至少一条 varbind")
	}
	varbinds, err := buildVarbindList(binds)
	if err != nil {
		return nil, err
	}
	pdu := buildPDU(pduGetResponse, p.RequestID, errorStatus, errorIndex, varbinds)
	return buildV3Frame(p, v3FlagReportable|v3FlagAuth, rebuildScopedPDU(p, pdu), true)
}

// BuildV3Report 构建 discovery 探测的 Report PDU（无需认证，内嵌测试 agent 用）。
func BuildV3Report(p V3SecurityParams, binds []VarBind) ([]byte, error) {
	if len(binds) == 0 {
		return nil, fmt.Errorf("snmp: v3 Report 至少一条 varbind")
	}
	varbinds, err := buildVarbindList(binds)
	if err != nil {
		return nil, err
	}
	pdu := buildPDU(pduReport, p.RequestID, 0, 0, varbinds)
	return buildV3Frame(p, v3FlagReportable, rebuildScopedPDU(p, pdu), false)
}

// tlvHeadLen 返回 raw 起始 TLV 的头部长度（tag+len 字节数）。
func tlvHeadLen(raw []byte) int {
	if len(raw) < 2 {
		return len(raw)
	}
	if raw[1]&0x80 == 0 {
		return 2
	}
	return 2 + int(raw[1]&0x7f)
}

// v3Frame v3 报文骨架解析结果。
type v3Frame struct {
	MsgID     int32
	MsgFlags  byte
	Params    *USMSecurityParams
	ScopedPDU []byte // scopedPDU 内层内容（contextEngineID/contextName/PDU）
	zeroed    []byte // 摘要段零化后的整报文副本（验签输入）
}

// parseV3Frame 解析 v3 报文骨架：version/globalData/securityParameters/scopedPDU，
// 并产出「摘要零化副本」供验签（响应侧与内嵌 agent 共用，避免摘要自指参与 HMAC）。
func parseV3Frame(raw []byte) (*v3Frame, error) {
	msg, err := parseTLV(raw)
	if err != nil || msg.tag != 0x30 {
		return nil, fmt.Errorf("snmp: v3 报文非 SEQUENCE")
	}
	ver, rest, err := consumeTLV(msg.body)
	if err != nil || ver.tag != 0x02 {
		return nil, fmt.Errorf("snmp: v3 报文缺 version")
	}
	if v, _ := decodeInteger(ver.body); v != snmpVersion3 {
		return nil, fmt.Errorf("snmp: 不支持版本 %d（需 v3=3）", v)
	}
	glob, rest, err := consumeTLV(rest)
	if err != nil || glob.tag != 0x30 {
		return nil, fmt.Errorf("snmp: v3 报文缺 msgGlobalData")
	}
	f := &v3Frame{}
	gid, gRest, err := consumeTLV(glob.body)
	if err != nil || gid.tag != 0x02 {
		return nil, fmt.Errorf("snmp: v3 globalData 缺 msgID")
	}
	f.MsgID = int32(mustInt(gid.body))
	if _, gRest, err = consumeTLV(gRest); err != nil { // msgMaxSize
		return nil, fmt.Errorf("snmp: v3 globalData 缺 msgMaxSize")
	}
	gflags, _, err := consumeTLV(gRest)
	if err != nil || gflags.tag != 0x04 {
		return nil, fmt.Errorf("snmp: v3 globalData 缺 msgFlags")
	}
	if len(gflags.body) > 0 {
		f.MsgFlags = gflags.body[0]
	}
	// securityParameters：记录整报文内偏移，摘要是其内容末 12 字节。
	secOffset := len(raw) - len(rest)
	sec, rest, err := consumeTLV(rest)
	if err != nil || sec.tag != 0x04 {
		return nil, fmt.Errorf("snmp: v3 报文缺 msgSecurityParameters")
	}
	params, err := UnmarshalUSMParams(sec.body)
	if err != nil {
		return nil, err
	}
	f.Params = params
	if len(params.AuthParams) != 12 {
		return nil, fmt.Errorf("snmp: v3 authParams 长度非法 %d", len(params.AuthParams))
	}
	scoped, _, err := consumeTLV(rest)
	if err != nil || scoped.tag != 0x30 {
		return nil, fmt.Errorf("snmp: v3 报文缺 scopedPDU")
	}
	f.ScopedPDU = scoped.body
	authOffset := secOffset + tlvHeadLen(raw[secOffset:]) + len(sec.body) - 12
	if authOffset < 0 || authOffset+12 > len(raw) {
		return nil, fmt.Errorf("snmp: v3 authParams 偏移越界")
	}
	zeroed := append([]byte{}, raw...)
	for i := 0; i < 12; i++ {
		zeroed[authOffset+i] = 0
	}
	f.zeroed = zeroed
	return f, nil
}

// parseV3ScopedPDU 取 scopedPDU 内的 PDU（跳过 contextEngineID/contextName）并校验 PDU 类型。
func parseV3ScopedPDU(scoped []byte, wantPDU byte) (tlv, error) {
	ctxEngine, restSP, err := consumeTLV(scoped)
	if err != nil || ctxEngine.tag != 0x04 {
		return tlv{}, fmt.Errorf("snmp: scopedPDU 缺 contextEngineID")
	}
	_ = ctxEngine
	ctxName, restSP, err := consumeTLV(restSP)
	if err != nil || ctxName.tag != 0x04 {
		return tlv{}, fmt.Errorf("snmp: scopedPDU 缺 contextName")
	}
	_ = ctxName
	pdu, _, err := consumeTLV(restSP)
	if err != nil {
		return tlv{}, err
	}
	if pdu.tag != wantPDU {
		return tlv{}, fmt.Errorf("snmp: v3 PDU 类型不符 (0x%02x，期望 0x%02x)", pdu.tag, wantPDU)
	}
	return pdu, nil
}

// ParseV3Response 解析并验签 SNMPv3 authNoPriv GetResponse：
// 以 kul 对摘要零化副本做常量时间校验，验签失败/PDU 非法一律拒绝（fail-closed）。
func ParseV3Response(raw []byte, auth AuthProtocol, kul []byte) (*Response, error) {
	frame, err := parseV3Frame(raw)
	if err != nil {
		return nil, err
	}
	if !VerifyAuthDigest(auth, kul, frame.zeroed, frame.Params.AuthParams) {
		return nil, fmt.Errorf("snmp: v3 响应认证校验失败")
	}
	pdu, err := parseV3ScopedPDU(frame.ScopedPDU, pduGetResponse)
	if err != nil {
		return nil, err
	}
	return parseResponsePDU(pdu)
}

// V3RequestInfo 内嵌测试 agent 视角的 v3 请求。
type V3RequestInfo struct {
	MsgID     int32
	RequestID int32
	Params    *USMSecurityParams
	OIDs      []string
	zeroed    []byte // 摘要零化整报文（VerifyAuth 输入）
}

// ParseV3Request 解析 v3 GetRequest（内嵌测试 agent 用；GetRequest PDU 体与
// GetResponse 布局一致，复用 parseResponsePDU 还原 varbinds）。
func ParseV3Request(raw []byte) (*V3RequestInfo, error) {
	frame, err := parseV3Frame(raw)
	if err != nil {
		return nil, err
	}
	pdu, err := parseV3ScopedPDU(frame.ScopedPDU, pduGetRequest)
	if err != nil {
		return nil, err
	}
	resp, err := parseResponsePDU(pdu)
	if err != nil {
		return nil, err
	}
	oids := make([]string, 0, len(resp.Varbinds))
	for oid := range resp.Varbinds {
		oids = append(oids, oid)
	}
	return &V3RequestInfo{
		MsgID:     frame.MsgID,
		RequestID: int32(mustInt(pduRequestID(frame.ScopedPDU))),
		Params:    frame.Params,
		OIDs:      oids,
		zeroed:    frame.zeroed,
	}, nil
}

// pduRequestID 从 scopedPDU 内取出 PDU 的 request-id 原始字节。
func pduRequestID(scoped []byte) []byte {
	_, restSP, err := consumeTLV(scoped)
	if err != nil {
		return nil
	}
	_, restSP, err = consumeTLV(restSP)
	if err != nil {
		return nil
	}
	pdu, _, err := consumeTLV(restSP)
	if err != nil {
		return nil
	}
	rid, _, err := consumeTLV(pdu.body)
	if err != nil {
		return nil
	}
	return rid.body
}

// VerifyAuth 以本地化密钥常量时间校验请求摘要（agent 侧准入判断）。
func (r *V3RequestInfo) VerifyAuth(auth AuthProtocol, kul []byte) bool {
	return VerifyAuthDigest(auth, kul, r.zeroed, r.Params.AuthParams)
}

// DiscoverEngine 完成 RFC 3411 引擎发现：发送 noAuthNoPriv 可上报探测，从响应
// msgSecurityParameters 取回权威 engineID/boots/time。响应兼容 Report(0xA8)/GetResponse(0xA2)。
func DiscoverEngine(addr string, timeout time.Duration) (engineID []byte, boots, engineTime int32, err error) {
	varbinds, err := buildVarbindListForGet([]string{usmStatsUnknownEngineIDs})
	if err != nil {
		return nil, 0, 0, err
	}
	p := V3SecurityParams{MsgID: nextV3MsgID(), RequestID: nextV3MsgID()}
	pdu := buildPDU(pduGetRequest, p.RequestID, 0, 0, varbinds)
	probe, err := buildV3Frame(p, v3FlagReportable, rebuildScopedPDU(p, pdu), false)
	if err != nil {
		return nil, 0, 0, err
	}
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("snmp: v3 discovery 连接失败: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(probe); err != nil {
		return nil, 0, 0, fmt.Errorf("snmp: v3 discovery 发送失败: %w", err)
	}
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("snmp: v3 discovery 响应超时: %w", err)
	}
	frame, err := parseV3Frame(buf[:n])
	if err != nil {
		return nil, 0, 0, fmt.Errorf("snmp: v3 discovery 响应解析失败: %w", err)
	}
	if len(frame.Params.EngineID) == 0 {
		return nil, 0, 0, fmt.Errorf("snmp: v3 discovery 响应未携带 engineID")
	}
	return frame.Params.EngineID, frame.Params.EngineBoots, frame.Params.EngineTime, nil
}

// V3Client SNMPv3 authNoPriv Get 客户端（TB-22）。
// 无会话语义与 v2c Get 一致：每次请求独立 UDP 事务；engineID 未知时首次 Get 前自动 discovery。
type V3Client struct {
	Addr     string
	UserName string
	Auth     AuthProtocol

	timeout    time.Duration
	passphrase string // 内存持有：discovery 取得 engineID 后派生 Kul 用
	engineID   []byte
	boots      int32
	etime      int32
	kul        []byte
}

// NewV3Client 构造 v3 客户端（discovery 模式：engineID 由首次 Get 前的探测取得）。
func NewV3Client(addr, user string, auth AuthProtocol, passphrase string, timeout time.Duration) (*V3Client, error) {
	return NewV3ClientWithEngineID(addr, user, auth, passphrase, nil, timeout)
}

// NewV3ClientWithEngineID 同 NewV3Client，但允许显式提供权威 engineID（已知引擎场景免 discovery）。
// 参数校验 fail-closed：地址/用户/口令必填，认证协议仅 md5/sha。
func NewV3ClientWithEngineID(addr, user string, auth AuthProtocol, passphrase string, engineID []byte, timeout time.Duration) (*V3Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("snmp: v3 客户端地址为空")
	}
	if user == "" {
		return nil, fmt.Errorf("snmp: v3 用户名为空")
	}
	if auth != AuthHMACMD5 && auth != AuthHMACSHA {
		return nil, fmt.Errorf("snmp: v3 需要认证协议 md5/sha，实际 %v", auth)
	}
	if passphrase == "" {
		return nil, fmt.Errorf("snmp: v3 认证口令为空")
	}
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	c := &V3Client{Addr: addr, UserName: user, Auth: auth, timeout: timeout, passphrase: passphrase}
	if len(engineID) > 0 {
		c.engineID = append([]byte{}, engineID...)
		c.boots = 1
		c.etime = 1
		if err := c.deriveKeys(); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// deriveKeys 以权威 engineID 派生本地化密钥（Ku→Kul，口径同 usm.go）。
func (c *V3Client) deriveKeys() error {
	ku, err := PasswordToKey(c.Auth, c.passphrase, c.engineID)
	if err != nil {
		return err
	}
	c.kul = LocalizeKey(c.Auth, ku, c.engineID)
	return nil
}

// EngineID 返回已取得的权威引擎 ID（未 discovery 时 nil，测试与诊断面用）。
func (c *V3Client) EngineID() []byte { return c.engineID }

// LocalizedKey 返回当前本地化密钥 Kul（未 discovery 时 nil，测试与诊断面用）。
func (c *V3Client) LocalizedKey() []byte { return c.kul }

// Get 单次认证 Get：engineID 未知时先 discovery（独立 UDP 事务）再认证请求。
// discovery 与请求各占用一次 timeout 预算（最坏 2×timeout，量级与 v2c 单事务预算一致）。
func (c *V3Client) Get(oids []string) (*Response, error) {
	if len(oids) == 0 {
		return nil, fmt.Errorf("snmp: OID 列表为空")
	}
	if c.engineID == nil {
		engineID, boots, etime, err := DiscoverEngine(c.Addr, c.timeout)
		if err != nil {
			return nil, err
		}
		c.engineID = engineID
		c.boots = boots
		c.etime = etime
		if err := c.deriveKeys(); err != nil {
			return nil, err
		}
	}
	return c.authGet(oids)
}

// authGet 认证 Get 事务：构建带摘要请求 → UDP 收发 → 验签解析。
func (c *V3Client) authGet(oids []string) (*Response, error) {
	varbinds, err := buildVarbindListForGet(oids)
	if err != nil {
		return nil, err
	}
	msgID := nextV3MsgID()
	p := V3SecurityParams{
		UserName:    c.UserName,
		Auth:        c.Auth,
		AuthKey:     c.kul,
		EngineID:    c.engineID,
		EngineBoots: c.boots,
		EngineTime:  c.etime,
		MsgID:       msgID,
		RequestID:   msgID,
	}
	pdu := buildPDU(pduGetRequest, p.RequestID, 0, 0, varbinds)
	req, err := buildV3Frame(p, v3FlagReportable|v3FlagAuth, rebuildScopedPDU(p, pdu), true)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("udp", c.Addr, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("snmp: v3 Get 连接失败: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("snmp: v3 Get 发送失败: %w", err)
	}
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("snmp: v3 Get 响应超时: %w", err)
	}
	return ParseV3Response(buf[:n], c.Auth, c.kul)
}
