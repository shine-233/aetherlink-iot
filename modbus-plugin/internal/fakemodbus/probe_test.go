package fakemodbus

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
	"time"
)

func TestRawFrameProbe(t *testing.T) {
	s := Start(t)
	s.SetInput(50, 0xFFCE)
	conn, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	req := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x04, 0x00, 0x32, 0x00, 0x01}
	if _, err := conn.Write(req); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	t.Logf("resp bytes=%d hex=%s", n, hex.EncodeToString(buf[:n]))
	t.Logf("len field=%d func=0x%x unit=%d byteCount=%d value=%d",
		binary.BigEndian.Uint16(buf[4:6]), buf[7], buf[6], buf[8], binary.BigEndian.Uint16(buf[9:11]))
}
