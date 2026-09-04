package model

import (
	"encoding/json"
	"time"
)

// MarshalJSON keeps the JSONB graph as a JSON object instead of []byte base64.
func (r RuleChain) MarshalJSON() ([]byte, error) {
	type ruleChainJSON struct {
		ID          string          `json:"id"`
		TenantID    string          `json:"tenant_id"`
		Name        string          `json:"name"`
		Description *string         `json:"description"`
		Enabled     bool            `json:"enabled"`
		Graph       json.RawMessage `json:"graph"`
		CreatedAt   *time.Time      `json:"created_at"`
		UpdatedAt   *time.Time      `json:"updated_at"`
	}

	return json.Marshal(ruleChainJSON{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Name:        r.Name,
		Description: r.Description,
		Enabled:     r.Enabled,
		Graph:       json.RawMessage(r.Graph),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	})
}
