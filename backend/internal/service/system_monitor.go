// 文件用途：维护系统运行指标和监控数据查询服务。
// 核心逻辑：读取主机、进程或应用运行状态，并整理成管理后台可展示的监控响应。
// 关键注意事项：监控数据可能暴露部署环境信息，需限制权限、脱敏路径并处理采集失败。
// 重构建议：抽出指标采集接口，补齐权限、采集错误、超时和字段脱敏测试。
package service

import (
	"time"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/metrics"
	utils "aetherlink-iot/backend/pkg/utils"
)

type SystemMonitor struct{}

var metricsManager *metrics.Metrics

func SetMetricsManager(m *metrics.Metrics) {
	metricsManager = m
}

func requireSystemMonitorAdmin(claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query system metrics")
	}
	return nil
}

func (s *SystemMonitor) GetCurrentMetrics(claims *utils.UserClaims) (*metrics.SystemMetrics, error) {
	if err := requireSystemMonitorAdmin(claims); err != nil {
		return nil, err
	}
	if metricsManager == nil {
		return nil, nil
	}
	return metricsManager.GetCurrentMetrics()
}

func (s *SystemMonitor) GetHistoryData(metricType string, duration time.Duration, claims *utils.UserClaims) ([]metrics.MetricDataPoint, error) {
	if err := requireSystemMonitorAdmin(claims); err != nil {
		return nil, err
	}
	if metricsManager == nil {
		return nil, nil
	}
	return metricsManager.GetHistoryData(metricType, duration)
}

func (s *SystemMonitor) GetCombinedHistoryData(duration time.Duration, claims *utils.UserClaims) ([]metrics.MetricsTimePoint, error) {
	if err := requireSystemMonitorAdmin(claims); err != nil {
		return nil, err
	}
	if metricsManager == nil {
		return nil, nil
	}
	return metricsManager.GetCombinedHistoryData(duration)
}
