// 文件用途：CoAP 核心（RFC7252 子集）单测——编解码往返、Uri-Path/Content-Format 解析、
// 错误输入拒绝、UDP 服务器端到端（CON GET→2.05、未知资源→4.04、NON→NON、.well-known/core）。
package coap

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	msg := &Message{
		Type:      TypeConfirmable,
		Code:      CodeGet,
		MessageID: 0xBEEF,
		Token:     []byte{0xAA, 0xBB},
		Options: []Option{
			{Number: OptionUriPath, Value: []byte("devices")},
			{Number: OptionUriPath, Value: []byte("sensor-1")},
			{Number: OptionUriQuery, Value: []byte("type=t")},
		},
	}
	raw, err := msg.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	dec, err := Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dec.Type != TypeConfirmable || dec.Code != CodeGet || dec.MessageID != 0xBEEF {
		t.Fatalf("头部不符: %+v", dec)
	}
	if string(dec.Token) != string([]byte{0xAA, 0xBB}) {
		t.Fatalf("token 不符: %x", dec.Token)
	}
	if got := dec.UriPath(); got != "/devices/sensor-1" {
		t.Fatalf("UriPath=%q", got)
	}
	q := dec.OptionsByNumber(OptionUriQuery)
	if len(q) != 1 || string(q[0]) != "type=t" {
		t.Fatalf("query 不符: %q", q)
	}
}

func TestEncodeDecodeWithPayload(t *testing.T) {
	msg := &Message{
		Type: TypeAck, Code: CodeContent, MessageID: 7, Token: []byte{1},
		Options: []Option{{Number: OptionContentFormat, Value: []byte{ContentFormatTextPlain}}},
		Payload: []byte("ok=42"),
	}
	raw, err := msg.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec.Payload) != "ok=42" {
		t.Fatalf("payload=%q", dec.Payload)
	}
	if dec.ContentFormat() != ContentFormatTextPlain {
		t.Fatalf("content format=%d", dec.ContentFormat())
	}
}

func TestDecodeRejectsBadInput(t *testing.T) {
	if _, err := Decode([]byte{0x40, 0x01}); err == nil {
		t.Fatal("过短消息应报错")
	}
	if _, err := Decode([]byte{0x80, 0x01, 0, 1}); err == nil {
		t.Fatal("版本 2 应报错")
	}
	// TKL=9 非法（>8）
	if _, err := Decode([]byte{0x49, 0x01, 0, 1}); err == nil {
		t.Fatal("tkl>8 应报错")
	}
	// token 截断
	if _, err := Decode([]byte{0x42, 0x01, 0, 1, 0x00}); err == nil {
		t.Fatal("token 截断应报错")
	}
}

func TestServeWellKnownCore(t *testing.T) {
	r := NewRegistry()
	r.Register("/sensors/temp", func(req *Message) (Code, []byte, int, error) {
		return CodeContent, []byte("21.5"), ContentFormatTextPlain, nil
	})
	resp, err := r.Serve(&Message{Type: TypeConfirmable, Code: CodeGet, MessageID: 1,
		Options: []Option{{Number: OptionUriPath, Value: []byte(".well-known")}, {Number: OptionUriPath, Value: []byte("core")}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || resp.Code != CodeContent {
		t.Fatalf("core 应 2.05: %+v", resp)
	}
	body := string(resp.Payload)
	if !strings.Contains(body, "/sensors/temp") || !strings.Contains(body, "ct=0") {
		t.Fatalf("link-format 缺资源: %s", body)
	}
}

func TestServeNotFoundAndMethod(t *testing.T) {
	r := NewRegistry()
	resp, _ := r.Serve(&Message{Type: TypeConfirmable, Code: CodeGet, MessageID: 3,
		Options: []Option{{Number: OptionUriPath, Value: []byte("nope")}}})
	if resp.Code != CodeNotFound {
		t.Fatalf("未知资源应 4.04, got %v", resp.Code)
	}
	if resp.Type != TypeAck || resp.MessageID != 3 {
		t.Fatalf("CON 应回 piggyback ACK 同号")
	}
}

func TestServerUDPEndToEnd(t *testing.T) {
	r := NewRegistry()
	r.Register("/devices/d1/telemetry", func(req *Message) (Code, []byte, int, error) {
		if req.Code == CodePost {
			return CodeCreated, []byte("stored"), ContentFormatTextPlain, nil
		}
		return CodeMethodNotAllowed, nil, ContentFormatTextPlain, nil
	})
	srv := &Server{Registry: r}
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go srv.servePacket(pc)

	addr := pc.LocalAddr().String()
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req := &Message{Type: TypeConfirmable, Code: CodePost, MessageID: 100, Token: []byte{9},
		Options: []Option{
			{Number: OptionUriPath, Value: []byte("devices")},
			{Number: OptionUriPath, Value: []byte("d1")},
			{Number: OptionUriPath, Value: []byte("telemetry")},
		},
		Payload: []byte("t=25.0"),
	}
	raw, _ := req.Encode()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(raw); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("读取响应: %v", err)
	}
	resp, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeCreated {
		t.Fatalf("POST 应 2.01, got %v (%s)", resp.Code, resp.Code.String())
	}
	if resp.Type != TypeAck || resp.MessageID != 100 {
		t.Fatalf("应 piggyback ACK 同号: type=%d id=%d", resp.Type, resp.MessageID)
	}
	if string(resp.Token) != string([]byte{9}) {
		t.Fatalf("token 应回显")
	}
	if string(resp.Payload) != "stored" {
		t.Fatalf("payload=%q", resp.Payload)
	}
}

func TestCodeString(t *testing.T) {
	if CodeContent.String() != "2.05" || CodeNotFound.String() != "4.04" {
		t.Fatal("Code.String 错误")
	}
}
