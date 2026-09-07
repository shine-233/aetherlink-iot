// 文件用途：OpenAPI 3.0 规范生成器（PHASE-D-D8a）。
// 核心逻辑：遍历 gin 路由表（经 RouterInit 或测试引擎构造），产出稳定排序的
//
//	openapi.json——路径字典序、方法按固定次序、无时间戳噪声，可 diff 可快照。
//
// 关键注意事项：schema 为 MVP 推断层（object+说明），动态路径参数 :name 映射为
// path 参数；鉴权契约 = Bearer JWT 或 OpenAPI Key（X-API-Key，出处 internal/middleware/cors.go:20）。
package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// methodOrder 方法固定次序（稳定输出）。
var methodOrder = map[string]int{"GET": 0, "POST": 1, "PUT": 2, "DELETE": 3, "PATCH": 4}

// Generate 从 gin 路由表生成 OpenAPI 3.0 文档（map 形式，调用方负责序列化）。
func Generate(routes []gin.RouteInfo) map[string]any {
	paths := map[string]map[string]any{}
	for _, route := range routes {
		if route.Path == "" {
			continue
		}
		method := strings.ToLower(route.Method)
		if _, ok := methodOrder[strings.ToUpper(method)]; !ok {
			continue // HEAD/OPTIONS 等不入契约
		}
		entry, exists := paths[route.Path]
		if !exists {
			entry = map[string]any{}
			paths[route.Path] = entry
		}
		entry[method] = operation(route)
	}

	sortedPaths := make([]string, 0, len(paths))
	for path := range paths {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	pathsOut := map[string]any{}
	for _, path := range sortedPaths {
		methods := paths[path]
		pathItem := map[string]any{}
		// path 参数对所有方法一致，挂到 path-item 层。
		if params := pathParams(path); len(params) > 0 {
			pathItem["parameters"] = params
		}
		methodNames := make([]string, 0, len(methods))
		for method := range methods {
			methodNames = append(methodNames, method)
		}
		sort.Slice(methodNames, func(i, j int) bool {
			return methodOrder[methodNames[i]] < methodOrder[methodNames[j]]
		})
		for _, method := range methodNames {
			pathItem[method] = methods[method]
		}
		pathsOut[path] = pathItem
	}

	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "AetherLink IoT Platform API",
			"description": "由 cmd/openapigen 从 gin 路由表自动生成;schema 为推断骨架,动态参数以 :name 声明。",
			"version":     "phase-d-d8a",
		},
		"servers": []any{map[string]any{"url": "/"}},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"BearerAuth": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
				"ApiKeyAuth": map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"},
			},
		},
		"paths": pathsOut,
	}
}

// operation 单操作描述:tag 取模块段;受保护路由声明双鉴权方案。
func operation(route gin.RouteInfo) map[string]any {
	op := map[string]any{
		"tags":      []string{tagFor(route.Path)},
		"summary":   handlerSummary(route.Handler),
		"responses": map[string]any{"200": map[string]any{"description": "OK(响应包络见平台 response 中间件)"}},
	}
	if strings.HasPrefix(route.Path, "/api/v1/") {
		op["security"] = []any{
			map[string]any{"BearerAuth": []string{}},
			map[string]any{"ApiKeyAuth": []string{}},
		}
	}
	return op
}

// tagFor 从路径取模块段(去掉 api/v1 前缀后的第一段;无前缀归 platform)。
func tagFor(path string) string {
	trimmed := strings.Trim(path, "/")
	trimmed = strings.TrimPrefix(trimmed, "api/v1/")
	if trimmed == "" || trimmed == "api/v1" {
		return "platform"
	}
	segment := trimmed
	if idx := strings.Index(segment, "/"); idx >= 0 {
		segment = segment[:idx]
	}
	return segment
}

// pathParams 解析 :name 动态段为 OpenAPI path 参数。
func pathParams(path string) []map[string]any {
	params := []map[string]any{}
	for _, segment := range strings.Split(path, "/") {
		if strings.HasPrefix(segment, ":") {
			name := strings.TrimPrefix(segment, ":")
			params = append(params, map[string]any{
				"name":     name,
				"in":       "path",
				"required": true,
				"schema":   map[string]any{"type": "string"},
			})
		}
	}
	return params
}

// handlerSummary 由 gin 注册的 handler 名生成摘要(包名.函数名,去噪音)。
func handlerSummary(handler string) string {
	if idx := strings.LastIndex(handler, "."); idx >= 0 && idx+1 < len(handler) {
		return fmt.Sprintf("%s (%s)", handler[idx+1:], handler[:idx])
	}
	return handler
}

// Serialize 稳定序列化(Go map JSON 序列化键自动字典序 + 禁 HTML 转义 + 固定缩进)。
func Serialize(doc map[string]any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}
