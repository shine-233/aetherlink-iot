// 文件用途：定义 devices 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

type CreateDeviceReq struct {
	ID             *string `json:"id" validate:"omitempty,min=8,max=36"`            // 设备ID（可选，如提供则使用，否则自动生成）
	Name           *string `json:"name" validate:"required,max=255"`                // 设备名称
	Voucher        *string `json:"voucher" validate:"omitempty,max=500"`            // 凭证
	DeviceNumber   *string `json:"device_number" validate:"omitempty,max=36"`       // 设备编号
	PIDNumber      *string `json:"pid_number" validate:"omitempty,alphanum,len=12"` // RDI PID号
	ProductID      *string `json:"product_id" validate:"omitempty,max=36"`          // 产品ID
	ParentID       *string `json:"parent_id" validate:"omitempty,max=36"`           // 父设备ID
	Protocol       *string `json:"protocol" validate:"omitempty,max=36"`            // 协议
	Label          *string `json:"label" validate:"omitempty,max=255"`              // 标签
	Location       *string `json:"location" validate:"omitempty,max=36"`            // 位置
	SubDeviceAddr  *string `json:"sub_device_addr" validate:"omitempty,max=36"`     // 子设备地址
	CurrentVersion *string `json:"current_version" validate:"omitempty,max=36"`     // 当前版本
	AdditionalInfo *string `json:"additional_info" validate:"omitempty"`            // 附加信息
	ProtocolConfig *string `json:"protocol_config" validate:"omitempty"`            // 协议配置
	Remark1        *string `json:"remark1" validate:"omitempty,max=255"`            // 备注1
	Remark2        *string `json:"remark2" validate:"omitempty,max=255"`            // 备注2
	Remark3        *string `json:"remark3" validate:"omitempty,max=255"`            // 备注3
	DeviceConfigId *string `json:"device_config_id" validate:"omitempty,max=36"`    // 设备配置ID
	AccessWay      *string `json:"access_way" validate:"omitempty,max=36"`          // 接入方式
	Description    *string `json:"description" validate:"omitempty,max=500"`        // 接入方式
}

type BatchCreateDevice struct {
	DeviceName     string  `json:"device_name" validate:"required,max=255"`
	DeviceNumber   string  `json:"device_number" validate:"required,max=36"`
	Description    *string `json:"description" validate:"omitempty,max=500"`
	DeviceConfigId string  `json:"device_config_id" validate:"required,max=36"`
}

type BatchCreateDeviceReq struct {
	ServiceAccessId string              `json:"service_access_id" validate:"required,max=36"` // 服务接入点ID
	DeviceList      []BatchCreateDevice `json:"device_list" validate:"required"`
}

type UpdateDeviceReq struct {
	Id             string  `json:"id" validate:"required,max=36"`                   // 设备ID
	Name           *string `json:"name" validate:"omitempty,max=255"`               // 设备名称
	Voucher        *string `json:"voucher" validate:"omitempty,max=500"`            // 凭证
	DeviceNumber   *string `json:"device_number" validate:"omitempty,max=36"`       // 设备编号
	PIDNumber      *string `json:"pid_number" validate:"omitempty,alphanum,len=12"` // RDI PID号
	ProductID      *string `json:"product_id" validate:"omitempty,max=36"`          // 产品ID
	ParentID       *string `json:"parent_id" validate:"omitempty,max=36"`           // 父设备ID
	Label          *string `json:"label" validate:"omitempty,max=255"`              // 标签
	Location       *string `json:"location" validate:"omitempty,max=100"`           // 位置
	SubDeviceAddr  *string `json:"sub_device_addr" validate:"omitempty,max=36"`     // 子设备地址
	CurrentVersion *string `json:"current_version" validate:"omitempty,max=36"`     // 当前版本
	AdditionalInfo *string `json:"additional_info" validate:"omitempty"`            // 附加信息
	ProtocolConfig *string `json:"protocol_config" validate:"omitempty"`            // 协议配置
	Remark1        *string `json:"remark1" validate:"omitempty,max=255"`            // 备注1
	Remark2        *string `json:"remark2" validate:"omitempty,max=255"`            // 备注2
	Remark3        *string `json:"remark3" validate:"omitempty,max=255"`            // 备注3
	DeviceConfigId *string `json:"device_config_id" validate:"omitempty,max=36"`    // 设备配置ID
	AccessWay      *string `json:"access_way" validate:"omitempty,max=36"`          // 接入方式
	Description    *string `json:"description" validate:"omitempty,max=500"`        // 接入方式
	IsOnline       *int16  `json:"is_online" validate:"omitempty"`                  // 是否在线
}

type ActiveDeviceReq struct {
	DeviceNumber string `json:"device_number" validate:"required,alphanum,len=12"` // 设备编号/RDI PID
	Name         string `json:"name" validate:"max=255"`                           // 设备名称
}

type GetDeviceListByPageReq struct {
	PageReq
	ActivateFlag       *string `json:"activate_flag" form:"activate_flag" validate:"omitempty,max=36"`           // 激活状态
	DeviceNumber       *string `json:"device_number" form:"device_number" validate:"omitempty,max=36"`           // 设备编号
	IsEnabled          *string `json:"is_enabled" form:"is_enabled" validate:"omitempty,max=36"`                 // 是否启用
	ProductID          *string `json:"product_id" form:"product_id" validate:"omitempty,max=36"`                 // 产品ID
	ProtocolType       *string `json:"protocol_type" form:"protocol_type" validate:"omitempty,max=36"`           // 协议
	Label              *string `json:"label" form:"label" validate:"omitempty,max=255"`                          // 标签
	Name               *string `json:"name" form:"name" validate:"omitempty,max=255"`                            // 设备名称
	CurrentVersion     *string `json:"current_version" form:"current_version" validate:"omitempty,max=36"`       // 当前版本
	PIDNumber          *string `json:"pid_number" form:"pid_number" validate:"omitempty,max=36"`                 // RDI PID号
	FirmwareVersion    *string `json:"firmware_version" form:"firmware_version" validate:"omitempty,max=64"`     // 固件版本
	Description        *string `json:"description" form:"description" validate:"omitempty,max=500"`              // 描述
	SharedStatus       *string `json:"shared_status" form:"shared_status" validate:"omitempty,max=32"`           // RDI分享状态 shared/unshared
	GroupId            *string `json:"group_id" form:"group_id" validate:"omitempty,max=36"`                     //组id
	DeviceConfigId     *string `json:"device_config_id" form:"device_config_id" validate:"omitempty,max=36"`     // 设备配置ID
	DeviceTemplateID   *string `json:"device_template_id" form:"device_template_id" validate:"omitempty,max=36"` // 设备物模型ID
	IsOnline           *int    `json:"is_online" form:"is_online" validate:"omitempty,max=36"`                   // 组id
	WarnStatus         *string `json:"warn_status" form:"warn_status" validate:"omitempty,max=36"`               // 告警状态
	Search             *string `json:"search" form:"search" validate:"omitempty,max=255"`                        // 设备名称、编号、PID、固件或描述的模糊匹配
	AccessWay          *string `json:"access_way" form:"access_way" validate:"omitempty,max=36"`                 // 接入方式
	BatchNumber        *string `json:"batch_number" form:"batch_number" validate:"omitempty"`
	DeviceType         *string `json:"device_type" form:"device_type" validate:"omitempty,oneof=1 2 3"`            // 设备类型
	ServiceIdentifier  *string `json:"service_identifier" form:"service_identifier" validate:"omitempty,max=36"`   // 服务标识
	ServiceAccessID    *string `json:"service_access_id" form:"service_access_id" validate:"omitempty,max=36"`     // 服务接入点ID
	LastReportedAfter  *int64  `json:"last_reported_after" form:"last_reported_after" validate:"omitempty,gt=0"`   // 最近上报时间下界（Unix毫秒，含）
	LastReportedBefore *int64  `json:"last_reported_before" form:"last_reported_before" validate:"omitempty,gt=0"` // 最近上报时间上界（Unix毫秒，不含）
	NeverReported      *bool   `json:"never_reported" form:"never_reported"`                                       // true=从未上报，false=至少上报一次
	// LifecycleStatus 是 opt-in 的生命周期状态筛选（REQ-05b）。
	// 省略或空值时保持既有默认行为：只返回 activate_flag=active 的设备。
	// activated=已激活；inactive=已安装但未激活；transmitted=至少成功上报过一次；all=全部。
	// transmitted 由 telemetry_current_datas 是否存在纯查询派生，不引入可漂移的冗余状态列。
	LifecycleStatus *string `json:"lifecycle_status" form:"lifecycle_status" validate:"omitempty,oneof=activated inactive transmitted all"`
	// IncludeRDISystemInfoSummary is an explicit opt-in for overview-style list consumers.
	// The response remains a fixed, read-only projection instead of exposing additional_info.
	IncludeRDISystemInfoSummary bool `json:"include_rdi_system_info_summary" form:"include_rdi_system_info_summary"`
	// AllTenants is an explicit system-administrator-only scope expansion. The
	// default device-list contract remains restricted to the caller tenant.
	AllTenants  bool    `json:"all_tenants" form:"all_tenants"`
	OwnerUserID *string `json:"-" form:"-"` // 仅用于后端普通用户设备归属过滤
}

// RDISystemInfoSummary is the minimum installation projection used by device
// list cards. It intentionally excludes customer contacts, warranty data,
// arbitrary extra_fields, RDI configuration, and sharing metadata.
type RDISystemInfoSummary struct {
	InstallationLocation   string `json:"installation_location,omitempty"`
	Address                string `json:"address,omitempty"`
	InstallationDate       string `json:"installation_date,omitempty"`
	InstallerCompany       string `json:"installer_company,omitempty"`
	InstallerContact       string `json:"installer_contact,omitempty"`
	InstallerName          string `json:"installer_name,omitempty"`
	InstallerPhone         string `json:"installer_phone,omitempty"`
	InstallerEmail         string `json:"installer_email,omitempty"`
	ControllerSerialNumber string `json:"controller_serial_number,omitempty"`
	MaintenanceTechnician  string `json:"maintenance_technician,omitempty"`
}

type GetDeviceListByPageRsp struct {
	ID                   string                `json:"id"`                                         // 设备ID
	DeviceNumber         string                `json:"device_number"`                              // 设备编号
	Name                 string                `json:"name"`                                       // 设备名称
	DeviceConfigID       string                `json:"device_config_id"`                           // 设备配置ID
	DeviceConfigName     string                `json:"device_config_name"`                         // 设备配置名称
	Ts                   *time.Time            `json:"ts"`                                         // 上次推送时间
	ActivateFlag         string                `json:"activate_flag"`                              // 激活状态
	ActivateAt           *time.Time            `json:"activate_at"`                                // 激活时间
	BatchNumber          string                `json:"batch_number"`                               // 批次编号
	CurrentVersion       string                `json:"current_version"`                            // 当前版本
	PIDNumber            string                `json:"pid_number"`                                 // RDI PID号
	Description          string                `json:"description"`                                // 设备描述
	SharedStatus         string                `json:"shared_status"`                              // RDI分享状态
	CreatedAt            *time.Time            `json:"created_at"`                                 // 创建时间
	IsOnline             int                   `json:"is_online"`                                  // 是否在线
	Location             string                `json:"location"`                                   // 位置
	AccessWay            string                `json:"access_way"`                                 // 接入方式
	ProtocolType         string                `json:"protocol_type"`                              // 协议类型
	DeviceStatus         int                   `json:"device_status"`                              // 设备状态
	WarnStatus           string                `json:"warn_status"`                                //设备是否告警 Y告警 N未告警
	DeviceType           string                `json:"device_type"`                                // 设备类型 1-网关 2-网关子设备 3-网关子设备子设备
	ImageURL             string                `json:"image_url"`                                  // 图片地址
	RDISystemInfoSummary *RDISystemInfoSummary `json:"rdi_system_info_summary,omitempty" gorm:"-"` // 显式请求时返回的只读 RDI 安装摘要（后端组装，不参与 GORM 列映射）
	ScopeTenantID        string                `json:"scope_tenant_id,omitempty"`                  // 仅在系统管理员显式跨租户查询时返回
	TenantID             string                `json:"-"`                                          // 仅用于后端批量命令预览保留租户上下文
	OwnerUserID          *string               `json:"-"`                                          // 仅用于后端普通用户设备归属过滤
	AdditionalInfo       *string               `json:"-"`                                          // 仅用于后端计算扩展状态
	ParentID             *string               `json:"-"`                                          // 仅用于后端批量命令预览构建网关链路
	SubDeviceAddr        *string               `json:"-"`                                          // 仅用于后端批量命令预览构建网关链路
}

type CreateDeviceGroupReq struct {
	ParentId    *string `json:"parent_id" validate:"omitempty,max=36"`    // 父设备组ID
	Name        string  `json:"name" validate:"required,max=255"`         // 设备组名称
	Description *string `json:"description" validate:"omitempty,max=255"` // 描述
	Remark      *string `json:"remark" validate:"omitempty,max=255"`      // 备注
}

type UpdateDeviceGroupReq struct {
	Id          string  `json:"id" validate:"required,max=36"`            // 设备组ID
	ParentId    string  `json:"parent_id" validate:"required,max=36"`     // 父设备组ID
	Name        string  `json:"name" validate:"required,max=255"`         // 设备组名称
	Description *string `json:"description" validate:"omitempty,max=255"` // 描述
	Remark      *string `json:"remark" validate:"omitempty,max=255"`      // 备注
}

type GetDeviceGroupsListByPageReq struct {
	PageReq
	ParentId *string `json:"parent_id" form:"parent_id" validate:"omitempty,max=36"` // 父设备组ID
	Name     *string `json:"name" form:"name" validate:"omitempty,max=255"`          // 设备组名称
}

type GetDeviceListByGroup struct {
	PageReq
	GroupId string `json:"group_id" form:"group_id" validate:"required,max=36"` // 设备组ID
}

type GetDeviceListByGroupRsp struct {
	GroupId            string  `json:"group_id"`
	Id                 string  `json:"id"`
	DeviceNumber       string  `json:"device_number"`
	Name               string  `json:"name"`
	Device_config_id   *string `json:"device_config_id"`
	Device_config_name *string `json:"device_config_name"`
}

type GetDeviceGroupListByDeviceIdReq struct {
	DeviceId string `json:"device_id" form:"device_id" validate:"required,max=36"` // 父设备组ID
}

type DeviceGroupStatistics struct {
	DeviceTotal  int64 `json:"device_total"`
	OnlineTotal  int64 `json:"online_total"`
	OfflineTotal int64 `json:"offline_total"`
	AlarmTotal   int64 `json:"alarm_total"`
}

type CreateDeviceGroupRelationReq struct {
	GroupId      string   `json:"group_id" validate:"required,max=36"` // 设备组ID
	DeviceIDList []string `json:"device_id_list" validate:"required"`  // 设备ID列表
}

type DeleteDeviceGroupRelationReq struct {
	GroupId  string `json:"group_id" form:"group_id" validate:"required,max=36"`   // 设备组ID
	DeviceId string `json:"device_id" form:"device_id" validate:"required,max=36"` // 设备ID
}

type CreateDevicePreRegisterReq struct {
	ProductID      string  `json:"product_id" validate:"required,max=36"`             // 产品ID
	BatchNumber    string  `json:"batch_number" validate:"required,max=36"`           // 批次编号
	CurrentVersion *string `json:"current_version" validate:"omitempty,max=36"`       // 固件版本
	DeviceCount    *int    `json:"device_count" validate:"omitempty,min=1,max=10000"` // 设备数量，添加类型为1时必填
	CreateType     string  `json:"create_type" validate:"required,oneof=1 2"`         // 添加类型1-自动 2-文件
	BatchFile      *string `json:"batch_file" validate:"omitempty,max=500"`           // 批次文件
}

type GetDevicePreRegisterListByPageReq struct {
	PageReq
	ProductID      string  `json:"product_id" form:"product_id" validate:"omitempty,max=36"`                       // 产品ID
	BatchNumber    *string `json:"batch_number" form:"batch_number" validate:"omitempty"`                          // 批次编号
	DeviceNumber   *string `json:"device_number" form:"device_number" validate:"omitempty"`                        // 设备编号
	IsEnabled      *string `json:"is_enabled" form:"is_enabled" validate:"omitempty"`                              // 是否启用
	ActivateFlag   *string `json:"activate_flag"  form:"activate_flag" validate:"omitempty,oneof=active inactive"` // 激活状态
	Name           *string `json:"name"  form:"name" validate:"omitempty"`                                         //
	DeviceConfigID *string `json:"device_config_id"  form:"device_config_id" validate:"omitempty"`                 //设备配置                  //
}

type GetDevicePreRegisterListByPageRsp struct {
	ID             string     `json:"id"`              // 设备ID
	Name           string     `json:"name"`            // 设备名称
	DeviceNumber   string     `json:"device_number"`   // 设备编号
	ActivateFlag   string     `json:"activate_flag"`   // 激活状态
	ActivateAt     *time.Time `json:"activate_at"`     // 激活时间
	BatchNumber    string     `json:"batch_number"`    // 批次编号
	CurrentVersion string     `json:"current_version"` // 当前版本
	CreatedAt      *time.Time `json:"created_at"`      // 创建时间
}

type ExportPreRegisterReq struct {
	ProductID    string  `json:"product_id" form:"product_id" validate:"required,max=36"`                        // 产品ID
	BatchNumber  *string `json:"batch_number" form:"batch_number" validate:"omitempty,max=36"`                   // 批次编号
	ActivateFlag *string `json:"activate_flag"  form:"activate_flag" validate:"omitempty,oneof=active inactive"` // 激活状态
}

// 移除子设备
type RemoveSonDeviceReq struct {
	SubDeviceId string `json:"sub_device_id" validate:"required,max=36"` // 设备 ID
}

// 获取设备下拉菜单
type GetDeviceMenuReq struct {
	GroupId     string  `json:"group_id" form:"group_id" validate:"omitempty,max=36"`        // 设备组ID
	DeviceName  string  `json:"device_name" form:"device_name" validate:"omitempty,max=255"` // 设备名称
	BindConfig  int     `json:"bind_config" form:"bind_config" validate:"omitempty"`         //绑定设置 0全部 1绑定 2未绑定
	OwnerUserID *string `json:"-" form:"-"`                                                  // 仅用于后端普通用户设备归属过滤
}

// 获取未绑定网关的子设备选择器
type GetUnboundGatewaySubDeviceReq struct {
	Search     *string `json:"search" form:"search" validate:"omitempty,max=255"`             // 设备名称
	DeviceType *string `json:"device_type" form:"device_type" validate:"omitempty,oneof=2 3"` // 设备类型过滤：2-网关设备 3-子设备，不传则返回两种类型
}

type GetTenantDeviceListReq struct {
	ID               string `json:"id"`                 // 设备 ID
	Name             string `json:"name"`               // 设备 名称
	DeviceConfigID   string `json:"device_config_id"`   // 设备配置 ID
	DeviceConfigName string `json:"device_config_name"` // 设备配置名称 device.configs.name
}

type CreateSonDeviceRes struct {
	ID    string `json:"id" validate:"required,max=36"`       // 设备 ID
	SonID string `json:"son_id" validate:"required,max=3600"` // 子设备 ID,英文逗号分割
}

type DeviceConnectFormReq struct {
	DeviceID string `query:"device_id" form:"device_id" json:"device_id" validate:"required,max=36"`
}

type DeviceConnectFormRes struct {
	DataKey     string                       `json:"dataKey"`
	Label       string                       `json:"label"`
	Placeholder string                       `json:"placeholder"`
	Type        string                       `json:"type"`
	Validate    DeviceConnectFormValidateRes `json:"validate"`
}

type DeviceConnectFormValidateRes struct {
	Message  string `json:"message,omitempty"`
	Required bool   `json:"required"`
	Type     string `json:"type"`
}

type DeviceIDReq struct {
	DeviceID string `query:"device_id" form:"device_id" json:"device_id" validate:"required,max=36"`
}

type GetVoucherTypeReq struct {
	DeviceType   string `json:"device_type"  form:"device_type"  validate:"required,max=36,oneof=1 2 3"`
	ProtocolType string `json:"protocol_type"  form:"protocol_type"  validate:"required,max=255"`
}

type UpdateDeviceVoucherReq struct {
	DeviceID string `json:"device_id" validate:"required,max=36"`
	Voucher  any    `json:"voucher" validate:"required"`
}

type GetSubListResp struct {
	Name          string `json:"name"`
	Id            string `json:"id"`
	SubDeviceAddr string `json:"subDeviceAddr"`
}

type GetDeviceTemplateChartSelectReq struct {
	GroupID string `json:"group_id" form:"group_id" validate:"required,max=36"`
}

type GetActionByDeviceConfigIDReq struct {
	DeviceConfigID string `json:"device_config_id" form:"device_config_id" validate:"required,max=36"`
}

type GetActionByDeviceIDReq struct {
	DeviceID string `json:"device_id" form:"device_id" validate:"required,max=36"`
}

// 更新设备配置
type ChangeDeviceConfigReq struct {
	DeviceID       string  `json:"device_id" validate:"required,max=36"` // 设备ID
	DeviceConfigID *string `json:"device_config_id" validate:"max=36"`
}

type GatewayRegisterReq struct {
	GatewayId string `json:"gateway_id" validate:"required,max=255"`
	TenantId  string `json:"tenant_id" validate:"omitempty,max=36"`
	Model     string `json:"model" validate:"required,max=255"`
}

type GatewayRegisterRes struct {
	MqttUsername string `json:"mqtt_username"`
	MqttPassword string `json:"mqtt_password"`
	MqttClientId string `json:"mqtt_client_id"`
}

type DeviceVoucher struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type DeviceRegisterReq struct {
	Type      string          `json:"type" validate:"omitempty,max=64"`
	DeviceId  string          `json:"device_id" validate:"required,max=36"`
	Registers []DeviceSubItem `json:"registers" validate:"required,dive"`
}

type DeviceSubItem struct {
	SubAddr  string `json:"sub_addr" validate:"required,max=255"`
	Model    string `json:"model" validate:"required,max=255"`
	Protocol string `json:"protocol" validate:"omitempty,max=255"`
}

type DeviceRegisterRes struct {
	Type         string                          `json:"type"`
	Status       string                          `json:"status"`
	Message      string                          `json:"message"`
	RegistersRes map[string]DeviceSubRegisterRes `json:"registersRes"`
}

type DeviceSubRegisterRes struct {
	Result    int    `json:"result"`
	Errorcode string `json:"errorcode"`
	Message   string `json:"message"`
	SubAddr   string `json:"sub_addr"`
}
