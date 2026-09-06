package service

import (
	"fmt"
	"regexp"
)

// PHASE-D-D2 BEGIN 通知模板变量引擎（D2 通知中心 2.0）
//
// 语法：{{key}}（允许占位符两侧空白）。变量来源为告警数据 alertData，
// 例如 {{subject}}、{{content}}、{{device_name}}。
// 规则：
//   - 命中的变量统一按 %v 字符串化后替换；
//   - 未命中的占位符原样保留（便于排查配置错误，而不是静默发空）；
//   - 不做递归展开（渲染结果中若再次出现 {{...}} 不会再被处理），防止构造环形模板。
// 该引擎为纯函数，供 IM/SMS 渠道共用；邮件渠道沿用既有 subject/content 直传语义不变。

var notificationTemplateVarPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-]+)\s*\}\}`)

func renderNotificationTemplate(tpl string, vars map[string]interface{}) string {
	if tpl == "" {
		return ""
	}
	return notificationTemplateVarPattern.ReplaceAllStringFunc(tpl, func(match string) string {
		key := notificationTemplateVarPattern.FindStringSubmatch(match)[1]
		value, ok := vars[key]
		if !ok || value == nil {
			return match
		}
		return stringifyNotificationTemplateValue(value)
	})
}

func stringifyNotificationTemplateValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

// buildIMTextContent 组装 IM 渠道纯文本：主题行 + 正文，均经模板引擎渲染。
// 主题缺省时降级为纯正文，避免出现空的【】头。
func buildIMTextContent(templateVars *executeNotificationTemplateVars) string {
	var alertData map[string]interface{}
	subject, content := "", ""
	if templateVars != nil {
		alertData = templateVars.alertData
		subject, content = templateVars.subject, templateVars.content
	}
	vars := map[string]interface{}{}
	for k, v := range alertData {
		vars[k] = v
	}
	// subject/content 兜底注入：即使告警数据缺失也保证占位符被消费，不残留 {{...}}。
	if _, ok := vars["subject"]; !ok {
		vars["subject"] = subject
	}
	if _, ok := vars["content"]; !ok {
		vars["content"] = content
	}
	rendered := renderNotificationTemplate("【{{subject}}】\n{{content}}", vars)
	return rendered
}

// PHASE-D-D2 END
