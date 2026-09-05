// 文件用途：边缘网关遥测云转发（ROADMAP 边缘计算 MVP）。
// 核心逻辑：订阅 uplink 总线的已受理消息（观察者扇出，不影响本地存储链路），
//
//	经 MQTT 转发到上级云 broker（topic = {prefix}/{type}/{device_id}，JSON 信封）。
//	broker 不可达时进入环形缓冲（满则丢最旧并计数），重连成功后按 FIFO 重投，
//	然后恢复实时转发——即"断网缓冲、联网续传"的边缘语义。
//
// 关键注意事项：
//   - fail-open：转发失败绝不阻塞/影响本地遥测入库；缓冲上限默认 10000 条；
//   - paho AutoReconnect 关闭，连接与重投由本包统一管理（保证缓冲顺序语义）；
//   - 配置 edge.forward.*（见 ConfigFromViper），默认关闭。
package edgeforward

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/internal/uplink"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// OperationTypeEdgeRelay 边缘中继命令的操作类型（命令审计留痕用）。
const OperationTypeEdgeRelay = "edge-relay"

// Config 边缘转发配置（edge.forward.*）。
type Config struct {
	Enabled     bool   // 总开关（默认 false）
	Broker      string // 云 broker 地址（tcp://host:port）
	TopicPrefix string // topic 前缀，默认 aetherlink/edge
	ClientID    string // MQTT ClientID，默认 aetherlink-edge
	Username    string // 可选
	Password    string // 可选
	BufferLimit int    // 断连缓冲上限（条），默认 10000，满则丢最旧
	QoS         byte   // 发布 QoS，默认 1

	// 云端下行命令（边缘 RPC）
	CommandEnabled    bool   // 是否订阅云端命令 topic（默认 false）
	CommandTopic      string // 命令 topic（支持 MQTT 通配符），默认 {prefix}/cmd/+
	CommandOperatorID string // 命令审计记录的操作人，默认 edge-relay
}

// ConfigFromViper 从 viper 读取 edge.forward.* 配置。
func ConfigFromViper() Config {
	getStr := func(key string) string { return strings.TrimSpace(viper.GetString(key)) }
	cfg := Config{
		Enabled:     viper.GetBool("edge.forward.enabled"),
		Broker:      getStr("edge.forward.broker"),
		TopicPrefix: getStr("edge.forward.topic-prefix"),
		ClientID:    getStr("edge.forward.client-id"),
		Username:    getStr("edge.forward.username"),
		Password:    getStr("edge.forward.password"),
		BufferLimit: int(viper.GetInt("edge.forward.buffer-limit")),
		QoS:         byte(viper.GetUint("edge.forward.qos")),
	}
	cfg.CommandEnabled = viper.GetBool("edge.forward.command-enabled")
	cfg.CommandTopic = getStr("edge.forward.command-topic")
	cfg.CommandOperatorID = getStr("edge.forward.command-operator-id")
	if cfg.TopicPrefix == "" {
		cfg.TopicPrefix = "aetherlink/edge"
	}
	if cfg.CommandTopic == "" {
		cfg.CommandTopic = cfg.TopicPrefix + "/cmd/+"
	}
	if cfg.CommandOperatorID == "" {
		cfg.CommandOperatorID = "edge-relay"
	}
	if cfg.ClientID == "" {
		cfg.ClientID = "aetherlink-edge"
	}
	if cfg.BufferLimit <= 0 {
		cfg.BufferLimit = 10000
	}
	if cfg.QoS > 1 {
		cfg.QoS = 1
	}
	return cfg
}

// outMessage 待转发消息（断连缓冲单元）。
type outMessage struct {
	topic   string
	payload []byte
}

// Forwarder 边缘转发器：bus 订阅 → 云 MQTT。
type Forwarder struct {
	cfg  Config
	bus  *uplink.Bus
	log  *logrus.Logger
	sub  *uplink.AcceptedMessageSubscription
	stop chan struct{}
	done chan struct{}
	// subscribed 在总线订阅生效后关闭（测试/调用方等待该信号后再发布）。
	subscribed chan struct{}
	subOnce    sync.Once

	mu         sync.Mutex
	client     mqtt.Client
	connected  bool
	buffer     []*outMessage // FIFO；满则丢最旧
	bufDropped uint64

	livePublished uint64
	bufFlushed    uint64

	// 云端下行命令
	sink            CommandSink
	commandsApplied uint64
}

// New 构造转发器并订阅总线（观察者缓冲 256，与总线语义一致）。
func New(bus *uplink.Bus, cfg Config, log *logrus.Logger) *Forwarder {
	if log == nil {
		log = logrus.New()
	}
	return &Forwarder{
		cfg:        cfg,
		bus:        bus,
		log:        log,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		subscribed: make(chan struct{}),
	}
}

// WithCommandSink 绑定云端命令落地下游（返回自身便于链式构造）。
func (f *Forwarder) WithCommandSink(sink CommandSink) *Forwarder {
	f.sink = sink
	return f
}

// Subscribed 返回总线订阅就绪信号（closed 即已生效）。
func (f *Forwarder) Subscribed() <-chan struct{} { return f.subscribed }

// Start 启动消费与连接管理 goroutine（不阻塞）。
func (f *Forwarder) Start() {
	go f.run()
}

// Stop 停止转发器（退出订阅与连接）。
func (f *Forwarder) Stop() {
	close(f.stop)
	<-f.done
	if f.sub != nil {
		f.sub.Close()
	}
	f.mu.Lock()
	if f.client != nil && f.connected {
		f.client.Disconnect(250)
	}
	f.mu.Unlock()
	f.log.WithFields(logrus.Fields{
		"live_published": atomic.LoadUint64(&f.livePublished),
		"buf_flushed":    atomic.LoadUint64(&f.bufFlushed),
		"buf_dropped":    atomic.LoadUint64(&f.bufDropped),
	}).Info("edge forward stopped")
}

func (f *Forwarder) run() {
	defer close(f.done)
	sub, err := f.bus.SubscribeAcceptedMessages(256)
	if err != nil {
		f.log.Warn("edge forward: 总线订阅失败，转发器退出: ", err)
		return
	}
	f.sub = sub
	f.subOnce.Do(func() { close(f.subscribed) })
	// 连接管理独立 goroutine：重连 + 重投。
	go f.connectLoop()
	for {
		select {
		case <-f.stop:
			return
		case msg, ok := <-sub.Messages:
			if !ok {
				return
			}
			f.handle(msg)
		}
	}
}

// handle 单条消息：在线直发，失败或离线进缓冲。
func (f *Forwarder) handle(msg *uplink.DeviceMessage) {
	topic := f.cfg.TopicPrefix + "/" + msg.Type + "/" + msg.DeviceID
	payload := envelope(msg)
	f.mu.Lock()
	if !f.connected {
		f.enqueueLocked(topic, payload)
		f.mu.Unlock()
		return
	}
	token := f.client.Publish(topic, f.cfg.QoS, false, payload)
	f.mu.Unlock()
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		f.log.Warn("edge forward: 云转发失败，转入缓冲: ", token.Error())
		f.setDisconnected()
		f.mu.Lock()
		f.enqueueLocked(topic, payload)
		f.mu.Unlock()
		return
	}
	atomic.AddUint64(&f.livePublished, 1)
}

// enqueueLocked 入环形缓冲；满则丢最旧并计数（调用方持有 f.mu）。
func (f *Forwarder) enqueueLocked(topic string, payload []byte) {
	if len(f.buffer) >= f.cfg.BufferLimit {
		f.buffer = f.buffer[1:]
		atomic.AddUint64(&f.bufDropped, 1)
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	f.buffer = append(f.buffer, &outMessage{topic: topic, payload: cp})
}

// setDisconnected 标记断连（触发 connectLoop 重连）。
func (f *Forwarder) setDisconnected() {
	f.mu.Lock()
	if f.connected {
		f.connected = false
	}
	f.mu.Unlock()
}

// connectLoop 连接管理：仅断开状态每 3s 重试；连上后先重投缓冲再恢复实时。
// 已连接时不得重复 tryConnect（会废弃在用连接造成消息丢失）。
func (f *Forwarder) connectLoop() {
	for {
		select {
		case <-f.stop:
			return
		default:
		}
		f.mu.Lock()
		connected := f.connected
		f.mu.Unlock()
		if connected {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if f.tryConnect() {
			f.redrive()
			time.Sleep(500 * time.Millisecond)
		} else {
			time.Sleep(3 * time.Second)
		}
	}
}

// tryConnect 建立云 MQTT 连接；成功后登记 client 并置 connected。
func (f *Forwarder) tryConnect() bool {
	opts := mqtt.NewClientOptions().
		AddBroker(f.cfg.Broker).
		SetClientID(f.cfg.ClientID).
		SetCleanSession(true).
		SetAutoReconnect(false).
		SetConnectRetry(false).
		SetConnectTimeout(3 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetWriteTimeout(5 * time.Second)
	if f.cfg.Username != "" {
		opts.SetUsername(f.cfg.Username)
		opts.SetPassword(f.cfg.Password)
	}
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		f.setDisconnected()
		f.log.Warn("edge forward: 云连接断开，进入缓冲模式: ", err)
	})
	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		return false
	}
	f.mu.Lock()
	f.client = client
	f.connected = true
	f.mu.Unlock()
	f.log.WithField("broker", f.cfg.Broker).Info("edge forward: 云连接已建立")
	if f.cfg.CommandEnabled && f.sink != nil {
		token := client.Subscribe(f.cfg.CommandTopic, f.cfg.QoS, func(_ mqtt.Client, msg mqtt.Message) {
			f.handleCommand(msg.Topic(), msg.Payload())
		})
		if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
			f.log.Warn("edge forward: 云端命令订阅失败 topic=", f.cfg.CommandTopic, " err=", token.Error())
		} else {
			f.log.WithField("topic", f.cfg.CommandTopic).Info("edge forward: 云端命令订阅已就绪")
		}
	}
	return true
}

// redrive 按序重投缓冲（成功一条删一条；失败立即停止保留剩余）。
func (f *Forwarder) redrive() {
	for {
		f.mu.Lock()
		if !f.connected || len(f.buffer) == 0 {
			f.mu.Unlock()
			return
		}
		m := f.buffer[0]
		client := f.client
		f.mu.Unlock()
		token := client.Publish(m.topic, f.cfg.QoS, false, m.payload)
		if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
			f.setDisconnected()
			return
		}
		f.mu.Lock()
		// 防御：redrive 期间缓冲可能被并发追加深拷贝条目，仅移除已确认的首条。
		if len(f.buffer) > 0 && string(f.buffer[0].payload) == string(m.payload) && f.buffer[0].topic == m.topic {
			f.buffer = f.buffer[1:]
		}
		f.mu.Unlock()
		atomic.AddUint64(&f.bufFlushed, 1)
	}
}

// envelope 把 DeviceMessage 包装为转发 JSON（payload 保持原始 JSON 内嵌）。
func envelope(msg *uplink.DeviceMessage) []byte {
	buf := make([]byte, 0, len(msg.Payload)+128)
	buf = append(buf, `{"device_id":`...)
	buf = appendJSONString(buf, msg.DeviceID)
	buf = append(buf, `,"tenant_id":`...)
	buf = appendJSONString(buf, msg.TenantID)
	buf = append(buf, `,"type":`...)
	buf = appendJSONString(buf, msg.Type)
	buf = append(buf, `,"ts":`...)
	buf = appendInt64(buf, msg.Timestamp)
	if len(msg.Payload) > 0 {
		buf = append(buf, `,"payload":`...)
		if isJSONObject(msg.Payload) {
			buf = append(buf, msg.Payload...)
		} else {
			buf = appendJSONString(buf, string(msg.Payload))
		}
	}
	if len(msg.Metadata) > 0 {
		buf = append(buf, `,"metadata":`...)
		meta := make(map[string]interface{}, len(msg.Metadata))
		for k, v := range msg.Metadata {
			meta[k] = v
		}
		metaJSON, err := jsonMarshal(meta)
		if err == nil {
			buf = append(buf, metaJSON...)
		} else {
			buf = append(buf, `{}`...)
		}
	}
	buf = append(buf, '}')
	return buf
}
