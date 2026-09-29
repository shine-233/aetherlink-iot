// 文件用途：定义数据策略接口的 API Handler，负责处理数据策略查询与更新请求。
// 核心链路：Gin 路由进入本文件中的 Handler 后，先执行参数绑定与校验，再从上下文提取 claims，
// 然后调用 DataPolicy service 完成业务处理，最后通过 c.Set("data", ...) 交给统一响应中间件输出。
// 使用注意：此层应保持“薄控制器”职责，只做请求适配、鉴权上下文透传和错误上抛，不承载业务决策。
// 静态审查建议：重点检查 claims 是否总在鉴权中间件后注入、分页/更新请求字段是否覆盖必填校验、
// 以及 service 返回错误时是否始终通过统一错误链路处理，避免出现响应格式分叉。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DataPolicyApi struct{}

// UpdateDataPolicy 更新当前租户可见范围内的数据策略配置。
// 核心步骤：绑定更新请求、读取登录用户 claims、调用 service 执行写入，并将空结果交给统一响应层。
// 审查重点：确认请求体校验足够约束可修改字段，避免越权修改；同时关注 nil data 响应是否与接口契约一致。
// @Router   /api/v1/datapolicy [put]
func (*DataPolicyApi) UpdateDataPolicy(c *gin.Context) {
	HandleAction(c, func(req *model.UpdateDataPolicyReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.DataPolicy.UpdateDataPolicy(req, userClaims)
	})
}

// CreateDataPolicy 创建行级（租户/档案粒度）数据保留策略（TB-15R）。
// 核心步骤：绑定创建请求、读取登录用户 claims、调用 service 落库（唯一索引兜底重复）。
// 审查重点：行级只允许设备数据类型，重复 (租户,档案) 返回参数错误而非 500；返回新建行 id。
// @Router   /api/v1/datapolicy [post]
func (*DataPolicyApi) CreateDataPolicy(c *gin.Context) {
	Handle(c, func(req *model.CreateDataPolicyReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DataPolicy.CreateDataPolicy(req, userClaims)
	})
}

// DeleteDataPolicy 删除行级数据保留策略（TB-15R）；全局默认行由 service 层拒绝删除。
// @Router   /api/v1/datapolicy/:id [delete]
func (*DataPolicyApi) DeleteDataPolicy(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return service.GroupApp.DataPolicy.DeleteDataPolicy(id, userClaims)
	})
}

// HandleDataPolicyListByPage 分页查询数据策略列表。
// 核心步骤：绑定分页参数、透传用户 claims 给 service、返回列表结果供统一响应中间件序列化。
// 审查重点：确认分页参数边界已在校验层约束，并检查列表查询是否始终按 claims 限定租户/项目数据范围。
// @Router   /api/v1/datapolicy [get]
func (*DataPolicyApi) HandleDataPolicyListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetDataPolicyListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DataPolicy.GetDataPolicyListByPage(req, userClaims)
	})
}
