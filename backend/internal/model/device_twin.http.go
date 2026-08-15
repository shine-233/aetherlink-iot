package model

import (
	"encoding/json"
	"time"
)

type DeviceTwinRow struct {
	Key              string      `json:"key"`
	Label            string      `json:"label"`
	Source           string      `json:"source"`
	Desired          interface{} `json:"desired"`
	Reported         interface{} `json:"reported"`
	Comparable       bool        `json:"comparable"`
	ReportedFresh    bool        `json:"reported_fresh"` // reported 时间存在，且 desired 时间缺失或 reported 不早于 desired
	Matched          bool        `json:"matched"`
	Status           string      `json:"status"`
	DesiredUpdatedAt *time.Time  `json:"desired_updated_at,omitempty"`
	DesiredExpiresAt *time.Time  `json:"desired_expires_at,omitempty"`
	ReportedAt       *time.Time  `json:"reported_at,omitempty"`
	DesiredRevision  *string     `json:"desired_revision,omitempty"`
	LastWriteSource  *string     `json:"last_write_source,omitempty"`
}

type DeviceTwinSummary struct {
	DesiredCount      int    `json:"desiredCount"`
	ReportedCount     int    `json:"reportedCount"`
	MatchedCount      int    `json:"matchedCount"`
	DeltaCount        int    `json:"deltaCount"`
	UnavailableCount  int    `json:"unavailableCount"`
	StaleDesiredCount int    `json:"staleDesiredCount"`
	ConvergenceStatus string `json:"convergenceStatus"`
	NextAction        string `json:"nextAction"`
	EvidenceBoundary  string `json:"evidenceBoundary"`
}

type DeviceTwinState struct {
	Rows    []DeviceTwinRow   `json:"rows"`
	Summary DeviceTwinSummary `json:"summary"`
}

type UpsertDeviceTwinDesiredReq struct {
	Source  string          `json:"source" form:"source" validate:"required,max=50,oneof=telemetry attribute"`
	Key     string          `json:"key" form:"key" validate:"required,max=100"`
	Desired json.RawMessage `json:"desired" form:"desired" validate:"required"`
	Expiry  *time.Time      `json:"expiry" form:"expiry" validate:"omitempty"`
}

// DeviceTwinDriftIndexReq 是 fleet drift 可查询索引的查询入参。
// 它复用现有设备列表筛选字段(仅取本次需要的少量维度),并限定枚举上限,
// 避免在没有分页/上限保护的情况下扫描过大的设备集。
type DeviceTwinDriftIndexReq struct {
	GroupId        *string `json:"group_id" form:"group_id" validate:"omitempty,max=36"`
	DeviceConfigId *string `json:"device_config_id" form:"device_config_id" validate:"omitempty,max=36"`
	ProductID      *string `json:"product_id" form:"product_id" validate:"omitempty,max=36"`
	Search         *string `json:"search" form:"search" validate:"omitempty,max=255"`
	IsOnline       *int    `json:"is_online" form:"is_online" validate:"omitempty"`
	MaxDevices     int     `json:"max_devices" form:"max_devices" validate:"omitempty,min=1,max=500"`
	OnlyDrift      bool    `json:"only_drift" form:"only_drift" validate:"omitempty"`
}

// DeviceTwinDriftEntry 是 fleet drift 索引中的单台设备条目。
// 它复用单台设备的 twin summary 分类结论,把"哪些设备处于漂移/等待/过期"聚合成可查询索引。
type DeviceTwinDriftEntry struct {
	DeviceID          string            `json:"device_id"`
	DeviceName        string            `json:"device_name,omitempty"`
	ConvergenceStatus string            `json:"convergence_status"`
	NextAction        string            `json:"next_action"`
	DeltaCount        int               `json:"delta_count"`
	UnavailableCount  int               `json:"unavailable_count"`
	StaleDesiredCount int               `json:"stale_desired_count"`
	DesiredCount      int               `json:"desired_count"`
	Severity          int               `json:"severity"` // 越大越需要关注,用于排序
	Summary           DeviceTwinSummary `json:"summary"`
}

// DeviceTwinDriftIndex 是 fleet 级 drift 可查询索引。
// Entries 已按 severity 降序排列;各 *Count 为整个查询范围内的分类计数。
type DeviceTwinDriftIndex struct {
	Entries          []DeviceTwinDriftEntry `json:"entries"`
	TotalDevices     int                    `json:"total_devices"`
	DriftDevices     int                    `json:"drift_devices"`
	WaitingDevices   int                    `json:"waiting_devices"`
	ExpiredDevices   int                    `json:"expired_devices"`
	ReadyDevices     int                    `json:"ready_devices"`
	NoDesiredDevices int                    `json:"no_desired_devices"`
	EvidenceBoundary string                 `json:"evidence_boundary"`
}
