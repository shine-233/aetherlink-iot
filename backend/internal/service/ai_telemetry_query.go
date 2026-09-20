// 文件用途：AI 集成——自然语言查询遥测数据（ROADMAP C4 首项）。
// 核心逻辑：LLM 只负责把用户问题解析成结构化查询意图（设备、字段、时间范围），
//
//	实际取数走白名单 DAL 路径并强制租户过滤；不执行模型生成的裸 SQL，杜绝注入面。
//
// 关键注意事项：未配置 ai.llm.api_key 时显式报"未配置"，不伪装成功；
//
//	意图参数必须钳制上限（设备数/字段数/回溯时长），防止放大查询。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	aiIntentMaxDevices   = 10
	aiIntentMaxKeys      = 10
	aiIntentMinHoursBack = 1
	aiIntentMaxHoursBack = 24 * 30
)

// AiQuery AI 集成服务入口。
type AiQuery struct{}

// AiTelemetryQueryReq 自然语言遥测查询请求。
type AiTelemetryQueryReq struct {
	Question string `json:"question" validate:"required,max=500"`
	TenantID string `json:"tenant_id"` // 仅 SYS_ADMIN 生效
}

// AiTelemetryQueryResp 查询响应：意图 + 各设备当前遥测快照。
type AiTelemetryQueryResp struct {
	Question    string                      `json:"question"`
	Intent      telemetryQueryIntent        `json:"intent"`
	Devices     []aiDeviceTelemetrySnapshot `json:"devices"`
	Model       string                      `json:"model"`
	GeneratedAt time.Time                   `json:"generated_at"`
}

type telemetryQueryIntent struct {
	DeviceIDs []string `json:"device_ids"`
	Keys      []string `json:"keys"`
	HoursBack int      `json:"hours_back"`
}

type aiDeviceTelemetrySnapshot struct {
	DeviceID     string            `json:"device_id"`
	DeviceNumber string            `json:"device_number"`
	Name         string            `json:"name"`
	Latest       map[string]string `json:"latest"`
	UpdatedAt    *time.Time        `json:"updated_at"`
}

func aiLLMBaseURL() string {
	if v := strings.TrimSpace(viper.GetString("ai.llm.base_url")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.openai.com/v1"
}

func aiLLMAPIKey() string {
	if v := strings.TrimSpace(viper.GetString("ai.llm.api_key")); v != "" {
		return v
	}
	return strings.TrimSpace(viper.GetString("AI_LLM_API_KEY"))
}

func aiLLMModel() string {
	if v := strings.TrimSpace(viper.GetString("ai.llm.model")); v != "" {
		return v
	}
	return "gpt-4o-mini"
}

// buildTelemetryIntentPrompt 生成结构化意图抽取的系统与用户提示词。
func buildTelemetryIntentPrompt(question string) (system, user string) {
	system = `你是 IoT 平台的遥测查询意图解析器。只输出一个 JSON 对象，不要输出任何其他文字或代码块标记。
JSON 字段：
- device_ids: 字符串数组，问题中提到的设备 ID 或设备编号；无法确定时给空数组。
- keys: 字符串数组，用户关心的遥测字段名（如 temperature、humidity）；无法确定时给空数组。
- hours_back: 数字，回溯时间范围（小时），默认 24。`
	user = fmt.Sprintf("用户问题：%s", question)
	return system, user
}

// parseAiIntentJson 从模型回复中提取意图 JSON（容忍代码块包裹）。
func parseAiIntentJson(content string) (telemetryQueryIntent, error) {
	intent := telemetryQueryIntent{}
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return intent, fmt.Errorf("no json object in llm reply")
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &intent); err != nil {
		return intent, err
	}
	return clampTelemetryIntent(intent), nil
}

func clampTelemetryIntent(intent telemetryQueryIntent) telemetryQueryIntent {
	if len(intent.DeviceIDs) > aiIntentMaxDevices {
		intent.DeviceIDs = intent.DeviceIDs[:aiIntentMaxDevices]
	}
	if len(intent.Keys) > aiIntentMaxKeys {
		intent.Keys = intent.Keys[:aiIntentMaxKeys]
	}
	cleaned := make([]string, 0, len(intent.Keys))
	for _, k := range intent.Keys {
		k = strings.TrimSpace(k)
		if k != "" && len(k) <= 64 {
			cleaned = append(cleaned, k)
		}
	}
	intent.Keys = cleaned
	if intent.HoursBack < aiIntentMinHoursBack || intent.HoursBack > aiIntentMaxHoursBack {
		intent.HoursBack = 24
	}
	return intent
}

// callLLMChat 调用统一的 OpenAI 兼容公网出站边界。
func callLLMChat(ctx context.Context, system, user string) (string, error) {
	return aiChatCompletion(ctx, aiLLMBaseURL(), aiLLMAPIKey(), aiLLMModel(), []aiLLMChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, 0, nil)
}

// QueryTelemetry 自然语言查询遥测：意图解析（LLM）→ 白名单取数（DAL，强制租户过滤）。
func (*AiQuery) QueryTelemetry(ctx context.Context, req *AiTelemetryQueryReq, claims *utils.UserClaims) (*AiTelemetryQueryResp, error) {
	if req == nil || strings.TrimSpace(req.Question) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "question is required")
	}
	if claims == nil {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if aiLLMAPIKey() == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			"AI integration is not configured; set ai.llm.api_key (and optionally ai.llm.base_url / ai.llm.model)")
	}

	tenantID := strings.TrimSpace(claims.TenantID)
	if claims.Authority == constant.SYS_ADMIN {
		tenantID = strings.TrimSpace(req.TenantID)
	}
	if tenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required")
	}

	system, user := buildTelemetryIntentPrompt(strings.TrimSpace(req.Question))
	content, err := callLLMChat(ctx, system, user)
	if err != nil {
		logrus.Warn("ai telemetry query llm call failed")
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "AI request failed")
	}
	intent, err := parseAiIntentJson(content)
	if err != nil {
		logrus.Warn("ai telemetry query intent parse failed")
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "AI returned an unreadable query intent")
	}

	// 取数路径 1：指定了设备 → 批量校验设备属于该租户后按 keys 拉当前遥测。
	// 取数路径 2：未指定 → 复用租户最近活跃设备的既有白名单查询。
	// P2 修复（2026-08-25）：设备归属校验由逐个 GetDeviceByID 改为单条
	// IN 查询（GetDevicesByIDsForTenant），租户过滤下沉到 SQL，消除 N+1 并收紧越权面。
	snapshots := make([]aiDeviceTelemetrySnapshot, 0, len(intent.DeviceIDs))
	if len(intent.DeviceIDs) > 0 {
		devicesByID, err := dal.GetDevicesByIDsForTenant(intent.DeviceIDs, tenantID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error": "batch load devices failed: " + err.Error(),
			})
		}
		for _, deviceID := range intent.DeviceIDs {
			deviceInfo := devicesByID[deviceID]
			if deviceInfo == nil {
				continue
			}
			snapshot := aiDeviceTelemetrySnapshot{
				DeviceID:     deviceInfo.ID,
				DeviceNumber: deviceInfo.DeviceNumber,
				Name:         safeDeviceName(deviceInfo.Name),
				Latest:       map[string]string{},
			}
			var evolution []*model.TelemetryCurrentData
			if len(intent.Keys) > 0 {
				evolution, _ = dal.GetCurrentTelemetryDataEvolutionByKeys(deviceInfo.ID, intent.Keys)
			} else {
				evolution, _ = dal.GetCurrentTelemetryDataEvolution(deviceInfo.ID)
			}
			fillSnapshotFromEvolution(&snapshot, evolution)
			snapshots = append(snapshots, snapshot)
		}
	}

	modelName := aiLLMModel()
	return &AiTelemetryQueryResp{
		Question:    strings.TrimSpace(req.Question),
		Intent:      intent,
		Devices:     snapshots,
		Model:       modelName,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func fillSnapshotFromEvolution(snapshot *aiDeviceTelemetrySnapshot, evolution []*model.TelemetryCurrentData) {
	for _, item := range evolution {
		if item == nil {
			continue
		}
		var value string
		switch {
		case item.StringV != nil:
			value = *item.StringV
		case item.NumberV != nil:
			value = fmt.Sprintf("%v", *item.NumberV)
		case item.BoolV != nil:
			value = fmt.Sprintf("%v", *item.BoolV)
		}
		if _, exists := snapshot.Latest[item.Key]; !exists {
			snapshot.Latest[item.Key] = value
			ts := item.T
			if snapshot.UpdatedAt == nil || ts.After(*snapshot.UpdatedAt) {
				snapshot.UpdatedAt = &ts
			}
		}
	}
}

func safeDeviceName(name *string) string {
	if name == nil {
		return ""
	}
	return *name
}
