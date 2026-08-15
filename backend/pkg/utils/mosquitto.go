// 文件用途：提供 mosquitto 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 func BuildMosquittoPubCommand、func quoteCommandArg、type MQTTParams、func ParseMosquittoPubCommand 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/go-basic/uuid"
)

// mosquitto_pub -h 192.0.2.10 -p 1883 -t "devices/telemetry" -m "{\"temperature\":25}" -u "device_001" -P "copy_device_password" -i "aetherlink_device_001"
func BuildMosquittoPubCommand(host string, port string, username string, password string, topic string, payload string, clientId string) string {
	var sb strings.Builder
	sb.WriteString("mosquitto_pub")
	sb.WriteString(fmt.Sprintf(" -h %s", quoteCommandArg(host)))
	sb.WriteString(fmt.Sprintf(" -p %s", quoteCommandArg(port)))

	if topic != "" {
		sb.WriteString(fmt.Sprintf(" -t %s", quoteCommandArg(topic)))
	}
	if payload != "" {
		sb.WriteString(fmt.Sprintf(" -m %s", quoteCommandArg(payload)))
	}
	if username != "" {
		sb.WriteString(fmt.Sprintf(" -u %s", quoteCommandArg(username)))
	}
	if password != "" {
		sb.WriteString(fmt.Sprintf(" -P %s", quoteCommandArg(password)))
	}
	if clientId != "" {
		sb.WriteString(fmt.Sprintf(" -i %s", quoteCommandArg(clientId)))
	}
	return sb.String()
}

func quoteCommandArg(value string) string {
	escaped := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		"`", "\\`",
	).Replace(value)

	return `"` + escaped + `"`
}

type MQTTParams struct {
	Host     string
	Port     string
	Username string
	Password string
	Topic    string
	Payload  string
	ClientId string
}

// Parse mosquitto_pub command text submitted by the telemetry simulation form.
// mosquitto_pub -h 192.0.2.10 -p 1883 -t "devices/telemetry" -m "{\"temperature\":25}" -u "device_001" -P "copy_device_password" -i "aetherlink_device_001"
func ParseMosquittoPubCommand(command string) (*MQTTParams, error) {
	args, err := splitCommandLine(command)
	if err != nil {
		return nil, err
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	if args[0] != "mosquitto_pub" {
		return nil, fmt.Errorf("invalid command: %s", args[0])
	}

	args = args[1:]

	f := flag.NewFlagSet("mqtt", flag.ContinueOnError)
	f.SetOutput(io.Discard)

	host := f.String("h", "localhost", "MQTT broker host")
	port := f.String("p", "1883", "MQTT broker port")
	user := f.String("u", "", "MQTT username")
	password := f.String("P", "", "MQTT password")
	topic := f.String("t", "", "MQTT topic")
	message := f.String("m", "", "MQTT payload")
	clientId := f.String("i", "", "MQTT client id")

	err = f.Parse(args)
	if err != nil {
		return nil, err
	}

	if *clientId == "" || *clientId == "0" {
		*clientId = "mosquitto_pub_" + uuid.New()[0:8]
	}

	params := &MQTTParams{
		Host:     *host,
		Port:     *port,
		Username: *user,
		Password: *password,
		Topic:    *topic,
		Payload:  *message,
		ClientId: *clientId,
	}

	return params, nil
}

func splitCommandLine(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	hasArg := false
	escaped := false

	flush := func() {
		if hasArg {
			args = append(args, current.String())
			current.Reset()
			hasArg = false
		}
	}

	for _, r := range strings.TrimSpace(command) {
		if escaped {
			current.WriteRune(r)
			hasArg = true
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			hasArg = true
			continue
		}

		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			hasArg = true
			continue
		}

		if r == '"' || r == '\'' {
			quote = r
			hasArg = true
			continue
		}

		if unicode.IsSpace(r) {
			flush()
			continue
		}

		current.WriteRune(r)
		hasArg = true
	}

	if escaped {
		return nil, fmt.Errorf("unfinished escape sequence")
	}

	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted argument")
	}

	flush()
	return args, nil
}
