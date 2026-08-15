// 文件用途：封装后端公共用户角色判断。
// 核心逻辑：将传入权限标识与系统管理员常量比较，返回是否为系统管理员。
// 关键注意事项：该函数只做字符串判断，不代表完整鉴权；业务权限仍应由中间件和 service 校验。
// 重构建议：后续可迁移到权限/身份领域包，并支持更清晰的角色枚举类型。
package common

import (
	constant "aetherlink-iot/backend/pkg/constant"
)

func CheckUserIsAdmin(authority string) bool {
	return authority == constant.SYS_ADMIN
}
