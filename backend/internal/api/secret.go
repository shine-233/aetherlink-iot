// 文件用途：通用 Secrets Storage（ROADMAP TB-18）API 接口层。
// 核心逻辑：提供通用密钥的新增、列表、详情、更新、删除、解密查看和重加密接口。
// 关键注意事项：
//  1. 默认接口严格只返回不可逆脱敏掩码，避免调用端日志和内存无意泄漏密钥；
//  2. /reveal 接口受到 Casbin 与 Service 双重权限保护，且强制留痕审计。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type SecretApi struct{}

// CreateSecret 创建通用密钥
// @Router /api/v1/secrets [post]
func (*SecretApi) CreateSecret(c *gin.Context) {
	Handle(c, func(req *model.CreateSecretReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Secret.CreateSecret(c.Request.Context(), req, userClaims)
	})
}

// GetSecretList 获取通用密钥列表（脱敏）
// @Router /api/v1/secrets [get]
func (*SecretApi) GetSecretList(c *gin.Context) {
	Handle(c, func(req *model.SecretListReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Secret.ListSecrets(c.Request.Context(), req, userClaims)
	})
}

// GetSecretDetail 获取通用密钥详情（脱敏）
// @Router /api/v1/secrets/:id [get]
func (*SecretApi) GetSecretDetail(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Secret.GetSecret(c.Request.Context(), id, userClaims)
	})
}

// UpdateSecret 更新通用密钥元数据或更新值
// @Router /api/v1/secrets/:id [put]
func (*SecretApi) UpdateSecret(c *gin.Context) {
	HandlePathBody(c, "id", func(id string, req *model.UpdateSecretReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Secret.UpdateSecret(c.Request.Context(), id, req, userClaims)
	})
}

// DeleteSecret 删除通用密钥
// @Router /api/v1/secrets/:id [delete]
func (*SecretApi) DeleteSecret(c *gin.Context) {
	// 迁移形态：HandlePath（路径参数 id + claims）。成功包络保留旧的 data 对象 {"deleted": true}，
	// 因此不能改用 HandlePathAction（其成功时 data 为 nil 并从包络中省略该字段）。
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		if err := service.GroupApp.Secret.DeleteSecret(c.Request.Context(), id, userClaims); err != nil {
			return nil, err
		}
		return gin.H{"deleted": true}, nil
	})
}

// RevealSecret 解密查看密钥明文（管理员受审操作）
// @Router /api/v1/secrets/:id/reveal [post]
func (*SecretApi) RevealSecret(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Secret.RevealSecret(c.Request.Context(), id, userClaims)
	})
}

// ResealSecret 轮换重加密
// @Router /api/v1/secrets/:id/reseal [post]
func (*SecretApi) ResealSecret(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Secret.ResealSecret(c.Request.Context(), id, userClaims)
	})
}
