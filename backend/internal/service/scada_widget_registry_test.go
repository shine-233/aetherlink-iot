package service

import "testing"

func sampleWidget(typ, version string, caps ...string) WidgetDefinition {
	t := WidgetDefinition{
		Type:         typ,
		Version:      version,
		Schema:       `{"type":"object"}`,
		Capabilities: caps,
		Commands: []CommandDefinition{
			{Name: "refresh", RequiresConfirmation: false},
			{Name: "write_valve", RequiresConfirmation: true},
		},
	}
	return t
}

func TestWidgetRegistryRejectsInvalidDefinitions(t *testing.T) {
	reg := NewWidgetRegistry()

	cases := []struct {
		name string
		def  WidgetDefinition
		want error
	}{
		{name: "missing type", def: WidgetDefinition{Version: "1", Schema: "{}", Capabilities: []string{WidgetCapability2D}}, want: ErrWidgetMissingType},
		{name: "missing version", def: WidgetDefinition{Type: "gauge", Schema: "{}", Capabilities: []string{WidgetCapability2D}}, want: ErrWidgetMissingVersion},
		{name: "missing schema", def: WidgetDefinition{Type: "gauge", Version: "1", Capabilities: []string{WidgetCapability2D}}, want: ErrWidgetMissingSchema},
		{name: "schema not object", def: WidgetDefinition{Type: "gauge", Version: "1", Schema: `[]`, Capabilities: []string{WidgetCapability2D}}, want: ErrWidgetBadSchema},
		// 空能力不是"默认 2D"：静默兜底会让依赖 WebGL 的 Widget 渲染成一块空白。
		{name: "no capability", def: WidgetDefinition{Type: "gauge", Version: "1", Schema: "{}"}, want: ErrWidgetNoCapability},
		{name: "bad capability", def: WidgetDefinition{Type: "gauge", Version: "1", Schema: "{}", Capabilities: []string{"4d"}}, want: ErrWidgetBadCapability},
		{name: "duplicate command", def: WidgetDefinition{
			Type: "gauge", Version: "1", Schema: "{}", Capabilities: []string{WidgetCapability2D},
			Commands: []CommandDefinition{{Name: "go"}, {Name: "go"}},
		}, want: ErrWidgetDuplicateCmd},
		{name: "empty command name", def: WidgetDefinition{
			Type: "gauge", Version: "1", Schema: "{}", Capabilities: []string{WidgetCapability2D},
			Commands: []CommandDefinition{{Name: "  "}},
		}, want: ErrWidgetBadCommand},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := reg.Register(tc.def); err != tc.want {
				t.Fatalf("Register(%s) error = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
	if len(reg.List()) != 0 {
		t.Fatalf("registry accepted invalid definitions: %d registered", len(reg.List()))
	}
}

func TestWidgetRegistryLatestPicksGreatestVersion(t *testing.T) {
	reg := NewWidgetRegistry()
	for _, v := range []string{"1.0.0", "1.2.0", "1.10.0"} {
		if err := reg.Register(sampleWidget("gauge", v, WidgetCapability2D)); err != nil {
			t.Fatalf("register %s: %v", v, err)
		}
	}
	got := reg.Latest("gauge")
	if got == nil {
		t.Fatal("Latest(gauge) = nil, want a definition")
	}
	// 字典序比较：1.10.0 与 1.2.0 的语义化版本顺序与字典序不一致，
	// 这里锁定的是"按字典序取最大"这一明确声明的行为，避免后人误以为做了语义化比较。
	if got.Version != "1.2.0" {
		t.Fatalf("Latest(gauge).Version = %q, want %q (documented lexicographic behavior)", got.Version, "1.2.0")
	}
	if reg.Latest("missing") != nil {
		t.Fatal("Latest(missing) should be nil")
	}
}

// TestWidgetResolutionDegradesOnly3D 锁定门禁"3D/WebGL 降级不影响 2D 看板"：
// 环境无 WebGL 时，3D Widget 降级为占位，2D Widget 全部照常可用，
// 且未注册 Widget 不阻断整块看板。
func TestWidgetResolutionDegradesOnly3D(t *testing.T) {
	reg := NewWidgetRegistry()
	mustReg := func(d WidgetDefinition) {
		if err := reg.Register(d); err != nil {
			t.Fatalf("register %s@%s: %v", d.Type, d.Version, err)
		}
	}
	mustReg(sampleWidget("gauge", "1", WidgetCapability2D))
	mustReg(sampleWidget("chart", "1", WidgetCapability2D))
	mustReg(sampleWidget("twin3d", "1", WidgetCapability3D))

	instances := []WidgetInstance{
		{ID: "w1", WidgetType: "gauge", Version: "1"},
		{ID: "w2", WidgetType: "chart", Version: "1"},
		{ID: "w3", WidgetType: "twin3d", Version: "1"},
		{ID: "w4", WidgetType: "retired-widget", Version: "9"},
	}

	withWebGL := reg.ResolveCanvas(instances, RenderEnvironment{WebGLAvailable: true})
	if len(withWebGL.Available) != 3 || len(withWebGL.Degraded) != 0 || len(withWebGL.Unknown) != 1 {
		t.Fatalf("with WebGL: available=%d degraded=%d unknown=%d, want 3/0/1",
			len(withWebGL.Available), len(withWebGL.Degraded), len(withWebGL.Unknown))
	}

	noWebGL := reg.ResolveCanvas(instances, RenderEnvironment{WebGLAvailable: false})
	if len(noWebGL.Available) != 2 {
		t.Fatalf("without WebGL: available = %d, want 2 (the two 2D widgets)", len(noWebGL.Available))
	}
	if len(noWebGL.Degraded) != 1 || noWebGL.Degraded[0].Type != "twin3d" {
		t.Fatalf("without WebGL: degraded = %+v, want exactly twin3d", noWebGL.Degraded)
	}
	// 关键：2D 看板不因 3D 缺失而受损。
	for _, d := range noWebGL.Available {
		if d.RequiresWebGL() {
			t.Fatalf("widget %s requires WebGL but was reported available", d.Type)
		}
	}
	if len(noWebGL.Unknown) != 1 || noWebGL.Unknown[0].WidgetType != "retired-widget" {
		t.Fatalf("unregistered widget must surface as unknown, got %+v", noWebGL.Unknown)
	}
}

func TestWidgetFindCommandReportsConfirmationRequirement(t *testing.T) {
	reg := NewWidgetRegistry()
	if err := reg.Register(sampleWidget("valve", "1", WidgetCapability2D)); err != nil {
		t.Fatalf("register: %v", err)
	}
	def := reg.Get("valve", "1")
	if def == nil {
		t.Fatal("missing valve widget")
	}
	if cmd := def.FindCommand("write_valve"); cmd == nil || !cmd.RequiresConfirmation {
		t.Fatal("write_valve must require explicit confirmation")
	}
	if cmd := def.FindCommand("refresh"); cmd == nil || cmd.RequiresConfirmation {
		t.Fatal("refresh must not require confirmation")
	}
	if def.FindCommand("nope") != nil {
		t.Fatal("unknown command must not be found")
	}
	if reg.Get("valve", "2") != nil {
		t.Fatal("unregistered version must be nil")
	}
}
