// device_topic_mapping.go 负责设备 Topic 映射规则相关 HTTP Handler。
// 核心职责：完成 Topic 映射请求的参数绑定、claims 注入和 service 调用，把 MQTT Topic 与物模型/解析规则之间的映射维护能力暴露给前端管理页面。
// 主链路：前端映射配置页发起请求 -> handler_adapter 绑定 DTO 或读取路径参数 -> 适配器取 claims 透传租户边界 -> DeviceTopicMapping service 完成创建/查询/更新/删除 -> 统一响应输出。
// 关键注意事项：Topic 映射是协议接入、上行解析和设备模型落库的前置配置，API 层只做入口校验，不应在这里实现规则编译或解析细节。
// 静态审查建议：
// 1. 本文件已迁至 handler_adapter（绑定/校验/取 claims/写响应四步收敛到适配器），后续新增接口请沿用同一形态。
// 2. Topic 映射与设备配置、协议插件强耦合，建议在目录 README 中同步记录调用链，避免维护者只看 Handler 误判影响面。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceTopicMappingApi struct{}

// DeviceTopicMappingApi 是设备 Topic 映射控制层入口。
// 该结构体无状态，用于承接路由并把请求生命周期留给 gin.Context 管理。

// CreateDeviceTopicMapping 创建设备 Topic 映射规则。
// 参数绑定：请求体绑定 CreateDeviceTopicMappingReq。
// claims 注入：claims 用于限制当前用户只能在所属租户/项目内创建映射。
// 链路说明：映射规则通常决定上行 Topic 如何对应设备模型字段，具体规则校验与持久化由 service 层统一负责。
// @Router   /api/v1/device/topic-mappings [post]
func (*DeviceTopicMappingApi) CreateDeviceTopicMapping(c *gin.Context) {
	Handle(c, func(req *model.CreateDeviceTopicMappingReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTopicMapping.CreateDeviceTopicMapping(req, userClaims)
	})
}

// GetDeviceTopicMappings 获取 Topic 映射分页或列表结果。
// 参数绑定：绑定 ListDeviceTopicMappingReq，用于承接筛选、分页与设备范围条件。
// claims 注入：通过 claims 收口当前用户可见的映射记录，避免通过设备 ID 或 Topic 模糊查询越权。
// 静态审查建议：如果后续筛选条件持续增加，建议在 DTO 注释中继续明确每个字段的组合语义。
// @Router   /api/v1/device/topic-mappings [get]
func (*DeviceTopicMappingApi) GetDeviceTopicMappings(c *gin.Context) {
	Handle(c, func(req *model.ListDeviceTopicMappingReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTopicMapping.ListDeviceTopicMappings(req, userClaims)
	})
}

// DryRunDeviceTopicMapping 预演 Topic 映射，不保存配置。
// 参数绑定：请求体绑定 DryRunDeviceTopicMappingReq。
// 链路说明：该接口用于在保存前验证测试设备 Topic 是否匹配当前映射，并返回可读诊断，避免客户把遥测或命令路由到错误系统 Topic。
// @Router   /api/v1/device/topic-mappings/dry-run [post]
func (*DeviceTopicMappingApi) DryRunDeviceTopicMapping(c *gin.Context) {
	Handle(c, func(req *model.DryRunDeviceTopicMappingReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTopicMapping.DryRunDeviceTopicMapping(req, userClaims)
	})
}

// UpdateDeviceTopicMapping 更新 Topic 映射规则。
// 参数绑定：路径参数 id + 请求体 UpdateDeviceTopicMappingReq（HandlePathBody 先绑定请求体再取 claims，与迁移前顺序一致）。
// 链路说明：该接口属于高影响配置写操作，更新后可能影响解析链路与数据入库结果，因此 API 层应避免吞掉 service 返回的领域错误。
// @Router   /api/v1/device/topic-mappings/{id} [put]
func (*DeviceTopicMappingApi) UpdateDeviceTopicMapping(c *gin.Context) {
	HandlePathBody(c, "id", func(id string, req *model.UpdateDeviceTopicMappingReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTopicMapping.UpdateDeviceTopicMapping(id, req, userClaims)
	})
}

// DeleteDeviceTopicMapping 删除 Topic 映射规则。
// 参数绑定：目标映射 ID 来自路径参数；无返回数据，成功响应省略 data 字段（HandlePathAction）。
// claims 注入：claims 确保只能删除当前可管理范围内的映射记录。
// 静态审查建议：如果后续删除路径拓展为复合主键或批量删除，可及时补充专用 DTO 或辅助解析函数。
// @Router   /api/v1/device/topic-mappings/{id} [delete]
func (*DeviceTopicMappingApi) DeleteDeviceTopicMapping(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return service.GroupApp.DeviceTopicMapping.DeleteDeviceTopicMapping(id, userClaims)
	})
}
