// 文件用途：初始化 Casbin 权限引擎，并把实例挂入全局状态供后续鉴权流程复用。
// 核心逻辑：基于已建立的 GORM 数据库连接创建 adapter、装载模型文件并加载策略。
// 关键注意事项：该初始化依赖数据库先成功建立，且模型文件路径必须与启动工作目录保持一致。
// 重构建议：后续可将模型路径和全局赋值行为参数化，便于测试环境替换策略源。

package initialize

import (
	"fmt"
	"log"

	"aetherlink-iot/backend/internal/adapter/casbinadapter"
	"aetherlink-iot/backend/internal/adapter/casbinwatcher"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/spf13/viper"
)

// CasbinInit 在数据库可用后装配 Casbin，并同步加载 OTA 下载地址配置。
func CasbinInit() error {
	log.Println("casbin启动...")

	a, err := casbinadapter.New(global.DB)
	if err != nil {
		return fmt.Errorf("failed to initialize local Casbin adapter: %w", err)
	}

	e, err := casbin.NewSyncedEnforcer("./configs/casbin.conf", a)
	if err != nil {
		return fmt.Errorf("failed to create enforcer: %v", err)
	}

	// urlPatternMatch：锚定 RESTful 模式匹配（configs/casbin.conf matcher 引用）。
	// 不用内置 keyMatch2：其非锚定正则会子串误命中（如模式 api/v1/devices 命中
	// api/v1/devicesXYZ），构成越权放大；统一走 utils.URLPatternCasbinFunction 锚定实现。
	// （casbin v2.135 的 AddFunction 无返回值，注册失败以其内部 panic 暴露。）
	e.AddFunction("urlPatternMatch", utils.URLPatternCasbinFunction())

	if err := e.LoadPolicy(); err != nil {
		return fmt.Errorf("failed to load policy: %v", err)
	}

	global.CasbinEnforcer = e
	log.Println("casbin启动完成")

	global.OtaAddress = viper.GetString("ota.download_address")
	return nil
}

// attachCasbinWatcher 在 Redis 就绪后按配置挂载集群策略同步 watcher（ROADMAP C7+）。
// 单实例部署无需开启（默认关闭）；集群多实例开启后，任一实例的策略变更经 Redis
// Pub/Sub 广播，其余实例即时 LoadPolicy 收敛（SyncedEnforcer 保证与并发 Enforce 互斥）。
// 依赖顺序：RedisInit（本函数挂载点）晚于 CasbinInit，故此处全局 enforcer 必已就绪；
// 测试等未初始化场景静默跳过。
func attachCasbinWatcher() error {
	if !viper.GetBool("casbin.watcher.enabled") {
		return nil
	}
	if global.CasbinEnforcer == nil {
		log.Println("casbin watcher: 全局 enforcer 未就绪，跳过挂载")
		return nil
	}
	channel := viper.GetString("casbin.watcher.channel")
	opts := []casbinwatcher.Option{}
	if channel != "" {
		opts = append(opts, casbinwatcher.WithChannel(channel))
	}
	w, err := casbinwatcher.New(global.REDIS, opts...)
	if err != nil {
		return fmt.Errorf("构造 watcher 失败: %w", err)
	}
	if err := global.CasbinEnforcer.SetWatcher(w); err != nil {
		w.Close()
		return fmt.Errorf("挂载到 enforcer 失败: %w", err)
	}
	// 覆盖默认回调：补一条观测日志后全量重载（SyncedEnforcer 的 LoadPolicy 自带锁）。
	if err := w.SetUpdateCallback(func(string) {
		if err := global.CasbinEnforcer.LoadPolicy(); err != nil {
			log.Printf("casbin watcher: 跨实例策略重载失败: %v", err)
			return
		}
		log.Println("casbin watcher: 收到跨实例变更通知，策略已重载")
	}); err != nil {
		w.Close()
		return fmt.Errorf("设置回调失败: %w", err)
	}
	log.Printf("casbin watcher: 已挂载（channel=%s）——集群实例策略同步生效", w.Channel())
	return nil
}
