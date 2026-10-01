// 文件用途：Server 填充 Message.RemoteAddr 的单测（TB-22 多客户端隔离的归因载体）——
// UDP 全链路校验处理器可见的 RemoteAddr 与客户端源地址一致；Encode/Decode 往返不携带该字段。
package coap

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestServerFillsRemoteAddr(t *testing.T) {
	var mu sync.Mutex
	var got string
	r := NewRegistry()
	r.Register("/whoami", func(req *Message) (Code, []byte, int, error) {
		mu.Lock()
		got = req.RemoteAddr
		mu.Unlock()
		return CodeContent, []byte("ok"), ContentFormatTextPlain, nil
	})
	srv := &Server{Registry: r}
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go srv.servePacket(pc)

	conn, err := net.Dial("udp", pc.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	req := &Message{Type: TypeConfirmable, Code: CodeGet, MessageID: 7,
		Options: []Option{{Number: OptionUriPath, Value: []byte("whoami")}}}
	raw, _ := req.Encode()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(raw); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("读取响应: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got == "" {
		t.Fatal("处理器必须收到 RemoteAddr")
	}
	if host, _, err := net.SplitHostPort(got); err != nil || host != "127.0.0.1" {
		t.Fatalf("RemoteAddr=%q 应为客户端源地址", got)
	}
}

func TestRemoteAddrNotEncodedOnWire(t *testing.T) {
	m := &Message{Type: TypeConfirmable, Code: CodeGet, MessageID: 1, RemoteAddr: "10.0.0.1:5683"}
	raw, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if back.RemoteAddr != "" {
		t.Fatal("RemoteAddr 为服务器出站字段，不得进入线上编码")
	}
}
