// 文件用途：维护 Casbin 权限策略加载和角色授权服务。
// 核心逻辑：封装策略模型、适配器和角色路径匹配，供用户与菜单权限判断使用。
// 关键注意事项：策略缺失或加载失败会影响全局鉴权，不能静默放行敏感接口。
// 重构建议：抽出策略加载接口，补齐加载失败、角色继承、路径归一化和拒绝优先级测试。
package service

import (
	"fmt"

	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type Casbin struct {
}

// 角色添加多个功能
func (*Casbin) AddFunctionToRole(role string, functions []string) bool {
	var rules [][]string
	for _, function := range functions {
		rule := []string{role, function, "allow"}
		rules = append(rules, rule)
	}
	isSuccess, _ := global.CasbinEnforcer.AddNamedPolicies("p", rules)
	return isSuccess
}

// 查询角色的功能
func (*Casbin) GetFunctionFromRole(role string) ([]string, bool) {
	if global.CasbinEnforcer == nil {
		return nil, false
	}
	policys, err := global.CasbinEnforcer.GetFilteredPolicy(0, role)
	if err != nil {
		return nil, false
	}
	functions := make([]string, 0, len(policys))
	for _, policy := range policys {
		functions = append(functions, policy[1])
	}
	return functions, true
}

// 删除角色和功能
func (*Casbin) RemoveRoleAndFunction(role string) bool {
	isSuccess, _ := global.CasbinEnforcer.RemoveFilteredPolicy(0, role)
	return isSuccess

}

// 用户添加多个角色
func (*Casbin) AddRolesToUser(user string, roles []string) bool {
	ok, _ := (&Casbin{}).AddRolesToUserWithError(user, roles)
	return ok
}

func (*Casbin) AddRolesToUserWithError(user string, roles []string) (bool, error) {
	var rules [][]string
	for _, role := range roles {
		rule := []string{user, role}
		rules = append(rules, rule)
	}
	if global.CasbinEnforcer == nil {
		return false, fmt.Errorf("casbin enforcer is not initialized")
	}
	return global.CasbinEnforcer.AddNamedGroupingPolicies("g", rules)
}

// 查询用户的角色
func (*Casbin) GetRoleFromUser(user string) ([]string, bool) {
	if global.CasbinEnforcer == nil {
		return nil, false
	}
	policys, err := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g", 0, user)
	if err != nil {
		return nil, false
	}
	roles := make([]string, 0, len(policys))
	for _, policy := range policys {
		roles = append(roles, policy[1])
	}
	return roles, true
}

// 删除用户和角色
func (*Casbin) RemoveUserAndRole(user string) bool {
	ok, _ := (&Casbin{}).RemoveUserAndRoleWithError(user)
	return ok
}

func (*Casbin) RemoveUserAndRoleWithError(user string) (bool, error) {
	if global.CasbinEnforcer == nil {
		return false, fmt.Errorf("casbin enforcer is not initialized")
	}
	return global.CasbinEnforcer.RemoveFilteredNamedGroupingPolicy("g", 0, user)
}

// 查询是否存在某个资源（注册判定）。
// 双通道：①精确 g2 分组命中（既有约定：具体 URL 挂到资源组）；
// ②锚定模式命中——g2 主体可登记 RESTful 模式（如 api/v1/device/:id），
//
//	请求路径（含真实参数值）与任一模式匹配即视为已登记。
//
// 背景与边界：gin 参数路由的请求路径是具体值，纯精确匹配永远无法命中参数模式，
//
//	导致参数路由在旧模型下不可能被保护（审计持续报未登记）；
//	模式枚举在内存策略上做（当前规模 ~300 模式），仅在精确未命中时触发。
//	不用 casbin util.KeyMatch2：其非锚定正则会子串误命中（越权放大），
//	统一走 utils.MatchURLPattern 锚定实现（与 Enforce 侧 urlPatternMatch 同源）。
func (*Casbin) GetUrl(url string) bool {
	if global.CasbinEnforcer == nil {
		return false
	}
	stringList, err := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g2", 0, url)
	if err == nil && len(stringList) != 0 {
		return true
	}
	// 不用 GetAllNamedSubjects：其在 NewModelFromString 构造的模型上会触发
	// casbin 内部 GetFieldIndex 空指针（v2.135 实测）；全量行枚举等价且安全。
	rules, err := global.CasbinEnforcer.GetNamedGroupingPolicy("g2")
	if err != nil {
		return false
	}
	for _, rule := range rules {
		if len(rule) > 0 && utils.MatchURLPattern(url, rule[0]) {
			return true
		}
	}
	return false
}

// 查询用户角色中是否存在某个角色
func (*Casbin) HasRole(role string) bool {
	if global.CasbinEnforcer == nil {
		return false
	}
	stringList, err := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g", 1, role)
	if err != nil {
		return false
	}
	return len(stringList) != 0
}

// 校验
func (*Casbin) Verify(user string, url string) bool {
	isTrue, err := global.CasbinEnforcer.Enforce(user, url, "allow")
	if err != nil {
		// Enforce 内部错误（存储/模型异常）时拒绝并记录：fail-closed 保护所有接口。
		// user/url 来自请求属污染数据，禁止写入日志（防日志注入），仅记录错误详情；
		// 需要定位时可在 DB 审计层按时间与 err 关联查询。
		logrus.Errorf("casbin enforce failed, deny by default: err=%v", err)
		return false
	}
	return isTrue
}
