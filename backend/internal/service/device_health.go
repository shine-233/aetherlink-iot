// 文件用途：设备综合健康度评估（Device Health Score）业务逻辑层。
// 核心逻辑：结合活跃告警严重度、在线/离线持续时长与物模型状态，执行多维扣分算法计算健康分与等级，生成处置建议。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	errcode "aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
)

type DeviceHealthService struct{}

// EvaluateDeviceHealth 对单设备执行健康度诊断评分并落库
func (*DeviceHealthService) EvaluateDeviceHealth(ctx context.Context, deviceID string, claims *utils.UserClaims) (*model.DeviceHealthDetailResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}

	device, err := dal.GetTenantDeviceByID(deviceID, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "device not found")
	}

	alarms, err := dal.GetDeviceActiveAlarms(claims.TenantID, device.ID)
	if err != nil {
		alarms = []*model.AlarmHistory{}
	}

	scoreEntity, detailResp := ComputeDeviceHealth(device, alarms)
	scoreEntity.TenantID = claims.TenantID

	// 尝试读取已有评分 ID 保持唯一稳定
	if existing, err := dal.GetDeviceHealthScoreByDeviceID(device.ID, claims.TenantID); err == nil && existing != nil {
		scoreEntity.ID = existing.ID
		scoreEntity.CreatedAt = existing.CreatedAt
	} else {
		scoreEntity.ID = uuid.New().String()
		scoreEntity.CreatedAt = time.Now()
	}

	if err := dal.UpsertDeviceHealthScore(scoreEntity); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	return detailResp, nil
}

// EvaluateTenantDeviceHealth 对租户全部设备执行批量健康评估
func (s *DeviceHealthService) EvaluateTenantDeviceHealth(ctx context.Context, claims *utils.UserClaims) (*model.DeviceHealthSummaryResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}

	devices, err := dal.GetTenantDevicesForHealthEvaluation(claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	allAlarms, err := dal.GetAllTenantActiveAlarms(claims.TenantID)
	if err != nil {
		allAlarms = []*model.AlarmHistory{}
	}

	// 按设备 ID 分组告警
	alarmsByDevice := make(map[string][]*model.AlarmHistory)
	for _, alarm := range allAlarms {
		var devIDs []string
		if err := json.Unmarshal([]byte(alarm.AlarmDeviceList), &devIDs); err == nil {
			for _, id := range devIDs {
				alarmsByDevice[id] = append(alarmsByDevice[id], alarm)
			}
		} else {
			// 兼容文本匹配
			for _, d := range devices {
				if strings.Contains(alarm.AlarmDeviceList, d.ID) {
					alarmsByDevice[d.ID] = append(alarmsByDevice[d.ID], alarm)
				}
			}
		}
	}

	// 批量评估并更新
	for _, dev := range devices {
		devAlarms := alarmsByDevice[dev.ID]
		scoreEntity, _ := ComputeDeviceHealth(dev, devAlarms)
		scoreEntity.TenantID = claims.TenantID

		if existing, err := dal.GetDeviceHealthScoreByDeviceID(dev.ID, claims.TenantID); err == nil && existing != nil {
			scoreEntity.ID = existing.ID
			scoreEntity.CreatedAt = existing.CreatedAt
		} else {
			scoreEntity.ID = uuid.New().String()
			scoreEntity.CreatedAt = time.Now()
		}

		_ = dal.UpsertDeviceHealthScore(scoreEntity)
	}

	return s.GetTenantHealthSummary(ctx, claims)
}

// GetDeviceHealthDetail 获取单设备健康度详情（未评估过则即时计算）
func (s *DeviceHealthService) GetDeviceHealthDetail(ctx context.Context, deviceID string, claims *utils.UserClaims) (*model.DeviceHealthDetailResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}

	device, err := dal.GetTenantDeviceByID(deviceID, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "device not found")
	}

	scoreEntity, err := dal.GetDeviceHealthScoreByDeviceID(device.ID, claims.TenantID)
	if err != nil || scoreEntity == nil {
		// 未生成过评分，执行即时评估
		return s.EvaluateDeviceHealth(ctx, deviceID, claims)
	}

	alarms, _ := dal.GetDeviceActiveAlarms(claims.TenantID, device.ID)
	_, detailResp := ComputeDeviceHealth(device, alarms)
	return detailResp, nil
}

// GetTenantHealthSummary 获取租户健康大盘汇总统计
func (s *DeviceHealthService) GetTenantHealthSummary(ctx context.Context, claims *utils.UserClaims) (*model.DeviceHealthSummaryResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}

	devices, err := dal.GetTenantDevicesForHealthEvaluation(claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	deviceMap := make(map[string]*model.Device)
	for _, d := range devices {
		deviceMap[d.ID] = d
	}

	scores, err := dal.GetDeviceHealthScoresByTenant(claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	// 如果评分记录数小于设备总数，触发一次全量评估以补齐数据
	if len(scores) < len(devices) {
		return s.EvaluateTenantDeviceHealth(ctx, claims)
	}

	summary := &model.DeviceHealthSummaryResp{
		TotalDevices:     len(devices),
		UnhealthyDevices: make([]model.DeviceHealthScoreItem, 0),
		EvaluatedAt:      time.Now().Format(time.RFC3339),
	}

	var scoreSum float64
	for _, sc := range scores {
		dev := deviceMap[sc.DeviceID]
		devName := sc.DeviceID
		devNumber := ""
		isOnline := false
		if dev != nil {
			if dev.Name != nil && *dev.Name != "" {
				devName = *dev.Name
			}
			devNumber = dev.DeviceNumber
			isOnline = dev.IsOnline == 1
		}

		scoreSum += sc.Score

		switch sc.HealthStatus {
		case model.HealthStatusHealthy:
			summary.HealthyCount++
		case model.HealthStatusSubHealthy:
			summary.SubHealthyCount++
		case model.HealthStatusWarning:
			summary.WarningCount++
		case model.HealthStatusCritical:
			summary.CriticalCount++
		}

		// 收集非健康设备（分数 < 85）入榜
		if sc.Score < 85.0 {
			summary.UnhealthyDevices = append(summary.UnhealthyDevices, model.DeviceHealthScoreItem{
				DeviceID:       sc.DeviceID,
				DeviceName:     devName,
				DeviceNumber:   devNumber,
				Score:          sc.Score,
				HealthStatus:   sc.HealthStatus,
				AlarmPenalty:   sc.AlarmPenalty,
				OfflinePenalty: sc.OfflinePenalty,
				AnomalyPenalty: sc.AnomalyPenalty,
				IsOnline:       isOnline,
				EvaluatedAt:    sc.EvaluatedAt.Format(time.RFC3339),
			})
		}
	}

	if len(scores) > 0 {
		summary.AverageScore = math.Round((scoreSum/float64(len(scores)))*100) / 100
	} else {
		summary.AverageScore = 100.00
	}

	// 排序按分数升序（最差的排前面），截取 Top 20
	sort.Slice(summary.UnhealthyDevices, func(i, j int) bool {
		return summary.UnhealthyDevices[i].Score < summary.UnhealthyDevices[j].Score
	})
	if len(summary.UnhealthyDevices) > 20 {
		summary.UnhealthyDevices = summary.UnhealthyDevices[:20]
	}

	return summary, nil
}

// ComputeDeviceHealth 核心算法：多维量化设备综合健康度
func ComputeDeviceHealth(device *model.Device, alarms []*model.AlarmHistory) (*model.DeviceHealthScore, *model.DeviceHealthDetailResp) {
	now := time.Now()
	var alarmPenalty float64
	var offlinePenalty float64
	var anomalyPenalty float64
	var suggestions []string

	alarmList := make([]map[string]interface{}, 0)

	// 1. 告警扣分维度（根据活跃未恢复告警严重度）
	for _, a := range alarms {
		var penalty float64
		switch strings.ToUpper(a.AlarmStatus) {
		case "H": // High / Critical
			penalty = 30.0
		case "M": // Medium / Major
			penalty = 15.0
		case "L": // Low / Minor
			penalty = 5.0
		}
		alarmPenalty += penalty

		alarmList = append(alarmList, map[string]interface{}{
			"id":           a.ID,
			"name":         a.Name,
			"alarm_status": a.AlarmStatus,
			"create_at":    a.CreateAt.Format(time.RFC3339),
			"penalty":      penalty,
		})
	}
	// 告警扣分上限 70 分
	if alarmPenalty > 70.0 {
		alarmPenalty = 70.0
	}

	if len(alarms) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("设备存在 %d 起活跃告警未解除，请重点检查相关传感器参数与阈值配置", len(alarms)))
	}

	// 2. 在线/离线稳定性维度
	isOnline := device.IsOnline == 1
	var offlineDurationSeconds int64 = 0

	if !isOnline {
		offlineTime := now
		if device.LastOfflineTime != nil && !device.LastOfflineTime.IsZero() {
			offlineTime = *device.LastOfflineTime
		} else if device.UpdateAt != nil && !device.UpdateAt.IsZero() {
			offlineTime = *device.UpdateAt
		} else if device.CreatedAt != nil && !device.CreatedAt.IsZero() {
			offlineTime = *device.CreatedAt
		}

		dur := now.Sub(offlineTime)
		if dur > 0 {
			offlineDurationSeconds = int64(dur.Seconds())
		}

		if dur >= 7*24*time.Hour {
			offlinePenalty = 50.0
			suggestions = append(suggestions, "设备长时间离线（>7天），硬件或供电可能已断开，建议现场运维检修")
		} else if dur >= 24*time.Hour {
			offlinePenalty = 35.0
			suggestions = append(suggestions, "设备离线已超 24 小时，建议排查现场无线基站或本地网络网关")
		} else {
			offlinePenalty = 20.0
			suggestions = append(suggestions, "设备当前处于离线状态，请确认设备供电及网络信号正常")
		}
	}

	// 3. 物模型异常与状态管控维度
	if device.ActivateFlag != "active" {
		anomalyPenalty += 10.0
		suggestions = append(suggestions, "设备尚未完成上线激活接入流程")
	}
	if device.IsEnabled == "disabled" {
		anomalyPenalty += 15.0
		suggestions = append(suggestions, "设备已被管理员手动停用")
	}

	// 4. 计算综合分并截断于 [0.00, 100.00]
	score := 100.0 - alarmPenalty - offlinePenalty - anomalyPenalty
	if score < 0.0 {
		score = 0.0
	} else if score > 100.0 {
		score = 100.0
	}
	score = math.Round(score*100) / 100

	// 5. 判定健康等级
	var status string
	switch {
	case score >= 85.0:
		status = model.HealthStatusHealthy
	case score >= 70.0:
		status = model.HealthStatusSubHealthy
	case score >= 50.0:
		status = model.HealthStatusWarning
	default:
		status = model.HealthStatusCritical
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "设备运行工况良好，各项指标均在稳定基线以内")
	}

	devName := device.ID
	if device.Name != nil && *device.Name != "" {
		devName = *device.Name
	}

	detailsMap := map[string]interface{}{
		"active_alarm_count":        len(alarms),
		"is_online":                 isOnline,
		"offline_duration_seconds":  offlineDurationSeconds,
		"suggestions":               suggestions,
	}
	detailsJSON, _ := json.Marshal(detailsMap)

	scoreEntity := &model.DeviceHealthScore{
		DeviceID:       device.ID,
		Score:          score,
		HealthStatus:   status,
		AlarmPenalty:   alarmPenalty,
		OfflinePenalty: offlinePenalty,
		AnomalyPenalty: anomalyPenalty,
		Details:        string(detailsJSON),
		EvaluatedAt:    now,
	}

	detailResp := &model.DeviceHealthDetailResp{
		DeviceID:               device.ID,
		DeviceName:             devName,
		DeviceNumber:           device.DeviceNumber,
		Score:                  score,
		HealthStatus:           status,
		AlarmPenalty:           alarmPenalty,
		OfflinePenalty:         offlinePenalty,
		AnomalyPenalty:         anomalyPenalty,
		IsOnline:               isOnline,
		OfflineDurationSeconds: offlineDurationSeconds,
		ActiveAlarmCount:       len(alarms),
		ActiveAlarms:           alarmList,
		Suggestions:            suggestions,
		EvaluatedAt:            now.Format(time.RFC3339),
	}

	return scoreEntity, detailResp
}
