package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateEdgeNodeRegistrationRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		req  EdgeNodeRegistration
		want string
	}{
		{"missing node id", EdgeNodeRegistration{TenantID: "t", Version: "1.0"}, "node_id is required"},
		{"missing tenant", EdgeNodeRegistration{NodeID: "n", Version: "1.0"}, "tenant_id is required"},
		{"empty version", EdgeNodeRegistration{NodeID: "n", TenantID: "t"}, "version must be dotted numeric"},
		{"non numeric version", EdgeNodeRegistration{NodeID: "n", TenantID: "t", Version: "1.0-beta"}, "version must be dotted numeric"},
		{"trailing dot", EdgeNodeRegistration{NodeID: "n", TenantID: "t", Version: "1."}, "version must be dotted numeric"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, reason, _ := EvaluateEdgeNodeRegistration(tc.req, nil)
			require.Equal(t, EdgeNodeRegistrationRejected, outcome)
			require.Contains(t, reason, tc.want)
		})
	}
}

// 同一 NodeID 换租户是安全事件：必须拒绝，不能静默覆盖。
// 否则一个租户就能把别人的边缘节点抢过来，后续下发全打到错误的地方。
func TestEvaluateEdgeNodeRegistrationRefusesTenantReassignment(t *testing.T) {
	existing := &EdgeNodeRegistration{NodeID: "n1", TenantID: "tenant-a", Version: "1.0"}
	outcome, reason, _ := EvaluateEdgeNodeRegistration(
		EdgeNodeRegistration{NodeID: "n1", TenantID: "tenant-b", Version: "1.0"}, existing)
	require.Equal(t, EdgeNodeRegistrationRejected, outcome)
	require.Contains(t, reason, "another tenant")
}

func TestEvaluateEdgeNodeRegistrationFirstTime(t *testing.T) {
	outcome, reason, normalized := EvaluateEdgeNodeRegistration(
		EdgeNodeRegistration{NodeID: "n1", TenantID: "t", Version: "1.0"}, nil)
	require.Equal(t, EdgeNodeRegistrationUpdated, outcome)
	require.Contains(t, reason, "first registration")
	require.Equal(t, "1.0", normalized.Version)
}

func TestEvaluateEdgeNodeRegistrationUnchangedIsIdempotent(t *testing.T) {
	existing := &EdgeNodeRegistration{
		NodeID:       "n1",
		TenantID:     "t",
		Version:      "1.0",
		Capabilities: []string{"mqtt", "ota"},
	}
	// 顺序不同、带空白：规范化后应视为无变化
	outcome, reason, _ := EvaluateEdgeNodeRegistration(
		EdgeNodeRegistration{NodeID: "n1", TenantID: "t", Version: "1.0", Capabilities: []string{" ota ", "mqtt"}}, existing)
	require.Equal(t, EdgeNodeRegistrationUnchanged, outcome)
	require.Contains(t, reason, "no change")
}

// 能力列表必须规范化：去空白、去重、排序。
// 不这样的话同一节点重复注册会因顺序差异被判成"变了"，产生无意义变更记录。
func TestNormalizeCapabilitiesViaRegistration(t *testing.T) {
	_, _, normalized := EvaluateEdgeNodeRegistration(
		EdgeNodeRegistration{
			NodeID:       "n1",
			TenantID:     "t",
			Version:      "1.0",
			Capabilities: []string{" ota ", "mqtt", "", "ota", "  "},
		}, nil)
	require.Equal(t, []string{"mqtt", "ota"}, normalized.Capabilities,
		"capabilities must be trimmed, deduplicated and sorted")
}

func TestEvaluateEdgeNodeRegistrationDetectsChanges(t *testing.T) {
	existing := &EdgeNodeRegistration{
		NodeID: "n1", TenantID: "t", Version: "1.0", Capabilities: []string{"mqtt"},
	}

	// 仅版本变化
	outcome, reason, _ := EvaluateEdgeNodeRegistration(
		EdgeNodeRegistration{NodeID: "n1", TenantID: "t", Version: "2.0", Capabilities: []string{"mqtt"}}, existing)
	require.Equal(t, EdgeNodeRegistrationUpdated, outcome)
	require.Contains(t, reason, "version changed")

	// 仅能力变化
	outcome, reason, _ = EvaluateEdgeNodeRegistration(
		EdgeNodeRegistration{NodeID: "n1", TenantID: "t", Version: "1.0", Capabilities: []string{"mqtt", "ota"}}, existing)
	require.Equal(t, EdgeNodeRegistrationUpdated, outcome)
	require.Contains(t, reason, "capabilities changed")

	// 两者都变
	outcome, reason, _ = EvaluateEdgeNodeRegistration(
		EdgeNodeRegistration{NodeID: "n1", TenantID: "t", Version: "2.0", Capabilities: []string{"ota"}}, existing)
	require.Equal(t, EdgeNodeRegistrationUpdated, outcome)
	require.Contains(t, reason, "version and capabilities changed")
}

func TestIsDottedNumericVersion(t *testing.T) {
	require.True(t, isDottedNumericVersion("1"))
	require.True(t, isDottedNumericVersion("1.2.3"))
	require.False(t, isDottedNumericVersion(""))
	require.False(t, isDottedNumericVersion("1.2.x"))
	require.False(t, isDottedNumericVersion("v1"))
	require.False(t, isDottedNumericVersion("1..2"))
}
