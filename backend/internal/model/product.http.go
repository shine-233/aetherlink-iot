// 文件用途：产品选择列表的请求/响应模型（预注册建档等下拉面专用最小契约）。
package model

type GetProductSelectListReq struct {
	PageReq
	Name string `json:"name" form:"name" validate:"omitempty,max=255"`
}

type ProductSelectItem struct {
	ID   string `json:"id" gorm:"column:id"`
	Name string `json:"name" gorm:"column:name"`
}
