// 文件用途：覆盖API 路由契约测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
)

func parseAPIFile(t *testing.T, file string) *ast.File {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	return parsed
}

func requireAPIMethods(t *testing.T, file string, receiver string, methods ...string) {
	t.Helper()
	parsed := parseAPIFile(t, file)
	found := map[string]bool{}

	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		if receiverName(fn.Recv.List[0].Type) == receiver {
			found[fn.Name.Name] = true
		}
	}

	for _, method := range methods {
		if !found[method] {
			t.Fatalf("%s missing method %s.%s", file, receiver, method)
		}
	}
}

func requireAPIIdentifiers(t *testing.T, file string, identifiers ...string) {
	t.Helper()
	parsed := parseAPIFile(t, file)
	found := map[string]bool{}

	ast.Inspect(parsed, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.Ident:
			found[expr.Name] = true
		case *ast.SelectorExpr:
			found[expr.Sel.Name] = true
		}
		return true
	})

	for _, identifier := range identifiers {
		if !found[identifier] {
			t.Fatalf("%s missing identifier %s", file, identifier)
		}
	}
}

func requireAPIStringLiterals(t *testing.T, file string, literals ...string) {
	t.Helper()
	parsed := parseAPIFile(t, file)
	found := map[string]bool{}

	ast.Inspect(parsed, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil {
			found[value] = true
		}
		return true
	})

	for _, literal := range literals {
		if !found[literal] {
			t.Fatalf("%s missing string literal %q", file, literal)
		}
	}
}

func receiverName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}

func TestAPIRouterContractKeepsP0P1ControllerFields(t *testing.T) {
	controllerType := reflect.TypeOf(Controller{})
	fields := map[string]bool{}
	for i := 0; i < controllerType.NumField(); i++ {
		fields[controllerType.Field(i).Name] = true
	}

	for _, field := range []string{
		"DeviceApi",
		"TelemetryDataApi",
		"RDIApi",
		"AlarmApi",
		"NotificationGroupApi",
		"NotificationHistoryApi",
		"RoleApi",
		"CasbinApi",
		"SceneApi",
		"SceneAutomationsApi",
		"OpenAPIKeyApi",
		"ServicePluginApi",
		"SystemApi",
	} {
		if !fields[field] {
			t.Fatalf("Controller missing P0/P1 API field %s", field)
		}
	}
}

// This source-structure contract does not execute an HTTP route or prove
// authentication, status-code, response-body, service, or persistence behavior.
func TestOTASourceStructureContractDeclaresPackageTaskDownloadAndRangeHelpers(t *testing.T) {
	requireAPIMethods(t, "ota.go", "OTAApi",
		"CreateOTAUpgradePackage",
		"DeleteOTAUpgradePackage",
		"UpdateOTAUpgradePackage",
		"HandleOTAUpgradePackageByPage",
		"CreateOTAUpgradeTask",
		"PreviewOTAUpgradeTask",
		"DeleteOTAUpgradeTask",
		"HandleOTAUpgradeTaskByPage",
		"HandleOTAUpgradeTaskDetailByPage",
		"GetOTAUpgradeTaskSupportBundle",
		"UpdateOTAUpgradeTaskStatus",
		"DownloadOTAUpgradePackage",
	)
	requireAPIIdentifiers(t, "ota.go",
		"safeOTAUpgradePackagePath",
		"serveRangeFile",
		"parseByteRange",
		"rangeCRC16",
		"crc16Digest",
		"CopyN",
		"Seek",
		"AbortWithStatus",
		"AbortWithError",
	)
	requireAPIStringLiterals(t, "ota.go",
		"Range",
		"Crc16-Method",
		"Content-Range",
		"Accept-Ranges",
		"Content-Length",
		"Content-Type",
		"X-CRC16",
		"bytes=",
		"CCITT",
		"MODBUS",
	)
}

func TestOTACreatePackageHandlerRejectsMissingName(t *testing.T) {
	status, got := performAPIValidationRequest(
		t,
		http.MethodPost,
		"/api/v1/ota/package",
		`{}`,
		(&OTAApi{}).CreateOTAUpgradePackage,
	)

	if status != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", status, http.StatusOK)
	}
	if got.Code != errcode.CodeParamError {
		t.Fatalf("response code = %d, want %d", got.Code, errcode.CodeParamError)
	}
	const wantMessage = "Field 'Name' is required"
	if got.Message != wantMessage {
		t.Fatalf("response message = %q, want %q", got.Message, wantMessage)
	}
	if got.Data != nil {
		t.Fatalf("response data = %#v, want omitted", got.Data)
	}
}

func TestOTAParseByteRangeKeepsSupportedAndRejectedForms(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		fileSize  int64
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{name: "bounded range", header: "bytes=2-5", fileSize: 10, wantStart: 2, wantEnd: 5},
		{name: "open ended range", header: "bytes=7-", fileSize: 10, wantStart: 7, wantEnd: 9},
		{name: "missing bytes prefix", header: "2-5", fileSize: 10, wantErr: true},
		{name: "suffix range remains unsupported", header: "bytes=-5", fileSize: 10, wantErr: true},
		{name: "range past file end", header: "bytes=9-10", fileSize: 10, wantErr: true},
		{name: "start after end", header: "bytes=5-2", fileSize: 10, wantErr: true},
		{name: "empty file", header: "bytes=0-", fileSize: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseByteRange(tt.header, tt.fileSize)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseByteRange(%q, %d) returned nil error", tt.header, tt.fileSize)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseByteRange(%q, %d) returned error: %v", tt.header, tt.fileSize, err)
			}
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("parseByteRange(%q, %d) = (%d, %d), want (%d, %d)", tt.header, tt.fileSize, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
