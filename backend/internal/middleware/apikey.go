package middleware

import (
	"context"
	"errors"

	"aetherlink-iot/backend/internal/dal"
)

var (
	ErrInvalidAPIKey = errors.New("invalid api key")
)

type APIKeyInfo struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Status    int    `json:"status"`
	Name      string `json:"name"`
	CreatedID string `json:"created_id"`
}

// APIKeyValidator is kept as a compatibility wrapper for historical websocket auth.
// The source of truth is dal.VerifyOpenAPIKey, the same path used by OpenAPIKeyAuth.
type APIKeyValidator struct {
	ctx context.Context
}

func NewAPIKeyValidator() *APIKeyValidator {
	return &APIKeyValidator{ctx: context.Background()}
}

func (v *APIKeyValidator) ValidateAPIKey(apiKey string) (*APIKeyInfo, error) {
	tenantID, createdID, err := dal.VerifyOpenAPIKey(v.ctx, apiKey)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}
	return &APIKeyInfo{
		TenantID:  tenantID,
		Status:    1,
		CreatedID: createdID,
	}, nil
}
