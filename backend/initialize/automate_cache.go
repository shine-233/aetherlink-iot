// 文件用途：自动化触发缓存的兼容门面。
// 实现已迁入 initialize/automatecache（无状态、可并发的 Cache）；这里只保留历史导出名，
// 让 scene_automations.go 与 integration 测试等既有调用方零改动继续编译。
// 新代码请直接依赖 automatecache 包或 service 层注入的缓存接口。

package initialize

import "aetherlink-iot/backend/initialize/automatecache"

const (
	AUTOMATE_CACHE_RESULT_NOT_FOUND = automatecache.ResultNotFound // 缓存中无数据
	AUTOMATE_CACHE_RESULT_NOT_TASK  = automatecache.ResultNoTask   // 缓存有数据 无任务
	AUTOMATE_CACHE_RESULT_OK        = automatecache.ResultOK       // 缓存有数据 有任务
)

const AUTOMATE_CACHE_CONTENT_NOT_TASK = automatecache.ContentNoTask

type (
	AutomateCache           = automatecache.Cache
	AutimateCacheKeyDevice  = automatecache.KeyDimension
	AutomateDeviceInfo      = automatecache.AutomateDeviceInfo
	AutomateActionInfo      = automatecache.AutomateActionInfo
	AutomateExecteSceneInfo = automatecache.AutomateExecteSceneInfo
	AutomateExecteParams    = automatecache.AutomateExecteParams
	AutomateDeviceInfos     = automatecache.AutomateDeviceInfos
	DTConditions            = automatecache.DTConditions
)

// NewAutomateCache 返回绑定 global.REDIS 的进程级自动化缓存。
func NewAutomateCache() *AutomateCache {
	return automatecache.Default()
}
