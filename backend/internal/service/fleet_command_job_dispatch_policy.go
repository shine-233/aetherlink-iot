package service

import (
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
)

const (
	defaultFleetCommandJobGlobalMaxConcurrent = 16
	defaultFleetCommandJobTenantMaxConcurrent = 4
	defaultFleetCommandJobGlobalRatePerSecond = 20.0
	defaultFleetCommandJobTenantRatePerSecond = 5.0
	defaultFleetCommandJobContentionRetry     = 500 * time.Millisecond
	defaultFleetCommandJobGateWaitLimit       = 2 * time.Second
)

func currentFleetCommandJobDispatchPolicy() (dal.CommandJobDispatchPolicy, time.Duration) {
	globalMaxConcurrent := boundedPositiveInt(
		viper.GetInt("command_jobs.dispatch.global_max_concurrent"),
		defaultFleetCommandJobGlobalMaxConcurrent,
		1024,
	)
	tenantMaxConcurrent := boundedPositiveInt(
		viper.GetInt("command_jobs.dispatch.tenant_max_concurrent"),
		defaultFleetCommandJobTenantMaxConcurrent,
		globalMaxConcurrent,
	)
	globalRatePerSecond := boundedPositiveFloat(
		viper.GetFloat64("command_jobs.dispatch.global_rate_per_second"),
		defaultFleetCommandJobGlobalRatePerSecond,
		0.1,
		10_000,
	)
	tenantRatePerSecond := boundedPositiveFloat(
		viper.GetFloat64("command_jobs.dispatch.tenant_rate_per_second"),
		defaultFleetCommandJobTenantRatePerSecond,
		0.1,
		globalRatePerSecond,
	)
	contentionRetry := boundedPositiveDuration(
		viper.GetDuration("command_jobs.dispatch.contention_retry_interval"),
		defaultFleetCommandJobContentionRetry,
		50*time.Millisecond,
		5*time.Second,
	)
	gateWaitLimit := boundedPositiveDuration(
		viper.GetDuration("command_jobs.dispatch.gate_wait_limit"),
		defaultFleetCommandJobGateWaitLimit,
		100*time.Millisecond,
		30*time.Second,
	)
	return dal.CommandJobDispatchPolicy{
		GlobalMaxConcurrent:     globalMaxConcurrent,
		TenantMaxConcurrent:     tenantMaxConcurrent,
		GlobalRatePerSecond:     globalRatePerSecond,
		TenantRatePerSecond:     tenantRatePerSecond,
		ContentionRetryInterval: contentionRetry,
	}, gateWaitLimit
}

func boundedPositiveInt(value, fallback, maximum int) int {
	if value <= 0 {
		value = fallback
	}
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func boundedPositiveFloat(value, fallback, minimum, maximum float64) float64 {
	if value <= 0 {
		value = fallback
	}
	if value < minimum {
		return minimum
	}
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func boundedPositiveDuration(value, fallback, minimum, maximum time.Duration) time.Duration {
	if value <= 0 {
		value = fallback
	}
	if value < minimum {
		return minimum
	}
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func fleetCommandJobDispatchGovernanceItem() model.FleetCommandJobGovernanceItem {
	policy, _ := currentFleetCommandJobDispatchPolicy()
	return model.FleetCommandJobGovernanceItem{
		Key:   "dispatch_quota",
		Label: "下发配额",
		Value: fmt.Sprintf(
			"全局 %d 并发 / %.1f 每秒；单租户 %d 并发 / %.1f 每秒",
			policy.GlobalMaxConcurrent,
			policy.GlobalRatePerSecond,
			policy.TenantMaxConcurrent,
			policy.TenantRatePerSecond,
		),
		State:  "done",
		Detail: "每次领取行都会锁定数据库中的全局与租户配额游标，并复核仍在租约内的 dispatching 行；进程内计时只负责高效唤醒，不负责执行配额。",
	}
}
