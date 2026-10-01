// 文件用途：scadadoc 画布壳层契约的单元测试——TP-22 display_mode oneof 与
// fixed1080 尺寸契约的行为锁。
// 核心逻辑：表驱动覆盖"缺省回落 responsive（向后兼容）""双键等价/冲突""枚举
// fail-closed""尺寸底线"四组行为，另设旧版 widgets[] 画布回归样例。
// 关键注意事项：这里的每条拒绝路径都对应一种"前端按一种布局画、后端存成另
// 一种布局"的静默错位风险；放宽任何一条前先确认前端渲染语义已同步。
// 重构建议：新增固定分辨率档位时，直接在用例表里加档位行，保持表驱动结构。
package scadadoc

import (
	"errors"
	"strings"
	"testing"
)

// mustParse 解析成功则返回文档，失败则让用例失败。
func mustParse(t *testing.T, raw string) *CanvasDocument {
	t.Helper()
	doc, err := ParseCanvasDocument(raw)
	if err != nil {
		t.Fatalf("ParseCanvasDocument(%s) unexpected error: %v", raw, err)
	}
	return doc
}

// TestParseDefaultsBackwardCompatible 锁定向后兼容：旧画布（无 display_mode、
// 无 schemaVersion、旧版 widgets[] 形状）零改动通过，且一律回落 responsive。
func TestParseDefaultsBackwardCompatible(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"空串", ""},
		{"空白串", "   "},
		{"空对象", `{}`},
		{"标准 v1 画布", `{"schemaVersion":1,"width":1280,"height":720,"nodes":[]}`},
		{"旧版 widgets 形状", `{"width":1280,"height":720,"widgets":[{"id":"w1","widget_type":"gauge","version":"1.0","config":{}}]}`},
		{"未知未来字段", `{"schemaVersion":1,"width":1920,"height":1080,"nodes":[],"extra":{"future":true}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.raw)
			if doc.DisplayMode != DisplayModeResponsive {
				t.Fatalf("DisplayMode = %q, want %q (backward-compat default)", doc.DisplayMode, DisplayModeResponsive)
			}
			if doc.SchemaVersion != SchemaVersionV1 {
				t.Fatalf("SchemaVersion = %d, want %d", doc.SchemaVersion, SchemaVersionV1)
			}
		})
	}
}

// TestParseDisplayModeEnum 锁定 oneof：camelCase 键、snake_case 别名、
// 大小写敏感、未知值拒绝。
func TestParseDisplayModeEnum(t *testing.T) {
	t.Run("camelCase canonical", func(t *testing.T) {
		doc := mustParse(t, `{"displayMode":"fixed1080","width":1920,"height":1080}`)
		if doc.DisplayMode != DisplayModeFixed1080 {
			t.Fatalf("DisplayMode = %q, want fixed1080", doc.DisplayMode)
		}
	})
	t.Run("snake_case alias", func(t *testing.T) {
		doc := mustParse(t, `{"display_mode":"fixed1080","width":1920,"height":1080}`)
		if doc.DisplayMode != DisplayModeFixed1080 {
			t.Fatalf("DisplayMode = %q, want fixed1080", doc.DisplayMode)
		}
	})
	t.Run("双键同值等价", func(t *testing.T) {
		doc := mustParse(t, `{"displayMode":"responsive","display_mode":"responsive"}`)
		if doc.DisplayMode != DisplayModeResponsive {
			t.Fatalf("DisplayMode = %q, want responsive", doc.DisplayMode)
		}
	})
	t.Run("双键冲突拒绝", func(t *testing.T) {
		_, err := ParseCanvasDocument(`{"displayMode":"responsive","display_mode":"fixed1080","width":1920,"height":1080}`)
		if !errors.Is(err, ErrDisplayModeConflict) {
			t.Fatalf("err = %v, want ErrDisplayModeConflict", err)
		}
	})
	t.Run("未知值拒绝", func(t *testing.T) {
		for _, bad := range []string{"FIXED1080", "Fixed1080", "fixed4k", "", " responsive"} {
			_, err := ParseCanvasDocument(`{"displayMode":` + quote(bad) + `}`)
			if !errors.Is(err, ErrDisplayModeUnknown) {
				t.Fatalf("displayMode %q: err = %v, want ErrDisplayModeUnknown", bad, err)
			}
		}
	})
	t.Run("非字符串值拒绝", func(t *testing.T) {
		if _, err := ParseCanvasDocument(`{"displayMode":1}`); err == nil || !strings.Contains(err.Error(), "must be a string") {
			t.Fatalf("err = %v, want string-type error", err)
		}
	})
}

// quote 生成带引号的 JSON 字符串字面量，避免测试里手写转义。
func quote(s string) string {
	return "\"" + s + "\""
}

// TestParseFixed1080SizeContract 锁定 fixed1080 的 1920×1080 尺寸契约。
func TestParseFixed1080SizeContract(t *testing.T) {
	t.Run("1920x1080 通过", func(t *testing.T) {
		doc := mustParse(t, `{"display_mode":"fixed1080","width":1920,"height":1080,"nodes":[]}`)
		if !doc.HasSize || doc.Width != Fixed1080Width || doc.Height != Fixed1080Height {
			t.Fatalf("doc = %+v, want 1920x1080", doc)
		}
	})
	t.Run("缺尺寸拒绝", func(t *testing.T) {
		_, err := ParseCanvasDocument(`{"display_mode":"fixed1080","nodes":[]}`)
		if err == nil || !strings.Contains(err.Error(), "must declare width") {
			t.Fatalf("err = %v, want missing-size error", err)
		}
	})
	t.Run("只给一半尺寸拒绝", func(t *testing.T) {
		if _, err := ParseCanvasDocument(`{"display_mode":"fixed1080","width":1920}`); err == nil {
			t.Fatal("half-declared size must be rejected")
		}
	})
	t.Run("其他尺寸拒绝", func(t *testing.T) {
		for _, raw := range []string{
			`{"display_mode":"fixed1080","width":1280,"height":720}`,
			`{"display_mode":"fixed1080","width":1920,"height":1920}`,
			`{"display_mode":"fixed1080","width":1920.5,"height":1080}`,
			`{"display_mode":"fixed1080","width":0,"height":0}`,
		} {
			if _, err := ParseCanvasDocument(raw); err == nil {
				t.Fatalf("%s must be rejected", raw)
			}
		}
	})
	t.Run("responsive 不受尺寸契约约束", func(t *testing.T) {
		doc := mustParse(t, `{"display_mode":"responsive","width":3840,"height":2160}`)
		if doc.DisplayMode != DisplayModeResponsive || doc.Width != 3840 {
			t.Fatalf("doc = %+v, want responsive 3840x2160", doc)
		}
	})
}

// TestParseStructuralFloor 锁定壳层结构底线：对象、正整数 schemaVersion、
// 节点数组类型。
func TestParseStructuralFloor(t *testing.T) {
	t.Run("数组载荷拒绝", func(t *testing.T) {
		if _, err := ParseCanvasDocument(`[1,2,3]`); !errors.Is(err, ErrCanvasNotObject) {
			t.Fatalf("err = %v, want ErrCanvasNotObject", err)
		}
	})
	t.Run("null 载荷拒绝", func(t *testing.T) {
		if _, err := ParseCanvasDocument(`null`); !errors.Is(err, ErrCanvasNotObject) {
			t.Fatalf("err = %v, want ErrCanvasNotObject", err)
		}
	})
	t.Run("坏 JSON 拒绝", func(t *testing.T) {
		if _, err := ParseCanvasDocument(`{"width":`); !errors.Is(err, ErrCanvasInvalidJSON) {
			t.Fatalf("err = %v, want ErrCanvasInvalidJSON", err)
		}
	})
	t.Run("schemaVersion 底线", func(t *testing.T) {
		for _, raw := range []string{`{"schemaVersion":0}`, `{"schemaVersion":-1}`, `{"schemaVersion":1.5}`, `{"schemaVersion":"1"}`} {
			if _, err := ParseCanvasDocument(raw); !errors.Is(err, ErrSchemaVersionInvalid) {
				t.Fatalf("%s: err = %v, want ErrSchemaVersionInvalid", raw, err)
			}
		}
		if doc := mustParse(t, `{"schemaVersion":2,"width":1920,"height":1080}`); doc.SchemaVersion != 2 {
			t.Fatalf("SchemaVersion = %d, want 2", doc.SchemaVersion)
		}
	})
	t.Run("尺寸必须为正数", func(t *testing.T) {
		for _, raw := range []string{`{"width":0,"height":720}`, `{"width":1280,"height":-1}`, `{"width":"big","height":720}`} {
			if _, err := ParseCanvasDocument(raw); err == nil || !strings.Contains(err.Error(), "positive finite number") {
				t.Fatalf("%s: err = %v, want positive-size error", raw, err)
			}
		}
	})
	t.Run("节点字段必须是数组", func(t *testing.T) {
		if _, err := ParseCanvasDocument(`{"nodes":{}}`); err == nil || !strings.Contains(err.Error(), "nodes must be an array") {
			t.Fatalf("err = %v, want nodes-array error", err)
		}
		if _, err := ParseCanvasDocument(`{"widgets":"x"}`); err == nil || !strings.Contains(err.Error(), "widgets must be an array") {
			t.Fatalf("err = %v, want widgets-array error", err)
		}
	})
	t.Run("只有一半尺寸的 responsive 记为无尺寸", func(t *testing.T) {
		// responsive 模式下宽度缺失不算错（旧画布可能只有高度），
		// 但 HasSize 必须为 false，不能假装拿到了完整设计尺寸。
		doc := mustParse(t, `{"height":720}`)
		if doc.HasSize {
			t.Fatalf("HasSize = true, want false for partial size")
		}
	})
}

// TestIsAllowedDisplayMode 枚举判定函数的行为锁。
func TestIsAllowedDisplayMode(t *testing.T) {
	if !IsAllowedDisplayMode(DisplayModeResponsive) || !IsAllowedDisplayMode(DisplayModeFixed1080) {
		t.Fatal("enum members must be allowed")
	}
	for _, bad := range []string{"", "FIXED1080", "fixed4k"} {
		if IsAllowedDisplayMode(bad) {
			t.Fatalf("%q must not be allowed", bad)
		}
	}
}
