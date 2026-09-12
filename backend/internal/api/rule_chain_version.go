// 文件用途：规则链版本 HTTP 入口（ROADMAP P1.2 草稿/发布/回滚）。
// 核心逻辑：绑定校验 → 取 claims → 调用 service → 统一响应中间件封装。
// 关键注意事项：
//  1. 租户一律取 claims.TenantID，不接受请求体里的 tenant_id，避免越权指定租户。
//  2. 发布/回滚失败必须原样返回错误（含"已发布"拒绝），不得吞成成功。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// HandleListRuleChainVersions 列出某条链的草稿/已发布版本
// @Router   /api/v1/rule-chains/:id/versions [get]
func (*RuleChainApi) HandleListRuleChainVersions(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rows, err := service.ListRuleChainVersions(userClaims.TenantID, c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rows)
}

// HandleCreateRuleChainDraftVersion 基于当前序列创建下一个 draft；图未变时不产生空版本
// @Router   /api/v1/rule-chains/:id/versions [post]
func (*RuleChainApi) HandleCreateRuleChainDraftVersion(c *gin.Context) {
	var req model.CreateRuleChainVersionReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	version, audit, err := service.CreateRuleChainDraftVersion(
		userClaims.TenantID, c.Param("id"), req.GraphHash, nil)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{
		"version": version,
		"audit":   audit,
	})
}

// HandlePublishRuleChainVersion 发布指定版本（draft -> published，仅一次）
// @Router   /api/v1/rule-chains/versions/publish [post]
func (*RuleChainApi) HandlePublishRuleChainVersion(c *gin.Context) {
	var req model.RuleChainVersionActionReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	audit, err := service.PublishRuleChainVersionRecord(userClaims.TenantID, req.ChainID, req.Version)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"audit": audit})
}

// HandleRollbackRuleChainVersion 从已发布版本回滚，产生一个新 draft（不改写历史）
// @Router   /api/v1/rule-chains/versions/rollback [post]
func (*RuleChainApi) HandleRollbackRuleChainVersion(c *gin.Context) {
	var req model.RuleChainVersionActionReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	version, audits, err := service.RollbackRuleChainVersionRecord(userClaims.TenantID, req.ChainID, req.Version)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{
		"version": version,
		"audits":  audits,
	})
}
