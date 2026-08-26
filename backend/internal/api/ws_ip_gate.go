// 文件用途：对公开 WebSocket 端点做每来源 IP 的并发连接数闸门。
// 核心逻辑：升级握手前 TryAcquire，会话关闭时 Release；未认证连接在首消息鉴权前
//
//	同样占用额度，防止握手洪泛在鉴权前耗尽会话资源（审计 P1 #13 收口）。
//
// 关键注意事项：计数为进程内尽力而为语义（多副本部署各自独立）；无法解析来源的
//
//	连接不做归因也不拒绝（与 broker 认证限流的同类取舍一致）。上限经
//	ws.max_conns_per_ip（env GOTP_WS_MAX_CONNS_PER_IP）配置，默认 64/IP。
package api

import (
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// defaultWSMaxConnsPerIP 是未配置时的每 IP 并发上限。
const defaultWSMaxConnsPerIP = 64

type wsIPGate struct {
	mu     sync.Mutex
	counts map[string]int
	max    int
}

func newWSIPGate(max int) *wsIPGate {
	if max <= 0 {
		max = defaultWSMaxConnsPerIP
	}
	return &wsIPGate{counts: make(map[string]int), max: max}
}

func wsMaxConnsPerIPFromConfig() int {
	if v := viper.GetInt("ws.max_conns_per_ip"); v > 0 {
		return v
	}
	return defaultWSMaxConnsPerIP
}

var defaultWSIPGate = newWSIPGate(wsMaxConnsPerIPFromConfig())

// tryAcquire 为该 IP 占用一个并发槽位；达到上限返回 false。空 IP 直接放行。
func (g *wsIPGate) tryAcquire(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.counts[ip] >= g.max {
		return false
	}
	g.counts[ip]++
	return true
}

// release 归还槽位；对未知 IP 是 no-op（防御重复释放导致负数计数）。
func (g *wsIPGate) release(ip string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.counts[ip] <= 0 {
		return
	}
	g.counts[ip]--
	if g.counts[ip] == 0 {
		delete(g.counts, ip)
	}
}

func (g *wsIPGate) current(ip string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.counts[ip]
}
