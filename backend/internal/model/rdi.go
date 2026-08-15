// 文件用途：提供 rdi 相关模型补充类型、常量或转换 helper，支撑 backend/internal/model 内的共享数据契约。
// 核心逻辑：围绕模型层的通用结构、枚举和轻量转换函数组织代码，供 API、DAL 与 service 层调用。
// 关键注意事项：模型文件应保持无副作用和轻业务逻辑，复杂校验、权限判断或事务编排应留在 service/DAL 层。
// 重构建议：随着模型职责增多，可按领域拆分文件并为关键转换补充单元测试，避免通用文件继续膨胀。

package model

type RDIThingModelItem struct {
	Identifier  string      `json:"identifier"`
	Name        string      `json:"name"`
	Kind        string      `json:"kind,omitempty"`
	DataType    string      `json:"data_type"`
	Unit        string      `json:"unit,omitempty"`
	Range       string      `json:"range,omitempty"`
	ReadWrite   string      `json:"read_write,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Required    bool        `json:"required"`
	Description string      `json:"description,omitempty"`
}

type RDIServiceModelItem struct {
	Identifier  string   `json:"identifier"`
	Name        string   `json:"name"`
	CallType    string   `json:"call_type"`
	Inputs      []string `json:"inputs"`
	Outputs     []string `json:"outputs"`
	Description string   `json:"description,omitempty"`
}

type RDIThingModel struct {
	Telemetry  []RDIThingModelItem   `json:"telemetry"`
	Properties []RDIThingModelItem   `json:"properties"`
	Events     []RDIThingModelItem   `json:"events"`
	Services   []RDIServiceModelItem `json:"services"`
}

type RDIConfig struct {
	DataCollectionInterval       int                    `json:"data_collection_interval"`
	AlarmSensor1Enabled          bool                   `json:"alarm_sensor_1_enabled"`
	AlarmSensor2Enabled          bool                   `json:"alarm_sensor_2_enabled"`
	Sensor1Upper                 float64                `json:"sensor_1_upper"`
	Sensor1Lower                 float64                `json:"sensor_1_lower"`
	Sensor2Upper                 float64                `json:"sensor_2_upper"`
	Sensor2Lower                 float64                `json:"sensor_2_lower"`
	Sensor1Duration              int                    `json:"sensor_1_duration"`
	Sensor2Duration              int                    `json:"sensor_2_duration"`
	Switch1AlarmMode             string                 `json:"switch_1_alarm_mode"`
	Switch2AlarmMode             string                 `json:"switch_2_alarm_mode"`
	Switch1AlarmDuration         int                    `json:"switch_1_alarm_duration"`
	Switch2AlarmDuration         int                    `json:"switch_2_alarm_duration"`
	DryContactAlarmLevel         string                 `json:"dry_contact_alarm_level"`
	DryContactNormalLevel        string                 `json:"dry_contact_normal_level"`
	DryContactAlarmDelay         int                    `json:"dry_contact_alarm_delay"`
	DryContactNormalDelay        int                    `json:"dry_contact_normal_delay"`
	NotificationEnabled          bool                   `json:"notification_enabled"`
	NotificationTemperatureAlarm bool                   `json:"notification_temperature_alarm"`
	NotificationSwitchAlarm      bool                   `json:"notification_switch_alarm"`
	NotificationWarrantyAlarm    bool                   `json:"notification_warranty_alarm"`
	SensorAlarmEmails            string                 `json:"sensor_alarm_emails"`
	SwitchAlarmEmails            string                 `json:"switch_alarm_emails"`
	WarrantyAlarmEmails          string                 `json:"warranty_alarm_emails"`
	Sensor1AlarmEmails           string                 `json:"sensor_1_alarm_emails"`
	Sensor2AlarmEmails           string                 `json:"sensor_2_alarm_emails"`
	Switch1AlarmEmails           string                 `json:"switch_1_alarm_emails"`
	Switch2AlarmEmails           string                 `json:"switch_2_alarm_emails"`
	FieldSetting                 map[string]interface{} `json:"field_setting,omitempty"`
}

type RDISystemInfo struct {
	InstallationLocation   string                 `json:"installation_location,omitempty"`
	Address                string                 `json:"address,omitempty"`
	InstallationDate       string                 `json:"installation_date,omitempty"`
	InstallerCompany       string                 `json:"installer_company,omitempty"`
	InstallerContact       string                 `json:"installer_contact,omitempty"`
	InstallerName          string                 `json:"installer_name,omitempty"`
	InstallerPhone         string                 `json:"installer_phone,omitempty"`
	InstallerEmail         string                 `json:"installer_email,omitempty"`
	ControllerSerialNumber string                 `json:"controller_serial_number,omitempty"`
	MaintenanceTechnician  string                 `json:"maintenance_technician,omitempty"`
	CustomerName           string                 `json:"customer_name,omitempty"`
	ContactEmail           string                 `json:"contact_email,omitempty"`
	ContactPhone           string                 `json:"contact_phone,omitempty"`
	WarrantyStatus         string                 `json:"warranty_status,omitempty"`
	ExtraFields            map[string]interface{} `json:"extra_fields,omitempty"`
}

type RDIDeviceConfigResponse struct {
	DeviceID        string                 `json:"device_id"`
	PIDNumber       string                 `json:"pid_number"`
	DeviceName      string                 `json:"device_name"`
	FirmwareVersion string                 `json:"firmware_version"`
	Online          bool                   `json:"online"`
	ConnectionType  string                 `json:"connection_type"`
	Config          RDIConfig              `json:"config"`
	SystemInfo      RDISystemInfo          `json:"system_info"`
	AdditionalInfo  map[string]interface{} `json:"additional_info"`
	ThingModel      RDIThingModel          `json:"thing_model"`
	CommandTracking *RDICommandTracking    `json:"command_tracking,omitempty"`
}

type RDICommandTracking struct {
	MessageID     string `json:"message_id"`
	Status        string `json:"status"`
	DeviceID      string `json:"device_id"`
	Identifier    string `json:"identifier"`
	OperationType string `json:"operation_type"`
	LogRecorded   bool   `json:"log_recorded"`
}

type UpdateRDIConfigReq struct {
	Config        RDIConfig      `json:"config" validate:"required"`
	SystemInfo    *RDISystemInfo `json:"system_info" validate:"omitempty"`
	ApplyToDevice bool           `json:"apply_to_device"`
}

type ActivateRDIDeviceReq struct {
	PIDNumber string `json:"pid_number" validate:"required"`
	Name      string `json:"name" validate:"omitempty,max=255"`
}

type RDICommandReq struct {
	Identifier string                 `json:"identifier" validate:"required"`
	Params     map[string]interface{} `json:"params" validate:"omitempty"`
}

type RDIHistoryReq struct {
	Key          string  `json:"key" form:"key" validate:"required,max=255"`
	StartTime    int64   `json:"start_time" form:"start_time" validate:"required"`
	EndTime      int64   `json:"end_time" form:"end_time" validate:"required"`
	ExportExcel  *bool   `json:"export_excel" form:"export_excel" validate:"omitempty"`
	ExportFormat *string `json:"export_format" form:"export_format" validate:"omitempty,oneof=excel csv"`
	Page         *int    `json:"page" form:"page" validate:"omitempty"`
	PageSize     *int    `json:"page_size" form:"page_size" validate:"omitempty"`
}

type RDIShareTokenReq struct {
	DeviceID  string `json:"device_id" validate:"omitempty,max=36"`
	ExpiresIn int    `json:"expires_in" validate:"omitempty"`
}

type RDIShareTokenResponse struct {
	DeviceID   string `json:"device_id"`
	Token      string `json:"token"`
	SharePath  string `json:"share_path"`
	AcceptPath string `json:"accept_path,omitempty"`
	ExpiresAt  int64  `json:"expires_at"`
}

type RDIShareTokenRecord struct {
	TokenHash string `json:"token_hash"`
	CreatedBy string `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

type RDIShareRecipientRecord struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	TenantID   string `json:"tenant_id"`
	TokenHash  string `json:"token_hash"`
	AcceptedAt int64  `json:"accepted_at"`
}

type RDISharedDeviceListReq struct {
	DeviceID   string `json:"device_id" form:"device_id" validate:"omitempty,max=36"`
	DeviceName string `json:"device_name" form:"device_name" validate:"omitempty,max=255"`
	Page       int    `json:"page" form:"page" validate:"omitempty,min=1"`
	PageSize   int    `json:"page_size" form:"page_size" validate:"omitempty,min=1,max=200"`
}

type RDISharedDeviceRecord struct {
	Device     RDIDeviceConfigResponse `json:"device"`
	AcceptedAt int64                   `json:"accepted_at"`
}

type RDISharedDeviceListResponse struct {
	Total int                     `json:"total"`
	List  []RDISharedDeviceRecord `json:"list"`
}

type RDILatestFirmwareResponse struct {
	DeviceID        string             `json:"device_id"`
	CurrentVersion  string             `json:"current_version"`
	UpdateAvailable bool               `json:"update_available"`
	Package         *OtaUpgradePackage `json:"package,omitempty"`
}

type RDIAcceptShareResponse struct {
	Device          RDIDeviceConfigResponse `json:"device"`
	AcceptedAt      int64                   `json:"accepted_at"`
	AlreadyAccepted bool                    `json:"already_accepted"`
	SharedWithMe    bool                    `json:"shared_with_me"`
}
