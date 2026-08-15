package model

type GetAttributeEventDeadLetterListReq struct {
	PageReq
	TenantID string `json:"tenant_id" form:"tenant_id" validate:"omitempty,max=36"`
	DeviceID string `json:"device_id" form:"device_id" validate:"omitempty,max=36"`
	DataType string `json:"data_type" form:"data_type" validate:"omitempty,oneof=attribute event"`
	Status   string `json:"status" form:"status" validate:"omitempty,oneof=pending processing retrying resolved dead"`
}

type UpdateAttributeEventDeadLetterStatusReq struct {
	Action         string `json:"action" form:"action" validate:"required,oneof=retry resolve ignore replay"`
	ExpectedStatus string `json:"expected_status" form:"expected_status" validate:"required,oneof=pending processing retrying resolved dead"`
}

type DrainAttributeEventDeadLetterReq struct {
	TenantID string `json:"tenant_id" form:"tenant_id" validate:"omitempty,max=36"`
	DeviceID string `json:"device_id" form:"device_id" validate:"omitempty,max=36"`
	DataType string `json:"data_type" form:"data_type" validate:"omitempty,oneof=attribute event"`
	Status   string `json:"status" form:"status" validate:"omitempty,oneof=pending processing retrying resolved dead"`
	Limit    int    `json:"limit" form:"limit" validate:"omitempty,gte=1,lte=100"`
}

// AttributeEventDeadLetterRsp is metadata-only by contract. Canonical raw
// payload is intentionally not part of the HTTP response shape.
type AttributeEventDeadLetterRsp struct {
	ID          string  `json:"id"`
	DataType    string  `json:"data_type"`
	DeviceID    string  `json:"device_id"`
	TenantID    string  `json:"tenant_id"`
	TS          int64   `json:"ts"`
	Status      string  `json:"status"`
	Attempts    int     `json:"attempts"`
	LastError   string  `json:"last_error,omitempty"`
	NextRetryAt *string `json:"next_retry_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type DrainAttributeEventDeadLetterItemRsp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DrainAttributeEventDeadLetterRsp struct {
	TotalReady int64                                  `json:"total_ready"`
	Attempted  int                                    `json:"attempted"`
	Replayed   int                                    `json:"replayed"`
	Failed     int                                    `json:"failed"`
	Items      []DrainAttributeEventDeadLetterItemRsp `json:"items"`
}
