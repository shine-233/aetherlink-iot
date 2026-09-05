// 文件用途：边缘转发验证用最小 MQTT broker（ROADMAP 边缘计算 MVP 的云侧替身）。
// 核心逻辑：监听 TCP，处理 CONNECT（回 CONNACK）、QoS0/1 PUBLISH（捕获并追加写
//
//	日志文件 + QoS1 回 PUBACK）、PINGREQ（回 PINGRESP）。收到的每条消息按行
//	追加写入 -log 指定文件：`<iso时间>\t<topic>\t<payload>`。
//
// 关键注意事项：仅供边缘转发 E2E 使用（无订阅/保留消息/鉴权，非生产 broker）。
// 用法：go build -o edgemqttbroker.exe ./cmd/edgemqttbroker
//
//	./edgemqttbroker.exe -addr 127.0.0.1:18883 -log edge-broker-received.log
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	logFile *os.File
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18883", "监听地址")
	logPath := flag.String("log", "", "接收日志文件（追加写；空则打 stdout）")
	pubTopic := flag.String("pub", "", "启动后向指定 topic 发布一条消息（配合 -pub-payload）")
	pubPayload := flag.String("pub-payload", "{}", "-pub 的消息体")
	pubDelay := flag.Int("pub-delay", 0, "-pub 前的等待秒数（等订阅就绪）")
	flag.Parse()

	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("打开日志文件失败: %v", err)
		}
		logFile = f
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", *addr, err)
	}
	log.Printf("edgemqttbroker: listening on %s\n", *addr)
	if *pubTopic != "" {
		go func() {
			time.Sleep(time.Duration(*pubDelay) * time.Second)
			if err := newPublishClient(*addr, *pubTopic, []byte(*pubPayload))(); err != nil {
				log.Printf("edgemqttbroker: publish failed: %v", err)
			}
		}()
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept 失败: %v\n", err)
			return
		}
		go handleConn(conn)
	}
}

// subscribers 订阅表：conn -> 主题过滤器（支持 + 与 # 通配）。
var (
	subsMu sync.Mutex
	subs   = map[net.Conn][]string{}
)

// matchTopic MQTT 主题过滤匹配（+ 单层，# 多层）。
func matchTopic(filter, topic string) bool {
	f := strings.Split(filter, "/")
	t := strings.Split(topic, "/")
	for i := range f {
		if f[i] == "#" {
			return true
		}
		if i >= len(t) {
			return false
		}
		if f[i] != "+" && f[i] != t[i] {
			return false
		}
	}
	return len(f) == len(t)
}

// forwardPublish 把原始 PUBLISH 包转发给匹配的订阅者。
func forwardPublish(raw []byte, topic string) {
	subsMu.Lock()
	defer subsMu.Unlock()
	for conn, filters := range subs {
		for _, f := range filters {
			if matchTopic(f, topic) {
				_, _ = conn.Write(raw)
				break
			}
		}
	}
}

func handleConn(conn net.Conn) {
	defer func() {
		subsMu.Lock()
		delete(subs, conn)
		subsMu.Unlock()
		conn.Close()
	}()
	header := make([]byte, 1)
	for {
		if _, err := readFull(conn, header); err != nil {
			return
		}
		switch header[0] >> 4 {
		case 1: // CONNECT -> CONNACK
			remain, err := readVarint(conn)
			if err != nil {
				return
			}
			if remain > 0 {
				if _, err := readFull(conn, make([]byte, remain)); err != nil {
					return
				}
			}
			if _, err := conn.Write([]byte{0x20, 0x02, 0x00, 0x00}); err != nil {
				return
			}
		case 3: // PUBLISH
			remain, err := readVarint(conn)
			if err != nil {
				return
			}
			body := make([]byte, remain)
			if _, err := readFull(conn, body); err != nil {
				return
			}
			qos := (header[0] >> 1) & 0x03
			rawPacket := append([]byte{header[0]}, encodeVarint(remain)...)
			rawPacket = append(rawPacket, body...)
			topicLen := int(body[0])<<8 | int(body[1])
			topic := string(body[2 : 2+topicLen])
			payloadStart := 2 + topicLen
			if qos >= 1 {
				payloadStart += 2
			}
			payload := body[payloadStart:]
			logReceived(topic, payload)
			forwardPublish(rawPacket, topic)
			if qos == 1 {
				pid := body[2+topicLen : 2+topicLen+2]
				if _, err := conn.Write(append([]byte{0x40, 0x02}, pid...)); err != nil {
					return
				}
			}
		case 8: // SUBSCRIBE -> SUBACK
			remain, err := readVarint(conn)
			if err != nil {
				return
			}
			body := make([]byte, remain)
			if _, err := readFull(conn, body); err != nil {
				return
			}
			pid := body[0:2]
			filters := []string{}
			codes := []byte{}
			pos := 2
			for pos+2 <= len(body) {
				flen := int(body[pos])<<8 | int(body[pos+1])
				pos += 2
				filter := string(body[pos : pos+flen])
				pos += flen
				filters = append(filters, filter)
				codes = append(codes, 0x01) // granted QoS1
			}
			subsMu.Lock()
			subs[conn] = append(subs[conn], filters...)
			subsMu.Unlock()
			log.Printf("edgemqttbroker: subscribe %v", filters)
			ack := append([]byte{0x90}, 0x00) // 占位，下面重算长度
			payload := append(pid, codes...)
			ack = append([]byte{0x90, byte(len(payload))}, payload...)
			if _, err := conn.Write(ack); err != nil {
				return
			}
		case 4: // PUBACK（不重传，但必须读净剩余字节，否则后续包解析错位）
			remain, err := readVarint(conn)
			if err != nil {
				return
			}
			if remain > 0 {
				if _, err := readFull(conn, make([]byte, remain)); err != nil {
					return
				}
			}
		case 12: // PINGREQ -> PINGRESP
			if _, err := readVarint(conn); err != nil {
				return
			}
			if _, err := conn.Write([]byte{0xd0, 0x00}); err != nil {
				return
			}
		case 14: // DISCONNECT
			return
		default:
			if _, err := readVarint(conn); err != nil {
				return
			}
			remain2 := 0
			_ = remain2
		}
	}
}

func logReceived(topic string, payload []byte) {
	line := fmt.Sprintf("%s\t%s\t%s\n", time.Now().Format(time.RFC3339Nano), topic, string(payload))
	if logFile != nil {
		_, _ = logFile.WriteString(line)
		_ = logFile.Sync()
	} else {
		fmt.Print(line)
	}
}

func readVarint(conn net.Conn) (int, error) {
	mul := 1
	value := 0
	for i := 0; i < 4; i++ {
		buf := make([]byte, 1)
		if _, err := conn.Read(buf); err != nil {
			return 0, err
		}
		value += int(buf[0]&0x7f) * mul
		if buf[0]&0x80 == 0 {
			return value, nil
		}
		mul *= 128
	}
	return value, nil
}

func encodeVarint(value int) []byte {
	out := []byte{}
	for {
		b := byte(value % 128)
		value /= 128
		if value > 0 {
			b |= 0x80
		}
		out = append(out, b)
		if value == 0 {
			return out
		}
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

// newPublishClient 建立一条连接并发布一条 QoS1 消息后断开（模拟云端下发命令）。
func newPublishClient(addr, topic string, payload []byte) func() error {
	return func() error {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return err
		}
		defer conn.Close()
		connect := []byte{0x00, 0x04, 'M', 'Q', 'T', 'T', 0x04, 0x02, 0x00, 0x3c, 0x00, 0x06, 'c', 'l', 'o', 'u', 'd', '1'}
		if _, err := conn.Write(append([]byte{0x10, byte(len(connect))}, connect...)); err != nil {
			return err
		}
		buf := make([]byte, 4)
		if _, err := conn.Read(buf); err != nil || buf[0] != 0x20 {
			return fmt.Errorf("CONNACK 失败: %v %v", buf, err)
		}
		pid := []byte{0x00, 0x0a}
		tp := []byte(topic)
		body := append([]byte{byte(len(tp) >> 8), byte(len(tp))}, tp...)
		body = append(body, pid...)
		body = append(body, payload...)
		if _, err := conn.Write(append([]byte{0x32, byte(len(body))}, body...)); err != nil {
			return err
		}
		log.Printf("edgemqttbroker: published %s -> %s", topic, string(payload))
		time.Sleep(2 * time.Second) // 等订阅侧处理
		return nil
	}
}
