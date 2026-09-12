package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildMarketTemplateImportAuditRecordsOutcome(t *testing.T) {
	// 新建成功
	created := BuildMarketTemplateImportAudit("t1", "u1", "motor", "v1", true, nil)
	require.Equal(t, MarketTemplateImportCreated, created.Outcome)
	require.Equal(t, "t1", created.TenantID)
	require.Equal(t, "u1", created.ActorID)
	require.Empty(t, created.Reason)

	// 幂等命中：无错误但未新建。这不是"没发生"，必须留下痕迹，
	// 否则重复导入这条路径上完全没有证据，出问题无法归因。
	idempotent := BuildMarketTemplateImportAudit("t1", "u1", "motor", "v1", false, nil)
	require.Equal(t, MarketTemplateImportIdempotent, idempotent.Outcome,
		"a deduplicated import is still an import and must be auditable")

	// 失败：记录原因，便于事后回答"为什么没导进来"
	rejected := BuildMarketTemplateImportAudit("t1", "u1", "motor", "v1", false, errors.New("bad signature"))
	require.Equal(t, MarketTemplateImportRejected, rejected.Outcome)
	require.Equal(t, "bad signature", rejected.Reason)
}

// 审计不得成为模板载荷的第二份副本——模板描述符里可能有连接参数等敏感内容。
// 与 trace / 死信一致，只记标识。
func TestMarketTemplateImportAuditCarriesNoPayload(t *testing.T) {
	audit := BuildMarketTemplateImportAudit("t1", "u1", "motor", "v1", true, nil)
	fields := audit.Fields()

	for _, forbidden := range []string{"payload", "content", "descriptor", "data", "template"} {
		require.NotContains(t, fields, forbidden,
			"audit must not carry template payload under key %q", forbidden)
	}

	// 应当包含的标识与结果
	require.Equal(t, "t1", fields["tenant_id"])
	require.Equal(t, "u1", fields["actor_id"])
	require.Equal(t, "motor", fields["template_name"])
	require.Equal(t, "v1", fields["template_version"])
	require.Equal(t, string(MarketTemplateImportCreated), fields["outcome"])
	require.Contains(t, fields, "occurred_at")
}

// 三种结果的日志字段都带 outcome，便于按结果检索与告警。
func TestMarketTemplateImportAuditFieldsAlwaysCarryOutcome(t *testing.T) {
	cases := []struct {
		name    string
		created bool
		err     error
		want    string
	}{
		{"created", true, nil, "created"},
		{"idempotent", false, nil, "idempotent"},
		{"rejected", false, errors.New("x"), "rejected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			audit := BuildMarketTemplateImportAudit("t", "u", "n", "v", tc.created, tc.err)
			require.Equal(t, tc.want, audit.Fields()["outcome"])
		})
	}
}
