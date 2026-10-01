// groupapp_freeze_test.go 是 Wave7-C 子阶段 1（api→service 调用面冻结）的机械守卫。
// 核心逻辑：解析 internal/api 全部非测试 Go 源文件，统计对 service.GroupApp 的直接引用数，
// 断言不超过基线 groupAppRefBaseline——每完成一个域的注入迁移就下调一次，只许减不许增。
// 关键注意事项：
//  1. 用 go/parser 做选择器计数（pkg.GroupApp 形态），不依赖 go list，测试内即可运行；
//  2. 基线 667 = 冻结时点实测 676 − report 域迁走的 9；新增 handler 一律走已注入的
//     XxxSvc 接口字段，不得再加 service.GroupApp 直引；
//  3. 若未来 GroupApp 门面删除，本测试应随最后一个域的迁移一并删除。
package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// groupAppRefBaseline：api 包允许残留的 service.GroupApp 直引上限。
// 每迁移一个域并全量门禁通过后，把基线下调到当前实测值。
// 571 = wave7-C 首次冻结时点实测（含 report 域 9 处迁走与 enter.go 组装点 1 处）。
const groupAppRefBaseline = 571

func TestAPIPackageGroupAppReferencesFrozen(t *testing.T) {
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve api dir: %v", err)
	}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read api dir: %v", err)
	}

	total := 0
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".go" || isTestFile(name) {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		total += countGroupAppSelectors(file)
	}

	if total > groupAppRefBaseline {
		t.Fatalf("service.GroupApp direct references = %d, baseline = %d\n"+
			"新增 handler 必须走已注入的 XxxSvc 域接口字段；"+
			"如确属合法新增（如新域尚未抽接口），请先完成该域的接口抽取再上调基线并在提交说明中记录",
			total, groupAppRefBaseline)
	}
	t.Logf("service.GroupApp references = %d (baseline %d)", total, groupAppRefBaseline)
}

// countGroupAppSelectors 统计单个文件中 pkg.GroupApp 形态的选择器出现次数。
// 覆盖读（service.GroupApp.X）与写（GroupApp 被赋值/取址）两类位置。
func countGroupAppSelectors(file *ast.File) int {
	count := 0
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "service" && sel.Sel.Name == "GroupApp" {
			count++
		}
		return true
	})
	return count
}

func isTestFile(name string) bool {
	return len(name) > 8 && name[len(name)-8:] == "_test.go"
}
