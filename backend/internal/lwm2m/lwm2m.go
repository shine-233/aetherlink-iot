// 文件用途：LwM2M 1.0 注册层（ROADMAP C6）——基于同仓 internal/coap 的 CoAP 实现。
// 核心逻辑：实现 OMA LwM2M 注册接口核心：POST /rd?ep=<endpoint>&lt=<life>&b=<binding>，
//   返回 2.01 Created 并登记客户端（endpoint name 唯一）；DELETE /rd/{id} 注销；
//   提供按 lifetime 的过期清理与在线查询。
// 关键注意事项：
//   - 本层为注册簿 + 生命周期语义，不解析 DTLS/队列模式/对象模型；对象实例（/19/0 等）与
//     observe 订阅在后续迭代接入 coap.Registry（Uri-Path 已支持多段）；
//   - 超长 payload、空 endpoint、非法 lifetime 一律拒绝（4.00）；
//   - Location-Path 携带由上层在接入路由时补齐（coap.Handler 返回签名暂不含 options）。
package lwm2m

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/coap"
)

// ContentFormatTextPlain CoAP text/plain。
const textPlain = 0

// Client 一个已注册的 LwM2M 客户端。
type Client struct {
	ID           string
	Endpoint     string
	Binding      string
	Address      string // 触发注册的远端地址（由上层注入）
	Lifetime     time.Duration
	RegisteredAt time.Time
}

// Expired 判断客户端是否已过生命周期。
func (c *Client) Expired(now time.Time) bool {
	return now.After(c.RegisteredAt.Add(c.Lifetime))
}

// Registry LwM2M 注册簿（线程安全）。
type Registry struct {
	mu      sync.Mutex
	clients map[string]*Client // key=ID
	byEP    map[string]string  // endpoint→ID
	now     func() time.Time
	nextID  uint64
}

// NewRegistry 新建注册簿。
func NewRegistry() *Registry {
	return &Registry{
		clients: map[string]*Client{},
		byEP:    map[string]string{},
		now:     time.Now,
	}
}

// HandleRegister 返回挂到 coap.Registry "/rd" 的处理器。
func (r *Registry) HandleRegister() coap.Handler {
	return func(req *coap.Message) (coap.Code, []byte, int, error) {
		switch req.Code {
		case coap.CodePost:
			ep, lt, binding, err := parseRegisterParams(req)
			if err != nil {
				return coap.CodeBadRequest, []byte(err.Error()), textPlain, nil
			}
			id, created := r.Register(ep, lt, binding, "")
			if created {
				return coap.CodeCreated, []byte("id=" + id), textPlain, nil
			}
			return coap.CodeChanged, []byte("id=" + id), textPlain, nil
		case coap.CodeDelete:
			id := strings.TrimPrefix(req.UriPath(), "/rd/")
			if r.Delete(id) {
				return coap.CodeDeleted, nil, textPlain, nil
			}
			return coap.CodeNotFound, []byte("registration not found"), textPlain, nil
		default:
			return coap.CodeMethodNotAllowed, nil, textPlain, nil
		}
	}
}

func parseRegisterParams(req *coap.Message) (ep string, lt time.Duration, binding string, err error) {
	for _, q := range req.OptionsByNumber(coap.OptionUriQuery) {
		kv := string(q)
		switch {
		case strings.HasPrefix(kv, "ep="):
			ep = strings.TrimSpace(strings.TrimPrefix(kv, "ep="))
		case strings.HasPrefix(kv, "lt="):
			secs, cerr := strconv.Atoi(strings.TrimPrefix(kv, "lt="))
			if cerr != nil || secs <= 0 {
				return "", 0, "", errBad("lt 必须为正整数秒")
			}
			lt = time.Duration(secs) * time.Second
		case strings.HasPrefix(kv, "b="):
			binding = strings.TrimSpace(strings.TrimPrefix(kv, "b="))
		}
	}
	if ep == "" {
		return "", 0, "", errBad("缺少 ep=<endpoint>")
	}
	if lt <= 0 {
		lt = 86400 * time.Second // 默认 24h
	}
	if binding == "" {
		binding = "U"
	}
	return ep, lt, binding, nil
}

type errBad string

func (e errBad) Error() string { return string(e) }

// Register 登记/更新客户端；返回分配 ID 与是否新建。
func (r *Registry) Register(endpoint string, lifetime time.Duration, binding, addr string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.byEP[endpoint]; ok {
		if c, ok2 := r.clients[id]; ok2 {
			c.Lifetime = lifetime
			c.Binding = binding
			c.Address = addr
			c.RegisteredAt = r.now()
			return id, false
		}
	}
	r.nextID++
	id := strconv.FormatUint(r.nextID, 10)
	now := r.now()
	r.clients[id] = &Client{
		ID: id, Endpoint: endpoint, Binding: binding, Address: addr,
		Lifetime: lifetime, RegisteredAt: now,
	}
	r.byEP[endpoint] = id
	return id, true
}

// Delete 注销客户端。
func (r *Registry) Delete(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[id]
	if !ok {
		return false
	}
	delete(r.clients, id)
	delete(r.byEP, c.Endpoint)
	return true
}

// PruneExpired 清理过期客户端，返回清除数量。
func (r *Registry) PruneExpired() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	removed := 0
	for id, c := range r.clients {
		if c.Expired(now) {
			delete(r.clients, id)
			delete(r.byEP, c.Endpoint)
			removed++
		}
	}
	return removed
}

// Count 当前存活注册数。
func (r *Registry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

// Snapshot 返回客户端快照（测试与状态接口用）。
func (r *Registry) Snapshot() []Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Client, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, *c)
	}
	return out
}
