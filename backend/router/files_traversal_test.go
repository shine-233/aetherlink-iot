// 文件用途：钉死 `/files/*filepath` 直接文件服务的路径穿越行为边界（挂账项见 references/source-quality-review.md）。
// 核心逻辑：用 httptest 表驱动覆盖 ../、URL 编码（%2e%2e%2f、..%2f、..%5C）、反斜杠、盘符、
// 空路径等攻击向量并断言根外诱饵文件永不下发；同时验证合法子路径（含 `./` 归一化）仍正常返回。
// 关键注意事项：serving harness 复刻 router_init.go 中 /files 闭包的「ResolveRelativePath -> os.OpenRoot ->
// 仅常规文件」处理顺序；生产路由本身由本文件的 AST 契约测试锁死，两处需同步演进。
// 重构建议：后续将闭包提取为具名 handler 后可直接对生产 handler 做 httptest，删除本地 harness。
package router

import (
	"go/ast"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aetherlink-iot/backend/router/publicfiles"

	"github.com/gin-gonic/gin"
)

const secretOutsideRoot = "TOP-SECRET-OUTSIDE-ROOT"

// setupPublicFilesRoot 在临时目录构造 ./files 公开根与根外诱饵文件，并把工作目录切过去。
func setupPublicFilesRoot(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	root := filepath.Join(tmp, "files")
	for _, rel := range []string{"avatar.png", "notes.txt", filepath.Join("sub", "a.bin")} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("payload:"+rel), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "secret.txt"), []byte(secretOutsideRoot), 0o644); err != nil {
		t.Fatalf("write decoy secret: %v", err)
	}
	t.Chdir(tmp)
}

// servePublicFilesRoute 以与 router_init.go 相同的顺序处理请求：
// ResolveRelativePath 校验 -> os.OpenRoot("./files") 打开 -> 仅下发常规文件。
func servePublicFilesRoute(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/files/*filepath", func(c *gin.Context) {
		relativePath, err := publicfiles.ResolveRelativePath(c.Param("filepath"))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		root, err := os.OpenRoot("./files")
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer root.Close()
		file, err := root.Open(filepath.FromSlash(relativePath))
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.DataFromReader(http.StatusOK, info.Size(), "application/octet-stream", file, nil)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestPublicFilesRouteRejectsTraversalVectors(t *testing.T) {
	setupPublicFilesRoot(t)

	cases := []struct {
		name   string
		target string
	}{
		{name: "empty filepath", target: "/files/"},
		{name: "dotdot segment", target: "/files/.."},
		{name: "parent escape", target: "/files/../secret.txt"},
		{name: "encoded slash after dotdot", target: "/files/..%2fsecret.txt"},
		{name: "fully encoded dotdot", target: "/files/%2e%2e%2fsecret.txt"},
		{name: "encoded dotdot segments", target: "/files/%2e%2e/%2e%2e/secret.txt"},
		{name: "mixed dotdot encoding", target: "/files/.%2e/secret.txt"},
		{name: "deep parent chain", target: "/files/sub/../../secret.txt"},
		{name: "backslash separator", target: "/files/..%5Csecret.txt"},
		{name: "encoded backslash mid path", target: "/files/sub%5C..%5Csecret.txt"},
		{name: "windows drive letter", target: "/files/C:/secret.txt"},
		{name: "encoded windows drive", target: "/files/C:%5Csecret.txt"},
		{name: "double slash prefix", target: "/files//secret.txt"},
		{name: "leading encoded slash", target: "/files/%2Fetc%2Fpasswd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := servePublicFilesRoute(t, tc.target)
			if rec.Code == http.StatusOK {
				t.Fatalf("GET %s = %d, want rejection", tc.target, rec.Code)
			}
			if strings.Contains(rec.Body.String(), secretOutsideRoot) {
				t.Fatalf("GET %s leaked root decoy content", tc.target)
			}
		})
	}
}

func TestPublicFilesRouteServesOnlyInsideRoot(t *testing.T) {
	setupPublicFilesRoot(t)

	cases := []struct {
		name     string
		target   string
		wantCode int
		wantBody string
	}{
		{name: "plain file", target: "/files/avatar.png", wantCode: http.StatusOK, wantBody: "payload:avatar.png"},
		{name: "dot normalization still served", target: "/files/./avatar.png", wantCode: http.StatusOK, wantBody: "payload:avatar.png"},
		{name: "nested file", target: "/files/sub/a.bin", wantCode: http.StatusOK, wantBody: "payload:" + filepath.Join("sub", "a.bin")},
		{name: "missing file is 404", target: "/files/absent.txt", wantCode: http.StatusNotFound},
		{name: "directory is not served", target: "/files/sub", wantCode: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := servePublicFilesRoute(t, tc.target)
			if rec.Code != tc.wantCode {
				t.Fatalf("GET %s = %d, want %d", tc.target, rec.Code, tc.wantCode)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Fatalf("GET %s body = %q, want %q", tc.target, rec.Body.String(), tc.wantBody)
			}
			if strings.Contains(rec.Body.String(), secretOutsideRoot) {
				t.Fatalf("GET %s leaked root decoy content", tc.target)
			}
		})
	}
}

// TestRouterContractKeepsFilesHandlerOnSafeResolver 用静态 AST 锁死 /files 路由必须走
// publicfiles.ResolveRelativePath + os.OpenRoot 组合，禁止退化为 c.File/http.ServeFile 直接服务。
func TestRouterContractKeepsFilesHandlerOnSafeResolver(t *testing.T) {
	parsed := parseRouterInit(t)

	var handlerBody *ast.BlockStmt
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "GET" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Value != `"/files/*filepath"` {
			return true
		}
		fn, ok := call.Args[1].(*ast.FuncLit)
		if !ok {
			t.Fatal("/files/*filepath must stay an inline closure over the safe resolver")
		}
		handlerBody = fn.Body
		return false
	})
	if handlerBody == nil {
		t.Fatal("router_init.go no longer registers GET /files/*filepath")
	}

	required := map[string]bool{
		"publicfiles.ResolveRelativePath": false,
		"os.OpenRoot":                     false,
	}
	ast.Inspect(handlerBody, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch name := selectorPath(call.Fun); name {
		case "publicfiles.ResolveRelativePath":
			required[name] = true
		case "os.OpenRoot":
			required[name] = true
		case "c.File", "c.FileAttachment", "c.FileFromFS", "http.ServeFile", "http.ServeContent":
			t.Fatalf("/files handler must not fall back to direct serving call %s", name)
		}
		return true
	})
	for name, found := range required {
		if !found {
			t.Fatalf("/files handler missing required call %s", name)
		}
	}
}
