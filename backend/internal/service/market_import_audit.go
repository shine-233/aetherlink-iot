// 文件用途：P1.6 模板市场导入动作的审计记录。
// 核心逻辑：把"谁在什么时候导入了哪个模板、结果是什么"落成一条结构化审计事件。
// 关键注意事项：
//   - **审计必须记录结果，不能只记"执行了导入"**。只记动作不记结果的审计等于没审：
//     事后无法回答"这个模板到底建出来没有"。
//   - **幂等命中同样要审计**。同租户同名同版本重复导入会返回既有模板，
//     若不留下痕迹，这条路径上就完全没有证据，出问题无法归因——
//     "什么都没发生"和"发生了一次幂等命中"必须可区分。
//   - **只记标识，不记载荷**（沿用 trace / 死信的审计最小化约定）：
//     模板描述符里可能带连接参数等敏感内容，审计不得成为第二份数据副本。
//   - 租户上下文缺失视为不可审计，直接判 rejected——宁可不记，也不记一条归因不了的。
package service

import (
	"time"

	"github.com/sirupsen/logrus"
)

// MarketTemplateImportOutcome 导入动作的结果。
type MarketTemplateImportOutcome string

const (
	// MarketTemplateImportCreated 新建成功。
	MarketTemplateImportCreated MarketTemplateImportOutcome = "created"
	// MarketTemplateImportIdempotent 幂等命中：同名同版本已存在，返回既有模板。
	// 这不是"没发生"，而是一次被去重的导入，必须留痕。
	MarketTemplateImportIdempotent MarketTemplateImportOutcome = "idempotent"
	// MarketTemplateImportRejected 被拒绝（校验失败、权限不足或租户上下文缺失）。
	MarketTemplateImportRejected MarketTemplateImportOutcome = "rejected"
)

// MarketTemplateImportAudit 一次模板导入的审计事件。
// 只承载标识与结果，不承载模板载荷。
type MarketTemplateImportAudit struct {
	TenantID        string                      `json:"tenant_id"`
	ActorID         string                      `json:"actor_id"`
	TemplateName    string                      `json:"template_name"`
	TemplateVersion string                      `json:"template_version"`
	Outcome         MarketTemplateImportOutcome `json:"outcome"`
	Reason          string                      `json:"reason,omitempty"`
	OccurredAt      time.Time                   `json:"occurred_at"`
}

// MarketTemplateImportAuditLogMessage 审计日志的固定文案，便于检索与告警匹配。
const MarketTemplateImportAuditLogMessage = "market template import audited"

// BuildMarketTemplateImportAudit 依据导入结果构造审计事件。
// created=false 且无错误即幂等命中——它同样是一次导入，必须留下痕迹。
func BuildMarketTemplateImportAudit(tenantID, actorID, templateName, templateVersion string, created bool, importErr error) MarketTemplateImportAudit {
	audit := MarketTemplateImportAudit{
		TenantID:        tenantID,
		ActorID:         actorID,
		TemplateName:    templateName,
		TemplateVersion: templateVersion,
		OccurredAt:      time.Now().UTC(),
	}
	switch {
	case importErr != nil:
		audit.Outcome = MarketTemplateImportRejected
		audit.Reason = importErr.Error()
	case created:
		audit.Outcome = MarketTemplateImportCreated
	default:
		// 无错误但未新建 = 命中既有模板。
		audit.Outcome = MarketTemplateImportIdempotent
	}
	return audit
}

// Fields 转成日志字段。审计里出现的键保持稳定，便于检索。
func (a MarketTemplateImportAudit) Fields() logrus.Fields {
	fields := logrus.Fields{
		"tenant_id":        a.TenantID,
		"actor_id":         a.ActorID,
		"template_name":    a.TemplateName,
		"template_version": a.TemplateVersion,
		"outcome":          string(a.Outcome),
		"occurred_at":      a.OccurredAt.Format(time.RFC3339),
	}
	if a.Reason != "" {
		fields["reason"] = a.Reason
	}
	return fields
}

// EmitMarketTemplateImportAudit 落审计日志。
// 当前没有审计表（升级/回滚的迁移尚未排期），先落到结构化日志；
// 字段结构按审计事件设计，后续接入存储时不必改语义。
func EmitMarketTemplateImportAudit(audit MarketTemplateImportAudit) {
	logrus.WithFields(audit.Fields()).Info(MarketTemplateImportAuditLogMessage)
}
