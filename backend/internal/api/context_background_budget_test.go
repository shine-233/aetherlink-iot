// 文件用途：冻结 api/middleware 请求路径上 context.Background() 的存量口径。
// 核心逻辑：AST 扫描非测试 Go 文件中的 context.Background() 调用；数量超过本文件
//   登记的允许值即失败。每个允许位必须附注释说明为何请求上下文不可用。
// 关键注意事项：这是 #11 断链治理的结构性守卫——新增请求路径代码应透传
//   c.Request.Context()；确需后台语义时请更新 allowlist 并注明理由。

package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// allowedContextBackgroundCounts 按「目录 → 允许出现次数」登记存量口径：
//   - device_status_ws.go / telemetry_data.go 各 1 处：WS 连接级生命周期（升级后
//     不能绑 HTTP 请求上下文），已就地注释。
//   - telemetry_ws_auth.go 1 处：WS 首消息认证拿不到 gin 上下文，已改包短超时。
//   - middleware/apikey.go 1 处：APIKeyValidator 携带的连接级上下文，仅被 WS 认证使用。
var allowedContextBackgroundCounts = map[string]int{
	"api":        3,
	"middleware": 1,
}

func countContextBackground(t *testing.T, dir string) int {
	t.Helper()
	count := 0
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".go" || isTestFileName(name) {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if ok && ident.Name == "context" && sel.Sel.Name == "Background" {
				count++
			}
			return true
		})
	}
	return count
}

func isTestFileName(name string) bool {
	return filepath.Base(name) == "export_test.go" ||
		len(name) > 8 && name[len(name)-8:] == "_test.go"
}

func TestRequestPathContextBackgroundBudget(t *testing.T) {
	for _, dirName := range []string{"api", "middleware"} {
		dir := filepath.Join("..", dirName)
		got := countContextBackground(t, dir)
		want, registered := allowedContextBackgroundCounts[dirName]
		if !registered {
			t.Fatalf("directory %q has no registered budget; add one before introducing context.Background()", dirName)
		}
		if got > want {
			t.Fatalf("context.Background() count in internal/%s = %d, budget %d. Pass the request context (c.Request.Context()) instead; if background semantics are intended, update the budget with a justification comment.", dirName, got, want)
		}
		t.Logf("internal/%s: context.Background()=%d (budget %d)", dirName, got, want)
	}
}
