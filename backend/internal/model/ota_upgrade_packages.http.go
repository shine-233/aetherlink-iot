// 文件用途：定义 ota upgrade packages 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateOTAUpgradePackageReq struct {
	Name           string  `json:"name" validate:"required,max=200"`                     // 升级包名称
	Version        string  `json:"version"  validate:"required,max=36"`                  // 版本号
	TargetVersion  *string `json:"target_version" validate:"omitempty,max=36"`           // 目标版本号
	DeviceConfigID string  `json:"device_config_id" validate:"required,max=36"`          // 设备配置ID
	Module         *string `json:"module" validate:"omitempty,max=36"`                   // 模块名称
	PackageType    *int16  `json:"package_type" validate:"required,oneof=1 2"`           // 升级包类型升级包类型1-差分 2-整包
	SignatureType  *string `json:"signature_type" validate:"omitempty,oneof=MD5 SHA256"` // 签名算法 MD5 SHA256
	AdditionalInfo *string `json:"additional_info" validate:"omitempty" example:"{}"`    // 附加信息,json格式
	Description    *string `json:"description" validate:"omitempty,max=500"`             // 描述
	PackageUrl     *string `json:"package_url" validate:"required,max=500"`              // 升级包地址
	Remark         *string `json:"remark" validate:"omitempty,max=255"`
}

type UpdateOTAUpgradePackageReq struct {
	Id             string  `json:"id" validate:"required,max=36"`                        // 升级包ID
	Name           string  `json:"name" validate:"omitempty,max=200"`                    // 升级包名称
	Version        string  `json:"version"  validate:"omitempty,max=36"`                 // 版本号
	TargetVersion  *string `json:"target_version" validate:"omitempty,max=36"`           // 目标版本号
	DeviceConfigID string  `json:"device_config_id" validate:"omitempty,max=36"`         // 设备配置ID
	Module         *string `json:"module" validate:"omitempty,max=36"`                   // 模块名称
	PackageType    *int16  `json:"package_type" validate:"omitempty,oneof=1 2"`          // 升级包类型
	SignatureType  *string `json:"signature_type" validate:"omitempty,oneof=MD5 SHA256"` // 签名算法 MD5 SHA256
	AdditionalInfo *string `json:"additional_info" validate:"omitempty"`                 // 附加信息,json格式
	Description    *string `json:"description" validate:"omitempty,max=500"`             // 描述
	PackageUrl     *string `json:"package_url" validate:"omitempty,max=500"`             // 升级包地址
	Remark         *string `json:"remark" validate:"omitempty,max=255"`                  // 备注
}

type GetOTAUpgradePackageLisyByPageReq struct {
	PageReq
	DeviceConfigID string `json:"device_configs_id" form:"device_config_id" validate:"omitempty,max=36" example:"uuid"` // 设备配置ID
	Name           string `json:"name" form:"name" validate:"omitempty,max=200"`                                        //  升级包名称
	Version        string `json:"version" form:"version" validate:"omitempty,max=36"`                                   // 升级包版本号
}

type GetOTAUpgradeTaskListByPageRsp struct {
	OtaUpgradePackage
	DeviceConfigName string `json:"device_config_name" validate:"omitempty,max=200"` // 设备配置名称
}
