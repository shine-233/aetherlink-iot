// 文件用途：OPC UA 采集器本地 E2E stub 服务器（ROADMAP OPC UA 行运行期验证工具）。
// 核心逻辑：基于 gopcua server 包起一个 None 安全/匿名认证的最小 opc.tcp 服务器，
//   MapNamespace 暴露 temperature=26.5 / humidity=61.0 两个静态节点，
//   供 backend 以 collectors.opcua.enabled 轮询并验证遥测落库全链路。
// 关键注意事项：
//   - 仅供本地/隔离栈 E2E 使用（None 安全模式，非生产配置）；不参与生产部署
//     （无 Dockerfile/compose 条目）；
//   - gopcua Server.Start 非阻塞（监听在后台 goroutine），主协程必须挂住等待退出
//     信号，否则进程立即退出（与官方 map_server 示例一致）；
//   - 命名空间索引由 server 动态分配，启动时打印实际 NodeID（设备点表需按日志中的
//     ns 索引填写）。
// 用法：go build -o opcuastub.exe ./cmd/opcuastub && ./opcuastub.exe -port 14840
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/ua"
)

func main() {
	port := flag.Int("port", 14840, "OPC UA 监听端口")
	flag.Parse()

	opts := []server.Option{
		server.EnableSecurity("None", ua.MessageSecurityModeNone),
		server.EnableAuthMode(ua.UserTokenTypeAnonymous),
		server.EndPoint("127.0.0.1", *port),
		server.EndPoint("localhost", *port),
	}
	s := server.New(opts...)

	ns := server.NewMapNamespace(s, "collectore2e")
	ns.Data["temperature"] = 26.5
	ns.Data["humidity"] = 61.0
	log.Printf("opcuastub: namespace index=%d - nodes: ns=%d;s=temperature (26.5), ns=%d;s=humidity (61.0)",
		ns.ID(), ns.ID(), ns.ID())

	// 把命名空间挂到 Objects 节点下（浏览可见；读取不依赖此步，但保持服务端行为完整）。
	rootNS, err := s.Namespace(0)
	if err != nil {
		log.Fatalf("opcuastub: 根命名空间缺失: %v", err)
	}
	rootNS.Objects().AddRef(ns.Objects(), id.HasComponent, true)

	log.Printf("opcuastub: listening opc.tcp://127.0.0.1:%d", *port)
	if err := s.Start(context.Background()); err != nil {
		log.Fatalf("opcuastub: 启动失败: %v", err)
	}
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
	<-sigch
	log.Printf("opcuastub: shutting down")
	_ = s.Close()
}
