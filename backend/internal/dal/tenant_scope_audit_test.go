// 文件用途：租户隔离的结构性静态守卫（#8）——清单棘轮版。
// 核心逻辑：AST 扫描 dal 非测试文件中「查询型导出函数」，凡函数体既无
//   tenant_id/TenantID 证据、也无 all-tenants 显式授权语义、且不含
//   `tenant-scope:` 标记者记为 suspect；与 tenant_scope_suspects.json 基线
//   按函数名对比：新增未登记函数即失败；存量清一个减一格（env 触发重写）。
// 关键注意事项：这是启发式护栏而非完备证明。为历史函数补 tenant_id 条件后，
//   用 `TENANT_SCOPE_AUDIT_UPDATE=1 go test ./internal/dal/ -run TestTenantScopeQueryAudit`
//   重写基线并提交 diff（应只出现删除行）。新写查询请直接带租户过滤或标注
//   `tenant-scope: caller-enforced|system-table|no-tenant-column` 等证据注释。
package dal

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const tenantScopeBaselineFile = "tenant_scope_suspects.json"

var tenantScopeQueryNameRe = regexp.MustCompile(`^(Get|List|Find|Page|Count|Search|Query)`)

func isTestGoFileName(name string) bool {
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
	if strings.Contains(lower, "alltenants") || strings.Contains(lower, "all-tenants") {
		return true
	}
	// 显式标注约定：函数头注释写明 tenant-scope: <类别> 即视为已人工核验。
	return strings.Contains(lower, "tenant-scope:")
}

func collectTenantScopeSuspects(t *testing.T) map[string]string {
	t.Helper()
	suspects := make(map[string]string)
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dal dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".go" || isTestGoFileName(name) || name == "gen.go" || name == tenantScopeBaselineFile {
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
				suspects[fn.Name.Name] = name
			}
		}
	}
	return suspects
}

func TestTenantScopeQueryAudit(t *testing.T) {
	current := collectTenantScopeSuspects(t)

	if os.Getenv("TENANT_SCOPE_AUDIT_UPDATE") == "1" {
		payload, _ := json.MarshalIndent(current, "", "  ")
		if err := os.WriteFile(tenantScopeBaselineFile, append(payload, '\n'), 0o644); err != nil {
			t.Fatalf("write baseline: %v", err)
		}
		t.Logf("baseline rewritten: %d entries", len(current))
		return
	}

	raw, err := os.ReadFile(tenantScopeBaselineFile)
	if err != nil {
		t.Fatalf("read %s: %v (generate it with TENANT_SCOPE_AUDIT_UPDATE=1)", tenantScopeBaselineFile, err)
	}
	baseline := make(map[string]string)
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatalf("parse %s: %v", tenantScopeBaselineFile, err)
	}

	newDebt := make([]string, 0)
	pruned := make([]string, 0)
	for name, file := range current {
		if _, ok := baseline[name]; !ok {
			newDebt = append(newDebt, file+":"+name)
		}
	}
	for name := range baseline {
		if _, ok := current[name]; !ok {
			pruned = append(pruned, name)
		}
	}

	if len(pruned) > 0 {
		t.Logf("%d suspect(s) resolved since baseline; rewrite it with TENANT_SCOPE_AUDIT_UPDATE=1 to lock in progress: %v", len(pruned), pruned)
	}
	if len(newDebt) > 0 {
		t.Fatalf("new unguarded tenant-scope query(ies) detected: %v. Add tenant_id filtering, explicit all-tenants semantics, or a reviewed `tenant-scope:` marker comment.", newDebt)
	}
	t.Logf("tenant-scope suspects=%d (frozen baseline)", len(current))
}
