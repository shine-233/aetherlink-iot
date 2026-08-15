// 文件用途：覆盖 mosquitto 工具函数的 Go 测试。
// 核心逻辑：通过表驱动或边界用例验证通用工具的输入校验、格式转换和错误返回，主要围绕 func TestParseMosquittoPubCommandRejectsEmptyCommand、func TestParseMosquittoPubCommandSupportsQuotedPayload、func TestBuildAndParseMosquittoPubCommandRoundTrip 等声明展开。
// 关键注意事项：工具包被多处业务代码复用，测试断言需保持跨调用方的兼容契约。
// 重构建议：后续可按工具类别拆分公共夹具，并补充失败路径和异常输入覆盖。

package utils

import "testing"

func TestParseMosquittoPubCommandRejectsEmptyCommand(t *testing.T) {
	if _, err := ParseMosquittoPubCommand("   "); err == nil {
		t.Fatal("expected empty command to be rejected")
	}
}

func TestParseMosquittoPubCommandSupportsQuotedPayload(t *testing.T) {
	command := `mosquitto_pub -h 127.0.0.1 -p 1883 -t "devices/telemetry" -m "{\"temperature\":25.5,\"humidity\":60}" -u "user" -P "pass" -i "client"`

	params, err := ParseMosquittoPubCommand(command)
	if err != nil {
		t.Fatalf("ParseMosquittoPubCommand() error = %v", err)
	}

	if params.Host != "127.0.0.1" {
		t.Fatalf("Host = %q", params.Host)
	}
	if params.Port != "1883" {
		t.Fatalf("Port = %q", params.Port)
	}
	if params.Topic != "devices/telemetry" {
		t.Fatalf("Topic = %q", params.Topic)
	}
	if params.Payload != `{"temperature":25.5,"humidity":60}` {
		t.Fatalf("Payload = %q", params.Payload)
	}
	if params.Username != "user" || params.Password != "pass" || params.ClientId != "client" {
		t.Fatalf("unexpected auth/client params: %+v", params)
	}
}

func TestBuildAndParseMosquittoPubCommandRoundTrip(t *testing.T) {
	payload := `{"temp":25.5,"message":"hello world"}`
	password := "pa\"$`\\word"
	command := BuildMosquittoPubCommand(
		"example.com",
		"1883",
		"user name",
		password,
		"devices/telemetry",
		payload,
		"client;id",
	)

	params, err := ParseMosquittoPubCommand(command)
	if err != nil {
		t.Fatalf("ParseMosquittoPubCommand() error = %v; command = %s", err, command)
	}

	if params.Host != "example.com" {
		t.Fatalf("Host = %q", params.Host)
	}
	if params.Port != "1883" {
		t.Fatalf("Port = %q", params.Port)
	}
	if params.Username != "user name" {
		t.Fatalf("Username = %q", params.Username)
	}
	if params.Password != password {
		t.Fatalf("Password = %q", params.Password)
	}
	if params.Topic != "devices/telemetry" {
		t.Fatalf("Topic = %q", params.Topic)
	}
	if params.Payload != payload {
		t.Fatalf("Payload = %q", params.Payload)
	}
	if params.ClientId != "client;id" {
		t.Fatalf("ClientId = %q", params.ClientId)
	}
}
