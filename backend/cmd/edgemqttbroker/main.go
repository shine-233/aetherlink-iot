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
	"time"
)

var (
	logFile *os.File
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18883", "监听地址")
	logPath := flag.String("log", "", "接收日志文件（追加写；空则打 stdout）")
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
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept 失败: %v\n", err)
			return
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
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
			topicLen := int(body[0])<<8 | int(body[1])
			topic := string(body[2 : 2+topicLen])
			payloadStart := 2 + topicLen
			if qos >= 1 {
				payloadStart += 2
			}
			payload := body[payloadStart:]
			logReceived(topic, payload)
			if qos == 1 {
				pid := body[2+topicLen : 2+topicLen+2]
				if _, err := conn.Write(append([]byte{0x40, 0x02}, pid...)); err != nil {
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
