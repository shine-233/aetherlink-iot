// 文件用途：SNMP 采集器本地 E2E stub agent（ROADMAP C6 SNMP 行运行期验证工具）。
// 核心逻辑：监听 UDP，对任意入站报文回复固定的 SNMPv2c GetResponse
//   （sysUpTime=TIMETICKS 600 + 自定义 OCTET STRING "collectore2e"），
//   供 backend 以 collectors.snmp.enabled 轮询并验证遥测落库全链路。
// 关键注意事项：仅供本地/隔离栈 E2E 使用（社区明文，非真实设备代理）；
//   不参与生产部署（无 Dockerfile/compose 条目）。
// 用法：go build -o snmpstub.exe ./cmd/snmpstub && ./snmpstub.exe -addr 127.0.0.1:1161
package main

import (
	"flag"
	"log"
	"net"

	"aetherlink-iot/backend/internal/snmp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:1161", "UDP 监听地址")
	flag.Parse()

	binds := []snmp.VarBind{
		{OID: "1.3.6.1.2.1.1.3.0", Value: snmp.Counter32Value(600)}, // uptime
		{OID: "1.3.6.1.2.1.1.5.0", Value: snmp.OctetStringValue("collectore2e")}, // hostname
	}

	conn, err := net.ListenPacket("udp", *addr)
	if err != nil {
		log.Fatalf("snmpstub: 监听 %s 失败: %v", *addr, err)
	}
	defer conn.Close()
	log.Printf("snmpstub: listening on %s (uptime=600, hostname=collectore2e)", *addr)

	buf := make([]byte, 65536)
	for {
		n, client, err := conn.ReadFrom(buf)
		if err != nil {
			log.Fatalf("snmpstub: 读取失败: %v", err)
		}
		_ = n
		resp, err := snmp.BuildGetResponse("public", 1, 0, 0, binds)
		if err != nil {
			log.Printf("snmpstub: 构建响应失败: %v", err)
			continue
		}
		if _, err := conn.WriteTo(resp, client); err != nil {
			log.Printf("snmpstub: 回复 %s 失败: %v", client, err)
		}
	}
}
