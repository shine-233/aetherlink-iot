// 文件用途：AI 遥测查询服务的纯逻辑回归测试（ROADMAP C4）。
// 核心逻辑：验证意图解析、参数钳制、提示词和未配置守卫；公网出站契约由 ai_llm_client_test.go 覆盖。
// 关键注意事项：不发起真实外网请求；租户边界由 service 层守卫保证。
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestParseAiIntentJsonClampsValues(t *testing.T) {
	content := "```json\n{\"device_ids\":[" +
		strings.Repeat(`"d",`, aiIntentMaxDevices+5) + `"x"],` +
		`"keys":["temperature","  ","` + strings.Repeat("k", 100) + `"],` +
		`"hours_back":99999}` + "\n```"

	intent, err := parseAiIntentJson(content)
	require.NoError(t, err)
	require.LessOrEqual(t, len(intent.DeviceIDs), aiIntentMaxDevices)
	require.Len(t, intent.Keys, 1)
	require.Equal(t, "temperature", intent.Keys[0])
	require.Equal(t, 24, intent.HoursBack, "out-of-range hours must fall back to 24")
}

func TestParseAiIntentJsonRejectsNonJson(t *testing.T) {
	_, err := parseAiIntentJson("抱歉，我无法解析")
	require.Error(t, err)
}

func TestQueryTelemetryRequiresConfiguredLLM(t *testing.T) {
	oldKey := viper.Get("ai.llm.api_key")
	defer func() {
		if oldKey != nil {
			viper.Set("ai.llm.api_key", oldKey)
		} else {
			viper.Set("ai.llm.api_key", "")
		}
	}()
	viper.Set("ai.llm.api_key", "")

	svc := &AiQuery{}
	_, err := svc.QueryTelemetry(context.Background(), &AiTelemetryQueryReq{Question: "现在温度多少"}, nil)
	require.Error(t, err, "unconfigured LLM must fail fast even before claims check")

	_, err = svc.QueryTelemetry(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestBuildTelemetryIntentPromptContainsQuestionAndContract(t *testing.T) {
	system, user := buildTelemetryIntentPrompt("过去一小时温度是多少？")
	for _, keyword := range []string{"device_ids", "keys", "hours_back"} {
		require.Contains(t, system, keyword)
	}
	require.Contains(t, user, "过去一小时温度是多少？")
}
