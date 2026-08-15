// 文件用途：提供SSE 推送相关的接口辅助能力。
// 核心逻辑：维护 SSE 连接生命周期、事件写入和客户端断开处理。
// 关键注意事项：长连接逻辑需要关注并发安全、资源释放和代理超时行为。
// 重构建议：如推送主题继续增加，优先抽取连接管理与消息编码的独立组件。
package sseapi

import (
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type SSEApi struct{}

// /api/v1/events

func (*SSEApi) HandleSystemEvents(c *gin.Context) {
	userClaims, ok := c.MustGet("claims").(*utils.UserClaims)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "UserClaims not found",
		}))
		return
	}

	logrus.WithFields(logrus.Fields{
		"tenantID":  userClaims.TenantID,
		"userEmail": userClaims.Email,
	}).Info("User connected to SSE")

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲,防止连接过早关闭

	clientID := global.TPSSEManager.AddClient(userClaims.TenantID, userClaims.ID, c.Writer)
	defer global.TPSSEManager.RemoveClient(userClaims.TenantID, clientID)

	// 发送初始成功消息
	c.SSEvent("message", "Connected to system events")
	c.Writer.Flush()

	// 创建一个用于发送心跳的计时器
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// 创建一个用于检查客户端是否仍然连接的通道
	done := make(chan bool)
	go func() {
		<-c.Request.Context().Done()
		done <- true
	}()

	for {
		select {
		case <-heartbeatTicker.C:
			// 发送心跳消息
			c.SSEvent("heartbeat", time.Now().Unix())
			c.Writer.Flush()
		case <-done:
			logrus.WithFields(logrus.Fields{
				"tenantID":  userClaims.TenantID,
				"userEmail": userClaims.Email,
			}).Info("User disconnected from SSE")
			return
		}
	}
}
