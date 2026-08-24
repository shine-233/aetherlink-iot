// 文件用途：锁定 router_init.go 的路由认证层级契约，堵住"新端点默认绕过 RBAC"的缺口。
// 核心逻辑：用 go/ast 按源码顺序解析路由注册与中间件 Use 位置，把 router_init.go 内
// 字面量注册的路由归入 根公开面/插件鉴权/匿名 v1/JWT-only 四个层级；任何未在合约清单
// 中分类的新路由都会让测试失败，强制开发者有意识地决定其鉴权层级（业务接口还需同步 seed casbin g2 策略）。
// 关键注意事项：本测试只锁定"代码侧路由清单"；casbin 策略本身是部署态数据（casbin_rule 表），
// 无法离线校验覆盖。CasbinRBAC 之后经 apps.Model.* 模块批量注册的路由受组级 Use 保护，
// 不在本合约的字面量范围内。
// 重构建议：若未来引入集中式路由表，可改为运行时枚举 gin.Routes() 做更强校验。

package router

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

type routeTier int

const (
	tierRootPublic routeTier = iota // 根路径无鉴权（运维/静态资源面）
	tierPluginAuth                  // PluginAuth 中间件保护
	tierPublicV1                    // v1 组内、JWTAuth 之前注册：匿名可访问
	tierJWTOnly                     // JWT 之后、CasbinRBAC 之前：登录态即可
)

func (t routeTier) String() string {
	switch t {
	case tierRootPublic:
		return "root-public"
	case tierPluginAuth:
		return "plugin-auth"
	case tierPublicV1:
		return "public-v1"
	case tierJWTOnly:
		return "jwt-only"
	default:
		return "unknown"
	}
}

type routeRecord struct {
	method string
	path   string
	line   int
}

// publicRouteContract 是各层级路由的显式清单。新增/删除路由时必须同步维护。
var publicRouteContract = map[routeTier]map[string]string{
	tierRootPublic: {
		"GET /swagger/*any":                  "仅非生产环境注册（GOTP_ENV=production 跳过），Swagger 文档面",
		"GET /metrics":                       "仅非生产环境注册，Prometheus 抓取面（loopback 绑定契约）",
		"GET /metrics-viewer":                "仅非生产环境注册，指标可视化页",
		"GET /metrics-viewer/echarts.min.js": "metrics-viewer 静态资源",
		"GET /files/*filepath":               "公开文件访问面（nosniff + 附件下发防护见 router_init.go）",
		"GET /health":                        "存活探针",
		"GET /ready":                         "就绪探针（依赖检查）",
		"GET /deployment/health":             "部署健康核验入口",
	},
	tierPluginAuth: {
		"POST /api/v1/plugin/heartbeat":           "协议插件心跳（PluginAuth：X-Plugin-Key 或回环/私网放行）",
		"POST /api/v1/plugin/device/config":       "协议插件设备配置分发",
		"POST /api/v1/plugin/devices":             "按协议类型拉取插件设备",
		"POST /api/v1/plugin/service/access/list": "插件服务接入列表",
		"POST /api/v1/plugin/service/access":      "插件服务接入写入",
	},
	tierPublicV1: {
		"POST /api/v1/login":                                        "登录入口（LoginRateLimit + LoginLock）",
		"GET /api/v1/verification/code":                             "图形验证码",
		"POST /api/v1/reset/password/link":                          "密码重置链接申请",
		"POST /api/v1/reset/password":                               "密码重置执行",
		"GET /api/v1/logo":                                          "站点 Logo 读取",
		"GET /api/v1/telemetry/datas/current/ws":                    "遥测 WS（首条消息内自鉴权）",
		"GET /api/v1/device/online/status/ws":                       "设备状态 WS 兼容实现（首条消息内自鉴权）",
		"GET /api/v1/device/online/status/ws/batch":                 "设备状态 WS 批量订阅（首条消息内自鉴权）",
		"GET /api/v1/telemetry/datas/current/keys/ws":               "遥测 keys WS（首条消息内自鉴权）",
		"GET /api/v1/ota/download/files/upgradePackage/:path/:file": "OTA 升级包下载",
		"GET /api/v1/rdi/shared/:token":                             "RDI 分享只读视图（分享 token 即凭证）",
		"GET /api/v1/board/shared/:token":                           "看板分享只读视图（分享 token 即凭证）",
		"GET /api/v1/systime":                                       "系统时间",
		"GET /api/v1/deployment/health":                             "部署健康核验（v1 别名）",
		"GET /api/v1/sys_function":                                  "系统功能开关读取",
		"POST /api/v1/tenant/email/register":                        "租户邮箱注册",
		"GET /api/v1/tenant/has-admin":                              "超管存在性检查（首次安装引导）",
		"GET /api/v1/tenant/setup-state":                            "首次安装状态",
		"POST /api/v1/tenant/super-admin/init":                      "首次安装超管初始化（幂等守卫在 service 层）",
		"POST /api/v1/tenant/market-register":                       "市场联动注册",
		"POST /api/v1/device/gateway-register":                      "网关自动注册（设备凭证语义）",
		"POST /api/v1/device/gateway-sub-register":                  "网关子设备注册",
		"GET /api/v1/sys_version":                                   "系统版本读取（迁移门禁依赖）",
		"POST /api/v1/device/auth":                                  "一型一密动态认证",
	},
	tierJWTOnly: {
		"GET /api/v1/devices/:device_id/diagnostics": "设备诊断需要登录态 claims 但不走 Casbin 菜单权限（router_init.go 注释契约）",
	},
}

func extractRouteInventory(t *testing.T) (routes []routeRecord, jwtUseLine, casbinUseLine int) {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "router_init.go", nil, 0)
	if err != nil {
		t.Fatalf("parse router_init.go: %v", err)
	}

	groups := map[string]string{"router": "", "api": "/api"}

	ast.Inspect(parsed, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if len(v.Lhs) == 1 && len(v.Rhs) == 1 {
				lhs, okLhs := v.Lhs[0].(*ast.Ident)
				rhs, okRhs := v.Rhs[0].(*ast.CallExpr)
				if !okLhs || !okRhs {
					return true
				}
				groupSel, okSel := rhs.Fun.(*ast.SelectorExpr)
				if !okSel || groupSel.Sel.Name != "Group" || len(rhs.Args) == 0 {
					return true
				}
				lit, okLit := rhs.Args[0].(*ast.BasicLit)
				baseIdent, okBase := groupSel.X.(*ast.Ident)
				if !okLit || !okBase {
					return true
				}
				groups[lhs.Name] = joinRoutePath(groups[baseIdent.Name], strings.Trim(lit.Value, `"`))
			}
		case *ast.ExprStmt:
			call, okCall := v.X.(*ast.CallExpr)
			if !okCall {
				return true
			}
			sel, okSel := call.Fun.(*ast.SelectorExpr)
			if !okSel {
				return true
			}
			line := fset.Position(call.Pos()).Line
			switch sel.Sel.Name {
			case "Use":
				if len(call.Args) == 0 {
					return true
				}
				argCall, okArg := call.Args[0].(*ast.CallExpr)
				if !okArg {
					return true
				}
				argSel, okArgSel := argCall.Fun.(*ast.SelectorExpr)
				if !okArgSel {
					return true
				}
				switch argSel.Sel.Name {
				case "JWTAuth":
					jwtUseLine = line
				case "CasbinRBAC":
					casbinUseLine = line
				}
			case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
				if len(call.Args) == 0 {
					return true
				}
				lit, okLit := call.Args[0].(*ast.BasicLit)
				groupIdent, okGroup := sel.X.(*ast.Ident)
				if !okLit || !okGroup || lit.Kind != token.STRING {
					return true
				}
				routes = append(routes, routeRecord{
					method: sel.Sel.Name,
					path:   joinRoutePath(groups[groupIdent.Name], strings.Trim(lit.Value, `"`)),
					line:   line,
				})
			}
		}
		return true
	})

	if jwtUseLine == 0 || casbinUseLine == 0 {
		t.Fatalf("router_init.go 缺少 JWTAuth(%d)/CasbinRBAC(%d) 的 Use 注册点，合约基准失效", jwtUseLine, casbinUseLine)
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].line < routes[j].line })
	return routes, jwtUseLine, casbinUseLine
}

func joinRoutePath(base, p string) string {
	if p == "" || p == "/" {
		return base
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimSuffix(base, "/") + p
}

func TestRouterAuthenticationTierContract(t *testing.T) {
	routes, jwtUseLine, casbinUseLine := extractRouteInventory(t)

	seen := map[string]routeTier{}
	for _, r := range routes {
		key := fmt.Sprintf("%s %s", r.method, r.path)

		var tier routeTier
		switch {
		case strings.HasPrefix(r.path, "/api/v1/plugin/"):
			tier = tierPluginAuth
		case !strings.HasPrefix(r.path, "/api/"):
			tier = tierRootPublic
		case r.line > jwtUseLine && r.line < casbinUseLine:
			tier = tierJWTOnly
		case r.line < jwtUseLine:
			tier = tierPublicV1
		default:
			// CasbinRBAC 之后由 apps.Model.* 模块批量注册，路径分散在各模块文件，
			// 本合约只锁定 router_init.go 字面量层级；模块内路由受组级 Use 保护。
			continue
		}

		if prev, dup := seen[key]; dup {
			t.Fatalf("路由 %s 在第 %d 行重复注册（首次归类为 %s）", key, r.line, prev)
		}
		seen[key] = tier

		if _, classified := publicRouteContract[tier][key]; !classified {
			t.Fatalf("发现未分类的新路由 %s（line %d）：请判定其认证层级并加入 publicRouteContract[%s]，附理由；"+
				"若是需要菜单权限的业务接口，还应同步 seed casbin g2 策略", key, r.line, tier)
		}
	}

	for tier, entries := range publicRouteContract {
		for key := range entries {
			if _, ok := seen[key]; !ok {
				t.Fatalf("白名单条目 %q（层级 %s）已不在 router_init.go 中注册，属陈旧条目，请从合约中移除", key, tier)
			}
		}
	}
}
