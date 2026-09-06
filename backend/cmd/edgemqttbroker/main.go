// 文件用途：边缘转发验证用最小 MQTT broker（ROADMAP 边缘计算 MVP 的云侧替身）。
// 核心逻辑：监听 TCP，处理 CONNECT（回 CONNACK）、QoS0/1 PUBLISH（捕获 + QoS1 回
//
//	PUBACK + 转发给匹配订阅者）、SUBSCRIBE（回 SUBACK，支持 +/# 通配）、
//	PINGREQ（回 PINGRESP）。收到的每条消息按行追加写日志：
//	`<RFC3339Nano>\t<topic>\t<payload>`。
//
// 可选发布模式：-pub 指定 topic，-pub-delay 秒后发布 -pub-payload 内容；
//
//	-pub-interval > 0 时按间隔周期重发（订阅就绪时序无关，幂等导入兜底）。
//
// 关键注意事项：仅供边缘转发/实体下发 E2E 使用（无鉴权/保留消息，非生产 broker）。
// 用法：go build -o edgemqttbroker.exe ./cmd/edgemqttbroker
//
//	./edgemqttbroker.exe -addr 127.0.0.1:18883 -log edge-broker-received.log
//	  [-pub aetherlink/edge/cmd/edge-node -pub-payload @file.json -pub-delay 5 -pub-interval 10]
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
	subsMu  sync.Mutex
	subs    = map[net.Conn][]string{}
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18883", "监听地址")
	logPath := flag.String("log", "", "接收日志文件（追加写；空则打 stdout）")
	pubTopic := flag.String("pub", "", "启动后向指定 topic 发布（配合 -pub-payload）")
	pubPayload := flag.String("pub-payload", "{}", "-pub 的消息体；@file 表示从文件读")
	pubDelay := flag.Int("pub-delay", 0, "-pub 前的等待秒数（等订阅就绪）")
	pubInterval := flag.Int("pub-interval", 0, "-pub 周期重发间隔秒数（0=只发一次）")
	flag.Parse()

	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("打开日志文件失败: %v", err)
		}
		logFile = f
	}

	payloadBytes := []byte(*pubPayload)
	if strings.HasPrefix(*pubPayload, "@") {
		raw, err := os.ReadFile(strings.TrimPrefix(*pubPayload, "@"))
		if err != nil {
			log.Fatalf("读取发布载荷文件失败: %v", err)
		}
		payloadBytes = raw
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", *addr, err)
	}
	log.Printf("edgemqttbroker: listening on %s", *addr)
	if *pubTopic != "" {
		go func() {
			time.Sleep(time.Duration(*pubDelay) * time.Second)
			for {
				if err := newPublishClient(*addr, *pubTopic, payloadBytes)(); err != nil {
					log.Printf("edgemqttbroker: publish failed: %v", err)
				} else {
					log.Printf("edgemqttbroker: published to %s", *pubTopic)
				}
				if *pubInterval <= 0 {
					return
				}
				time.Sleep(time.Duration(*pubInterval) * time.Second)
			}
		}()
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept 失败: %v", err)
			return
		}
		go handleConn(conn)
	}
}

// pubMsg 一条被捕获的 PUBLISH。
type pubMsg struct {
	topic   string
	payload []byte
}

// subscribers 订阅表：conn -> 主题过滤器（支持 + 与 # 通配）。
var subsRegistry = struct {
	sync.Mutex
	m map[net.Conn][]string
}{m: map[net.Conn][]string{}}

func registerSub(conn net.Conn, filters []string) {
	subsRegistry.Lock()
	subsRegistry.m[conn] = append(subsRegistry.m[conn], filters...)
	subsRegistry.Unlock()
}

func removeSub(conn net.Conn) {
	subsRegistry.Lock()
	delete(subsRegistry.m, conn)
	subsRegistry.Unlock()
}

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
	subsRegistry.Lock()
	defer subsRegistry.Unlock()
	for conn, filters := range subsRegistry.m {
		for _, f := range filters {
			if matchTopic(f, topic) {
				if _, err := conn.Write(raw); err != nil {
					log.Printf("转发失败: %v", err)
				}
				break
			}
		}
	}
}

func handleConn(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("edgemqttbroker: conn handler recovered: %v", r)
		}
	}()
	defer func() {
		removeSub(conn)
		conn.Close()
	}()
	header := make([]byte, 1)
	for {
		if _, err := readFull(conn, header); err != nil {
			return
		}
		remain, err := readVarint(conn)
		if err != nil {
			return
		}
		var body []byte
		if remain > 0 {
			body = make([]byte, remain)
			if _, err := readFull(conn, body); err != nil {
				return
			}
		}
		switch header[0] >> 4 {
		case 1: // CONNECT -> CONNACK
			if _, err := conn.Write([]byte{0x20, 0x02, 0x00, 0x00}); err != nil {
				return
			}
		case 3: // PUBLISH（QoS0/QoS1：QoS1 回 PUBACK 并转发订阅者）
			if remain < 2 {
				return
			}
			qos := (header[0] >> 1) & 0x03
			topicLen := int(body[0])<<8 | int(body[1])
			payloadStart := 2 + topicLen
			if qos >= 1 {
				payloadStart += 2
			}
			if topicLen <= 0 || payloadStart > len(body) {
				log.Printf("edgemqttbroker: malformed PUBLISH (remain=%d topicLen=%d), dropping connection", remain, topicLen)
				return
			}
			topic := string(body[2 : 2+topicLen])
			payload := body[payloadStart:]
			logReceived(topic, payload)
			forwardPublish(append(append([]byte{header[0]}, encodeVarint(remain)...), body...), topic)
			if qos == 1 {
				pid := body[2+topicLen : 2+topicLen+2]
				if _, err := conn.Write(append([]byte{0x40, 0x02}, pid...)); err != nil {
					return
				}
			}
		case 8: // SUBSCRIBE -> SUBACK
			pid := body[0:2]
			filters := []string{}
			codes := []byte{}
			pos := 2
			for pos+2 <= len(body) {
				flen := int(body[pos])<<8 | int(body[pos+1])
				pos += 2
				filter := string(body[pos : pos+flen])
				pos += flen + 1 // 过滤器后跟 QoS 字节
				filters = append(filters, filter)
				codes = append(codes, 0x01)
			}
			registerSub(conn, filters)
			log.Printf("edgemqttbroker: subscribe %v", filters)
			payload := append(pid, codes...)
			ack := append([]byte{0x90, byte(len(payload))}, payload...)
			if _, err := conn.Write(ack); err != nil {
				return
			}
		case 4, 12, 14: // PUBACK / PINGREQ(需回 PINGRESP) / DISCONNECT
			if header[0]>>4 == 12 {
				if _, err := conn.Write([]byte{0xd0, 0x00}); err != nil {
					return
				}
			}
			if header[0]>>4 == 14 {
				return
			}
		default:
			log.Printf("edgemqttbroker: 未支持的包类型 %d，断开", header[0]>>4)
			return
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

func encodeVarint(n int) []byte {
	out := []byte{}
	for {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			return out
		}
	}
}

// newPublishClient 建立一条连接并发布一条 QoS1 消息（PUBACK 等待由包内判定简化为连接级）。
func newPublishClient(addr, topic string, payload []byte) func() error {
	return func() error {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return err
		}
		defer conn.Close()
		// CONNECT
		proto := []byte{0x00, 0x04, 'M', 'Q', 'T', 'T', 0x04, 0x02, 0x00, 0x3c}
		cid := []byte{0x00, 0x05, 'p', 'u', 'b', 'l', 'r'}
		pl := append(proto, cid...)
		conn.Write(append([]byte{0x10, byte(len(pl))}, pl...))
		ack := make([]byte, 4)
		if _, err := readFull(conn, ack); err != nil {
			return err
		}
		// PUBLISH QoS1
		tl := []byte{byte(len(topic) >> 8), byte(len(topic))}
		body := append(append(tl, []byte(topic)...), 0x00, 0x01)
		body = append(body, payload...)
		remain := len(body)
		vl := []byte{}
		v := remain
		for {
			b := byte(v % 128)
			v /= 128
			if v > 0 {
				b |= 0x80
			}
			vl = append(vl, b)
			if v == 0 {
				break
			}
		}
		packet := append([]byte{0x32}, vl...)
		packet = append(packet, body...)
		if _, err := conn.Write(packet); err != nil {
			return err
		}
		puback := make([]byte, 4)
		_, _ = readFull(conn, puback) // PUBACK（QoS1）
		return nil
	}
}
