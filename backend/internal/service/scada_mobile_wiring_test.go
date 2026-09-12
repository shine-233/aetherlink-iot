package service

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	utils "aetherlink-iot/backend/pkg/utils"
)

// ---------------------------------------------------------------------------
// 内置 Widget 注册表
// ---------------------------------------------------------------------------

func TestDefaultWidgetRegistryRegistersAllBuiltins(t *testing.T) {
	registry, err := DefaultWidgetRegistry()
	if err != nil {
		t.Fatalf("DefaultWidgetRegistry: %v", err)
	}
	for _, def := range builtinWidgetDefinitions() {
		got := registry.Get(def.Type, def.Version)
		if got == nil {
			t.Fatalf("builtin widget %s@%s not retrievable after registration", def.Type, def.Version)
		}
		if len(got.Commands) != len(def.Commands) {
			t.Fatalf("widget %s@%s commands = %d, want %d", def.Type, def.Version, len(got.Commands), len(def.Commands))
		}
	}
	if len(registry.List()) != len(builtinWidgetDefinitions()) {
		t.Fatalf("registry size = %d, want %d", len(registry.List()), len(builtinWidgetDefinitions()))
	}
}

// TestValveCommandsRequireConfirmation 锁住内置定义里最危险的一条：
// 阀门开合必须要求二次确认。改成 false 等于把现场设备的一键开阀放出去。
func TestValveCommandsRequireConfirmation(t *testing.T) {
	registry, err := DefaultWidgetRegistry()
	if err != nil {
		t.Fatalf("DefaultWidgetRegistry: %v", err)
	}
	valve := registry.Get("valve", "1")
	if valve == nil {
		t.Fatal("valve@1 is not registered")
	}
	cmd := valve.FindCommand("open_valve")
	if cmd == nil {
		t.Fatal("valve@1 does not declare open_valve")
	}
	if !cmd.RequiresConfirmation {
		t.Fatal("valve open_valve must require confirmation")
	}
}

// ---------------------------------------------------------------------------
// 前后端 Widget 注册表一致性
// ---------------------------------------------------------------------------

// frontendWidget 从前端源码解析出的 Widget 注册项。
type frontendWidget struct {
	widgetType  string
	version     string
	capability  string
	commands    map[string]bool // 命令名 -> requires_confirmation
	hasCommands bool
}

var (
	frontendTypeRe       = regexp.MustCompile(`type:\s*'([^']+)'`)
	frontendVersionRe    = regexp.MustCompile(`version:\s*'([^']+)'`)
	frontendCapRe        = regexp.MustCompile(`capabilities:\s*\[([^\]]*)\]`)
	frontendCommandRe    = regexp.MustCompile(`\{\s*name:\s*'([^']+)',\s*requires_confirmation:\s*(true|false)\s*\}`)
	frontendCommandsRe   = regexp.MustCompile(`commands:\s*\[(.*?)\]`)
	frontendRegistryRe   = regexp.MustCompile(`(?s)const WIDGET_REGISTRY[^=]*=\s*\[(.*?)\n\]`)
	frontendNoCommandsRe = regexp.MustCompile(`commands:\s*\[\s*\]`)
)

// TestBuiltinWidgetRegistryMatchesFrontend 两处注册表必须逐项一致。
// 前后端各写一份常量本来就是重复来源，靠这个测试兜住：
// 前端能画出来的控件后端必须认得，后端放行的命令前端必须存在。
func TestBuiltinWidgetRegistryMatchesFrontend(t *testing.T) {
	path := filepath.Join("..", "..", "..", "frontend", "src", "views", "visualization", "scada-editor", "index.vue")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read frontend scada editor: %v", path)
	}
	block := frontendRegistryRe.FindStringSubmatch(string(raw))
	if block == nil {
		t.Fatalf("WIDGET_REGISTRY not found in %s; update this test if it moved", path)
	}

	frontend := make(map[string]frontendWidget)
	for _, line := range strings.Split(block[1], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		m := frontendTypeRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v := frontendVersionRe.FindStringSubmatch(line)
		if v == nil {
			t.Fatalf("frontend widget %s has no version: %s", m[1], line)
		}
		fw := frontendWidget{
			widgetType:  m[1],
			version:     v[1],
			commands:    make(map[string]bool),
			hasCommands: true,
		}
		if cap := frontendCapRe.FindStringSubmatch(line); cap != nil {
			fw.capability = strings.Trim(strings.ReplaceAll(cap[1], "'", ""), " ")
		}
		if frontendNoCommandsRe.MatchString(line) {
			fw.hasCommands = false
		} else if cmds := frontendCommandsRe.FindStringSubmatch(line); cmds != nil {
			for _, c := range frontendCommandRe.FindAllStringSubmatch(cmds[1], -1) {
				fw.commands[c[1]] = c[2] == "true"
			}
		}
		frontend[fw.widgetType+"@"+fw.version] = fw
	}

	if len(frontend) == 0 {
		t.Fatalf("parsed 0 widgets from %s; parser is broken", path)
	}

	registry, err := DefaultWidgetRegistry()
	if err != nil {
		t.Fatalf("DefaultWidgetRegistry: %v", err)
	}
	backend := make(map[string]WidgetDefinition)
	for _, def := range registry.List() {
		backend[def.Type+"@"+def.Version] = def
	}

	if len(frontend) != len(backend) {
		t.Fatalf("widget count mismatch: frontend=%d backend=%d (frontend=%v backend-keys=%v)",
			len(frontend), len(backend), keysOf(frontend), keysOfBackend(backend))
	}

	for key, fw := range frontend {
		bd, ok := backend[key]
		if !ok {
			t.Fatalf("frontend widget %s is not registered on the backend", key)
		}
		if fw.capability != "" && !containsCapability(bd.Capabilities, fw.capability) {
			t.Fatalf("widget %s capability: frontend=%q backend=%v", key, fw.capability, bd.Capabilities)
		}
		if len(bd.Commands) != len(fw.commands) {
			t.Fatalf("widget %s command count: frontend=%d backend=%d", key, len(fw.commands), len(bd.Commands))
		}
		for name, wantConfirm := range fw.commands {
			cmd := bd.FindCommand(name)
			if cmd == nil {
				t.Fatalf("widget %s: frontend command %s missing on backend", key, name)
			}
			if cmd.RequiresConfirmation != wantConfirm {
				t.Fatalf("widget %s command %s requires_confirmation: frontend=%t backend=%t",
					key, name, wantConfirm, cmd.RequiresConfirmation)
			}
		}
	}
}

func containsCapability(caps []string, want string) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}

func keysOf(m map[string]frontendWidget) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func keysOfBackend(m map[string]WidgetDefinition) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// 命令下发执行器
// ---------------------------------------------------------------------------

// TestCommandDeliveryExecutorRefusesWithoutActorClaims 锁住最关键的一条：
// 没有真实凭证就拒绝下发。
// 命令通道在没有 claims 参数时会跳过设备写权限校验，带着空凭证转发等于
// 把"这台设备归不归你管"这道闸整个拆掉。
func TestCommandDeliveryExecutorRefusesWithoutActorClaims(t *testing.T) {
	exec := NewCommandDeliveryExecutor()
	err := exec.Execute(context.Background(), ControlExecution{
		TenantID:    "t1",
		DeviceID:    "dev1",
		Command:     "open_valve",
		ActorUserID: "u1",
		ActorClaims: nil,
	})
	if err != ErrControlActorClaimsMissing {
		t.Fatalf("Execute without claims = %v, want %v", err, ErrControlActorClaimsMissing)
	}
}

func TestCommandDeliveryExecutorRejectsIncompleteExecution(t *testing.T) {
	exec := NewCommandDeliveryExecutor()
	claims := &utils.UserClaims{ID: "u1", TenantID: "t1", Authority: "TENANT_ADMIN"}

	cases := []struct {
		name string
		exec ControlExecution
	}{
		{name: "missing device", exec: ControlExecution{TenantID: "t1", Command: "open_valve", ActorClaims: claims}},
		{name: "missing command", exec: ControlExecution{TenantID: "t1", DeviceID: "dev1", ActorClaims: claims}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := exec.Execute(context.Background(), tc.exec); err == nil {
				t.Fatal("incomplete execution must be rejected")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 装配
// ---------------------------------------------------------------------------

func TestAssembleScadaControlReportsIssuerState(t *testing.T) {
	// 未配置密钥：服务接线成功，但签发器未配置（issuerConfigured=false），
	// 需确认的命令将被拒。不能因为"控制服务存在"就以为危险操作能点。
	svc, configured, err := AssembleScadaControl(ScadaControlWiring{})
	if err != nil {
		t.Fatalf("assemble without secret: %v", err)
	}
	if configured {
		t.Fatal("issuerConfigured must be false when the secret is empty")
	}
	if svc == nil {
		t.Fatal("control service must still be wired")
	}
	if _, err := svc.IssueConfirmation(context.Background(), "t1", "d1", "w1", "open_valve", "u1"); err != ErrConfirmationNoIssuer {
		t.Fatalf("IssueConfirmation without secret = %v, want %v", err, ErrConfirmationNoIssuer)
	}

	svcWithSecret, configured, err := AssembleScadaControl(ScadaControlWiring{ConfirmationSecret: "s3cret"})
	if err != nil {
		t.Fatalf("assemble with secret: %v", err)
	}
	if !configured {
		t.Fatal("issuerConfigured must be true when a secret is configured")
	}
	token, err := svcWithSecret.IssueConfirmation(context.Background(), "t1", "d1", "w1", "open_valve", "u1")
	if err != nil || token == "" {
		t.Fatalf("IssueConfirmation = (%q, %v), want a token", token, err)
	}
	// 装配出来的服务必须认得内置控件，否则"接线成功"只是个空壳。
	if svcWithSecret.registry.Get("valve", "1") == nil {
		t.Fatal("assembled control service does not know the builtin valve widget")
	}
}

func TestAssembleMobileCapabilitiesAreHonest(t *testing.T) {
	svc := AssembleMobile(nil)
	caps := svc.Capabilities(context.Background())

	// 已接线：设备列表 / 告警 / 影子 / OTA / 看板 / 命令都接到真实实现上。
	for name, got := range map[string]bool{
		"telemetry":  caps.Telemetry,
		"alarms":     caps.Alarms,
		"shadow":     caps.Shadow,
		"ota":        caps.OTA,
		"dashboards": caps.Dashboards,
		"commands":   caps.Commands,
	} {
		if !got {
			t.Fatalf("capability %s must be true: it is wired to a real implementation", name)
		}
	}
	// 未传 PushService 时推送必须报 false，不能因为"接口在"就点亮。
	if caps.Push {
		t.Fatal("push must be false when no provider is configured")
	}
	// 离线缓存是客户端能力，服务端永远无从得知。
	if caps.OfflineCache {
		t.Fatal("offline_cache must stay false: the server cannot know what the client cached")
	}
}

// TestAssembleMobileAlarmsRequireBothListAndAck 告警能力必须"能看"且"能确认"
// 才算接线：只有确认没有列表，移动端拿不到告警 ID，入口根本点不到。
func TestAssembleMobileAlarmsRequireBothListAndAck(t *testing.T) {
	half := NewMobileService(MobileServiceDeps{Alarms: NewMobileAlarmAcker()})
	if half.Capabilities(context.Background()).Alarms {
		t.Fatal("alarms must be false when only acknowledgement is wired")
	}
	other := NewMobileService(MobileServiceDeps{AlarmLister: NewMobileAlarmLister()})
	if other.Capabilities(context.Background()).Alarms {
		t.Fatal("alarms must be false when only the list is wired")
	}
	both := NewMobileService(MobileServiceDeps{
		AlarmLister: NewMobileAlarmLister(),
		Alarms:      NewMobileAlarmAcker(),
	})
	if !both.Capabilities(context.Background()).Alarms {
		t.Fatal("alarms must be true when both list and acknowledgement are wired")
	}
}

// TestExecuteControlPassesActorClaimsToExecutor 端到端确认凭证真的传到了执行器。
// 只测执行器本身不够——链路上任何一层忘了传，前面那道闸就形同虚设。
func TestExecuteControlPassesActorClaimsToExecutor(t *testing.T) {
	svc, _, exec, doc, issuer := newControlFixture(t)
	ctx := context.Background()
	claims := &utils.UserClaims{ID: "u1", TenantID: "t1", Authority: "TENANT_ADMIN"}

	token, err := issuer.Issue("t1", doc.ID, "w1", "open_valve", "u1", time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	outcome, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID:          "t1",
		DeviceID:          "dev1",
		DocumentID:        doc.ID,
		WidgetID:          "w1",
		WidgetType:        "valve",
		Version:           "1",
		Command:           "open_valve",
		ConfirmationToken: token,
		Actor: ControlActor{
			UserID: "u1", TenantID: "t1", Authority: "TENANT_ADMIN", Claims: claims,
		},
	})
	if err != nil {
		t.Fatalf("ExecuteControl: %v", err)
	}
	if outcome != "success" {
		t.Fatalf("outcome = %q, want success", outcome)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(exec.calls))
	}
	if exec.calls[0].ActorClaims == nil {
		t.Fatal("actor claims were dropped before reaching the executor")
	}
	if exec.calls[0].ActorClaims.ID != "u1" {
		t.Fatalf("actor claims ID = %q, want u1", exec.calls[0].ActorClaims.ID)
	}
}
