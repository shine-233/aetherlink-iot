// 文件用途：初始化 Casbin 权限引擎，并把实例挂入全局状态供后续鉴权流程复用。
// 核心逻辑：基于已建立的 GORM 数据库连接创建 adapter、装载模型文件并加载策略。
// 关键注意事项：该初始化依赖数据库先成功建立，且模型文件路径必须与启动工作目录保持一致。
// 重构建议：后续可将模型路径和全局赋值行为参数化，便于测试环境替换策略源。

package initialize

import (
	"fmt"
	"log"

	"aetherlink-iot/backend/internal/adapter/casbinadapter"
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

	e, err := casbin.NewEnforcer("./configs/casbin.conf", a)
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
