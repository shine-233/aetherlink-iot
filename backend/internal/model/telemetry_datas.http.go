package model

type GetTelemetryHistoryDataReq struct {
	DeviceID  string `json:"device_id" form:"device_id" validate:"required,max=36"`
	Key       string `json:"key" form:"key" validate:"required,max=255"`
	StartTime int64  `json:"start_time" form:"start_time" validate:"required"`
	EndTime   int64  `json:"end_time" form:"end_time"  validate:"required"`
}

type DeleteTelemetryDataReq struct {
	DeviceID string `json:"device_id" form:"device_id" validate:"required,max=36"`
	Key      string `json:"key" form:"key" validate:"required,max=255"`
}

type GetTelemetryCurrentDataKeysReq struct {
	DeviceID string   `json:"device_id" form:"device_id" validate:"required,max=36"`
	Keys     []string `json:"key" form:"keys" validate:"required,max=255"`
}

type GetTelemetryHistoryDataByPageReq struct {
	DeviceID     string  `json:"device_id" form:"device_id" validate:"required,max=36"`
	Key          string  `json:"key" form:"key" validate:"required,max=255"`
	StartTime    int64   `json:"start_time" form:"start_time" validate:"required"`
	EndTime      int64   `json:"end_time" form:"end_time"  validate:"required"`
	ExportExcel  *bool   `json:"export_excel" form:"export_excel" validate:"omitempty"`
	ExportFormat *string `json:"export_format" form:"export_format" validate:"omitempty,oneof=excel csv"`
	Page         *int    `json:"page" form:"page" validate:"omitempty"`
	PageSize     *int    `json:"page_size" form:"page_size" validate:"omitempty"`
}

type GetTelemetrySetLogsListByPageReq struct {
	PageReq
	DeviceId      string  `json:"device_id" form:"device_id" validate:"required,max=36"`
	Status        *string `json:"status" form:"status" validate:"omitempty,oneof=1 2"`
	OperationType *string `json:"operation_type" form:"operation_type" validate:"omitempty,oneof=1 2"`
}

type GetTelemetryDeadLetterListReq struct {
	PageReq
	TenantID string `json:"tenant_id" form:"tenant_id" validate:"omitempty,max=36"`
	DeviceID string `json:"device_id" form:"device_id" validate:"omitempty,max=36"`
	Key      string `json:"key" form:"key" validate:"omitempty,max=255"`
	Status   string `json:"status" form:"status" validate:"omitempty,oneof=pending processing retrying resolved dead"`
}

type UpdateTelemetryDeadLetterStatusReq struct {
	Action string `json:"action" form:"action" validate:"required,oneof=retry resolve ignore replay"`
}

type DrainTelemetryDeadLetterReq struct {
	TenantID string `json:"tenant_id" form:"tenant_id" validate:"omitempty,max=36"`
	DeviceID string `json:"device_id" form:"device_id" validate:"omitempty,max=36"`
	Key      string `json:"key" form:"key" validate:"omitempty,max=255"`
	Limit    int    `json:"limit" form:"limit" validate:"omitempty,gte=1,lte=100"`
}

type TelemetryDeadLetterRsp struct {
	ID          string   `json:"id"`
	DeviceID    string   `json:"device_id"`
	TenantID    string   `json:"tenant_id"`
	Key         string   `json:"key"`
	TS          int64    `json:"ts"`
	BoolV       *bool    `json:"bool_v,omitempty"`
	NumberV     *float64 `json:"number_v,omitempty"`
	StringV     *string  `json:"string_v,omitempty"`
	Status      string   `json:"status"`
	Attempts    int      `json:"attempts"`
	LastError   string   `json:"last_error,omitempty"`
	NextRetryAt *string  `json:"next_retry_at,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type DrainTelemetryDeadLetterItemRsp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DrainTelemetryDeadLetterRsp struct {
	TotalReady int64                             `json:"total_ready"`
	Attempted  int                               `json:"attempted"`
	Replayed   int                               `json:"replayed"`
	Failed     int                               `json:"failed"`
	Items      []DrainTelemetryDeadLetterItemRsp `json:"items"`
}

type SimulationTelemetryDataReq struct {
	Command string `json:"command" form:"command" validate:"required,max=500"`
}

type ServeEchoDataReq struct {
	DeviceId string `json:"device_id" form:"device_id" validate:"required,max=36"`
}

type SimulationInitReq struct {
	DeviceId string `json:"device_id" form:"device_id" validate:"required,max=36"`
}

type SimulationTopicOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type SimulationInitResp struct {
	Username         string                  `json:"username"`
	Password         string                  `json:"password"`
	ClientID         string                  `json:"client_id"`
	Server           string                  `json:"server"`
	Port             int                     `json:"port"`
	Topic            string                  `json:"topic"`
	TopicOptions     []SimulationTopicOption `json:"topic_options"`
	DefaultData      string                  `json:"default_data"`
	EventDefaultData string                  `json:"event_default_data"`
}

type SimulationSendReq struct {
	DeviceID string `json:"device_id" form:"device_id" validate:"required,max=36"`
	Data     string `json:"data" form:"data" validate:"required,max=5000"`
	Server   string `json:"server" form:"server" validate:"omitempty,max=255"`
	Port     *int   `json:"port" form:"port" validate:"omitempty,gte=1,lte=65535"`
	Topic    string `json:"topic" form:"topic" validate:"omitempty,max=255"`
}

type GetTelemetryStatisticReq struct {
	DeviceId          string `json:"device_id" form:"device_id" validate:"required,max=36"`
	Key               string `json:"key" form:"key" validate:"required"`
	StartTime         int64  `json:"start_time" form:"start_time" validate:"omitempty"`
	EndTime           int64  `json:"end_time" form:"end_time" validate:"omitempty"`
	TimeRange         string `json:"time_range" form:"time_range" validate:"required"`
	AggregateWindow   string `json:"aggregate_window" form:"aggregate_window" validate:"required"`
	AggregateFunction string `json:"aggregate_function" form:"aggregate_function" validate:"omitempty,max=255"`
	IsExport          bool   `json:"is_export" form:"is_export" validate:"omitempty"`
}

type GetTelemetryStatisticByDeviceIdReq struct {
	DeviceIds       []string `json:"device_ids" form:"device_ids" validate:"required,min=1"`
	Keys            []string `json:"keys" form:"keys" validate:"required,min=1"`
	TimeType        string   `json:"time_type" form:"time_type" validate:"required,oneof=hour day week month year"`
	Limit           *int     `json:"limit" form:"limit" validate:"omitempty,min=1,max=1000"`
	AggregateMethod string   `json:"aggregate_method" form:"aggregate_method" validate:"required,oneof=avg sum max min count diff"`
}

type ChartValue struct {
	Key   string  `json:"key"`
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}
