// 文件用途：保留后端业务逻辑层的用户入口。
// 核心逻辑：封装用户相关 query 操作和轻量规则，为上层服务提供稳定调用边界。
// 关键注意事项：用户路径涉及租户与角色边界，修改时不要绕过 service 层的鉴权和 Casbin 约定。
// 重构建议：若后续承载更多用户规则，应抽出可测试接口并补齐权限负例覆盖。

package logic

import (
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"context"
)

func UserIsEncrypt(ctx context.Context) bool {
	var (
		sysFunction = query.SysFunction
	)
	// 默认没有该配置为关闭状态
	info, err := sysFunction.WithContext(ctx).Where(sysFunction.Name.Eq("frontend_res")).First()
	if err != nil {
		return false
	}
	if info.EnableFlag == constant.DisableFlag {
		return false
	}
	return true
}

func UserIsShare(ctx context.Context) bool {
	var (
		sysFunction = query.SysFunction
	)
	// 默认没有该配置为关闭状态
	info, err := sysFunction.WithContext(ctx).Where(sysFunction.Name.Eq("shared_account")).First()
	if err != nil {
		return false
	}
	if info.EnableFlag == constant.DisableFlag {
		return false
	}
	return true
}
