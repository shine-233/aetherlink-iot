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
	"fmt"
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
	widgetMaxTypeLen     = 64
	widgetMaxVersionLen  = 32
	widgetMaxSchemaBytes = 256 * 1024
	widgetMaxCommandLen  = 64
	widgetMaxCommands    = 32
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
	// ErrWidgetCommandParamsInvalid 命令参数不是可校验的 JSON 对象。
	ErrWidgetCommandParamsInvalid = errors.New("control command params must be a JSON object")
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
	Capabilities []string            `json:"capabilities"`
	Commands     []CommandDefinition `json:"commands,omitempty"`
}

// WidgetInstance 画布中引用某个 Widget 的实例。
type WidgetInstance struct {
	ID         string `json:"id"`
	WidgetType string `json:"widget_type"`
	Version    string `json:"version"`
	// Config 该实例的配置，由 Widget 注册声明的 schema 校验。
	// nil 表示"还没配置"：校验时按空对象处理，因此 schema 声明的必填项会失败。
	// 这是刻意的——控件拖上画布但没接遥测点就允许保存，等于画布里留一个永远空白的框。
	Config map[string]any `json:"config,omitempty"`
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
	// schemas 与 defs 一一对应的已编译配置校验器。
	// 单独存一份而不是在校验时临时编译：编译会拒绝不支持的关键字，
	// 若留到保存画布时才做，用户会拿到"昨天还能存，今天存不了"的结果，
	// 而真正的原因（有人注册了坏 schema）在那时已经无从追查。
	schemas map[string]*WidgetSchema
	// paramSchemas key: type + "@" + version + "#" + command。
	paramSchemas map[string]*WidgetSchema
}

// NewWidgetRegistry 创建空注册表。
func NewWidgetRegistry() *WidgetRegistry {
	return &WidgetRegistry{
		defs:         make(map[string]WidgetDefinition),
		schemas:      make(map[string]*WidgetSchema),
		paramSchemas: make(map[string]*WidgetSchema),
	}
}

func widgetKey(widgetType, version string) string {
	return strings.TrimSpace(widgetType) + "@" + strings.TrimSpace(version)
}

func widgetCommandKey(widgetType, version, command string) string {
	return widgetKey(widgetType, version) + "#" + strings.TrimSpace(command)
}

// Register 注册一个 Widget 定义。校验失败返回错误；同 (type,version) 覆盖。
//
// 除了结构校验，还会在注册时编译 schema（见 widget_schema.go 文件头注意事项 1）：
// schema 用了不受支持的关键字 → 注册失败，绝不留到画布保存时才炸。
func (r *WidgetRegistry) Register(d WidgetDefinition) error {
	if err := ValidateWidgetDefinition(&d); err != nil {
		return err
	}
	if r.defs == nil {
		r.defs = make(map[string]WidgetDefinition)
	}
	if r.schemas == nil {
		r.schemas = make(map[string]*WidgetSchema)
	}
	if r.paramSchemas == nil {
		r.paramSchemas = make(map[string]*WidgetSchema)
	}
	key := widgetKey(d.Type, d.Version)
	schema, err := CompileWidgetSchema(d.Schema)
	if err != nil {
		return err
	}
	params := make(map[string]*WidgetSchema, len(d.Commands))
	for _, cmd := range d.Commands {
		if strings.TrimSpace(cmd.ParamsSchema) == "" {
			continue
		}
		ps, err := CompileWidgetSchema(cmd.ParamsSchema)
		if err != nil {
			return err
		}
		params[strings.TrimSpace(cmd.Name)] = ps
	}
	r.defs[key] = d
	r.schemas[key] = schema
	for name, ps := range params {
		r.paramSchemas[widgetCommandKey(d.Type, d.Version, name)] = ps
	}
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

// ValidateConfig 校验某个 Widget 实例的配置。
//
// 未注册的类型返回 nil：那不是"通过校验"，而是"没有 schema 无从校验"，
// 这类 Widget 会在 ResolveCanvas 里被判为 unknown 并渲染占位符。
func (r *WidgetRegistry) ValidateConfig(widgetType, version string, config map[string]any) error {
	if r == nil {
		return nil
	}
	s := r.schemaFor(widgetType, version)
	if s == nil {
		return nil
	}
	return s.ValidateConfig(config)
}

// ValidateInstance 校验画布中的一个 Widget 实例（当前即其配置）。
func (r *WidgetRegistry) ValidateInstance(inst WidgetInstance) error {
	if r == nil {
		return nil
	}
	if err := r.ValidateConfig(inst.WidgetType, inst.Version, inst.Config); err != nil {
		id := strings.TrimSpace(inst.ID)
		if id == "" {
			id = "<unnamed>"
		}
		return fmt.Errorf("widget %s (%s@%s): %w", id, inst.WidgetType, inst.Version, err)
	}
	return nil
}

// ValidateCommandParams 校验控制命令的参数。
// 命令未声明参数 schema 时不校验——参数该不该有、有什么，由注册声明决定；
// 没声明就按"不约束"处理，与"声明了但没校验"是两回事。
func (r *WidgetRegistry) ValidateCommandParams(widgetType, version, command string, params any) error {
	if r == nil {
		return nil
	}
	s, ok := r.paramSchemas[widgetCommandKey(widgetType, version, command)]
	if !ok || s == nil {
		return nil
	}
	value, ok := normalizeCommandParams(params)
	if !ok {
		return fmt.Errorf("%w: command %s on %s@%s", ErrWidgetCommandParamsInvalid, command, widgetType, version)
	}
	return s.Validate(value)
}

// canvasNodeShape 新版画布（views/scada canvasDocument）里的节点形状。
// 与旧版 `widgets[].config` 并存：两代编辑器的画布都会流进保存路径，都要校验。
type canvasNodeShape struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Ref        string         `json:"ref"`
	Props      map[string]any `json:"props"`
}

// ValidateCanvasJSON 校验画布 JSON 里每个 Widget 的配置。
//
// 兼容两种画布形状：
//   - 旧版（visualization/scada-editor）：顶层 `widgets` 数组，配置在 `config`；
//   - 新版（views/scada canvasDocument）：顶层 `nodes` 数组，kind=widget 的节点
//     以 `ref` 为 Widget 类型，配置在 `props`。
//
// 画布顶层允许带自定义字段（variables/bindings 等），强行要求某个形状会把合法画布拒掉。
// 字段存在但不是数组时报错——那是一份坏画布，不是"没有 widget"。
func (r *WidgetRegistry) ValidateCanvasJSON(canvas string) error {
	if r == nil {
		return nil
	}
	trimmed := strings.TrimSpace(canvas)
	if trimmed == "" || trimmed[0] != '{' {
		return nil
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		// 坏 JSON / 非对象：由 model.ValidateScadaCanvas 负责拦，这里不重复定性。
		return nil
	}
	if raw, ok := probe["widgets"]; ok {
		var widgets []WidgetInstance
		if err := json.Unmarshal(raw, &widgets); err != nil {
			return fmt.Errorf("%w: canvas widgets must be an array of objects", ErrWidgetConfigNotObject)
		}
		for _, w := range widgets {
			if err := r.ValidateInstance(w); err != nil {
				return err
			}
		}
	}
	if raw, ok := probe["nodes"]; ok {
		var nodes []canvasNodeShape
		if err := json.Unmarshal(raw, &nodes); err != nil {
			return fmt.Errorf("%w: canvas nodes must be an array of objects", ErrWidgetConfigNotObject)
		}
		for _, n := range nodes {
			if strings.TrimSpace(n.Kind) != "widget" {
				continue
			}
			inst := WidgetInstance{ID: n.ID, WidgetType: n.Ref, Config: n.Props}
			if err := r.ValidateInstance(inst); err != nil {
				return err
			}
		}
	}
	return nil
}

// schemaFor 取 (type, version) 的已编译 schema，精确版本缺失时回落到该类型最新版本
// （与 ResolveCanvas 的回落口径保持一致：两边对"用哪份定义"必须给出同一答案）。
func (r *WidgetRegistry) schemaFor(widgetType, version string) *WidgetSchema {
	key := widgetKey(widgetType, version)
	if s, ok := r.schemas[key]; ok {
		return s
	}
	if def := r.Latest(widgetType); def != nil {
		return r.schemas[widgetKey(def.Type, def.Version)]
	}
	return nil
}

// normalizeCommandParams 把命令参数归一成可校验的 JSON 值。
// 空参数按空对象处理：声明了必填参数却一个都不传，必须失败而不是跳过校验。
func normalizeCommandParams(params any) (any, bool) {
	switch v := params.(type) {
	case nil:
		return map[string]any{}, true
	case map[string]any:
		return v, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return map[string]any{}, true
		}
		var out any
		if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
			return nil, false
		}
		return out, true
	case []byte:
		if len(strings.TrimSpace(string(v))) == 0 {
			return map[string]any{}, true
		}
		var out any
		if err := json.Unmarshal(v, &out); err != nil {
			return nil, false
		}
		return out, true
	}
	return nil, false
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
