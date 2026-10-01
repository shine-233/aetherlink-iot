// 文件用途：SNMPv3 客户端层单测（TB-22）——构造参数校验、v3 报文封装/摘要回填、
// 响应验签（篡改与错误密钥拒绝）、engine discovery，及内嵌 v3 agent 的 UDP 全链路。
package snmp

import (
	"bytes"
	"net"
	"testing"
	"time"
)

const (
	testV3User = "authUser"
	testV3Pass = "authkey123"
	testV3OID  = "1.3.6.1.2.1.1.3.0"
)

// agentKeys 内嵌 agent 的 USM 配置（与客户端同源 usm.go 口径）。
type agentKeys struct {
	engineID []byte
	auth     AuthProtocol
	kul      []byte
}

func newAgentKeys(t *testing.T, auth AuthProtocol) *agentKeys {
	t.Helper()
	engineID := []byte{0x80, 0x00, 0x1f, 0x88, 0x80, 0x42, 0x42, 0x42}
	ku, err := PasswordToKey(auth, testV3Pass, engineID)
	if err != nil {
		t.Fatal(err)
	}
	return &agentKeys{engineID: engineID, auth: auth, kul: LocalizeKey(auth, ku, engineID)}
}

// startV3TestAgent 启动内嵌 SNMPv3 agent：discovery 探测回 Report（携带 engineID/boots/time），
// 认证 GetRequest 验签后回带认证摘要的 GetResponse。
func startV3TestAgent(t *testing.T, keys *agentKeys, binds []VarBind) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 65536)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			raw := append([]byte{}, buf[:n]...)
			req, rerr := ParseV3Request(raw)
			if rerr != nil {
				continue
			}
			if !req.VerifyAuth(keys.auth, keys.kul) {
				// 未认证请求：按 discovery 语义回 Report（携带权威引擎参数）。
				report, berr := BuildV3Report(V3SecurityParams{
					EngineID: keys.engineID, EngineBoots: 3, EngineTime: 1500,
					MsgID: req.MsgID, RequestID: req.RequestID,
				}, []VarBind{{OID: usmStatsUnknownEngineIDs, Value: OctetStringValue(string(keys.engineID))}})
				if berr != nil {
					continue
				}
				_, _ = conn.WriteTo(report, addr)
				continue
			}
			resp, rerr := BuildV3GetResponse(V3SecurityParams{
				UserName: req.Params.UserName, Auth: keys.auth, AuthKey: keys.kul,
				EngineID: keys.engineID, EngineBoots: 3, EngineTime: 1500,
				MsgID: req.MsgID, RequestID: req.RequestID,
			}, 0, 0, binds)
			if rerr != nil {
				continue
			}
			_, _ = conn.WriteTo(resp, addr)
		}
	}()
	return conn.LocalAddr().String(), func() {
		_ = conn.Close()
		<-done
	}
}

func TestNewV3ClientParamValidation(t *testing.T) {
	cases := []struct {
		name        string
		addr, user  string
		auth        AuthProtocol
		pass        string
		wantErrText string
	}{
		{"空地址", "", testV3User, AuthHMACSHA, testV3Pass, "地址"},
		{"空用户", "127.0.0.1:161", "", AuthHMACSHA, testV3Pass, "用户名"},
		{"无认证协议", "127.0.0.1:161", testV3User, AuthNone, testV3Pass, "md5/sha"},
		{"空口令", "127.0.0.1:161", testV3User, AuthHMACMD5, "", "口令"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewV3Client(tc.addr, tc.user, tc.auth, tc.pass, 200*time.Millisecond)
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(tc.wantErrText)) {
				t.Fatalf("期望错误含 %q，实际 %v", tc.wantErrText, err)
			}
		})
	}
	// 显式 engineID 构造：Kul 立即派生（构造参数可检视）。
	keys := newAgentKeys(t, AuthHMACSHA)
	c, err := NewV3ClientWithEngineID("127.0.0.1:161", testV3User, AuthHMACSHA, testV3Pass, keys.engineID, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(c.EngineID(), keys.engineID) {
		t.Fatalf("EngineID 不符: %x", c.EngineID())
	}
	if !bytes.Equal(c.LocalizedKey(), keys.kul) {
		t.Fatal("Kul 派生与 usm.go 口径不一致")
	}
	// discovery 模式构造：engineID/Kul 未定（nil）。
	d, err := NewV3Client("127.0.0.1:161", testV3User, AuthHMACMD5, testV3Pass, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if d.EngineID() != nil || d.LocalizedKey() != nil {
		t.Fatal("discovery 模式构造期不得预置 engineID/Kul")
	}
	if d.Auth != AuthHMACMD5 || d.UserName != testV3User || d.Addr != "127.0.0.1:161" {
		t.Fatalf("构造参数不符: %+v", d)
	}
}

func TestBuildV3GetRequestStructureAndDigest(t *testing.T) {
	keys := newAgentKeys(t, AuthHMACMD5)
	raw, err := BuildV3GetRequest(V3SecurityParams{
		UserName: testV3User, Auth: AuthHMACMD5, AuthKey: keys.kul,
		EngineID: keys.engineID, EngineBoots: 2, EngineTime: 300,
		MsgID: 42, RequestID: 7,
	}, []string{testV3OID})
	if err != nil {
		t.Fatal(err)
	}
	// 请求可被 agent 侧解析：安全参数逐字段一致，摘要通过常量时间校验。
	info, err := ParseV3Request(raw)
	if err != nil {
		t.Fatalf("v3 请求解析: %v", err)
	}
	if info.Params.UserName != testV3User {
		t.Fatalf("userName=%q", info.Params.UserName)
	}
	if !bytes.Equal(info.Params.EngineID, keys.engineID) || info.Params.EngineBoots != 2 || info.Params.EngineTime != 300 {
		t.Fatalf("引擎参数不符: %+v", info.Params)
	}
	if info.MsgID != 42 || info.RequestID != 7 {
		t.Fatalf("msgID/requestID=%d/%d", info.MsgID, info.RequestID)
	}
	if len(info.OIDs) != 1 || info.OIDs[0] != testV3OID {
		t.Fatalf("OID 解析不符: %v", info.OIDs)
	}
	if !info.VerifyAuth(AuthHMACMD5, keys.kul) {
		t.Fatal("回填摘要应通过校验")
	}
	// 篡改任一字节 → 重新解析后摘要校验拒绝（防报文中途篡改）。
	tampered := append([]byte{}, raw...)
	tampered[len(tampered)-2] ^= 0x01
	if info2, perr := ParseV3Request(tampered); perr == nil && info2.VerifyAuth(AuthHMACMD5, keys.kul) {
		t.Fatal("篡改报文应校验失败")
	}
	// 错误密钥 → 拒绝。
	wrong := newAgentKeys(t, AuthHMACSHA)
	if info.VerifyAuth(AuthHMACMD5, wrong.kul) {
		t.Fatal("错误密钥应校验失败")
	}
	if _, err := BuildV3GetRequest(V3SecurityParams{}, nil); err == nil {
		t.Fatal("空 OID 列表应报错")
	}
}

func TestParseV3ResponseRejectsTamperAndWrongKey(t *testing.T) {
	keys := newAgentKeys(t, AuthHMACSHA)
	binds := []VarBind{{OID: testV3OID, Value: IntegerValue(999)}}
	raw, err := BuildV3GetResponse(V3SecurityParams{
		UserName: testV3User, Auth: AuthHMACSHA, AuthKey: keys.kul,
		EngineID: keys.engineID, EngineBoots: 1, EngineTime: 10,
		MsgID: 1, RequestID: 1,
	}, 0, 0, binds)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ParseV3Response(raw, AuthHMACSHA, keys.kul)
	if err != nil {
		t.Fatalf("合法响应解析: %v", err)
	}
	if v, err := resp.Varbinds[testV3OID].AsInt(); err != nil || v != 999 {
		t.Fatalf("varbind 解析=%v err=%v", v, err)
	}
	// 篡改 payload → 验签拒绝。
	tampered := append([]byte{}, raw...)
	tampered[len(tampered)-1] ^= 0xFF
	if _, err := ParseV3Response(tampered, AuthHMACSHA, keys.kul); err == nil {
		t.Fatal("篡改响应应验签失败")
	}
	// 错误密钥 → 验签拒绝。
	if _, err := ParseV3Response(raw, AuthHMACSHA, newAgentKeys(t, AuthHMACMD5).kul); err == nil {
		t.Fatal("错误密钥应验签失败")
	}
	// v2c 报文（版本 1）→ v3 解析拒绝。
	v2c, _ := BuildGetResponse("public", 1, 0, 0, binds)
	if _, err := ParseV3Response(v2c, AuthHMACSHA, keys.kul); err == nil {
		t.Fatal("v2c 报文不得经 v3 解析通过")
	}
}

func TestV3ClientDiscoveryAndAuthGetEndToEnd(t *testing.T) {
	keys := newAgentKeys(t, AuthHMACMD5)
	binds := []VarBind{
		{OID: testV3OID, Value: IntegerValue(12345)},
		{OID: "1.3.6.1.2.1.1.1.0", Value: OctetStringValue("v3-agent")},
	}
	addr, stop := startV3TestAgent(t, keys, binds)
	defer stop()

	// discovery 模式：首次 Get 自动探测 engineID 再认证请求。
	c, err := NewV3Client(addr, testV3User, AuthHMACMD5, testV3Pass, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Get([]string{testV3OID, "1.3.6.1.2.1.1.1.0"})
	if err != nil {
		t.Fatalf("v3 Get: %v", err)
	}
	if resp.ErrorStatus != 0 {
		t.Fatalf("错误状态=%d", resp.ErrorStatus)
	}
	if v, err := resp.Varbinds[testV3OID].AsInt(); err != nil || v != 12345 {
		t.Fatalf("varbind=%v err=%v", v, err)
	}
	if s, ok := resp.Varbinds["1.3.6.1.2.1.1.1.0"].AsString(); !ok || s != "v3-agent" {
		t.Fatalf("OCTET STRING=%q ok=%v", s, ok)
	}
	// discovery 参数回填正确。
	if !bytes.Equal(c.EngineID(), keys.engineID) {
		t.Fatalf("discovery engineID=%x 期望 %x", c.EngineID(), keys.engineID)
	}
	if !bytes.Equal(c.LocalizedKey(), keys.kul) {
		t.Fatal("discovery 后 Kul 派生不符")
	}
	// 空 OID 拒绝。
	if _, err := c.Get(nil); err == nil {
		t.Fatal("空 OID 列表应报错")
	}
}

func TestDiscoverEngineAgainstReport(t *testing.T) {
	keys := newAgentKeys(t, AuthHMACSHA)
	addr, stop := startV3TestAgent(t, keys, []VarBind{{OID: testV3OID, Value: IntegerValue(1)}})
	defer stop()
	engineID, boots, etime, err := DiscoverEngine(addr, time.Second)
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if !bytes.Equal(engineID, keys.engineID) || boots != 3 || etime != 1500 {
		t.Fatalf("discovery 结果不符: id=%x boots=%d time=%d", engineID, boots, etime)
	}
	// 不可达目标 → fail-closed 错误。
	if _, _, _, err := DiscoverEngine("127.0.0.1:1", 150*time.Millisecond); err == nil {
		t.Fatal("不可达目标应报错")
	}
}
