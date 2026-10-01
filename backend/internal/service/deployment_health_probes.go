// 文件用途：部署健康检查的探针注册表与各依赖单项检查（database/redis/mqtt/文件卷/迁移）。
// 核心逻辑：checkXxx 只负责探测单一依赖并归一化成 DeploymentHealthCheck（含时延与下一步指引），
// 并发调度与结果聚合由 deployment_health.go 的 runDeploymentHealthCheckUncached 完成。
// 关键注意事项：checkMQTT 在 mqtt.enabled=false 时返回 OK 的跳过语义、探针未接线的
// requiredHealthError 文案、checkFileStorage 的临时文件写-删链条均是对外契约，改动前先看
// deployment_capability_test.go 与部署文档。
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// mqttHealthProbe 是 broker 连通性探针的注入点，避免 service 直接依赖 mqtt adapter。
var mqttHealthProbe struct {
	sync.RWMutex
	fn func() bool
}

// SetMQTTHealthProbe 注册 MQTT 健康探针并使健康报告缓存失效。
func SetMQTTHealthProbe(fn func() bool) {
	mqttHealthProbe.Lock()
	defer mqttHealthProbe.Unlock()
	mqttHealthProbe.fn = fn
	clearDeploymentHealthCache()
}

func mqttHealthProbeInstalled() bool {
	mqttHealthProbe.RLock()
	defer mqttHealthProbe.RUnlock()
	return mqttHealthProbe.fn != nil
}

func checkDatabase() DeploymentHealthCheck {
	if global.DB == nil {
		return requiredHealthError("数据库连接还没有初始化", "启动 Postgres 和后端服务后，重新执行部署检查。")
	}

	sqlDB, err := global.DB.DB()
	if err != nil {
		return requiredHealthError(err.Error(), "检查后端数据库配置和 Postgres 日志。")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	err = sqlDB.PingContext(ctx)
	return healthResult(started, err)
}

func checkRedis() DeploymentHealthCheck {
	if global.REDIS == nil {
		return requiredHealthError("Redis 客户端还没有初始化", "启动 Redis 和后端服务后，重新执行部署检查。")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	_, err := global.REDIS.Ping(ctx).Result()
	return healthResult(started, err)
}

func checkStatusRedis() DeploymentHealthCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	_, err := global.STATUS_REDIS.Ping(ctx).Result()
	return healthResult(started, err)
}

func checkMQTT() DeploymentHealthCheck {
	started := time.Now()
	if !deploymentConfigEnabled("mqtt.enabled") {
		return DeploymentHealthCheck{
			OK:        true,
			LatencyMS: time.Since(started).Milliseconds(),
			Required:  false,
			Detail:    "MQTT disabled by configuration",
		}
	}

	mqttHealthProbe.RLock()
	fn := mqttHealthProbe.fn
	mqttHealthProbe.RUnlock()

	if fn == nil {
		return requiredHealthError("MQTT 健康探针还没有初始化", "启动 MQTT 适配客户端，并确认后端 MQTT 配置。")
	}

	if !fn() {
		return DeploymentHealthCheck{
			OK:         false,
			LatencyMS:  time.Since(started).Milliseconds(),
			Required:   true,
			Error:      "MQTT 适配客户端尚未连接",
			NextAction: "检查 mqtt-broker 日志，以及后端 mqtt.broker / mqtt.access_address 配置。",
		}
	}

	return healthResult(started, nil)
}

func checkFileStorage() DeploymentHealthCheck {
	started := time.Now()
	baseDir := "./files"
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return healthResultWithAction(started, err, "确认后端文件卷已挂载，并且后端容器有写入权限。")
	}

	file, err := os.CreateTemp(baseDir, ".deployment-health-*")
	if err != nil {
		return healthResultWithAction(started, err, "确认后端文件卷可写；OTA 包和导出文件需要使用这个路径。")
	}

	tempName := file.Name()
	_, writeErr := file.WriteString("ok")
	closeErr := file.Close()
	removeErr := os.Remove(tempName)

	if writeErr != nil {
		return healthResultWithAction(started, writeErr, "确认后端文件卷可写；OTA 包和导出文件需要使用这个路径。")
	}
	if closeErr != nil {
		return healthResultWithAction(started, closeErr, "检查后端文件卷的文件系统错误。")
	}
	if removeErr != nil {
		return healthResultWithAction(started, removeErr, "后端可以写入文件，但无法清理临时文件；请检查卷权限。")
	}

	result := healthResult(started, nil)
	if absDir, err := filepath.Abs(baseDir); err == nil {
		result.Detail = fmt.Sprintf("可写路径：%s", absDir)
	}
	return result
}

func checkDBMigrations() DeploymentHealthCheck {
	started := time.Now()
	if global.DB == nil {
		return healthResultWithAction(started, errors.New("数据库连接还没有初始化"), "确认 Postgres 可访问后执行数据库迁移。")
	}

	var row struct {
		VersionNumber int32  `gorm:"column:version_number"`
		Version       string `gorm:"column:version"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := global.DB.WithContext(ctx).
		Raw("SELECT version_number, version FROM sys_version ORDER BY version_number DESC LIMIT 1").
		Scan(&row).Error
	if err != nil {
		return healthResultWithAction(started, err, "执行 deploy/postgres 迁移，并检查 backend/sql 文件是否已挂载到 Postgres 初始化目录。")
	}
	if row.VersionNumber == 0 && row.Version == "" {
		return healthResultWithAction(started, gorm.ErrRecordNotFound, "sys_version 表为空；请重新执行迁移后再确认本次部署。")
	}
	if row.VersionNumber != int32(global.VERSION_NUMBER) {
		return healthResultWithAction(
			started,
			fmt.Errorf("database migration version %d does not match expected version %d", row.VersionNumber, global.VERSION_NUMBER),
			fmt.Sprintf("执行 backend/sql 迁移到版本 %d 后再确认本次部署。", global.VERSION_NUMBER),
		)
	}

	result := healthResult(started, nil)
	result.Detail = fmt.Sprintf("最新迁移版本：%d %s", row.VersionNumber, row.Version)
	return result
}

func healthResult(started time.Time, err error) DeploymentHealthCheck {
	result := DeploymentHealthCheck{
		OK:        err == nil,
		LatencyMS: time.Since(started).Milliseconds(),
		Required:  true,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func healthResultWithAction(started time.Time, err error, nextAction string) DeploymentHealthCheck {
	result := healthResult(started, err)
	result.NextAction = nextAction
	return result
}

func requiredHealthError(message string, nextAction string) DeploymentHealthCheck {
	return DeploymentHealthCheck{
		OK:         false,
		Required:   true,
		Error:      message,
		NextAction: nextAction,
	}
}
