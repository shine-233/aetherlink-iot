package model

import "time"

type CreateOTAUpgradeTaskReq struct {
	Name                string                      `json:"name" validate:"required,max=200"`
	OTAUpgradePackageId string                      `json:"ota_upgrade_package_id" validate:"required,max=36"`
	Description         *string                     `json:"description" validate:"omitempty,max=500"`
	Remark              *string                     `json:"remark" validate:"omitempty,max=255"`
	DeviceIdList        []string                    `json:"device_id_list" validate:"omitempty"`
	DeviceFilter        *OTAUpgradeTaskDeviceFilter `json:"device_filter" validate:"omitempty"`
	ExcludeDeviceIdList []string                    `json:"exclude_device_id_list" validate:"omitempty"`
	ExpectedTotal       *int64                      `json:"expected_total" validate:"omitempty"`
	MaxDevices          *int                        `json:"max_devices" validate:"omitempty"`
	TargetMode          string                      `json:"-"`
	TargetFilter        *string                     `json:"-"`
	PreviewTotal        *int64                      `json:"-"`
	SelectedCount       *int                        `json:"-"`
	CreatedBy           *string                     `json:"-"`
	CreatedByAuthority  *string                     `json:"-"`
}

type OTAUpgradeTaskDeviceFilter struct {
	DeviceNumber       *string `json:"device_number" validate:"omitempty,max=36"`
	IsEnabled          *string `json:"is_enabled" validate:"omitempty,max=36"`
	ProductID          *string `json:"product_id" validate:"omitempty,max=36"`
	Label              *string `json:"label" validate:"omitempty,max=255"`
	Name               *string `json:"name" validate:"omitempty,max=255"`
	CurrentVersion     *string `json:"current_version" validate:"omitempty,max=36"`
	PIDNumber          *string `json:"pid_number" validate:"omitempty,max=36"`
	FirmwareVersion    *string `json:"firmware_version" validate:"omitempty,max=64"`
	Description        *string `json:"description" validate:"omitempty,max=500"`
	SharedStatus       *string `json:"shared_status" validate:"omitempty,max=32"`
	GroupId            *string `json:"group_id" validate:"omitempty,max=36"`
	DeviceConfigId     *string `json:"device_config_id" validate:"omitempty,max=36"`
	DeviceTemplateID   *string `json:"device_template_id" validate:"omitempty,max=36"`
	IsOnline           *int    `json:"is_online" validate:"omitempty,max=36"`
	WarnStatus         *string `json:"warn_status" validate:"omitempty,max=36"`
	Search             *string `json:"search" validate:"omitempty,max=255"`
	AccessWay          *string `json:"access_way" validate:"omitempty,max=36"`
	BatchNumber        *string `json:"batch_number" validate:"omitempty"`
	DeviceType         *string `json:"device_type" validate:"omitempty,oneof=1 2 3"`
	ServiceIdentifier  *string `json:"service_identifier" validate:"omitempty,max=36"`
	ServiceAccessID    *string `json:"service_access_id" validate:"omitempty,max=36"`
	LastReportedAfter  *int64  `json:"last_reported_after" validate:"omitempty,gt=0"`
	LastReportedBefore *int64  `json:"last_reported_before" validate:"omitempty,gt=0"`
	NeverReported      *bool   `json:"never_reported"`
	LifecycleStatus    *string `json:"lifecycle_status" validate:"omitempty,oneof=activated inactive transmitted all"`
}

type PreviewOTAUpgradeTaskReq struct {
	OTAUpgradePackageId string                      `json:"ota_upgrade_package_id" validate:"required,max=36"`
	DeviceFilter        *OTAUpgradeTaskDeviceFilter `json:"device_filter" validate:"required"`
	ExcludeDeviceIdList []string                    `json:"exclude_device_id_list" validate:"omitempty"`
	MaxDevices          *int                        `json:"max_devices" validate:"omitempty"`
}

type PreviewOTAUpgradeTaskRsp struct {
	TotalMatched       int64                    `json:"total_matched"`
	SelectedCount      int                      `json:"selected_count"`
	ExcludedCount      int                      `json:"excluded_count"`
	MaxDevices         int                      `json:"max_devices"`
	OverLimit          bool                     `json:"over_limit"`
	PermissionsChecked bool                     `json:"permissions_checked"`
	PreviewDevices     []GetDeviceListByPageRsp `json:"preview_devices"`
}

type GetOTAUpgradeTaskDetailReq struct {
	PageReq
	DeviceName       *string `json:"deivce_name" form:"device_name" validate:"omitempty,max=200"`
	TaskStatus       *int16  `json:"task_status" form:"task_status" validate:"omitempty,max=10"`
	OtaUpgradeTaskId string  `json:"ota_upgrade_task_id" form:"ota_upgrade_task_id" validate:"required,max=36"`
}

type GetOTAUpgradeTaskListByPageReq struct {
	PageReq
	OTAUpgradePackageId string `json:"ota_upgrade_package_id" form:"ota_upgrade_package_id" validate:"required,max=36"`
}

type UpdateOTAUpgradeTaskStatusReq struct {
	Id     string `json:"id" validate:"required,max=36"`
	Action int16  `json:"action" validate:"required,oneof=1 6"`
}

type OTAUpgradeTaskSupportDevice struct {
	DetailID       string      `json:"detail_id"`
	DeviceID       string      `json:"device_id"`
	DeviceNumber   string      `json:"device_number"`
	Name           string      `json:"name"`
	CurrentVersion string      `json:"current_version"`
	TargetVersion  string      `json:"target_version"`
	Progress       interface{} `json:"progress,omitempty"`
	UpdatedAt      interface{} `json:"updated_at,omitempty"`
	FailureReason  string      `json:"failure_reason"`
	ReadyCheckURL  string      `json:"ready_check_url,omitempty"`
}

type OTAUpgradeTaskFailureGroup struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type OTAUpgradeTaskSupportBundle struct {
	TaskID           string                        `json:"task_id"`
	TaskName         string                        `json:"task_name"`
	PackageID        string                        `json:"package_id"`
	TargetMode       string                        `json:"target_mode"`
	TargetFilter     *string                       `json:"target_filter,omitempty"`
	PreviewTotal     *int64                        `json:"preview_total,omitempty"`
	SelectedCount    *int                          `json:"selected_count,omitempty"`
	CreatedAt        interface{}                   `json:"created_at,omitempty"`
	GeneratedAt      interface{}                   `json:"generated_at"`
	Statistics       interface{}                   `json:"statistics"`
	TotalRows        int                           `json:"total_rows"`
	FailedCount      int                           `json:"failed_count"`
	FailedDevices    []OTAUpgradeTaskSupportDevice `json:"failed_devices"`
	FailureGroups    []OTAUpgradeTaskFailureGroup  `json:"failure_groups"`
	NextActions      []string                      `json:"next_actions"`
	EvidenceBoundary []string                      `json:"evidence_boundary"`
	ShareHint        string                        `json:"share_hint"`
}

// OTARolloutGovernanceInput 汇总规划下一步 rollout 动作所需的最小状态。
// 它把持久化 task 行的调度/限速/中止配置与聚合的 detail 状态计数抽象出来,
// 让规划逻辑成为不依赖数据库、broker 或设备的纯函数,可离线单测。
type OTARolloutGovernanceInput struct {
	Status                  string     // running | paused | canceled | completed | ...
	Now                     time.Time  // 评估时刻(UTC)
	ScheduledAt             *time.Time // 计划开始时间;为空表示立即
	TimeoutAt               *time.Time // 绝对截止;超过即超时
	RolloutRatePerMinute    int        // 每分钟允许下发的设备数
	AbortFailureRatePercent *float64   // 失败率阈值(百分比);为空表示不自动中止
	RateWindowStartedAt     *time.Time // 当前限速窗口起点
	RateWindowDispatched    int        // 当前限速窗口内已下发数
	PendingCount            int        // 待下发(pending)设备数
	UpgradingCount          int        // 升级中(pushed/upgrading)设备数
	SucceededCount          int        // 成功数
	FailedCount             int        // 失败数
}

// OTARolloutGovernanceDecision 是规划器对下一步 rollout 动作的结论。
// 它只回报"应该做什么"及原因,不真正下发、不落库、不连 broker。
type OTARolloutGovernanceDecision struct {
	Action           string   `json:"action"` // wait_schedule | dispatch_batch | hold_rate_window | abort | timeout | complete | hold
	BatchSize        int      `json:"batch_size"`
	RemainingInvalid int      `json:"remaining_invalid"`
	FailureRate      float64  `json:"failure_rate"`
	Reason           string   `json:"reason"`
	Warnings         []string `json:"warnings"`
	NextSteps        []string `json:"next_steps"`
	IsSimulation     bool     `json:"is_simulation"`
}
