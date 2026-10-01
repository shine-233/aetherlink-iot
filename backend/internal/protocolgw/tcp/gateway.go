// 文件用途：TCP 入站网关（TP-03）——net.Listen 监听、首帧 device_number 注册、遥测汇入与下行投递。
// 核心逻辑：配置门控 protocols.tcp.enabled（默认关闭）；每连接一个读 goroutine + 独立 FrameAccumulator，
// 首帧经 ParseRegistrationFrame+DeviceResolver 凭证映射（fail-closed：未知/禁用设备直接断开），
// 注册进 Registry 并触发缓冲冲刷；后续帧经 Ingestor 汇入 uplink 总线；下行经 DeliverCommand
// 在线直投/离线入环形缓冲（edgeforward 同款）。
// 关键注意事项：listen 在 Start 内同步完成（端口占用即启动失败，比 CoAP 的超时探测更确定）；
// 注册窗口（RegistrationWindow）内不注册的连接踢除；本层不处理 TLS（residual：明文传输，
// 与 CoAP/UDP 面同水位，公网部署需前置 TLS 终结）。
// 重构建议：多副本部署时把注册簿与命令缓冲外置（Redis/队列）；帧格式扩展点在 FrameMode。
package tcp

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/internal/protocolgw"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// writeTimeout 单次下行帧写超时（写死连接立即转缓冲，不拖调用方）。
const writeTimeout = 5 * time.Second

// readChunkSize 读循环单次读取缓冲大小。
const readChunkSize = 4096

// Config TCP 网关配置（protocols.tcp.*）。
type Config struct {
	Enabled            bool
	Addr               string    // 完整监听地址（host:port）
	Mode               FrameMode // 帧格式，默认 length-prefix
	MaxFrameSize       int       // 单帧 payload 上限，默认 DefaultMaxFrameSize
	CommandBufferLimit int       // 单设备下行缓冲上限，默认 DefaultCommandBufferLimit
	IdleTimeoutSeconds int       // 注册后读空闲超时（秒），0=不启用，默认 300
}

// DefaultConfig 读取 viper（protocols.tcp.enabled/port，host/frame-mode/max-frame-size/
// command-buffer-limit/idle-timeout-seconds 可选），默认关闭。
func DefaultConfig() Config {
	host := viper.GetString("protocols.tcp.host")
	if host == "" {
		host = "0.0.0.0"
	}
	port := viper.GetInt("protocols.tcp.port")
	if port <= 0 {
		port = DefaultTCPPort
	}
	mode := FrameMode(viper.GetString("protocols.tcp.frame-mode"))
	if mode == "" {
		mode = FrameModeLengthPrefix
	}
	maxFrame := viper.GetInt("protocols.tcp.max-frame-size")
	if maxFrame <= 0 {
		maxFrame = DefaultMaxFrameSize
	}
	bufLimit := viper.GetInt("protocols.tcp.command-buffer-limit")
	if bufLimit <= 0 {
		bufLimit = DefaultCommandBufferLimit
	}
	idle := viper.GetInt("protocols.tcp.idle-timeout-seconds")
	if idle < 0 {
		idle = DefaultIdleTimeoutSeconds
	}
	return Config{
		Enabled:            viper.GetBool("protocols.tcp.enabled"),
		Addr:               net.JoinHostPort(host, strconv.Itoa(port)),
		Mode:               mode,
		MaxFrameSize:       maxFrame,
		CommandBufferLimit: bufLimit,
		IdleTimeoutSeconds: idle,
	}
}

// normalize 兜底归一化（测试手工构造 Config 时保证字段可用）。
func (c Config) normalize() Config {
	if c.Addr == "" {
		c.Addr = net.JoinHostPort("0.0.0.0", strconv.Itoa(DefaultTCPPort))
	}
	if !ValidFrameMode(c.Mode) {
		c.Mode = FrameModeLengthPrefix
	}
	if c.MaxFrameSize <= 0 {
		c.MaxFrameSize = DefaultMaxFrameSize
	}
	if c.CommandBufferLimit <= 0 {
		c.CommandBufferLimit = DefaultCommandBufferLimit
	}
	if c.IdleTimeoutSeconds < 0 {
		c.IdleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}
	return c
}

// idleTimeout 读空闲超时（0 表示不启用）。
func (c Config) idleTimeout() time.Duration {
	if c.IdleTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(c.IdleTimeoutSeconds) * time.Second
}

// Dependencies 网关依赖（app 装配层注入）。
type Dependencies struct {
	Resolver  protocolgw.DeviceResolver  // 设备号 → 设备身份（凭证映射，fail-closed）
	Publisher protocolgw.UplinkPublisher // 遥测汇入下游（*uplink.Bus 满足）
}

// Gateway TCP 入站网关实例。
type Gateway struct {
	cfg      Config
	deps     Dependencies
	log      *logrus.Logger
	registry *Registry
	ingestor *Ingestor
	bufs     *downlinkBufferer

	ln      net.Listener
	stopped atomic.Bool
	stopMu  sync.Mutex
	wg      sync.WaitGroup

	// 诊断计数
	connections   atomic.Uint64 // 累计接受连接数
	registrations atomic.Uint64 // 累计成功注册次数（含顶替重连）
	rejected      atomic.Uint64 // 注册被拒（非法帧/未知设备）次数
	cmdDelivered  atomic.Uint64 // 在线直投成功条数
	cmdBuffered   atomic.Uint64 // 入缓冲条数
	cmdFlushed    atomic.Uint64 // 续传投递成功条数
}

// Start 按配置启动网关；未启用返回 nil,nil；依赖缺失或端口占用返回错误。
func Start(cfg Config, deps Dependencies, log *logrus.Logger) (*Gateway, error) {
	cfg = cfg.normalize()
	if !cfg.Enabled {
		return nil, nil
	}
	if log == nil {
		log = logrus.New()
	}
	if deps.Resolver == nil || deps.Publisher == nil {
		return nil, errors.New("tcp gateway: resolver and publisher are required")
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("tcp gateway listen %s: %w", cfg.Addr, err)
	}

	g := &Gateway{
		cfg:      cfg,
		deps:     deps,
		log:      log,
		ingestor: NewIngestor(deps.Publisher, log),
		bufs:     newDownlinkBufferer(cfg.CommandBufferLimit),
		ln:       ln,
	}
	// 注册即冲刷：设备重连后把离线期缓冲的命令按 FIFO 续传，然后恢复实时直投。
	g.registry = NewRegistry(g.flushOnRegister, nil)
	g.wg.Add(1)
	go g.acceptLoop()
	log.WithFields(logrus.Fields{
		"addr":      cfg.Addr,
		"mode":      string(cfg.Mode),
		"max_frame": cfg.MaxFrameSize,
	}).Info("tcp gateway listening")
	return g, nil
}

// acceptLoop 接受循环；Stop 关闭 listener 后退出。
func (g *Gateway) acceptLoop() {
	defer g.wg.Done()
	for {
		conn, err := g.ln.Accept()
		if err != nil {
			if g.stopped.Load() || errors.Is(err, net.ErrClosed) {
				return
			}
			g.log.WithError(err).Warn("tcp gateway accept failed")
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
		g.connections.Add(1)
		g.wg.Add(1)
		go func() {
			defer g.wg.Done()
			g.handleConn(conn)
		}()
	}
}

// handleConn 单连接读循环：首帧注册（fail-closed），后续帧遥测摄取；帧违规/超时即断开。
func (g *Gateway) handleConn(conn net.Conn) {
	remote := conn.RemoteAddr().String()
	defer conn.Close()

	acc, err := NewFrameAccumulator(g.cfg.Mode, g.cfg.MaxFrameSize)
	if err != nil {
		g.log.WithError(err).Error("tcp gateway: accumulator init failed")
		return
	}
	var sess *session
	defer func() {
		if g.registry.Deregister(sess) {
			g.log.WithField("device_number", sess.number).Info("tcp gateway: 会话断开")
		}
	}()

	// 注册窗口：未注册连接只给 RegistrationWindow 时间。
	_ = conn.SetReadDeadline(time.Now().Add(RegistrationWindow))
	buf := make([]byte, readChunkSize)
	for {
		n, readErr := conn.Read(buf)
		if n > 0 {
			frames, ferr := acc.Feed(buf[:n])
			if ferr != nil {
				g.log.WithFields(logrus.Fields{"remote": utils.SanitizeForLog(remote), "error": utils.SanitizeForLog(ferr.Error())}).
					Warn("tcp gateway: 帧违规，断开连接")
				return
			}
			for _, frame := range frames {
				if sess == nil {
					s, regErr := g.registerConn(conn, remote, frame)
					if regErr != nil {
						g.log.WithFields(logrus.Fields{"remote": utils.SanitizeForLog(remote), "error": utils.SanitizeForLog(regErr.Error())}).
							Warn("tcp gateway: 注册被拒，断开连接")
						g.rejected.Add(1)
						return
					}
					sess = s
					g.registrations.Add(1)
					// 注册成功后切换为空闲超时（未启用则清零为不超时）。
					g.applyIdleDeadline(conn)
					continue
				}
				g.ingestor.ingest(sess.number, sess.identity, frame)
			}
		}
		if readErr != nil {
			return // 对端关闭/超时：defer 完成去注册
		}
		if sess != nil {
			g.applyIdleDeadline(conn)
		}
	}
}

// applyIdleDeadline 按配置应用读空闲超时；未启用时清除 deadline。
func (g *Gateway) applyIdleDeadline(conn net.Conn) {
	d := g.cfg.idleTimeout()
	if d <= 0 {
		_ = conn.SetReadDeadline(time.Time{})
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(d))
}

// registerConn 注册帧处理：解析设备号 → 凭证映射 → 登记会话（顶替旧连接）。
// 任何失败返回错误（连接必须关闭——弱凭证边界与 CoAP 面一致：device_number 即准入凭证）。
func (g *Gateway) registerConn(conn net.Conn, remote string, frame []byte) (*session, error) {
	number, err := ParseRegistrationFrame(frame)
	if err != nil {
		return nil, err
	}
	identity, err := g.deps.Resolver.ResolveByNumber(number)
	if err != nil || identity == nil {
		return nil, fmt.Errorf("device_number %q 未映射到启用设备: %v", number, err)
	}
	s := &session{
		number:      number,
		remote:      remote,
		conn:        conn,
		identity:    identity,
		connectedAt: time.Now(),
	}
	if old := g.registry.Register(s); old != nil {
		g.log.WithField("device_number", utils.SanitizeForLog(number)).Warn("tcp gateway: 同号新会话顶替旧连接")
		_ = old.conn.Close() // 旧连接读循环感知关闭后自行去注册（身份校验防误删新会话）
	}
	return s, nil
}

// flushOnRegister 注册事件挂点：把该设备离线期缓冲的命令按 FIFO 续传。
// 写失败立即停止，剩余命令按原序塞回队首（下次注册或写恢复再续传）。
func (g *Gateway) flushOnRegister(number string) {
	sess := g.registry.Lookup(number)
	if sess == nil {
		return // 会话已被顶替/断开：留给下一次注册冲刷
	}
	remaining := g.bufs.drain(number)
	if len(remaining) == 0 {
		return
	}
	delivered := 0
	for _, payload := range remaining {
		if _, err := sess.writeFrame(g.cfg.Mode, payload, g.cfg.MaxFrameSize, writeTimeout); err != nil {
			g.log.WithFields(logrus.Fields{"device_number": number, "error": err}).
				Warn("tcp gateway: 缓冲命令续传中断")
			break
		}
		delivered++
		g.cmdFlushed.Add(1)
	}
	if delivered < len(remaining) {
		g.bufs.requeueFront(number, remaining[delivered:])
	}
	g.bufs.removeIfEmpty(number) // 全部续传完则回收缓冲条目
}

// DeliverCommand 平台下行命令入口：在线直投帧；离线（但本进程见过该设备）入环形缓冲；
// 从未注册过的设备号返回 ErrUnknownDevice（调用方回退 MQTT 通道）。
// 返回 nil 表示"已直投或已入缓冲"（与 edgeforward 断网缓冲同语义：接受即尽力而为）。
func (g *Gateway) DeliverCommand(number string, payload []byte) error {
	if g.stopped.Load() {
		return errors.New("tcp gateway: stopped")
	}
	if number == "" || len(payload) == 0 {
		return errors.New("tcp gateway: device number and payload are required")
	}
	sess := g.registry.Lookup(number)
	if sess == nil {
		if !g.registry.Known(number) {
			return ErrUnknownDevice
		}
		g.bufs.enqueue(number, payload)
		g.cmdBuffered.Add(1)
		return nil
	}
	if _, err := sess.writeFrame(g.cfg.Mode, payload, g.cfg.MaxFrameSize, writeTimeout); err != nil {
		// 写失败视为连接已死：去注册+关闭，命令转入缓冲等待重连续传。
		g.log.WithFields(logrus.Fields{"device_number": number, "error": err}).
			Warn("tcp gateway: 下行直投失败，转入缓冲")
		g.registry.Deregister(sess)
		_ = sess.conn.Close()
		g.bufs.enqueue(number, payload)
		g.cmdBuffered.Add(1)
		return nil
	}
	g.cmdDelivered.Add(1)
	return nil
}

// HandlesDevice 该设备号是否走 TCP 通道（本进程注册过会话，含当前离线）。
// 下行发布回退层据此决定"投 TCP 缓冲"还是"回退 MQTT"。
func (g *Gateway) HandlesDevice(number string) bool {
	return g != nil && g.registry.Known(number)
}

// Stop 停止网关：关 listener、踢全部会话、等读循环退出。
func (g *Gateway) Stop() {
	g.stopMu.Lock()
	if g.stopped.Swap(true) {
		g.stopMu.Unlock()
		return
	}
	_ = g.ln.Close()
	closed := g.registry.CloseAll()
	g.stopMu.Unlock()
	g.wg.Wait()
	g.log.WithFields(logrus.Fields{
		"sessions_closed": closed,
	}).Info("tcp gateway stopped")
}

// Stats 网关诊断计数快照（在线/已知/遥测/下行四组计数）。
type Stats struct {
	Sessions         int // 当前在线会话数
	KnownDevices     int // 本进程注册过的设备数（含离线）
	Connections      int // 累计接受连接数
	Registrations    int // 累计成功注册次数（含顶替重连）
	Rejected         int // 注册被拒次数（非法帧/未知设备）
	Published        int // 遥测已发布条数
	TelemetryDropped int // 遥测丢弃条数（非法帧/发布失败）
	CmdDelivered     int // 下行命令在线直投成功条数
	CmdBuffered      int // 下行命令入缓冲条数
	CmdFlushed       int // 下行命令续传成功条数
	CmdBufDropped    int // 下行缓冲溢出丢弃条数
	CmdPending       int // 当前缓冲中命令总条数
}

// Stats 返回诊断计数快照。
func (g *Gateway) Stats() Stats {
	pending, dropped := g.bufs.totals()
	return Stats{
		Sessions:         g.registry.Count(),
		KnownDevices:     g.registry.KnownCount(),
		Connections:      int(g.connections.Load()),
		Registrations:    int(g.registrations.Load()),
		Rejected:         int(g.rejected.Load()),
		Published:        int(g.ingestor.published.Load()),
		TelemetryDropped: int(g.ingestor.dropped.Load()),
		CmdDelivered:     int(g.cmdDelivered.Load()),
		CmdBuffered:      int(g.cmdBuffered.Load()),
		CmdFlushed:       int(g.cmdFlushed.Load()),
		CmdBufDropped:    dropped,
		CmdPending:       pending,
	}
}
