// 文件用途：AI 遥测查询服务的回归测试（ROADMAP C4）。
// 核心逻辑：用 httptest 模拟 OpenAI 兼容端点，验证意图解析、参数钳制与未配置守卫。
// 关键注意事项：不发起真实外网请求；租户边界由 service 层守卫保证。
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	defer func() { if oldKey != nil { viper.Set("ai.llm.api_key", oldKey) } else { viper.Set("ai.llm.api_key", "") } }()
	viper.Set("ai.llm.api_key", "")

	svc := &AiQuery{}
	_, err := svc.QueryTelemetry(&AiTelemetryQueryReq{Question: "现在温度多少"}, nil)
	require.Error(t, err, "unconfigured LLM must fail fast even before claims check")

	_, err = svc.QueryTelemetry(nil, nil)
	require.Error(t, err)
}

func TestBuildTelemetryIntentPromptContainsQuestionAndContract(t *testing.T) {
	system, user := buildTelemetryIntentPrompt("过去一小时温度是多少？")
	for _, keyword := range []string{"device_ids", "keys", "hours_back"} {
		require.Contains(t, system, keyword)
	}
	require.Contains(t, user, "过去一小时温度是多少？")
}

func TestCallLLMChatParsesChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"device_ids\":[\"dev-1\"],\"keys\":[\"temperature\"],\"hours_back\":1}"}}]}`))
	}))
	defer server.Close()

	oldBase, oldKey := viper.GetString("ai.llm.base_url"), viper.GetString("ai.llm.api_key")
	defer func() {
		viper.Set("ai.llm.base_url", oldBase)
		viper.Set("ai.llm.api_key", oldKey)
	}()
	// httptest server 根路径即 /，因此把 base_url 指到 server URL + /v1
	viper.Set("ai.llm.base_url", server.URL+"/v1")
	viper.Set("ai.llm.api_key", "test-key")

	content, err := callLLMChat(context.Background(), "sys", "user question")
	require.NoError(t, err)

	intent, err := parseAiIntentJson(content)
	require.NoError(t, err)
	require.Equal(t, []string{"dev-1"}, intent.DeviceIDs)
	require.Equal(t, []string{"temperature"}, intent.Keys)
	require.Equal(t, 1, intent.HoursBack)
}
