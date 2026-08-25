// 文件用途：数据转发引擎单元测试——来源映射、HTTP 投递与 MQTT seam 失败路径。
// 核心逻辑：httptest Server 承接 HTTP 目标投递；ForwardDispatchPublisher seam 注入
// 成功/失败假实现，验证 dispatch 的分支行为与 panic 防护。
// 关键注意事项：不依赖真实 broker/数据库；规则对象手工构造。

package forward

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/uplink"

	"github.com/sirupsen/logrus"
)

var errFakeMQTT = errors.New("fake mqtt publisher failure")

func forwardTestLogger() *logrus.Logger {
	return logrus.New()
}

func TestForwardSourceTypesForMapsBusTypes(t *testing.T) {
	cases := map[string]string{
		"telemetry":      "telemetry",
		"attribute":      "property",
		"event":          "event",
		"status":         "status",
		"device_online":  "status",
		"device_offline": "status",
		"unknown":        "",
	}
	for msgType, want := range cases {
		if got := forwardSourceTypesFor(msgType); got != want {
			t.Fatalf("forwardSourceTypesFor(%q) = %q, want %q", msgType, got, want)
		}
	}
}

func TestDispatchDeliversPayloadToHTTPTarget(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	method := "POST"
	url := server.URL
	rule := &model.ForwardRule{
		ID:         "rule-http",
		Name:       "http-forward",
		SourceType: "telemetry",
		TargetType: "http",
		HttpURL:    &url,
		HttpMethod: &method,
	}

	engine := NewForwardEngine(nil, forwardTestLogger())
	payload := `{"temperature":25.5,"device_id":"dev-1"}`
	engine.dispatch(rule, &uplink.DeviceMessage{DeviceID: "dev-1", Type: "telemetry"}, payload)

	select {
	case body := <-received:
		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("delivered payload is not valid JSON: %v", err)
		}
		if parsed["temperature"] != 25.5 {
			t.Fatalf("temperature = %v, want 25.5", parsed["temperature"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for HTTP delivery")
	}
}

func TestDispatchMQTTFailureIsLoggedNotPanicked(t *testing.T) {
	broker := "mqtt://127.0.0.1:1"
	topic := "t"
	original := ForwardDispatchPublisher
	ForwardDispatchPublisher = func(string, string, string, string, string) error {
		return errFakeMQTT
	}
	defer func() { ForwardDispatchPublisher = original }()

	rule := &model.ForwardRule{
		ID:         "rule-mqtt",
		Name:       "mqtt-forward",
		SourceType: "telemetry",
		TargetType: "mqtt",
		MqttBroker: &broker,
		MqttTopic:  &topic,
	}
	engine := NewForwardEngine(nil, forwardTestLogger())
	engine.dispatch(rule, &uplink.DeviceMessage{DeviceID: "dev-1", Type: "telemetry"}, "{}")
}

func TestMatchingRulesFiltersByTemplate(t *testing.T) {
	logger := forwardTestLogger()
	engine := NewForwardEngine(nil, logger)
	templateA := "tpl-a"
	withTemplate := &model.ForwardRule{ID: "r1", Enabled: true, SourceType: "telemetry", DeviceTemplateID: &templateA}
	noTemplate := &model.ForwardRule{ID: "r2", Enabled: true, SourceType: "telemetry"}

	engine.cacheBySrc["telemetry"] = []*model.ForwardRule{withTemplate, noTemplate}
	now := time.Now()
	engine.cacheAt["telemetry"] = now

	msg := &uplink.DeviceMessage{Type: "telemetry", DeviceID: "dev-unknown"}
	got := engine.matchingRules("telemetry", msg)
	if len(got) != 1 || got[0].ID != "r2" {
		t.Fatalf("matchingRules returned %d rules, want only the template-free rule r2", len(got))
	}
}
