// 文件用途：锚定的 RESTful URL 模式匹配（casbin 资源登记双通道之"模式通道"）。
// 核心逻辑：模式中 ":name" 段（gin 参数路由约定）匹配单个非 "/" 段，其余段按字面
//
//	（regexp.QuoteMeta 转义），整体 ^$ 锚定。
//
// 关键注意事项：不用 casbin util.KeyMatch2——其内部是非锚定正则，"api/v1/devices"
//
//	模式会子串命中 "api/v1/devicesXYZ"，在 Enforce 侧构成越权放大；本实现锚定整串，
//	段级通配仅限 ":name"，杜绝该类误匹配。
package utils

import (
	"regexp"
	"strings"
)

// MatchURLPattern 判断具体请求路径 url 是否命中登记模式 pattern。
// 双方均为去掉前导 "/" 的路径（与 CasbinRBAC 中间件口径一致）。
// 约定：":name" 仅在段首生效，整段通配（[^/]+）；空段按字面处理。
func MatchURLPattern(url, pattern string) bool {
	if pattern == "" {
		return false
	}
	segs := strings.Split(strings.Trim(pattern, "/"), "/")
	for i, s := range segs {
		if len(s) > 1 && strings.HasPrefix(s, ":") {
			segs[i] = "[^/]+"
		} else {
			segs[i] = regexp.QuoteMeta(s)
		}
	}
	re, err := regexp.Compile("^/" + strings.Join(segs, "/") + "/?$")
	if err != nil {
		return false
	}
	// url 与 pattern 同口径去前导斜杠后按锚定正则匹配。
	return re.MatchString("/" + strings.Trim(url, "/"))
}

// URLPatternCasbinFunction 返回可注入 casbin Enforcer 的自定义 matcher 函数
// （configs/casbin.conf 的 urlPatternMatch；initialize.CasbinInit 经 AddFunction 注册）。
// 生产与测试共用同一实现，杜绝夹具模型与线上 matcher 漂移。
func URLPatternCasbinFunction() func(args ...interface{}) (interface{}, error) {
	return func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return false, nil
		}
		url, ok1 := args[0].(string)
		pattern, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false, nil
		}
		return MatchURLPattern(url, pattern), nil
	}
}
