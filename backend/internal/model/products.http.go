// 文件用途：定义 products 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateProductReq struct {
	Name           string  `json:"name" validate:"required,max=255"`             // 产品名称
	Description    *string `json:"description"  validate:"omitempty,max=255"`    // 产品描述
	ProductType    *string `json:"product_type" validate:"omitempty,max=36"`     // 产品类型
	ProductKey     *string `json:"product_key" validate:"omitempty,max=255"`     // 产品key,为空后端自动生成
	ProductModel   *string `json:"product_model" validate:"omitempty,max=100"`   // 产品型号
	ImageUrl       *string `json:"image_url" validate:"omitempty,max=500"`       // 产品图片
	AdditionalInfo *string `json:"additional_info" validate:"omitempty"`         // 附加信息
	Remark         *string `json:"remark" validate:"omitempty,max=255"`          // 备注
	DeviceConfigID *string `json:"device_config_id" validate:"omitempty,max=36"` // 设备配置id
}

type UpdateProductReq struct {
	Id           string  `json:"id" validate:"required,max=36"`              // 产品id
	Name         *string `json:"name" validate:"omitempty,max=255"`          // 产品名称
	Description  *string `json:"description"  validate:"omitempty,max=255"`  // 产品描述
	ProductModel *string `json:"product_model" validate:"omitempty,max=100"` // 产品型号
	ImageUrl     *string `json:"image_url" validate:"omitempty,max=500"`     // 产品图片
	ProductType  *string `json:"product_type" validate:"omitempty,max=36"`   // 产品类型
}

type GetProductListByPageReq struct {
	PageReq
	Name         *string `json:"name" form:"name" validate:"omitempty,max=255"`                    // 产品名称
	ProductModel *string `json:"product_model" form:"product_model"  validate:"omitempty,max=100"` // 产品型号
	ProductType  *string `json:"product_type" form:"product_type" validate:"omitempty,max=36"`     // 产品类型
}

type ProductList struct {
	Product
	DeviceConfigName *string `json:"device_config_name"`
}
