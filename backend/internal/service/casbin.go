// 文件用途：维护 Casbin 权限策略加载和角色授权服务。
// 核心逻辑：封装策略模型、适配器和角色路径匹配，供用户与菜单权限判断使用。
// 关键注意事项：策略缺失或加载失败会影响全局鉴权，不能静默放行敏感接口。
// 重构建议：抽出策略加载接口，补齐加载失败、角色继承、路径归一化和拒绝优先级测试。
package service

import (
	"fmt"

	global "aetherlink-iot/backend/pkg/global"
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
	policys := global.CasbinEnforcer.GetFilteredPolicy(0, role)
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
	policys := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g", 0, user)
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

// 查询是否存在某个资源
func (*Casbin) GetUrl(url string) bool {
	stringList := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g2", 0, url)
	return len(stringList) != 0
}

// 查询用户角色中是否存在某个角色
func (*Casbin) HasRole(role string) bool {
	stringList := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g", 1, role)
	return len(stringList) != 0
}

// 校验
func (*Casbin) Verify(user string, url string) bool {
	isTrue, _ := global.CasbinEnforcer.Enforce(user, url, "allow")
	return isTrue
}
