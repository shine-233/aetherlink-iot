// 文件用途：定义 board users vo 相关视图对象结构，隔离数据库模型与前端展示/聚合响应字段。
// 核心逻辑：按页面或接口需要组合基础模型字段，减少上层直接暴露持久化结构的耦合。
// 关键注意事项：VO 字段应保持只表达响应形状，避免混入查询、副作用或权限判断。
// 重构建议：当多个接口复用相同展示字段时，可抽出共享 VO 或转换 helper，避免重复维护字段映射。

package model

type GetTenantRes struct {
	UserTotal          int64                    `json:"user_total"`
	UserAddedYesterday int64                    `json:"user_added_yesterday"`
	UserAddedMonth     int64                    `json:"user_added_month"`
	UserListMonth      []*GetBoardUserListMonth `json:"user_list_month"`
}

type GetBoardUserListMonth struct {
	Month int `json:"mon" gorm:"column:mon"`
	Num   int `json:"num"`
}
