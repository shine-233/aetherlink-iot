// 文件用途：AI 集成——告警智能分析（ROADMAP C4 剩余项：异常模式识别 + 根因建议）。
// 核心逻辑：LLM 只分析由白名单 DAL 按租户取出的单条告警上下文摘要（名称/级别/描述/内容/
// 触发设备/时间），不接触原始凭证与数据面；输出结构与条数均做钳制；未配置 ai.llm.api_key
// 时显式报"未配置"，不伪装成功。与 AI 遥测查询共用 callLLMChat/aiLLMAPIKey/aiLLMModel。
// 关键注意事项：租户归属在取数后强制校验（alarm.TenantID != claims.TenantID 即拒绝），
// 防止跨租户读取；LLM 输出按 JSON 片段解析，解析失败显式报错而非静默吞掉。
// 重构建议：若要"跨告警异常模式识别"，请新增按设备拉取近 N 条告警的 DAL 并同样做租户过滤。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// AiAlarmAnalysisReq 告警根因分析请求。AlarmID 必填；Question 为可选的补充说明。
type AiAlarmAnalysisReq struct {
	AlarmID  string `json:"alarm_id" validate:"required,max=36"`
	Question string `json:"question" validate:"omitempty,max=300"`
}

// AiAlarmAnalysisResp 告警根因分析响应。
type AiAlarmAnalysisResp struct {
	AlarmID            string    `json:"alarm_id"`
	Summary            string    `json:"summary"`
	ProbableCauses     []string  `json:"probable_causes"`
	RecommendedActions []string `json:"recommended_actions"`
	Model              string    `json:"model"`
	GeneratedAt        time.Time `json:"generated_at"`
}

// alarmAnalysisResult 解析自 LLM 输出的结构化分析（内部结构）。
type alarmAnalysisResult struct {
	Summary            string   `json:"summary"`
	ProbableCauses     []string `json:"probable_causes"`
	RecommendedActions []string `json:"recommended_actions"`
}

// alarmAnalysisScope 校验 claims 并返回租户作用域。
func alarmAnalysisScope(claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.New(errcode.CodeNoPermission)
	}
	tenantID := strings.TrimSpace(claims.TenantID)
	if tenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required")
	}
	return tenantID, nil
}

// buildAlarmAnalysisPrompt 把单条告警转成紧凑上下文；超长字段截断，防放大 token 消耗。
func buildAlarmAnalysisPrompt(alarm *model.AlarmHistory, question string) (system, user string) {
	level := strings.TrimSpace(alarm.AlarmStatus)
	desc := ""
	if alarm.Description != nil {
		desc = strings.TrimSpace(*alarm.Description)
	}
	if len(desc) > 400 {
		desc = desc[:400]
	}
	content := ""
	if alarm.Content != nil {
		content = strings.TrimSpace(*alarm.Content)
	}
	if len(content) > 800 {
		content = content[:800]
	}

	system = "你是物联网告警根因分析助手。只依据给出的告警上下文分析，不编造数据；严格输出 JSON：{\"summary\":\"\",\"probable_causes\":[],\"recommended_actions\":[]}。"

	var b strings.Builder
	b.WriteString("告警名称: ")
	b.WriteString(strings.TrimSpace(alarm.Name))
	b.WriteString("\n级别: ")
	b.WriteString(level)
	if desc != "" {
		b.WriteString("\n描述: ")
		b.WriteString(desc)
	}
	if content != "" {
		b.WriteString("\n内容: ")
		b.WriteString(content)
	}
	devices := strings.TrimSpace(alarm.AlarmDeviceList)
	if devices != "" {
		b.WriteString("\n触发设备: ")
		b.WriteString(devices)
	}
	b.WriteString("\n产生时间: ")
	b.WriteString(alarm.CreateAt.UTC().Format(time.RFC3339))
	if q := strings.TrimSpace(question); q != "" {
		b.WriteString("\n补充问题: ")
		b.WriteString(q)
	}
	return system, b.String()
}

// clampAlarmAnalysisResult 钳制 LLM 输出的条数与摘要长度，防止异常输出放大到响应。
func clampAlarmAnalysisResult(res alarmAnalysisResult) alarmAnalysisResult {
	if len(res.Summary) > 500 {
		res.Summary = res.Summary[:500]
	}
	if len(res.ProbableCauses) > 8 {
		res.ProbableCauses = res.ProbableCauses[:8]
	}
	if len(res.RecommendedActions) > 8 {
		res.RecommendedActions = res.RecommendedActions[:8]
	}
	return res
}

// parseAlarmAnalysisJSON 从 LLM 输出中截取首个 {…} JSON 片段并解析；失败显式报错。
func parseAlarmAnalysisJSON(content string) (alarmAnalysisResult, error) {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return alarmAnalysisResult{}, errcode.NewWithMessage(errcode.CodeParamError, "AI returned an unreadable alarm analysis")
	}
	var out alarmAnalysisResult
	if err := json.Unmarshal([]byte(content[start:end+1]), &out); err != nil {
		return alarmAnalysisResult{}, errcode.NewWithMessage(errcode.CodeParamError, "AI returned an unreadable alarm analysis")
	}
	return clampAlarmAnalysisResult(out), nil
}

// AnalyzeAlarm 分析单条告警的根因与处置建议（ROADMAP C4：AI 告警分析）。
func (*AiQuery) AnalyzeAlarm(req *AiAlarmAnalysisReq, claims *utils.UserClaims) (*AiAlarmAnalysisResp, error) {
	tenantID, err := alarmAnalysisScope(claims)
	if err != nil {
		return nil, err
	}
	if aiLLMAPIKey() == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "AI 告警分析未配置：请先设置 ai.llm.api_key/base_url/model")
	}

	alarm, err := dal.GetAlarmHistoryByID(strings.TrimSpace(req.AlarmID))
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm not found or inaccessible")
	}
	if alarm == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm not found or inaccessible")
	}
	if strings.TrimSpace(alarm.TenantID) != tenantID {
		// 不区分"不存在"与"属于其它租户"，避免泄露告警存在性。
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm not found or inaccessible")
	}

	system, user := buildAlarmAnalysisPrompt(alarm, req.Question)
	content, err := callLLMChat(context.Background(), system, user)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "AI request failed: "+err.Error())
	}
	result, err := parseAlarmAnalysisJSON(content)
	if err != nil {
		return nil, err
	}
	return &AiAlarmAnalysisResp{
		AlarmID:            alarm.ID,
		Summary:            result.Summary,
		ProbableCauses:     result.ProbableCauses,
		RecommendedActions: result.RecommendedActions,
		Model:              aiLLMModel(),
		GeneratedAt:        time.Now().UTC(),
	}, nil
}
