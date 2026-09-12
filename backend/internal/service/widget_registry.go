// 文件用途：Widget 注册表与渲染能力降级（ROADMAP P1.3）。
// 核心逻辑：按 (type, version) 注册 Widget 定义与命令，并按运行环境解析可渲染集合。
//
// 关键注意事项（门禁"3D/WebGL 降级不影响 2D 看板"落在这一层）：
//  1. 能力必须显式声明，注册时拒绝空能力。空能力不是"默认 2D"，而是"没想清楚"——
//     静默按 2D 处理会让一个其实依赖 WebGL 的 Widget 在不支持的环境里渲染成
//     一块空白，界面上完全看不出它坏掉了。
//  2. 降级是**逐个 Widget** 的，绝不因某个 Widget 不可用而让整块看板加载失败。
//     解析函数永不因"有 Widget 用不了"返回错误，只把结果分成可用/降级/未注册三类。
//  3. 未注册的 Widget 类型同样不阻断加载：画布可能比注册表活得更久
//     （Widget 下线了但历史画布还在）。此时返回 unknown 让前端渲染占位符，
//     而不是整块看板打不开——后者等于用一次组件下线毁掉用户所有历史画布。
//  4. 命令是否需二次确认由注册声明决定（RequiresConfirmation），
//     不在调用点各自判断，避免"同一个危险命令在不同画布上确认要求不一致"。
package service

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// 渲染能力。
const (
	// WidgetCapability2D 基础 2D 渲染，所有环境都支持。
	WidgetCapability2D = "2d"
	// WidgetCapability3D 依赖 WebGL 的三维渲染，环境不支持时必须降级。
	WidgetCapability3D = "3d"
)

// WidgetCapabilityWebGL 需要 WebGL 才能工作的能力集合。
var widgetCapabilityWebGL = map[string]bool{
	WidgetCapability3D: true,
}

// 大小上限。
const (
	widgetMaxTypeLen      = 64
	widgetMaxVersionLen   = 32
	widgetMaxSchemaBytes  = 256 * 1024
	widgetMaxCommandLen    = 64
	widgetMaxCommands      = 32
)

var (
	ErrWidgetNilDefinition  = errors.New("widget definition is nil")
	ErrWidgetMissingType    = errors.New("widget requires a type")
	ErrWidgetMissingVersion = errors.New("widget requires a version")
	ErrWidgetMissingSchema  = errors.New("widget requires a JSON schema")
	ErrWidgetBadSchema      = errors.New("widget schema must be a JSON object")
	ErrWidgetNoCapability   = errors.New("widget must declare at least one render capability")
	ErrWidgetBadCapability  = errors.New("widget capability is not allowed")
	ErrWidgetBadCommand     = errors.New("widget command is invalid")
	ErrWidgetTooManyCommand = errors.New("widget declares too many commands")
	ErrWidgetDuplicateCmd   = errors.New("widget declares duplicate command names")
	ErrWidgetTypeTooLong    = errors.New("widget type is too long")
	ErrWidgetVersionTooLong = errors.New("widget version is too long")
	ErrWidgetSchemaTooLarge = errors.New("widget schema is too large")
)

// CommandDefinition Widget 暴露的控制命令。
type CommandDefinition struct {
	// Name 命令名，同一 Widget 内唯一。
	Name string `json:"name"`
	// RequiresConfirmation 是否要求显式二次确认。危险操作（下发控制、写影子）
	// 必须为 true；由注册声明而非调用点决定，保证同一命令在不同画布上要求一致。
	RequiresConfirmation bool `json:"requires_confirmation"`
	// ParamsSchema 参数 JSON Schema（字符串形式）。
	ParamsSchema string `json:"params_schema,omitempty"`
}

// WidgetDefinition Widget 注册定义。
type WidgetDefinition struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	// Schema 配置 JSON Schema（JSONSchema7 序列化后的字符串）。
	Schema string `json:"schema"`
	// Capabilities 渲染能力，至少一项。含 3d 且环境无 WebGL 时降级。
	Capabilities []string `json:"capabilities"`
	Commands     []CommandDefinition `json:"commands,omitempty"`
}

// WidgetInstance 画布中引用某个 Widget 的实例。
type WidgetInstance struct {
	ID         string `json:"id"`
	WidgetType string `json:"widget_type"`
	Version    string `json:"version"`
}

// RenderEnvironment 客户端渲染环境。
type RenderEnvironment struct {
	// WebGLAvailable 是否可用 WebGL。false 时所有 3d Widget 降级。
	WebGLAvailable bool
}

// WidgetResolution 一次画布解析的结果。
type WidgetResolution struct {
	// Available 环境可直接渲染的 Widget。
	Available []WidgetDefinition `json:"available"`
	// Degraded 因环境能力不足而降级的 Widget（前端应渲染占位并提示）。
	Degraded []WidgetDefinition `json:"degraded"`
	// Unknown 注册表中不存在的 Widget（前端应渲染占位符，不阻断看板）。
	Unknown []WidgetInstance `json:"unknown"`
}

// ValidateWidgetDefinition 校验注册定义。
func ValidateWidgetDefinition(d *WidgetDefinition) error {
	if d == nil {
		return ErrWidgetNilDefinition
	}
	d.Type = strings.TrimSpace(d.Type)
	d.Version = strings.TrimSpace(d.Version)
	if d.Type == "" {
		return ErrWidgetMissingType
	}
	if len(d.Type) > widgetMaxTypeLen {
		return ErrWidgetTypeTooLong
	}
	if d.Version == "" {
		return ErrWidgetMissingVersion
	}
	if len(d.Version) > widgetMaxVersionLen {
		return ErrWidgetVersionTooLong
	}
	if strings.TrimSpace(d.Schema) == "" {
		return ErrWidgetMissingSchema
	}
	if len(d.Schema) > widgetMaxSchemaBytes {
		return ErrWidgetSchemaTooLarge
	}
	if !strings.HasPrefix(strings.TrimSpace(d.Schema), "{") {
		return ErrWidgetBadSchema
	}
	if len(d.Capabilities) == 0 {
		return ErrWidgetNoCapability
	}
	for _, c := range d.Capabilities {
		if c != WidgetCapability2D && c != WidgetCapability3D {
			return ErrWidgetBadCapability
		}
	}
	if len(d.Commands) > widgetMaxCommands {
		return ErrWidgetTooManyCommand
	}
	seen := make(map[string]bool, len(d.Commands))
	for _, cmd := range d.Commands {
		name := strings.TrimSpace(cmd.Name)
		if name == "" || len(name) > widgetMaxCommandLen {
			return ErrWidgetBadCommand
		}
		if seen[name] {
			return ErrWidgetDuplicateCmd
		}
		seen[name] = true
	}
	return nil
}

// RequiresWebGL 判断该 Widget 是否依赖 WebGL。
func (d *WidgetDefinition) RequiresWebGL() bool {
	if d == nil {
		return false
	}
	for _, c := range d.Capabilities {
		if widgetCapabilityWebGL[c] {
			return true
		}
	}
	return false
}

// FindCommand 查找命令定义。未找到返回 nil。
func (d *WidgetDefinition) FindCommand(name string) *CommandDefinition {
	if d == nil {
		return nil
	}
	target := strings.TrimSpace(name)
	for i := range d.Commands {
		if strings.TrimSpace(d.Commands[i].Name) == target {
			return &d.Commands[i]
		}
	}
	return nil
}

// WidgetRegistry 注册表。
type WidgetRegistry struct {
	defs map[string]WidgetDefinition // key: type + "@" + version
}

// NewWidgetRegistry 创建空注册表。
func NewWidgetRegistry() *WidgetRegistry {
	return &WidgetRegistry{defs: make(map[string]WidgetDefinition)}
}

func widgetKey(widgetType, version string) string {
	return strings.TrimSpace(widgetType) + "@" + strings.TrimSpace(version)
}

// Register 注册一个 Widget 定义。校验失败返回错误；同 (type,version) 覆盖。
func (r *WidgetRegistry) Register(d WidgetDefinition) error {
	if err := ValidateWidgetDefinition(&d); err != nil {
		return err
	}
	if r.defs == nil {
		r.defs = make(map[string]WidgetDefinition)
	}
	r.defs[widgetKey(d.Type, d.Version)] = d
	return nil
}

// Get 精确取 (type, version)。未注册返回 nil。
func (r *WidgetRegistry) Get(widgetType, version string) *WidgetDefinition {
	if r == nil {
		return nil
	}
	d, ok := r.defs[widgetKey(widgetType, version)]
	if !ok {
		return nil
	}
	return &d
}

// Latest 取该 type 的最高版本（按字典序，要求版本号可比；不猜测语义化版本）。
// 同类型多版本共存时取字典序最大者；未注册任何版本返回 nil。
func (r *WidgetRegistry) Latest(widgetType string) *WidgetDefinition {
	if r == nil {
		return nil
	}
	prefix := strings.TrimSpace(widgetType) + "@"
	var best *WidgetDefinition
	for k, v := range r.defs {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		if best == nil || v.Version > best.Version {
			current := v
			best = &current
		}
	}
	return best
}

// List 返回全部注册定义，按 (type, version) 稳定排序。
func (r *WidgetRegistry) List() []WidgetDefinition {
	if r == nil {
		return nil
	}
	out := make([]WidgetDefinition, 0, len(r.defs))
	for _, v := range r.defs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Version < out[j].Version
	})
	return out
}

// ResolveCanvas 解析画布中的 Widget 实例。
//
// 本函数**永不因 Widget 不可用而返回错误**（见文件头注意事项 2、3）：
// 缺失能力 => Degraded，未注册 => Unknown，两者都不影响其余 Widget 进入 Available。
func (r *WidgetRegistry) ResolveCanvas(instances []WidgetInstance, env RenderEnvironment) WidgetResolution {
	res := WidgetResolution{
		Available: []WidgetDefinition{},
		Degraded:  []WidgetDefinition{},
		Unknown:   []WidgetInstance{},
	}
	for _, inst := range instances {
		def := r.Get(inst.WidgetType, inst.Version)
		if def == nil {
			// 版本未精确命中时回落到该类型的最新版本；仍无则判为未注册。
			def = r.Latest(inst.WidgetType)
			if def == nil {
				res.Unknown = append(res.Unknown, inst)
				continue
			}
		}
		if def.RequiresWebGL() && !env.WebGLAvailable {
			res.Degraded = append(res.Degraded, *def)
			continue
		}
		res.Available = append(res.Available, *def)
	}
	return res
}

// MarshalResolution 将解析结果序列化为 JSON（供接口返回与审计留存）。
func MarshalResolution(res WidgetResolution) string {
	raw, err := json.Marshal(res)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
