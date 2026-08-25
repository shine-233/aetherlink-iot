// 文件用途：集中注册后端真实使用的 cron 定时任务，并负责启动全局调度器。
// 核心逻辑：在统一调度器上挂载自动化执行、数据清理、脚本执行、消息清理和设备统计任务。
// 关键注意事项：这里的 cron 表达式和执行体调用会直接影响后台负载与业务时效，调整时需同步审查依赖链路。
// 重构建议：后续可把任务注册声明与执行逻辑进一步拆分，降低该入口的维护耦合。

package croninit

import (
	"time"

	"aetherlink-iot/backend/internal/service"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
)

var c = cron.New()

// CronInit 注册全部定时任务并启动全局 cron 调度器。
func CronInit() {
	// 初始化设备统计定时任务
	InitDeviceStatsCron(c)

	recoverTimedOutFleetCommandJobs()
	resumeRunnableFleetCommandJobs()
	c.AddFunc("0 * * * * *", recoverTimedOutFleetCommandJobs)
	c.AddFunc("*/15 * * * * *", resumeRunnableFleetCommandJobs)

	// 单次定义成任务 - 每5秒执行一次
	c.AddFunc("*/5 * * * * *", func() {
		logrus.Debug("【定时任务】自动化单次任务开始：")
		service.GroupApp.OnceTaskExecute()
	})

	// 重复定义成任务 - 每5秒执行一次
	c.AddFunc("*/5 * * * * *", func() {
		logrus.Debug("【定时任务】自动化重复时间任务开始：")
		service.GroupApp.PeriodicTaskExecute()
	})

	// 每天凌晨2点执行数据清理
	c.AddFunc("0 2 * * *", func() {
		logrus.Debug("【定时任务】系统数据清理任务开始：")
		service.GroupApp.CleanSystemDataByCron()
	})

	// 每天凌晨1点执行脚本
	c.AddFunc("0 1 * * *", func() {
		logrus.Debug("【定时任务】每天凌晨1点执行脚本任务开始：")
		service.GroupApp.RunScript()
	})
	// 每天凌晨
	err := c.AddFunc("2 0 * * * *", func() {
		logrus.Debug("【定时任务】消息推送清理任务开始：", time.Now())
		service.GroupApp.MessagePush.MessagePushMangeClear()
	})
	if err != nil {
		logrus.Error("【定时任务】消息推送清理任务启动失败")
	}
	// 每 30 分钟清理设备影子消息：到期 pending → expired，7 天前终态物理删除（ROADMAP A3）
	c.AddFunc("*/30 * * * *", func() {
		expired, deleted := service.GroupApp.DeviceShadow.CleanupExpiredShadowMessages()
		if expired > 0 || deleted > 0 {
			logrus.Infof("【定时任务】影子消息清理完成: expired=%d deleted=%d", expired, deleted)
		}
	})

	c.Start()
}

func recoverTimedOutFleetCommandJobs() {
	if err := service.GroupApp.CommandData.RecoverTimedOutFleetCommandJobs(); err != nil {
		logrus.WithError(err).Warn("recover timed-out command jobs failed")
	}
}

func resumeRunnableFleetCommandJobs() {
	if err := service.GroupApp.CommandData.ResumeRunnableFleetCommandJobs(); err != nil {
		logrus.WithError(err).Warn("resume runnable command jobs failed")
	}
}

// Stop 停止全局调度器，释放后台 goroutine 资源。
func Stop() {
	c.Stop()
}
