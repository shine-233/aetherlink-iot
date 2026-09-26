// 文件用途：TCP 会话注册簿（TP-03）——device_number → 在线会话映射与注册/去注册事件。
// 核心逻辑：首帧 device_number 注册后登记会话；同号新会话顶替旧会话（旧连接关闭）；
// 连接断开按会话身份精确去注册；Known 记录"本进程见过"的设备号（断网缓冲的路由依据）。
// 关键注意事项：注册簿只保存在线会话，Known 是离线留存集合（Gateway 侧落环形缓冲依赖它）；
// 注册/去注册回调在持锁外调用，不得做重活（与 lwm2m Registry 同约定）。
// 重构建议：多副本部署时注册簿需上移到共享存储（Redis 会话表），本结构仅为单机形态。
package tcp

import (
	"net"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/protocolgw"
)

// session 一条已注册的 TCP 设备会话。
type session struct {
	number      string
	remote      string
	conn        net.Conn
	identity    *protocolgw.DeviceIdentity // 注册时解析并固写（写侧归因，运行期不再查库）
	connectedAt time.Time

	writeMu sync.Mutex // 下行帧写串行化（写超时互不踩踏）
}

// writeFrame 向会话连接写一帧下行数据（带写超时）；返回编码帧与写错误。
func (s *session) writeFrame(mode FrameMode, payload []byte, maxSize int, writeTimeout time.Duration) ([]byte, error) {
	frame, err := EncodeFrame(mode, payload, maxSize)
	if err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := s.conn.Write(frame); err != nil {
		return frame, err
	}
	return frame, nil
}

// Registry TCP 会话注册簿（线程安全）。
type Registry struct {
	mu           sync.Mutex
	byNumber     map[string]*session  // device_number → 在线会话
	known        map[string]time.Time // device_number → 最近注册时刻（离线留存，断网缓冲路由依据）
	onRegister   func(number string)  // 注册事件回调（顶替也算注册；下行缓冲冲刷挂点）
	onDeregister func(number string)  // 去注册事件回调（诊断挂点）
	now          func() time.Time
}

// NewRegistry 新建注册簿；回调可 nil。
func NewRegistry(onRegister, onDeregister func(number string)) *Registry {
	return &Registry{
		byNumber:     map[string]*session{},
		known:        map[string]time.Time{},
		onRegister:   onRegister,
		onDeregister: onDeregister,
		now:          time.Now,
	}
}

// Register 登记会话；同号已有在线会话时顶替并返回旧会话（调用方负责关闭旧连接）。
// 顶替即"最新连接胜出"：设备重连后旧半死连接不再吞新命令。
func (r *Registry) Register(s *session) (displaced *session) {
	r.mu.Lock()
	old := r.byNumber[s.number]
	r.byNumber[s.number] = s
	r.known[s.number] = r.now()
	cb := r.onRegister
	r.mu.Unlock()
	if cb != nil {
		cb(s.number)
	}
	return old
}

// Deregister 按会话身份去注册：仅当映射仍指向该会话才移除（防顶替后旧连接误删新会话）。
func (r *Registry) Deregister(s *session) bool {
	if s == nil {
		return false
	}
	r.mu.Lock()
	cur, ok := r.byNumber[s.number]
	if !ok || cur != s {
		r.mu.Unlock()
		return false
	}
	delete(r.byNumber, s.number)
	cb := r.onDeregister
	r.mu.Unlock()
	if cb != nil {
		cb(s.number)
	}
	return true
}

// Lookup 按设备号查在线会话（离线返回 nil）。
func (r *Registry) Lookup(number string) *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byNumber[number]
}

// Known 是否为本进程注册过的设备（含已离线——断网缓冲语义的关键判定）。
func (r *Registry) Known(number string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.known[number]
	return ok
}

// Count 当前在线会话数。
func (r *Registry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byNumber)
}

// KnownCount 本进程见过的设备数（含离线）。
func (r *Registry) KnownCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.known)
}

// CloseAll 关闭全部在线会话连接（Stop 收尾用）；返回关闭数。
func (r *Registry) CloseAll() int {
	r.mu.Lock()
	conns := make([]net.Conn, 0, len(r.byNumber))
	for _, s := range r.byNumber {
		conns = append(conns, s.conn)
	}
	r.byNumber = map[string]*session{}
	r.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
	return len(conns)
}
