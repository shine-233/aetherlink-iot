// 文件用途:OpenAPI 契约生成器 CLI(PHASE-D-D8a)。
// 核心逻辑:构建完整 gin 路由表(复用 router.RouterInit,无 DB 依赖——路由注册不触库),
// 遍历生成 openapi.json 并写盘;输出可 diff(稳定排序,无时间戳)。
// 运行:go run ./cmd/openapigen -out ../docs/openapi/openapi.json
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"aetherlink-iot/backend/internal/openapi"
	"aetherlink-iot/backend/router"
)

func main() {
	out := flag.String("out", "docs/openapi/openapi.json", "输出路径(相对 backend/)")
	flag.Parse()

	engine := router.RouterInit()
	doc := openapi.Generate(engine.Routes())

	raw, err := openapi.Serialize(doc)
	if err != nil {
		fatal("serialize: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal("mkdir: %v", err)
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		fatal("write: %v", err)
	}

	paths, _ := doc["paths"].(map[string]any)
	fmt.Printf("openapi.json written: %s (paths=%d)\n", *out, len(paths))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "openapigen: "+format+"\n", args...)
	os.Exit(1)
}
