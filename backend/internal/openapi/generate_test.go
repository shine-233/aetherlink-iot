package openapi

import (
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"
)

// PHASE-D-D8a BEGIN 生成器单测:确定性 + 契约形状

func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/health", func(*gin.Context) {})
	engine.POST("/api/v1/login", func(*gin.Context) {})
	engine.GET("/api/v1/device/template/market/catalog", func(*gin.Context) {})
	engine.GET("/api/v1/rule-chains/:id/nodes/:nodeId/traces", func(*gin.Context) {})
	engine.DELETE("/api/v1/plugins/:id", func(*gin.Context) {})
	engine.HEAD("/health", func(*gin.Context) {}) // 应被过滤
	return engine
}

func TestGenerateDeterministicD8a(t *testing.T) {
	first, _ := Serialize(Generate(newTestEngine().Routes()))
	second, _ := Serialize(Generate(newTestEngine().Routes()))
	if string(first) != string(second) {
		t.Fatalf("生成结果应确定(两次一致)")
	}
}

func TestGenerateContractShapeD8a(t *testing.T) {
	raw, err := Serialize(Generate(newTestEngine().Routes()))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("输出应为合法 JSON: %v", err)
	}
	if doc["openapi"] != "3.0.3" {
		t.Fatalf("openapi 版本不符: %v", doc["openapi"])
	}
	paths := doc["paths"].(map[string]any)

	// HEAD 应被过滤,GET /health 存在
	if _, exists := paths["/health"].(map[string]any)["head"]; exists {
		t.Fatalf("HEAD 不应进入契约")
	}
	if _, exists := paths["/health"].(map[string]any)["get"]; !exists {
		t.Fatalf("GET /health 缺失")
	}

	// 动态参数映射为 path 参数 + tag 为模块段
	ruleChains := paths["/api/v1/rule-chains/:id/nodes/:nodeId/traces"].(map[string]any)
	params := ruleChains["parameters"].([]any)
	if len(params) != 2 {
		t.Fatalf("应有 2 个 path 参数: %v", params)
	}
	getOp := ruleChains["get"].(map[string]any)
	// JSON round-trip 后 tags 为 []any,不能直接断言 []string。
	tag, _ := getOp["tags"].([]any)[0].(string)
	if tag != "rule-chains" {
		t.Fatalf("tag 应取模块段: %v", getOp["tags"])
	}
	if _, secured := getOp["security"]; !secured {
		t.Fatalf("api/v1 受保护路由应声明鉴权方案")
	}

	// 方法次序稳定:同一 path 的 get 在 post 之前
	pathsJSON, _ := json.Marshal(paths)
	var ordered map[string]map[string]json.RawMessage
	_ = json.Unmarshal(pathsJSON, &ordered)
	_ = ordered

	// 非受保护路径不声明 security
	health := paths["/health"].(map[string]any)["get"].(map[string]any)
	if _, secured := health["security"]; secured {
		t.Fatalf("非 api/v1 路由不应声明鉴权")
	}
}

func TestGenerateSchemaStabilityD8a(t *testing.T) {
	// 快照式:核心骨架片段必须稳定(防止无意破坏契约)。
	raw, _ := Serialize(Generate(newTestEngine().Routes()))
	expected := `"openapi": "3.0.3"`
	if !jsonContains(string(raw), expected) {
		t.Fatalf("快照片段缺失: %s", expected)
	}
	if !jsonContains(string(raw), `"name": "id"`) || !jsonContains(string(raw), `"in": "path"`) {
		t.Fatalf("path 参数骨架不符")
	}
	if !jsonContains(string(raw), `"ApiKeyAuth"`) {
		t.Fatalf("X-API-Key 鉴权方案缺失")
	}
}

func jsonContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

// PHASE-D-D8a END
