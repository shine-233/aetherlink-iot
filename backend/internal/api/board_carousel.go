// 文件用途：TP-22 大屏轮播/投屏的公开 HTTP 入口——/tv-preview 投屏端按
// share token 列表批量拉取已发布看板。
// 核心逻辑：读取 tokens 查询参数（支持逗号分隔与重复参数两种形式），交给
// service.GroupApp.Board.GetPublishedBoardsForCarousel 解析，统一走 c.Set("data") 响应。
// 关键注意事项：
//  1. 本 handler 注册在 JWT 之前的公开区块（router_init.go），鉴权边界完全
//     依赖"share token 是发布方主动公开的凭证"这一语义；绝不在此层放开
//     按 board id 取数——那会把内部 ID 暴露在无认证面上。
//  2. 不接受 interval 等展示参数：切换间隔是投屏端的展示职责，后端只对
//     "取哪些看板"负责，避免公开端点的参数面无谓扩大。
//
// 重构建议：若后续轮播需要携带每块看板的显示模式等派生字段，在 service 层
// 扩展返回结构，而不是在 handler 里拼装。
package api

import (
	service "aetherlink-iot/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// GetPublishedBoardsForCarousel 公开轮播端点。
// GET /api/v1/board/shared-carousel?tokens=t1,t2 或 ?tokens=t1&tokens=t2。
// 返回 {items: [已发布看板...], missing_tokens: [未解析 token...]}，
// items 保持请求顺序；参数为空/超限返回参数错误。
func (*BoardApi) GetPublishedBoardsForCarousel(c *gin.Context) {
	tokens := c.QueryArray("tokens")
	data, err := service.GroupApp.Board.GetPublishedBoardsForCarousel(tokens)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
