// 文件用途：CoAP 服务端核心（ROADMAP C6，RFC 7252 子集）——LwM2M 接入的地基。
// 核心逻辑：纯标准库实现消息编解码（Ver/T/Code/MsgID/Token/Options/Payload 0xFF 标记）、
//   基础方法（GET/POST/PUT/DELETE）与响应码、UDP 服务器（CON→piggyback ACK，
//   NON→NON 响应，Token 回显）、资源注册表 + /.well-known/core 链接格式（RFC 6690）。
// 关键注意事项：
//   - 仅支持 Message/Options 编解码、0xFF 载荷、Option delta/len 非扩展形式（<269）；
//     扩展 option 长度与 blockwise、observe 订阅留作下一步（注释扩展点）；
//   - 安全边界：maxPayload 钳制、message id 无状态、不自动信任任意路径注册。
// 重构建议：接入 LwM2M 时在 Registry 上增加 /rd 注册与对象实例管理，见同包 LwM2M 计划。
package coap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

// Type CoAP 消息类型。
type Type uint8

const (
	TypeConfirmable Type = 0 // CON
	TypeNonConfirm Type = 1 // NON
	TypeAck        Type = 2 // ACK
	TypeReset      Type = 3 // RST
)

// Code CoAP 方法/响应码（code = class<<5 | detail）。
type Code uint8

const (
	CodeEmpty      Code = 0
	CodeGet        Code = 1  // 0.01
	CodePost       Code = 2  // 0.02
	CodePut        Code = 3  // 0.03
	CodeDelete     Code = 4  // 0.04
	CodeCreated    Code = 65 // 2.01
	CodeDeleted    Code = 66 // 2.02
	CodeChanged    Code = 68 // 2.04
	CodeContent    Code = 69 // 2.05
	CodeBadRequest Code = 128 // 4.00
	CodeNotFound   Code = 132 // 4.04
	CodeMethodNotAllowed Code = 133 // 4.05
	CodeInternal   Code = 160 // 5.00
)

// CodeString 返回 "class.detail" 便于日志。
func (c Code) String() string {
	return fmt.Sprintf("%d.%02d", c>>5, c&0x1f)
}

// Option CoAP 选项。
type Option struct {
	Number uint16
	Value  []byte
}

// Common option numbers.
const (
	OptionIfMatch       = 1
	OptionUriHost       = 3
	OptionETag          = 4
	OptionIfNoneMatch   = 5
	OptionObserve       = 6
	OptionUriPort       = 7
	OptionLocationPath  = 8
	OptionUriPath       = 11
	OptionContentFormat = 12
	OptionMaxAge        = 14
	OptionUriQuery      = 15
)

// Message CoAP 消息。
type Message struct {
	Type      Type
	Code      Code
	MessageID uint16
	Token     []byte
	Options   []Option
	Payload   []byte
}

// maxMessageSize 单个 UDP 数据报上限（IPv4 建议 1152，取宽松值）。
const maxMessageSize = 65535
const maxPayloadSize = 64 * 1024

// Encode 将消息编码为字节流。
func (m *Message) Encode() ([]byte, error) {
	if len(m.Token) > 8 {
		return nil, fmt.Errorf("coap: token 超长 %d", len(m.Token))
	}
	head := byte(1<<6) | byte(m.Type&0x03)<<4 | byte(len(m.Token))
	var buf bytes.Buffer
	buf.WriteByte(head)
	buf.WriteByte(byte(m.Code))
	_ = binary.Write(&buf, binary.BigEndian, m.MessageID)
	buf.Write(m.Token)

	prev := 0
	for _, opt := range m.Options {
		delta := int(opt.Number) - prev
		if delta < 0 || delta > 268 || len(opt.Value) > 268 {
			return nil, fmt.Errorf("coap: 超出子集支持的 option delta/len（需扩展编码）")
		}
		buf.WriteByte(byte(delta<<4 | len(opt.Value)))
		buf.Write(opt.Value)
		prev = int(opt.Number)
	}
	if len(m.Payload) > 0 {
		buf.WriteByte(0xFF)
		buf.Write(m.Payload)
	}
	return buf.Bytes(), nil
}

// Decode 从字节流解析消息。
func Decode(raw []byte) (*Message, error) {
	if len(raw) < 4 {
		return nil, fmt.Errorf("coap: 消息过短 %d", len(raw))
	}
	ver := raw[0] >> 6
	if ver != 1 {
		return nil, fmt.Errorf("coap: 版本 %d 不支持", ver)
	}
	m := &Message{
		Type:      Type((raw[0] >> 4) & 0x03),
		Code:      Code(raw[1]),
		MessageID: binary.BigEndian.Uint16(raw[2:4]),
	}
	tkl := int(raw[0] & 0x0f)
	if tkl > 8 {
		return nil, fmt.Errorf("coap: tkl 非法 %d", tkl)
	}
	pos := 4
	if pos+tkl > len(raw) {
		return nil, fmt.Errorf("coap: token 截断")
	}
	m.Token = append([]byte{}, raw[pos:pos+tkl]...)
	pos += tkl

	prev := 0
	for pos < len(raw) {
		b := raw[pos]
		if b == 0xFF {
			pos++
			m.Payload = append([]byte{}, raw[pos:]...)
			if len(m.Payload) > maxPayloadSize {
				return nil, fmt.Errorf("coap: payload 超限")
			}
			break
		}
		delta := int(b >> 4)
		l := int(b & 0x0f)
		if delta == 13 || delta == 14 || l == 13 || l == 14 {
			return nil, fmt.Errorf("coap: 扩展 option 长度超出子集支持")
		}
		pos++
		if pos+l > len(raw) {
			return nil, fmt.Errorf("coap: option 值截断")
		}
		opt := Option{Number: uint16(prev + delta), Value: append([]byte{}, raw[pos:pos+l]...)}
		m.Options = append(m.Options, opt)
		prev = int(opt.Number)
		pos += l
	}
	if len(m.Payload) > maxMessageSize {
		return nil, fmt.Errorf("coap: payload 过大")
	}
	return m, nil
}

// OptionsByNumber 返回某 option 号的所有值（Uri-Path 可多段）。
func (m *Message) OptionsByNumber(n uint16) [][]byte {
	var out [][]byte
	for _, o := range m.Options {
		if o.Number == n {
			out = append(out, o.Value)
		}
	}
	return out
}

// UriPath 拼接 Uri-Path 段（以 "/" 分隔，与资源键对齐）。
func (m *Message) UriPath() string {
	segs := m.OptionsByNumber(OptionUriPath)
	if len(segs) == 0 {
		return "/"
	}
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, string(s))
	}
	return "/" + strings.Join(parts, "/")
}

// ContentFormat 返回 Content-Format option 或 -1。
func (m *Message) ContentFormat() int {
	if vals := m.OptionsByNumber(OptionContentFormat); len(vals) > 0 && len(vals[0]) > 0 {
		return int(vals[0][0])
	}
	return -1
}

// NewAck 构造对 CON 的 piggyback ACK（沿用 MsgID/Token，回填 Code 与载荷）。
func NewAck(req *Message, code Code, payload []byte) *Message {
	return &Message{
		Type:      TypeAck,
		Code:      code,
		MessageID: req.MessageID,
		Token:     req.Token,
		Payload:   payload,
	}
}

// --- 资源与服务器 ---

// ContentFormat 常量：0=text/plain, 40=application/link-format, 50=json。
const (
	ContentFormatTextPlain    = 0
	ContentFormatLinkFormat   = 40
	ContentFormatJSON         = 50
)

// Handler CoAP 资源处理器：入参为去头后的请求；返回 (code, payload, contentType, err)。
type Handler func(req *Message) (Code, []byte, int, error)

// Registry CoAP 资源注册表。
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry 新建资源注册表并挂载 /.well-known/core。
func NewRegistry() *Registry {
	r := &Registry{handlers: map[string]Handler{}}
	r.handlers["/.well-known/core"] = r.handleWellKnownCore
	return r
}

// Register 注册资源路径对应的处理器。
func (r *Registry) Register(path string, h Handler) {
	r.handlers[path] = h
}

func (r *Registry) handleWellKnownCore(req *Message) (Code, []byte, int, error) {
	if req.Code != CodeGet {
		return CodeMethodNotAllowed, nil, ContentFormatLinkFormat, nil
	}
	paths := make([]string, 0, len(r.handlers))
	for p := range r.handlers {
		if p != "/.well-known/core" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var sb strings.Builder
	for _, p := range paths {
		sb.WriteString("</")
		sb.WriteString(strings.TrimPrefix(p, "/"))
		sb.WriteString(">;ct=0,")
	}
	out := strings.TrimSuffix(sb.String(), ",")
	return CodeContent, []byte(out), ContentFormatLinkFormat, nil
}

// Serve 处理单个请求数据报并返回响应数据报（nil 表示无需回复，如 RST/空消息）。
// 路径匹配：先精确匹配，其次匹配注册键以 "*" 结尾的前缀通配（如 "/rd*" 同时命中 "/rd" 与 "/rd/1"）。
func (r *Registry) Serve(req *Message) (*Message, error) {
	if req.Code == CodeEmpty {
		return nil, nil
	}
	path := req.UriPath()
	h, ok := r.handlers[path]
	if !ok {
		// 前缀通配：按最长注册键优先。
		best := ""
		for key := range r.handlers {
			if strings.HasSuffix(key, "*") && strings.HasPrefix(path, strings.TrimSuffix(key, "*")) {
				if len(key) > len(best) {
					best = key
				}
			}
		}
		if best != "" {
			h = r.handlers[best]
			ok = true
		}
	}
	if !ok {
		return NewAck(req, CodeNotFound, []byte("resource not found")), nil
	}
	code, payload, _, err := h(req)
	if err != nil {
		return NewAck(req, CodeInternal, []byte(err.Error())), nil
	}
	if req.Type == TypeConfirmable {
		return NewAck(req, code, payload), nil
	}
	// NON：独立 NON 响应，消息号自增由调用方维护（此处复用请求号便于无状态测试）。
	return &Message{Type: TypeNonConfirm, Code: code, MessageID: req.MessageID, Token: req.Token, Payload: payload}, nil
}

// Server UDP CoAP 服务器。
type Server struct {
	Registry *Registry
	now      func() time.Time
}

// ListenAndServe 在 addr 上服务；每个连接处理单数据报（处理内并发上限由调用方把握）。
func (s *Server) ListenAndServe(addr string) error {
	if s.Registry == nil {
		return fmt.Errorf("coap: Registry 未设置")
	}
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer pc.Close()
	return s.servePacket(pc)
}

func (s *Server) servePacket(pc net.PacketConn) error {
	buf := make([]byte, maxMessageSize)
	for {
		n, raddr, err := pc.ReadFrom(buf)
		if err != nil {
			return err
		}
		raw := append([]byte{}, buf[:n]...)
		go func() {
			msg, derr := Decode(raw)
			if derr != nil {
				return
			}
			resp, serr := s.Registry.Serve(msg)
			if serr != nil || resp == nil {
				return
			}
			out, eerr := resp.Encode()
			if eerr == nil {
				_, _ = pc.WriteTo(out, raddr)
			}
		}()
	}
}
