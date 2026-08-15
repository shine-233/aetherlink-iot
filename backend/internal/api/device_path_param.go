// 文件用途：统一读取设备类路由的设备 ID 路径参数。
// 核心逻辑：同一批设备接口在 router 里混用了 `:id`（device 分组）和 `:device_id`
//
//	（rdi 分组、/devices/:device_id/diagnostics 等）两种占位符命名，而 handler 侧
//	曾一律写死 c.Param("device_id")。gin 只按注册时的占位符名提供参数，因此注册为
//	`:id` 的那批路由取到空串，handler 直接以 100002 "device_id is required" 拒绝，
//	接口永远不可用。此 helper 依次尝试两种命名，把差异收敛到一处。
//
// 关键注意事项：不要为了"对齐"去改 router 里已有的占位符名——同一 gin 分组下
//
//	把 `:id` 改成 `:device_id` 会与 detail/:id、metrics/:id 等既有路由冲突
//	（gin wildcard 冲突会 panic），也会破坏既有前端调用。
package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// devicePathID returns the device id from whichever placeholder the route was
// registered with, preferring the explicit `device_id` spelling.
func devicePathID(c *gin.Context) string {
	if deviceID := strings.TrimSpace(c.Param("device_id")); deviceID != "" {
		return deviceID
	}
	return strings.TrimSpace(c.Param("id"))
}
