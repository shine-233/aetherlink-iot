// 文件用途：SCADA 画布文档壳层（envelope）schema 契约——TP-22 大屏固定分辨率模式的
// 后端校验锚点。前端 owns 文档形状（views/scada/core/canvasDocument.ts schema v1），
// 本包只对"保存进库的画布必须满足的不变量"做恒生效校验，不解释节点内容。
// 核心逻辑：display_mode oneof（responsive|fixed1080）+ fixed1080 必须 1920×1080 的
// 尺寸契约；字段缺失一律回落 responsive——旧画布（无该字段）零改动通过，即向后兼容。
// 关键注意事项：
//  1. 双键兼容是刻意契约：canonical 键为 displayMode（与画布文档其余 camelCase 键
//     一致），display_mode 为别名（缺口任务书原文写法）。两键同时存在且值不同
//     必须报错——静默任选其一会让同一份文档在不同读方眼里是两种布局。
//  2. 未知 display_mode 值 fail-closed 拒绝，不做大小写折叠或近似匹配：
//     "FIXED1080" 这类值若被宽容收纳，前端将按 responsive 渲染，用户看到的
//     大屏与设计稿悄悄不一致，且无从追查。
//  3. schemaVersion > 1 的文档同样走本契约：新增显示模式时必须先扩展这里的
//     枚举，否则宁可拒绝保存也不能让"后端不认识的大屏模式"静默入库。
//
// 重构建议：若后续出现更多固定分辨率档位（如 fixed4k），把尺寸契约改成
// mode→(width,height) 的注册表结构，不要让 if 链增长。
package scadadoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// 画布文档 schema 版本（与前端 SCADA_CANVAS_SCHEMA_VERSION 对齐）。
const SchemaVersionV1 = 1

// 显示模式枚举（oneof）。
const (
	DisplayModeResponsive = "responsive"
	DisplayModeFixed1080  = "fixed1080"
)

// fixed1080 档位的设计分辨率：大屏按 1920×1080 设计，容器 transform scale 等比适配。
const (
	Fixed1080Width  = 1920.0
	Fixed1080Height = 1080.0
)

// 画布文档中的键名。
const (
	keySchemaVersion = "schemaVersion"
	keyDisplayMode   = "displayMode"
	keyDisplayModeSn = "display_mode"
	keyWidth         = "width"
	keyHeight        = "height"
	keyNodes         = "nodes"
	keyWidgets       = "widgets"
)

var (
	// ErrCanvasNotObject 画布载荷不是 JSON 对象。
	ErrCanvasNotObject = errors.New("scada canvas must be a JSON object")
	// ErrCanvasInvalidJSON 画布载荷不是合法 JSON。
	ErrCanvasInvalidJSON = errors.New("scada canvas is not valid JSON")
	// ErrDisplayModeUnknown display_mode 不在枚举内。
	ErrDisplayModeUnknown = fmt.Errorf("scada canvas display_mode must be one of [%s, %s]", DisplayModeResponsive, DisplayModeFixed1080)
	// ErrDisplayModeConflict 双键同时存在且值不同。
	ErrDisplayModeConflict = errors.New("scada canvas carries conflicting displayMode and display_mode values")
	// ErrSchemaVersionInvalid schemaVersion 非正整数。
	ErrSchemaVersionInvalid = errors.New("scada canvas schemaVersion must be a positive integer")
)

// CanvasDocument 画布壳层解析结果。只承载后端要校验/下游要读取的字段，
// 其余内容（nodes、props、extra…）保持不透明，不做有损归一化。
type CanvasDocument struct {
	SchemaVersion int
	// DisplayMode 归一化后的显示模式，恒为两个枚举值之一。
	DisplayMode string
	// Width/Height 画布设计尺寸；HasSize=false 表示文档未声明尺寸（旧画布）。
	Width   float64
	Height  float64
	HasSize bool
}

// IsAllowedDisplayMode 判断显示模式是否在枚举内。
func IsAllowedDisplayMode(mode string) bool {
	return mode == DisplayModeResponsive || mode == DisplayModeFixed1080
}

// jsonNumberAsFloat 把 JSON 解码出的数字安全转换为 float64。
// json.Number 场景下 UseNumber 已关闭，这里只做有限性与整数判定。
func jsonNumberAsFloat(raw json.RawMessage) (float64, bool) {
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// parseDisplayMode 提取归一化后的显示模式：缺省 responsive，双键冲突报错。
func parseDisplayMode(probe map[string]json.RawMessage) (string, error) {
	camel, hasCamel := probe[keyDisplayMode]
	snake, hasSnake := probe[keyDisplayModeSn]
	if !hasCamel && !hasSnake {
		// 字段缺失 = 旧画布：回落 responsive，这就是向后兼容的实现点。
		return DisplayModeResponsive, nil
	}
	value := ""
	if hasCamel {
		if err := json.Unmarshal(camel, &value); err != nil {
			return "", fmt.Errorf("scada canvas %s must be a string", keyDisplayMode)
		}
	}
	if hasSnake {
		snakeValue := ""
		if err := json.Unmarshal(snake, &snakeValue); err != nil {
			return "", fmt.Errorf("scada canvas %s must be a string", keyDisplayModeSn)
		}
		if hasCamel && snakeValue != value {
			return "", ErrDisplayModeConflict
		}
		value = snakeValue
	}
	// 不做 TrimSpace / 大小写折叠：前端序列化只会产出精确枚举值，
	// 带空格或大小写不同的值一律是坏文档，静默归一会让坏文档看起来合法。
	if !IsAllowedDisplayMode(value) {
		return "", ErrDisplayModeUnknown
	}
	return value, nil
}

// parsePositiveSize 提取正数尺寸；键缺失返回 ok=false 且不报错（旧画布可无尺寸）。
func parsePositiveSize(probe map[string]json.RawMessage, key string) (float64, bool, error) {
	raw, ok := probe[key]
	if !ok {
		return 0, false, nil
	}
	v, valid := jsonNumberAsFloat(raw)
	if !valid || v <= 0 {
		return 0, false, fmt.Errorf("scada canvas %s must be a positive finite number", key)
	}
	return v, true, nil
}

// ParseCanvasDocument 解析并校验画布壳层。
//
// 空串按"空画布"处理（与服务层 normalizeCanvas 的 "{}" 归一化口径一致）；
// 其余载荷必须是 JSON 对象。校验失败返回带字段定位的错误，由服务层映射成参数错误。
func ParseCanvasDocument(raw string) (*CanvasDocument, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return &CanvasDocument{SchemaVersion: SchemaVersionV1, DisplayMode: DisplayModeResponsive}, nil
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		// 区分"根本不是 JSON"与"是 JSON 但不是对象"：两者的修复动作不同，
		// 前者找序列化 bug，后者找载荷来源。
		var asAny interface{}
		if secondErr := json.Unmarshal([]byte(trimmed), &asAny); secondErr == nil {
			return nil, ErrCanvasNotObject
		}
		return nil, fmt.Errorf("%w: %v", ErrCanvasInvalidJSON, err)
	}
	if probe == nil {
		return nil, ErrCanvasNotObject
	}

	doc := &CanvasDocument{SchemaVersion: SchemaVersionV1, DisplayMode: DisplayModeResponsive}

	if rawVersion, ok := probe[keySchemaVersion]; ok {
		version, valid := jsonNumberAsFloat(rawVersion)
		if !valid || version != math.Trunc(version) || version < 1 {
			return nil, ErrSchemaVersionInvalid
		}
		doc.SchemaVersion = int(version)
	}

	mode, err := parseDisplayMode(probe)
	if err != nil {
		return nil, err
	}
	doc.DisplayMode = mode

	width, hasWidth, err := parsePositiveSize(probe, keyWidth)
	if err != nil {
		return nil, err
	}
	height, hasHeight, err := parsePositiveSize(probe, keyHeight)
	if err != nil {
		return nil, err
	}
	if hasWidth && hasHeight {
		doc.Width, doc.Height, doc.HasSize = width, height, true
	}

	// fixed1080 的尺寸契约：声明了该模式就必须按 1920×1080 设计。
	// 只有一半尺寸同样拒绝——半缺不是"另一维自适应"，而是一份写坏的文档。
	if mode == DisplayModeFixed1080 {
		if !hasWidth || !hasHeight {
			return nil, fmt.Errorf("scada canvas with display_mode=%s must declare width=%g and height=%g", DisplayModeFixed1080, Fixed1080Width, Fixed1080Height)
		}
		if width != Fixed1080Width || height != Fixed1080Height {
			return nil, fmt.Errorf("scada canvas with display_mode=%s must be %gx%g, got %gx%g", DisplayModeFixed1080, Fixed1080Width, Fixed1080Height, width, height)
		}
	}

	// 节点数组类型底线：存在但不是数组是一份坏文档，不是"没有节点"。
	// Widget 配置级校验由注册表负责（可能未接线），这里的结构底线必须恒生效。
	for _, key := range []string{keyNodes, keyWidgets} {
		if rawNodes, ok := probe[key]; ok {
			var arr []json.RawMessage
			if err := json.Unmarshal(rawNodes, &arr); err != nil {
				return nil, fmt.Errorf("scada canvas %s must be an array", key)
			}
		}
	}
	return doc, nil
}
