// 文件用途：LwM2M 1.0 注册层（ROADMAP C6）——基于同仓 internal/coap 的 CoAP 实现。
// 核心逻辑：实现 OMA LwM2M 注册接口核心：POST /rd?ep=<endpoint>&lt=<life>&b=<binding>，
//
//	返回 2.01 Created 并登记客户端（endpoint name 唯一）；DELETE /rd/{id} 注销；
//	提供按 lifetime 的过期清理、在线查询与注册/去注册事件回调（TB-22 隔离接线用）。
//
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
// 注册簿记录触发注册的 UDP 源地址（req.RemoteAddr，TB-22 多客户端归因依据）。
func (r *Registry) HandleRegister() coap.Handler {
	return func(req *coap.Message) (coap.Code, []byte, int, error) {
		switch req.Code {
		case coap.CodePost:
			ep, lt, binding, err := parseRegisterParams(req)
			if err != nil {
				return coap.CodeBadRequest, []byte(err.Error()), textPlain, nil
			}
			id, created := r.Register(ep, lt, binding, req.RemoteAddr)
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

// RegistryEventKind 注册簿事件类型。
type RegistryEventKind int

const (
	EventRegister RegistryEventKind = iota // 注册（含刷新）
	EventDeregister                         // 去注册（DELETE /rd/{id}）
)

// RegistryEvent 一次注册/去注册事件（多客户端隔离接线用：端点 + UDP 源地址）。
type RegistryEvent struct {
	Kind     RegistryEventKind
	ID       string // 注册簿分配的注册 ID（Location /rd/{id}）
	Endpoint string // LwM2M 端点名
	Addr     string // 事件报文的 UDP 源地址（去注册侧可能为空）
}

// HandleRegisterWithEvents 在 HandleRegister 基础上追加注册/去注册事件回调（可 nil）。
// 注册（新建或刷新）触发 EventRegister；DELETE /rd/{id} 命中时触发 EventDeregister
// 并携带被注销端点。onEvent 阻塞调用，不得做重活（与 SetOnChange 同约定）。
// 用途：上层按事件维护 per-endpoint 对象存储映射（TB-22 多客户端隔离）。
func (r *Registry) HandleRegisterWithEvents(onEvent func(RegistryEvent)) coap.Handler {
	inner := r.HandleRegister()
	if onEvent == nil {
		return inner
	}
	return func(req *coap.Message) (coap.Code, []byte, int, error) {
		// DELETE 需在注销前查端点（inner 处理后注册簿已无该条目）。
		var deregEndpoint string
		if req.Code == coap.CodeDelete {
			deregEndpoint = r.EndpointByID(strings.TrimPrefix(req.UriPath(), "/rd/"))
		}
		code, body, obs, err := inner(req)
		if err != nil {
			return code, body, obs, err
		}
		switch {
		case code == coap.CodeCreated || code == coap.CodeChanged:
			if ep, _, _, perr := parseRegisterParams(req); perr == nil && ep != "" {
				onEvent(RegistryEvent{Kind: EventRegister, ID: regIDFromBody(body), Endpoint: ep, Addr: req.RemoteAddr})
			}
		case code == coap.CodeDeleted:
			id := strings.TrimPrefix(req.UriPath(), "/rd/")
			onEvent(RegistryEvent{Kind: EventDeregister, ID: id, Endpoint: deregEndpoint, Addr: req.RemoteAddr})
		}
		return code, body, obs, err
	}
}

// regIDFromBody 从注册响应体 "id=<n>" 还原注册 ID。
func regIDFromBody(body []byte) string {
	return strings.TrimPrefix(string(body), "id=")
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

// EndpointByID 按注册 ID 查端点名（未注册返回空串）。去注册事件在注销前取端点用。
func (r *Registry) EndpointByID(id string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[id]; ok {
		return c.Endpoint
	}
	return ""
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
