// 文件用途：租户隔离的结构性静态守卫（#8）。
// 核心逻辑：AST 扫描 dal 非测试文件中「查询型导出函数」，凡函数体既无
//
//	tenant_id/TenantID 证据、也无 all-tenants 显式授权语义者记为 suspect；
//	当前存量以预算冻结（只许下降），新增未防护查询会立即使测试失败。
//
// 关键注意事项：这是启发式护栏而非完备证明——它捕捉“新写的租户表查询忘了带
//
//	tenant_id”这一最高频事故面；跨租户管理视图请在函数体显式声明
//	allTenants/all-tenants 语义以进入豁免。精确行为仍以 *_isolation_test.go 为准。
package dal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// tenantScopeBudget 是当前 suspect 存量上限；清一个降一格，禁止回升。
// 基线 2026-08-25 实测 177（含大量“按 ID 单查+上游归属校验”的合法形态）。
// 清零路径：为 suspect 函数补 tenant_id 条件 / 显式 all-tenants 授权语义 /
//
//	证明非租户表（移出查询名匹配或加豁免注释）。高频文件：device_query_reads(15)、
//	telemetry_datas(9)、service_plugin(8)、alarm(8)、device_model(7)。
var tenantScopeBudget = 177

var tenantScopeQueryNameRe = regexp.MustCompile(`^(Get|List|Find|Page|Count|Search|Query)`)

func isTestGoFile(name string) bool {
	return strings.HasSuffix(name, "_test.go")
}

func functionBodySource(src []byte, fset *token.FileSet, fn *ast.FuncDecl) string {
	start := fset.Position(fn.Pos()).Offset
	end := fset.Position(fn.End()).Offset
	if start < 0 || end > len(src) || start >= end {
		return ""
	}
	return string(src[start:end])
}

func hasTenantGuardEvidence(body string) bool {
	if strings.Contains(body, "tenant_id") || strings.Contains(body, "TenantID") {
		return true
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, "alltenants") || strings.Contains(lower, "all-tenants")
}

func collectTenantScopeSuspects(t *testing.T) []string {
	t.Helper()
	suspects := make([]string, 0)
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dal dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".go" || isTestGoFile(name) || name == "gen.go" {
			continue
		}
		path := filepath.Join(".", name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || fn.Body == nil {
				continue
			}
			if !tenantScopeQueryNameRe.MatchString(fn.Name.Name) {
				continue
			}
			body := functionBodySource(src, fset, fn)
			if !hasTenantGuardEvidence(body) {
				suspects = append(suspects, name+":"+fn.Name.Name)
			}
		}
	}
	return suspects
}

func TestTenantScopeQueryAudit(t *testing.T) {
	suspects := collectTenantScopeSuspects(t)
	for _, s := range suspects {
		t.Logf("TENANT_SCOPE_SUSPECT: %s", s)
	}
	if tenantScopeBudget == 0 {
		// 基线模式：首次启用时输出清单并通过，把实测数量回填进 tenantScopeBudget 后改为硬门禁。
		t.Logf("baseline mode: %d suspects found; backfill tenantScopeBudget to enable enforcement", len(suspects))
		return
	}
	if len(suspects) > tenantScopeBudget {
		t.Fatalf("tenant-scope suspect queries = %d, budget %d. Add tenant_id filtering (or explicit all-tenants semantics) to each new query before shipping.", len(suspects), tenantScopeBudget)
	}
	t.Logf("tenant-scope suspect queries = %d (budget %d)", len(suspects), tenantScopeBudget)
}
