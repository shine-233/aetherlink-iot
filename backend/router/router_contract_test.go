// 文件用途：用静态 AST 检查后端关键路由注册契约。
// 核心逻辑：解析 `router_init.go`，收集 Gin 路由字面量和模块 Init 调用，断言公开路径和 P0/P1 模块仍存在。
// 关键注意事项：该测试只证明注册入口未丢失，不验证 handler 业务结果或权限策略。
// 重构建议：后续可从机器可读路由清单生成断言，并同步 API 自动化目录。
package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func parseRouterInit(t *testing.T) *ast.File {
	return parseRouterFile(t, "router_init.go")
}

func parseRouterFile(t *testing.T, file string) *ast.File {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	return parsed
}

func collectGinRouteLiterals(t *testing.T) map[string]bool {
	return collectGinRouteLiteralsFromFile(t, "router_init.go")
}

func collectGinRouteLiteralsFromFile(t *testing.T, file string) map[string]bool {
	t.Helper()
	parsed := parseRouterFile(t, file)
	routes := map[string]bool{}

	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "DELETE", "GET", "PATCH", "POST", "PUT", "StaticFile", "Group":
		default:
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil {
			routes[value] = true
		}
		return true
	})

	return routes
}

func collectSelectorCalls(t *testing.T) map[string]bool {
	t.Helper()
	parsed := parseRouterInit(t)
	calls := map[string]bool{}

	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if path := selectorPath(selector); path != "" {
			calls[path] = true
		}
		return true
	})

	return calls
}

func selectorPath(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		prefix := selectorPath(typed.X)
		if prefix == "" {
			return typed.Sel.Name
		}
		return prefix + "." + typed.Sel.Name
	default:
		return ""
	}
}

func TestRouterContractKeepsRootPublicAndMiddlewareSurfaces(t *testing.T) {
	routes := collectGinRouteLiterals(t)

	for _, route := range []string{
		"/health",
		"/ready",
		"/metrics",
		"/metrics-viewer",
		"/files/*filepath",
		"/swagger/*any",
		"api",
		"v1",
	} {
		if !routes[route] {
			t.Fatalf("router missing route literal %s", route)
		}
	}
}

func TestRouterContractKeepsP0P1AppRouteRegistrations(t *testing.T) {
	calls := collectSelectorCalls(t)

	for _, call := range []string{
		"apps.Model.Device.InitDevice",
		"apps.Model.TelemetryData.InitTelemetryData",
		"apps.Model.Alarm.Init",
		"apps.Model.NotificationGroup.InitNotificationGroup",
		"apps.Model.NotificationHistoryGroup.InitNotificationHistory",
		"apps.Model.Role.Init",
		"apps.Model.Casbin.Init",
		"apps.Model.Scene.Init",
		"apps.Model.SceneAutomations.Init",
		"apps.Model.OpenAPIKey.InitOpenAPIKey",
		"apps.Model.ServicePlugin.Init",
		"apps.Model.RDI.InitRDI",
	} {
		if !calls[call] {
			t.Fatalf("router missing app registration %s", call)
		}
	}
}

func TestRouterContractKeepsDevicePutRoutes(t *testing.T) {
	routes := collectGinRouteLiteralsFromFile(t, "apps/device.go")

	for _, route := range []string{"", "active", "update/config", "twin/:id/desired"} {
		if !routes[route] {
			t.Fatalf("device router missing PUT route literal %s", route)
		}
	}
}

func TestRouterContractKeepsTelemetryDeadLetterRoutes(t *testing.T) {
	routes := collectGinRouteLiteralsFromFile(t, "apps/telemetry_data.go")

	for _, route := range []string{
		"telemetry/datas",
		"dead-letters",
		"dead-letters/drain",
		"dead-letters/:id/status",
		"uplink-dead-letters",
		"uplink-dead-letters/drain",
		"uplink-dead-letters/:id/status",
	} {
		if !routes[route] {
			t.Fatalf("telemetry router missing dead-letter route literal %s", route)
		}
	}
}

func TestRouterContractKeepsEmailTemplateManagementRoutes(t *testing.T) {
	routes := collectGinRouteLiteralsFromFile(t, "apps/notification_services_config.go")

	for _, route := range []string{
		"notification/e-mail/templates",
		"preview",
		":id",
		":id/default",
	} {
		if !routes[route] {
			t.Fatalf("notification router missing email-template route literal %s", route)
		}
	}
}
