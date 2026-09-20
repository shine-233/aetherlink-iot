// 文件用途：MQTT 摄取路径的负载生成器（ROADMAP P2.3）。
//
// 为什么需要它：`performance/` 的 tiers.json 里写着 `mqttClients: 50/200/500`，
// 但从来没有被施加过——而 MQTT 摄取恰恰是物联网平台**真正的瓶颈路径**。
// 只有 API 基线的数字（`/health`、部署健康检查）不覆盖它，用那批数字谈容量
// 正是路线图 §P2.3 警告的"用假数据掩盖数据库瓶颈"。
//
// 核心逻辑：建立 N 个 MQTT 连接，按目标速率向遥测话题发布载荷，
// 以 **QoS 1 的 PUBACK 往返**作为端到端延迟样本（QoS 0 没有确认，测不出延迟），
// 最后输出吞吐、错误率与延迟百分位。
//
// 关键注意事项：
//   - **这不是 tier 结果**：本工具不施加任何 CPU/内存配额，输出里显式标注
//     `evidenceKind: "local-baseline"` 且 `tierClaim: null`。
//   - QoS 0 下 paho 的 Token 会立即完成，延迟样本等于"本地入队耗时"而非网络往返。
//     因此默认 QoS 1，并在报告里写明实际使用的 QoS。
//   - 预热期样本不计入统计；百分位用**最近秩**而非插值（小样本插值会给出
//     实际从未发生过的延迟值）；错误计入错误率而非丢弃。
//   - 连接失败**立即中止**而不是"用剩下的连接继续跑"：那会得出一个看起来正常、
//     实际并发数不对的吞吐数字。
//
// 用法：
//
//	go run ./cmd/mqttbench -broker tcp://127.0.0.1:1883 -topic devices/telemetry \
//	  -clients 4 -rate 50 -duration 15 -warmup 3 \
//	  -payload '{"temperature":25.5,"humidity":60,"rssi":-52,"online":true}'
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type sample struct {
	elapsedMs float64
	ok        bool
}

// nearestRank 百分位：取第 ceil(p/100 * n) 个样本（1-based）。
// 刻意不用线性插值——小样本下插值会产出"从未实际发生过的延迟值"。
func nearestRank(sorted []float64, percentile float64) *float64 {
	if len(sorted) == 0 {
		return nil
	}
	rank := int(float64(len(sorted))*percentile/100 + 0.999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	value := sorted[rank-1]
	return &value
}

func main() {
	broker := flag.String("broker", "tcp://127.0.0.1:1883", "MQTT broker URL")
	clientIDPrefix := flag.String("client-id-prefix", "mqttbench-", "客户端 ID 前缀（会自动追加序号）")
	username := flag.String("username", "", "MQTT 用户名（broker 允许匿名时留空）")
	password := flag.String("password", "", "MQTT 密码")
	topic := flag.String("topic", "devices/telemetry", "遥测上报话题")
	clients := flag.Int("clients", 1, "并发 MQTT 连接数")
	ratePerClient := flag.Int("rate", 0, "每个连接的每秒发布数；0 表示不限速（尽快发）")
	durationSeconds := flag.Int("duration", 15, "测量时长（秒）")
	warmupSeconds := flag.Int("warmup", 3, "预热时长（秒），样本不计入统计")
	qos := flag.Int("qos", 1, "发布 QoS：0 或 1（QoS 0 无确认，延迟不可测）")
	connectTimeoutSeconds := flag.Int("connect-timeout", 10, "单次连接超时（秒）")
	payload := flag.String("payload", `{"temperature_1":25.5,"temperature_2":26.25,"switch_1":1,"switch_2":0}`, "发布载荷（JSON）")
	deviceID := flag.String("device-id", "", "设备内部 ID；配合 -envelope 把扁平载荷包成 adapter 的原生信封")
	useEnvelope := flag.Bool("envelope", false, "按 MQTT adapter 原生契约发信封 {device_id, values:<base64(扁平JSON)>}")
	outPath := flag.String("out", "", "报告输出路径（留空则只打印到 stdout）")
	flag.Parse()

	if *qos != 0 && *qos != 1 {
		fmt.Fprintln(os.Stderr, "qos must be 0 or 1")
		os.Exit(2)
	}
	if !json.Valid([]byte(*payload)) {
		fmt.Fprintln(os.Stderr, "payload must be valid JSON")
		os.Exit(2)
	}
	if *clients < 1 {
		fmt.Fprintln(os.Stderr, "clients must be >= 1")
		os.Exit(2)
	}

	// 载荷形态决定消息**会不会被后端摄取**，而不是只被 broker 收下。
	//
	// 这是本工具最容易得出假数字的地方：broker 对任何合法 MQTT 载荷都会回 PUBACK，
	// 所以"发布成功、0 失败、吞吐很高"完全可以在消息全被 adapter 丢弃的情况下出现。
	// adapter 的 verifyPayload 要求 `{"device_id":...,"values":<base64>}` 信封
	// （`publicPayload.Values` 是 []byte，Go 的 json 包把它编成 base64）；
	// 扁平载荷只有在 gmqtt 的 aetherlink 插件于 broker 侧补信封时才成立。
	published := *payload
	if *useEnvelope {
		if *deviceID == "" {
			fmt.Fprintln(os.Stderr, "-envelope 需要同时提供 -device-id")
			os.Exit(2)
		}
		envelope, err := json.Marshal(map[string]string{
			"device_id": *deviceID,
			"values":    base64.StdEncoding.EncodeToString([]byte(*payload)),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "build envelope failed: %v\n", err)
			os.Exit(1)
		}
		published = string(envelope)
	}

	// ---- 建立连接：任一连接失败即中止 ----
	connections := make([]mqtt.Client, 0, *clients)
	for index := 0; index < *clients; index++ {
		options := mqtt.NewClientOptions().
			AddBroker(*broker).
			SetClientID(fmt.Sprintf("%s%d", *clientIDPrefix, index)).
			SetConnectTimeout(time.Duration(*connectTimeoutSeconds) * time.Second).
			SetAutoReconnect(false).
			SetCleanSession(true)
		if *username != "" {
			options.SetUsername(*username)
		}
		if *password != "" {
			options.SetPassword(*password)
		}

		client := mqtt.NewClient(options)
		token := client.Connect()
		if !token.WaitTimeout(time.Duration(*connectTimeoutSeconds) * time.Second) {
			fmt.Fprintf(os.Stderr, "连接 %d 超时；中止而不是用剩余连接继续跑（那会得出并发数不对的吞吐数字）\n", index)
			os.Exit(1)
		}
		if err := token.Error(); err != nil {
			fmt.Fprintf(os.Stderr, "连接 %d 失败：%v；中止\n", index, err)
			os.Exit(1)
		}
		connections = append(connections, client)
	}
	defer func() {
		for _, client := range connections {
			client.Disconnect(250)
		}
	}()

	runPhase := func(durationSeconds int, collect bool) []sample {
		if durationSeconds <= 0 {
			return nil
		}
		deadline := time.Now().Add(time.Duration(durationSeconds) * time.Second)
		var mutex sync.Mutex
		collected := make([]sample, 0, 1024)

		var waitGroup sync.WaitGroup
		for _, client := range connections {
			waitGroup.Add(1)
			go func(client mqtt.Client) {
				defer waitGroup.Done()
				var ticker *time.Ticker
				var tick <-chan time.Time
				if *ratePerClient > 0 {
					ticker = time.NewTicker(time.Second / time.Duration(*ratePerClient))
					defer ticker.Stop()
					tick = ticker.C
				}
				for time.Now().Before(deadline) {
					if tick != nil {
						remaining := time.Until(deadline)
						if remaining <= 0 {
							return
						}
						select {
						case <-tick:
						case <-time.After(remaining):
							return
						}
					}
					startedAt := time.Now()
					token := client.Publish(*topic, byte(*qos), false, published)
					token.Wait()
					elapsedMs := float64(time.Since(startedAt).Nanoseconds()) / 1e6
					if collect {
						mutex.Lock()
						collected = append(collected, sample{elapsedMs: elapsedMs, ok: token.Error() == nil})
						mutex.Unlock()
					}
				}
			}(client)
		}
		waitGroup.Wait()
		return collected
	}

	// 预热：样本丢弃，只为让连接、TCP 窗口与 broker 内部结构进入稳态。
	runPhase(*warmupSeconds, false)

	startedAt := time.Now()
	samples := runPhase(*durationSeconds, true)
	finishedAt := time.Now()
	wallSeconds := finishedAt.Sub(startedAt).Seconds()

	latencies := make([]float64, 0, len(samples))
	failures := 0
	for _, item := range samples {
		if item.ok {
			latencies = append(latencies, item.elapsedMs)
		} else {
			failures++
		}
	}
	sort.Float64s(latencies)

	var mean *float64
	if len(latencies) > 0 {
		sum := 0.0
		for _, value := range latencies {
			sum += value
		}
		average := sum / float64(len(latencies))
		mean = &average
	}
	var minValue, maxValue *float64
	if len(latencies) > 0 {
		first, last := latencies[0], latencies[len(latencies)-1]
		minValue, maxValue = &first, &last
	}

	errorRate := 0.0
	if len(samples) > 0 {
		errorRate = float64(failures) / float64(len(samples))
	}

	report := map[string]any{
		"schema":       "aetherlink.performance.mqtt-ingest-baseline.v1",
		"evidenceKind": "local-baseline",
		"tierClaim":    nil,
		"tierClaimReason": "tiers.json 的 mqttClients 档位描述资源限制；本工具未施加任何 CPU/内存配额，" +
			"故不可作为任一档位的达标证据。",
		"target": map[string]any{"broker": *broker, "topic": *topic, "qos": *qos},
		"parameters": map[string]any{
			"clients":         *clients,
			"ratePerClient":   *ratePerClient,
			"durationSeconds": *durationSeconds,
			"warmupSeconds":   *warmupSeconds,
			"payloadBytes":    len(published),
			"envelope":        *useEnvelope,
		},
		"window": map[string]any{
			"startedAt":   startedAt.UTC().Format(time.RFC3339Nano),
			"finishedAt":  finishedAt.UTC().Format(time.RFC3339Nano),
			"wallSeconds": wallSeconds,
		},
		"results": map[string]any{
			"published":         len(samples),
			"failures":          failures,
			"errorRate":         errorRate,
			"messagesPerSecond": float64(len(samples)) / wallSeconds,
			"ackLatencyMs": map[string]any{
				"min":  minValue,
				"p50":  nearestRank(latencies, 50),
				"p90":  nearestRank(latencies, 90),
				"p95":  nearestRank(latencies, 95),
				"p99":  nearestRank(latencies, 99),
				"max":  maxValue,
				"mean": mean,
			},
		},
		"caveats": []string{
			"QoS 1 的样本本意是 PUBACK 往返；QoS 0 无确认，其延迟只反映本地入队耗时。",
			"**实测发现延迟样本不可信**：p50 恒为 0 ns，而 localhost 的 TCP 往返最快也要几十微秒，" +
				"0 ns 在物理上不可能——说明 paho 的 QoS1 token 对相当一部分发布在 Wait() 之前就已完成，" +
				"测到的是「本地入队」而不是「PUBACK 往返」。因此本报告的 ackLatencyMs **不得作为延迟结论引用**；" +
				"只有 messagesPerSecond / failures 可用（且需用读回后端遥测表来确认消息真的落库）。",
			"本工具只测发布侧，**不读回后端遥测表**。broker 对任何合法载荷都回 PUBACK，" +
				"所以「发布成功、0 失败、吞吐很高」完全可以在消息全被 adapter 丢弃时出现——" +
				"必须另用 GET /telemetry/datas/current/<device_id> 确认落库，否则吞吐数字是假的。",
			"单机回环，broker 与压测进程争用同一 CPU。",
		},
	}

	rendered, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report failed: %v\n", err)
		os.Exit(1)
	}
	if *outPath != "" {
		if err := os.WriteFile(*outPath, rendered, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write report failed: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println(string(rendered))
}
