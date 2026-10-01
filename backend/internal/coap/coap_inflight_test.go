package coap

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// UDP 洪泛下处理 goroutine 必须受 MaxInflight 约束：槽位占满时后续数据报被丢弃而非无界起协程。
func TestServerBoundsInflightHandlers(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	r := NewRegistry()
	r.Register("/slow", func(req *Message) (Code, []byte, int, error) {
		calls.Add(1)
		<-release
		return CodeContent, []byte("ok"), ContentFormatTextPlain, nil
	})
	srv := &Server{Registry: r, MaxInflight: 2}
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
	for i := 0; i < 20; i++ {
		req := &Message{Type: TypeNonConfirm, Code: CodeGet, MessageID: uint16(i),
			Options: []Option{{Number: OptionUriPath, Value: []byte("slow")}}}
		raw, _ := req.Encode()
		if _, err := conn.Write(raw); err != nil {
			t.Fatal(err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // 给多余数据报留出被（错误地）并发处理的机会
	if got := calls.Load(); got != 2 {
		close(release)
		t.Fatalf("concurrent handler calls = %d, want exactly MaxInflight=2", got)
	}
	close(release)
}
