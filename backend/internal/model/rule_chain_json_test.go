package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleChainMarshalJSONKeepsGraphAsObject(t *testing.T) {
	chain := RuleChain{
		ID:      "chain-1",
		Name:    "temperature alert",
		Enabled: false,
		Graph:   []byte(`{"nodes":[{"id":"t","type":"trigger.telemetry"}],"edges":[]}`),
	}

	raw, err := json.Marshal(chain)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	require.IsType(t, map[string]any{}, body["graph"])
	require.Equal(t, false, body["enabled"])
	require.Equal(t, "trigger.telemetry", body["graph"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["type"])
}
