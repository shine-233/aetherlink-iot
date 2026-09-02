// Package fakemodbus 提供内嵌最小 Modbus TCP 从站，仅供插件单元测试使用。
// 支持功能码：FC3 读保持寄存器、FC4 读输入寄存器、FC5 写线圈、FC6 写单寄存器、FC16 写多寄存器。
// 实现面向单连接顺序事务，不追求生产级并发语义。
package fakemodbus

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
)

// Server 测试从站实例。
type Server struct {
	listener net.Listener
	mu       sync.Mutex
	holding  map[uint16]uint16
	input    map[uint16]uint16
	coil     map[uint16]bool
}

// Start 在随机端口启动从站，并在测试结束时自动关闭。
func Start(t testing.TB) *Server {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake server listen: %v", err)
	}
	s := &Server{
		listener: listener,
		holding:  map[uint16]uint16{},
		input:    map[uint16]uint16{},
		coil:     map[uint16]bool{},
	}
	go s.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return s
}

// Addr 返回监听地址（host:port）。
func (s *Server) Addr() string { return s.listener.Addr().String() }

// SetHolding 预置保持寄存器值。
func (s *Server) SetHolding(addr, value uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holding[addr] = value
}

// SetInput 预置输入寄存器值。
func (s *Server) SetInput(addr, value uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.input[addr] = value
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	header := make([]byte, 7)
	for {
		if _, err := readFull(conn, header); err != nil {
			return
		}
		length := binary.BigEndian.Uint16(header[4:6])
		if length < 2 {
			return
		}
		// MBAP length 覆盖 unitId+PDU；header 已消费 unitId，因此 body 读 length-1 字节。
		body := make([]byte, length-1)
		if _, err := readFull(conn, body); err != nil {
			return
		}
		functionCode := body[0]
		response := s.dispatch(functionCode, body[1:])
		// 响应帧：MBAP(7B: tid/pid/len/unitId) + 功能码 + 数据；len 覆盖 unitId 起的全部字节。
		out := make([]byte, 7+1+len(response))
		copy(out[:7], header)
		binary.BigEndian.PutUint16(out[4:6], uint16(2+len(response)))
		out[7] = functionCode
		copy(out[8:], response)
		if _, err := conn.Write(out); err != nil {
			return
		}
	}
}

func (s *Server) dispatch(functionCode byte, data []byte) []byte {
	switch functionCode {
	case 3, 4:
		address := binary.BigEndian.Uint16(data[0:2])
		count := binary.BigEndian.Uint16(data[2:4])
		values := make([]byte, int(count)*2)
		s.mu.Lock()
		defer s.mu.Unlock()
		store := s.holding
		if functionCode == 4 {
			store = s.input
		}
		for i := uint16(0); i < count; i++ {
			value, ok := store[address+i]
			if !ok {
				value = 0
			}
			binary.BigEndian.PutUint16(values[i*2:], value)
		}
		return append([]byte{byte(len(values))}, values...)
	case 6:
		address := binary.BigEndian.Uint16(data[0:2])
		value := binary.BigEndian.Uint16(data[2:4])
		s.mu.Lock()
		s.holding[address] = value
		s.mu.Unlock()
		return data[:4]
	case 16:
		address := binary.BigEndian.Uint16(data[0:2])
		count := binary.BigEndian.Uint16(data[2:4])
		s.mu.Lock()
		for i := uint16(0); i < count; i++ {
			// PDU：addr(2)+qty(2)+byteCount(1)+寄存器值；data 已剥离功能码，值区自偏移 5 起。
			s.holding[address+i] = binary.BigEndian.Uint16(data[5+int(i)*2:])
		}
		s.mu.Unlock()
		return data[:4]
	case 5:
		address := binary.BigEndian.Uint16(data[0:2])
		on := binary.BigEndian.Uint16(data[2:4]) != 0
		s.mu.Lock()
		s.coil[address] = on
		s.mu.Unlock()
		return data[:4]
	default:
		return []byte{}
	}
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
