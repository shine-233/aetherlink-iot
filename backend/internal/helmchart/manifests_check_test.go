// 文件用途：TB-11 manifests 校验单测——离线渲染 deploy/helm/aetherlink 全部模板并做结构断言，
// 同时以根 docker-compose.yml 与 deploy/docker-compose.replicas.yml 为对齐基准做机械比对。
// 核心逻辑：①加载 values.yaml → ②RenderChart（text/template）渲染 → ③逐文件 yaml.Unmarshal
// 全部文档 → ④按 kind 断言必填键（副本/探针/资源限额/selector/安全上下文）→ ⑤backend/broker
// env 键集合与容器端口对齐 compose → ⑥双副本场景断言 broker 身份注入 → ⑦空密钥 fail-fast →
// ⑧postgres/redis 依赖开关与 external 地址 → ⑨replicas override 结构与前提标注。
// 关键注意事项：本测试只覆盖"渲染面"——helm install 与真实集群行为（含 federation 接线、
// 双副本吊销收敛演练）不在口径内；改 chart/values/compose 任何一方都必须保持本套对齐断言通过。
// 重构建议：模板继续增多时，把 per-kind 断言抽成表驱动辅助函数，避免单个用例过长。
package helmchart

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	global "aetherlink-iot/backend/pkg/global"
	"gopkg.in/yaml.v3"
)

// repoRoot 返回仓库根目录（backend/internal/helmchart → 上三级）。
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试源文件路径")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}

// loadYAMLMap 读取单个 YAML 文件为顶层 map。
func loadYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	var out map[string]any
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("解析 %s: %v", path, err)
	}
	if out == nil {
		t.Fatalf("%s 顶层不是映射", path)
	}
	return out
}

// testRelease/testChart 为渲染注入的固定上下文。
func testRelease() Release {
	return Release{Name: "aetherlink-test", Namespace: "aetherlink-test", Revision: 1, IsInstall: true}
}

func testChart() ChartMeta {
	return ChartMeta{Name: "aetherlink", Version: "0.1.0", AppVersion: global.VERSION}
}

// defaultValues 加载 chart values.yaml 并覆盖测试密钥（secrets 默认为空、模板 required fail-fast）。
func defaultValues(t *testing.T) map[string]any {
	t.Helper()
	values := loadYAMLMap(t, filepath.Join(repoRoot(t), "deploy", "helm", "aetherlink", "values.yaml"))
	secrets, ok := values["secrets"].(map[string]any)
	if !ok {
		t.Fatalf("values.yaml 缺少 secrets 映射")
	}
	secrets["jwtKey"] = "test-jwt-key-not-for-production"
	secrets["postgresPassword"] = "test-postgres-password"
	secrets["redisPassword"] = "test-redis-password"
	secrets["mqttRootPassword"] = "test-mqtt-root-password"
	secrets["mqttPluginPassword"] = "test-mqtt-plugin-password"
	return values
}

// docInfo 是一个渲染出的 K8s 对象。
type docInfo struct {
	apiVersion string
	kind       string
	name       string
	doc        map[string]any
}

// renderDocs 渲染 chart 并把全部 YAML 文档解析为 docInfo 列表。
func renderDocs(t *testing.T, values map[string]any) map[string][]docInfo {
	t.Helper()
	rendered, err := RenderChart(filepath.Join(repoRoot(t), "deploy", "helm", "aetherlink"), testRelease(), testChart(), values)
	if err != nil {
		t.Fatalf("渲染 chart 失败: %v", err)
	}
	docs := make(map[string][]docInfo)
	for name, out := range rendered {
		if strings.Contains(out, "<no value>") {
			t.Errorf("模板 %s 渲染产物包含 <no value>（存在 values 未命中路径）", name)
		}
		decoder := yaml.NewDecoder(strings.NewReader(out))
		for {
			var raw map[string]any
			if err := decoder.Decode(&raw); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				t.Errorf("模板 %s 渲染产物 yaml.Unmarshal 失败: %v\n产物:\n%s", name, err, out)
				break
			}
			if raw == nil {
				continue // 空文档（gated 模板渲染为空）
			}
			info := docInfo{doc: raw}
			info.apiVersion, _ = raw["apiVersion"].(string)
			info.kind, _ = raw["kind"].(string)
			if meta, ok := raw["metadata"].(map[string]any); ok {
				info.name, _ = meta["name"].(string)
			}
			if info.kind == "" || info.apiVersion == "" || info.name == "" {
				t.Errorf("模板 %s 产出的文档缺少 apiVersion/kind/metadata.name: %+v", name, info)
				continue
			}
			docs[name] = append(docs[name], info)
		}
	}
	return docs
}

// docsOfKind 汇总全部渲染对象中指定 kind 的 docInfo。
func docsOfKind(t *testing.T, docs map[string][]docInfo, kind string) []docInfo {
	t.Helper()
	var out []docInfo
	for _, list := range docs {
		for _, d := range list {
			if d.kind == kind {
				out = append(out, d)
			}
		}
	}
	return out
}

// getPath 按 a.b.c 路径取嵌套值（yaml.v3 嵌套 map 为 map[string]any）。
func getPath(t *testing.T, doc map[string]any, path string) any {
	t.Helper()
	cur := any(doc)
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("路径 %s 解析失败：节点不是映射（段 %s）", path, seg)
		}
		cur, ok = m[seg]
		if !ok {
			t.Fatalf("路径 %s 缺少键 %s", path, seg)
		}
	}
	return cur
}

// envNamesAndValues 提取容器 env 列表为 名称→值/来源 映射。
func envNamesAndValues(t *testing.T, container map[string]any) map[string]any {
	t.Helper()
	out := map[string]any{}
	env, ok := container["env"].([]any)
	if !ok {
		t.Fatalf("容器缺少 env 列表")
	}
	for _, e := range env {
		entry, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("env 条目不是映射: %v", e)
		}
		name, _ := entry["name"].(string)
		if name == "" {
			t.Fatalf("env 条目缺少 name: %v", entry)
		}
		if v, has := entry["value"]; has {
			if _, isStr := v.(string); !isStr {
				t.Errorf("env %s 的 value 必须是字符串（模板需 quote）: %v (%T)", name, v, v)
			}
			out[name] = v
			continue
		}
		out[name] = entry["valueFrom"] // secretKeyRef/fieldRef 原样返回
	}
	return out
}

// containerPorts 返回容器声明的端口集合。
func containerPorts(t *testing.T, container map[string]any) map[int]bool {
	t.Helper()
	out := map[int]bool{}
	ports, ok := container["ports"].([]any)
	if !ok || len(ports) == 0 {
		t.Fatalf("容器缺少 ports 声明")
	}
	for _, p := range ports {
		pm, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("端口条目不是映射: %v", p)
		}
		cp, ok := pm["containerPort"].(int)
		if !ok {
			t.Fatalf("containerPort 不是整数: %v", pm["containerPort"])
		}
		out[cp] = true
	}
	return out
}

// composeServiceEnvKeys 解析根 docker-compose.yml 指定服务的 environment 键集合。
func composeServiceEnvKeys(t *testing.T, compose map[string]any, service string) map[string]struct{} {
	t.Helper()
	services, ok := compose["services"].(map[string]any)
	if !ok {
		t.Fatal("docker-compose.yml 缺少 services")
	}
	svc, ok := services[service].(map[string]any)
	if !ok {
		t.Fatalf("docker-compose.yml 缺少服务 %s", service)
	}
	env, ok := svc["environment"].(map[string]any)
	if !ok {
		t.Fatalf("服务 %s 缺少 environment 映射", service)
	}
	keys := make(map[string]struct{}, len(env))
	for k := range env {
		keys[k] = struct{}{}
	}
	return keys
}

// composeServiceContainerPorts 解析根 docker-compose.yml 指定服务发布的容器侧端口集合。
// compose 端口形如 "127.0.0.1:5432:5432" 或 "${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${MQTT_PORT:-1883}:1883"，
// 容器侧端口恒为最后一个冒号段。
func composeServiceContainerPorts(t *testing.T, compose map[string]any, service string) map[int]bool {
	t.Helper()
	services := compose["services"].(map[string]any)
	svc, ok := services[service].(map[string]any)
	if !ok {
		t.Fatalf("docker-compose.yml 缺少服务 %s", service)
	}
	out := map[int]bool{}
	rawPorts, exists := svc["ports"]
	if !exists || rawPorts == nil {
		return out
	}
	ports, ok := rawPorts.([]any)
	if !ok {
		t.Fatalf("服务 %s 的 ports 不是列表", service)
	}
	for _, p := range ports {
		s, ok := p.(string)
		if !ok {
			t.Fatalf("服务 %s 端口条目不是字符串: %v", service, p)
		}
		idx := strings.LastIndex(s, ":")
		if idx < 0 {
			t.Fatalf("服务 %s 端口条目无法解析: %q", service, s)
		}
		port := sprigInt(s[idx+1:])
		if port <= 0 {
			t.Fatalf("服务 %s 端口条目容器侧端口非法: %q", service, s)
		}
		out[port] = true
	}
	return out
}

// assertSortedStringSetEqual 断言两个字符串集合相等（错误信息排序输出，便于定位差异）。
func assertSortedStringSetEqual(t *testing.T, what string, got map[string]struct{}, want map[string]struct{}) {
	t.Helper()
	gotList := make([]string, 0, len(got))
	for k := range got {
		gotList = append(gotList, k)
	}
	wantList := make([]string, 0, len(want))
	for k := range want {
		wantList = append(wantList, k)
	}
	sort.Strings(gotList)
	sort.Strings(wantList)
	if !reflect.DeepEqual(gotList, wantList) {
		t.Errorf("%s 键集合不一致:\n got(渲染/chart): %v\nwant(compose):   %v", what, gotList, wantList)
	}
}

// assertIntSetEqual 断言端口集合相等。
func assertIntSetEqual(t *testing.T, what string, got, want map[int]bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s 端口集合大小不一致: got %v want %v", what, got, want)
		return
	}
	for p := range want {
		if !got[p] {
			t.Errorf("%s 缺少端口 %d: got %v want %v", what, p, got, want)
		}
	}
}

// assertWorkloadShape 断言工作负载（StatefulSet/Deployment）的结构必填项。
func assertWorkloadShape(t *testing.T, d docInfo, opts workloadExpect) {
	t.Helper()
	if d.apiVersion != "apps/v1" {
		t.Errorf("%s: apiVersion 应为 apps/v1，实际 %q", d.name, d.apiVersion)
	}
	replicas, ok := getPath(t, d.doc, "spec.replicas").(int)
	if !ok || replicas < 1 {
		t.Errorf("%s: spec.replicas 必须是 >=1 的整数，实际 %v", d.name, getPath(t, d.doc, "spec.replicas"))
	} else if opts.minReplicas > 0 && replicas < opts.minReplicas {
		t.Errorf("%s: spec.replicas=%d 低于期望 %d", d.name, replicas, opts.minReplicas)
	}
	selector, ok := getPath(t, d.doc, "spec.selector.matchLabels").(map[string]any)
	if !ok || len(selector) == 0 {
		t.Fatalf("%s: spec.selector.matchLabels 缺失", d.name)
	}
	for _, key := range []string{"app.kubernetes.io/name", "app.kubernetes.io/instance"} {
		if _, has := selector[key]; !has {
			t.Errorf("%s: selector 缺少标准标签 %s", d.name, key)
		}
	}
	templateLabels, ok := getPath(t, d.doc, "spec.template.metadata.labels").(map[string]any)
	if !ok || len(templateLabels) == 0 {
		t.Fatalf("%s: spec.template.metadata.labels 缺失", d.name)
	}
	for k, v := range selector {
		if templateLabels[k] != v {
			t.Errorf("%s: 模板标签 %s=%v 与 selector %v 不一致", d.name, k, templateLabels[k], v)
		}
	}
	if opts.orderedReady {
		if got := getPath(t, d.doc, "spec.podManagementPolicy"); got != "OrderedReady" {
			t.Errorf("%s: podManagementPolicy 应为 OrderedReady（串行化首启迁移/身份就绪），实际 %v", d.name, got)
		}
		if got := getPath(t, d.doc, "spec.serviceName"); got == "" {
			t.Errorf("%s: StatefulSet 必须声明 spec.serviceName", d.name)
		}
	}
	if opts.perPodVolumes > 0 {
		vcts, ok := getPath(t, d.doc, "spec.volumeClaimTemplates").([]any)
		if !ok || len(vcts) != opts.perPodVolumes {
			t.Errorf("%s: volumeClaimTemplates 数量期望 %d，实际 %v", d.name, opts.perPodVolumes, len(vcts))
		}
	}
	podSec, ok := getPath(t, d.doc, "spec.template.spec.securityContext").(map[string]any)
	if !ok {
		t.Fatalf("%s: 缺少 pod securityContext", d.name)
	}
	if opts.runAsNonRoot {
		if podSec["runAsNonRoot"] != true {
			t.Errorf("%s: pod securityContext.runAsNonRoot 必须为 true（镜像 USER=10001）", d.name)
		}
		if uid := sprigInt(podSec["runAsUser"]); uid != 10001 {
			t.Errorf("%s: pod securityContext.runAsUser 必须为 10001（对齐镜像 USER），实际 %v", d.name, podSec["runAsUser"])
		}
	}
	containers, ok := getPath(t, d.doc, "spec.template.spec.containers").([]any)
	if !ok || len(containers) == 0 {
		t.Fatalf("%s: containers 为空", d.name)
	}
	for _, c := range containers {
		container, ok := c.(map[string]any)
		if !ok {
			t.Fatalf("%s: 容器不是映射", d.name)
		}
		cname, _ := container["name"].(string)
		image, _ := container["image"].(string)
		if cname == "" || image == "" {
			t.Errorf("%s/%s: 容器缺 name 或 image", d.name, cname)
		}
		if strings.Contains(image, "CHANGE_ME") || !strings.Contains(image, ":") {
			t.Errorf("%s/%s: image 口径非法: %q", d.name, cname, image)
		}
		for _, probe := range []string{"livenessProbe", "readinessProbe", "startupProbe"} {
			if _, has := container[probe]; !has {
				t.Errorf("%s/%s: 缺少 %s（对齐 compose healthcheck）", d.name, cname, probe)
			}
		}
		res, ok := container["resources"].(map[string]any)
		if !ok {
			t.Fatalf("%s/%s: 缺少 resources", d.name, cname)
		}
		limits, _ := res["limits"].(map[string]any)
		requests, _ := res["requests"].(map[string]any)
		if len(limits) == 0 || len(requests) == 0 {
			t.Errorf("%s/%s: resources.limits 与 resources.requests 都必须配置（资源限额口径对齐 compose mem_limit/cpus）", d.name, cname)
		}
		sec, ok := container["securityContext"].(map[string]any)
		if !ok {
			t.Fatalf("%s/%s: 缺少容器 securityContext", d.name, cname)
		}
		if opts.dropAllCaps {
			if sec["allowPrivilegeEscalation"] != false {
				t.Errorf("%s/%s: allowPrivilegeEscalation 应为 false（对齐 no-new-privileges）", d.name, cname)
			}
			caps, _ := sec["capabilities"].(map[string]any)
			drops, _ := caps["drop"].([]any)
			hasALL := false
			for _, drop := range drops {
				if drop == "ALL" {
					hasALL = true
				}
			}
			if !hasALL {
				t.Errorf("%s/%s: capabilities.drop 必须包含 ALL（对齐 cap_drop）", d.name, cname)
			}
		}
		if len(opts.expectPorts) > 0 {
			assertIntSetEqual(t, d.name+"/"+cname+" containerPorts", containerPorts(t, container), opts.expectPorts)
		}
	}
}

// workloadExpect 汇总 assertWorkloadShape 的可选项。
type workloadExpect struct {
	minReplicas   int
	orderedReady  bool
	perPodVolumes int
	runAsNonRoot  bool
	dropAllCaps   bool
	expectPorts   map[int]bool
}

// TestRenderedHelmManifestsParseAndCarryRequiredKeys 是主校验：默认 values 下全部模板渲染、
// 全部文档 yaml.Unmarshal、按 kind 断言必填键，并与根 docker-compose.yml 做 env/端口对齐。
func TestRenderedHelmManifestsParseAndCarryRequiredKeys(t *testing.T) {
	root := repoRoot(t)
	chartDir := filepath.Join(root, "deploy", "helm", "aetherlink")

	// Chart.yaml 元数据与后端版本钉死（appVersion 必须等于 global.VERSION）。
	chartYAML := loadYAMLMap(t, filepath.Join(chartDir, "Chart.yaml"))
	if chartYAML["apiVersion"] != "v2" {
		t.Errorf("Chart.yaml apiVersion 应为 v2")
	}
	if chartYAML["name"] != "aetherlink" {
		t.Errorf("Chart.yaml name 应为 aetherlink，实际 %v", chartYAML["name"])
	}
	if chartYAML["appVersion"] != global.VERSION {
		t.Errorf("Chart.yaml appVersion=%v 与 backend global.VERSION=%q 漂移，必须同步", chartYAML["appVersion"], global.VERSION)
	}

	docs := renderDocs(t, defaultValues(t))

	// 根 docker-compose.yml 是 env/端口/healthcheck 的对齐基准，先于结构断言加载。
	compose := loadYAMLMap(t, filepath.Join(root, "docker-compose.yml"))
	portsByService := map[string]map[int]bool{
		"backend":     composeServiceContainerPorts(t, compose, "backend"),
		"mqtt-broker": composeServiceContainerPorts(t, compose, "mqtt-broker"),
		"frontend":    composeServiceContainerPorts(t, compose, "frontend"),
		"postgres":    composeServiceContainerPorts(t, compose, "postgres"),
	}

	// 对象面结构 pin：4 StatefulSet + 1 Deployment + 9 Service + 1 Secret + 2 ConfigMap。
	assertKindCount(t, docs, "StatefulSet", 4)
	assertKindCount(t, docs, "Deployment", 1)
	assertKindCount(t, docs, "Service", 9)
	assertKindCount(t, docs, "Secret", 1)
	assertKindCount(t, docs, "ConfigMap", 2)

	// Secret 必填键与值非空（fail-fast 的正面路径）。
	for _, s := range docsOfKind(t, docs, "Secret") {
		stringData, ok := s.doc["stringData"].(map[string]any)
		if !ok {
			t.Fatalf("Secret %s 缺少 stringData", s.name)
		}
		for _, key := range []string{"jwt-key", "postgres-password", "redis-password", "mqtt-root-password", "mqtt-plugin-password"} {
			v, has := stringData[key].(string)
			if !has || v == "" || strings.HasPrefix(v, "CHANGE_ME") {
				t.Errorf("Secret %s 键 %s 缺失或为空/占位", s.name, key)
			}
		}
	}

	// ConfigMap：broker 配置保留 GMQTT_* 覆盖占位口径；frontend nginx 与源文件同源（只换 proxy 目标）。
	for _, cm := range docsOfKind(t, docs, "ConfigMap") {
		data, ok := cm.doc["data"].(map[string]any)
		if !ok {
			t.Fatalf("ConfigMap %s 缺少 data", cm.name)
		}
		if strings.HasSuffix(cm.name, "-broker-aetherlink-config") {
			yml, _ := data["aetherlink.yml"].(string)
			for _, token := range []string{"mqtt_session_revocations", "broker_id", "CHANGE_ME_OVERRIDDEN_BY_GMQTT_"} {
				if !strings.Contains(yml, token) {
					t.Errorf("broker ConfigMap 缺少标记 %s（对齐 aetherlink.example.yml 覆盖口径）", token)
				}
			}
		}
		if strings.HasSuffix(cm.name, "-frontend-nginx") {
			nginx, _ := data["nginx.conf"].(string)
			assertNginxConfigAligned(t, nginx, testRelease().Name)
		}
	}

	// 工作负载结构断言（端口期望一律取自 compose 解析结果）。
	for _, sts := range docsOfKind(t, docs, "StatefulSet") {
		expect := workloadExpect{orderedReady: true, perPodVolumes: 1, dropAllCaps: true, runAsNonRoot: true}
		switch {
		case strings.HasSuffix(sts.name, "-backend"):
			expect.perPodVolumes = 3 // files + telemetry-spool + uplink-spool
			expect.expectPorts = portsByService["backend"]
		case strings.HasSuffix(sts.name, "-mqtt-broker"):
			expect.expectPorts = portsByService["mqtt-broker"]
		case strings.HasSuffix(sts.name, "-postgres"):
			expect.expectPorts = portsByService["postgres"]
			// postgres 官方镜像 entrypoint 需要 root chown——对齐 compose 不做 cap_drop。
			expect.dropAllCaps = false
			expect.runAsNonRoot = false
		case strings.HasSuffix(sts.name, "-redis"):
			// compose 中 redis 完全不发布端口（仅内网）；K8s 容器/Service 需要显式 6379，属必要差异。
			expect.expectPorts = map[int]bool{6379: true}
			expect.dropAllCaps = false
			expect.runAsNonRoot = false
		default:
			t.Errorf("出现未预期的 StatefulSet: %s", sts.name)
		}
		assertWorkloadShape(t, sts, expect)
	}
	for _, dep := range docsOfKind(t, docs, "Deployment") {
		if !strings.HasSuffix(dep.name, "-frontend") {
			t.Errorf("出现未预期的 Deployment: %s", dep.name)
			continue
		}
		assertWorkloadShape(t, dep, workloadExpect{dropAllCaps: true, expectPorts: portsByService["frontend"]})
	}

	// Service 结构：端口 + targetPort + selector。
	for _, svc := range docsOfKind(t, docs, "Service") {
		ports, ok := svc.doc["spec"].(map[string]any)["ports"].([]any)
		if !ok || len(ports) == 0 {
			t.Errorf("Service %s 缺少 ports", svc.name)
			continue
		}
		selector, ok := svc.doc["spec"].(map[string]any)["selector"].(map[string]any)
		if !ok || len(selector) == 0 {
			t.Errorf("Service %s 缺少 selector", svc.name)
		}
		headless := strings.Contains(svc.name, "-headless")
		if headless {
			if svc.doc["spec"].(map[string]any)["clusterIP"] != "None" {
				t.Errorf("headless Service %s 必须 clusterIP: None", svc.name)
			}
		}
		for _, p := range ports {
			pm, _ := p.(map[string]any)
			if pm["port"] == nil || pm["targetPort"] == nil {
				t.Errorf("Service %s 端口条目缺少 port/targetPort: %v", svc.name, pm)
			}
		}
	}

	// 与根 docker-compose.yml 对齐：backend/broker env 键集合（端口已随结构断言完成比对）。
	backendWorkload := mustWorkloadByName(t, docs, testRelease().Name+"-backend")
	backendEnv := envNamesAndValues(t, mustFirstContainer(t, backendWorkload))
	chartBackendKeys := make(map[string]struct{}, len(backendEnv))
	for k := range backendEnv {
		chartBackendKeys[k] = struct{}{}
	}
	assertSortedStringSetEqual(t, "backend env", chartBackendKeys, composeServiceEnvKeys(t, compose, "backend"))

	brokerWorkload := mustWorkloadByName(t, docs, testRelease().Name+"-mqtt-broker")
	brokerEnv := envNamesAndValues(t, mustFirstContainer(t, brokerWorkload))
	chartBrokerKeys := make(map[string]struct{}, len(brokerEnv))
	for k := range brokerEnv {
		chartBrokerKeys[k] = struct{}{}
	}
	assertSortedStringSetEqual(t, "mqtt-broker env", chartBrokerKeys, composeServiceEnvKeys(t, compose, "mqtt-broker"))

	// 探针与 compose healthcheck 的路径口径对齐（compose 命令串包含目标路径）。
	assertComposeHealthcheckMentions(t, compose, "backend", "/ready")
	assertProbePath(t, backendWorkload, "readinessProbe", "/ready")
	assertProbePath(t, backendWorkload, "livenessProbe", "/health")
	assertComposeHealthcheckMentions(t, compose, "mqtt-broker", "/metrics")
	assertProbePath(t, brokerWorkload, "readinessProbe", "/metrics")
	assertProbePath(t, brokerWorkload, "livenessProbe", "/metrics")

	// 默认密钥注入路径：backend/broker env 的 secretKeyRef 指向 chart Secret。
	defaultSecretName := testRelease().Name + "-aetherlink"
	jwtEnv, has := backendEnv["GOTP_JWT_KEY"].(map[string]any)
	if !has {
		t.Fatalf("GOTP_JWT_KEY 应使用 valueFrom")
	}
	ref, _ := jwtEnv["secretKeyRef"].(map[string]any)
	if ref["name"] != defaultSecretName || ref["key"] != "jwt-key" {
		t.Errorf("GOTP_JWT_KEY secretKeyRef 口径错误: %v", ref)
	}
	// 默认单副本下 REQUIRED_BROKER_IDS 应只有一个 broker ID。
	ids, _ := backendEnv["GOTP_MQTT_SESSION_REVOCATIONS_REQUIRED_BROKER_IDS"].(string)
	if ids != testRelease().Name+"-mqtt-broker-0 " {
		t.Errorf("单副本 REQUIRED_BROKER_IDS 期望 %q，实际 %q", testRelease().Name+"-mqtt-broker-0 ", ids)
	}
}

// TestMultiReplicaScenarioInjectsBrokerIdentities 验证双副本渲染：broker per-Pod 身份与
// backend 全集 required_broker_ids，并钉住 spool per-pod / files RWX 前提在 values 的默认口径。
func TestMultiReplicaScenarioInjectsBrokerIdentities(t *testing.T) {
	values := defaultValues(t)
	setPath(t, values, "backend.replicas", 2)
	setPath(t, values, "broker.replicas", 2)
	docs := renderDocs(t, values)

	backend := mustWorkloadByName(t, docs, testRelease().Name+"-backend")
	backendEnv := envNamesAndValues(t, mustFirstContainer(t, backend))
	want := testRelease().Name + "-mqtt-broker-0 " + testRelease().Name + "-mqtt-broker-1 "
	ids, _ := backendEnv["GOTP_MQTT_SESSION_REVOCATIONS_REQUIRED_BROKER_IDS"].(string)
	if ids != want {
		t.Errorf("双副本 REQUIRED_BROKER_IDS 期望 %q（空格分隔，viper GetStringSlice 按空白切分），实际 %q", want, ids)
	}

	broker := mustWorkloadByName(t, docs, testRelease().Name+"-mqtt-broker")
	brokerEnv := envNamesAndValues(t, mustFirstContainer(t, broker))
	brokerID, has := brokerEnv["GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID"].(map[string]any)
	if !has {
		t.Fatalf("GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID 应使用 valueFrom")
	}
	fieldRef, _ := brokerID["fieldRef"].(map[string]any)
	if fieldRef["fieldPath"] != "metadata.name" {
		t.Errorf("broker_id 必须取 Pod 名（StatefulSet 稳定身份），实际 %v", fieldRef)
	}

	// spool 卷 per-pod（volumeClaimTemplates 各自独立），files 卷默认 RWO——多副本必须手工改 RWX
	// 的边界以 values 默认值口径钉住，防止有人误改成默认共享。
	filesAccessModes, ok := getPath(t, values, "backend.storage.files.accessModes").([]any)
	if !ok || len(filesAccessModes) != 1 || filesAccessModes[0] != "ReadWriteOnce" {
		t.Errorf("values 默认 files.accessModes 必须为 [ReadWriteOnce]（多副本 RWX 属部署侧显式决定）: %v", filesAccessModes)
	}
}

// TestEmptySecretsFailFast 验证 secrets 全空时渲染必须失败（拒绝空密钥安装）。
func TestEmptySecretsFailFast(t *testing.T) {
	values := loadYAMLMap(t, filepath.Join(repoRoot(t), "deploy", "helm", "aetherlink", "values.yaml"))
	_, err := RenderChart(filepath.Join(repoRoot(t), "deploy", "helm", "aetherlink"), testRelease(), testChart(), values)
	if err == nil {
		t.Fatal("secrets 为空时渲染应当失败（required fail-fast），实际成功")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("失败原因应来自 required 校验，实际: %v", err)
	}
}

// TestDependencySwitches 验证 postgresql/redis 依赖开关：关闭后不渲染对应对象，
// backend/broker 的数据面地址切到 external.*。
func TestDependencySwitches(t *testing.T) {
	values := defaultValues(t)
	setPath(t, values, "postgresql.enabled", false)
	setPath(t, values, "redis.enabled", false)
	setPath(t, values, "external.postgresHost", "pg-external.example")
	setPath(t, values, "external.redisAddr", "redis-external.example:6379")
	docs := renderDocs(t, values)

	for _, name := range []string{testRelease().Name + "-postgres", testRelease().Name + "-redis"} {
		for _, list := range docs {
			for _, d := range list {
				if d.name == name {
					t.Errorf("依赖关闭后不应渲染 %s（kind=%s）", name, d.kind)
				}
			}
		}
	}
	backend := mustWorkloadByName(t, docs, testRelease().Name+"-backend")
	backendEnv := envNamesAndValues(t, mustFirstContainer(t, backend))
	if got, _ := backendEnv["GOTP_DB_PSQL_HOST"].(string); got != "pg-external.example" {
		t.Errorf("外部 PG 主机未注入 backend: %q", got)
	}
	if got, _ := backendEnv["GOTP_DB_REDIS_ADDR"].(string); got != "redis-external.example:6379" {
		t.Errorf("外部 Redis 地址未注入 backend: %q", got)
	}
	broker := mustWorkloadByName(t, docs, testRelease().Name+"-mqtt-broker")
	brokerEnv := envNamesAndValues(t, mustFirstContainer(t, broker))
	if got, _ := brokerEnv["GMQTT_DB_REDIS_CONN"].(string); got != "redis-external.example:6379" {
		t.Errorf("外部 Redis 地址未注入 broker: %q", got)
	}
	assertKindCount(t, docs, "StatefulSet", 2) // 仅剩 backend + broker
	assertKindCount(t, docs, "Service", 5)     // backend/broker 各 2 + frontend 1
}

// TestComposeReplicasOverrideDeclaresDualReplicaPremises 校验 deploy/docker-compose.replicas.yml：
// backend 与 mqtt-broker 双副本、移除宿主机端口发布（!override）、且注明三条硬前提。
func TestComposeReplicasOverrideDeclaresDualReplicaPremises(t *testing.T) {
	path := filepath.Join(repoRoot(t), "deploy", "docker-compose.replicas.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 replicas override: %v", err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("解析 replicas override: %v", err)
	}
	services, ok := parsed["services"].(map[string]any)
	if !ok {
		t.Fatal("replicas override 缺少 services")
	}
	for _, svc := range []string{"backend", "mqtt-broker"} {
		block, ok := services[svc].(map[string]any)
		if !ok {
			t.Fatalf("replicas override 缺少服务 %s", svc)
		}
		deploy, _ := block["deploy"].(map[string]any)
		if deploy == nil || deploy["replicas"] != 2 {
			t.Errorf("%s 的 deploy.replicas 必须为 2，实际 %v", svc, deploy)
		}
		if ports, has := block["ports"]; has && ports != nil {
			// !override [] 被 yaml 解析为空切片；只要非空即视为未移除发布端口。
			if list, isList := ports.([]any); !isList || len(list) != 0 {
				t.Errorf("%s 必须以 ports: !override [] 移除宿主机发布（避免第二副本端口冲突），实际 %v", svc, ports)
			}
		}
	}
	text := string(raw)
	for _, marker := range []string{"共享存储", "外部 PG", "deploy/helm/aetherlink", "MQTT_BROKER_ID", "broker_id"} {
		if !strings.Contains(text, marker) {
			t.Errorf("replicas override 缺少前提标注关键词 %q", marker)
		}
	}
}

// ---- 辅助函数 ----

func assertKindCount(t *testing.T, docs map[string][]docInfo, kind string, want int) {
	t.Helper()
	if got := len(docsOfKind(t, docs, kind)); got != want {
		t.Errorf("kind=%s 数量期望 %d 实际 %d", kind, want, got)
	}
}

func mustWorkloadByName(t *testing.T, docs map[string][]docInfo, name string) docInfo {
	t.Helper()
	for _, list := range docs {
		for _, d := range list {
			if d.name == name && (d.kind == "StatefulSet" || d.kind == "Deployment") {
				return d
			}
		}
	}
	t.Fatalf("未找到工作负载 %s", name)
	return docInfo{}
}

func mustFirstContainer(t *testing.T, workload docInfo) map[string]any {
	t.Helper()
	containers, ok := getPath(t, workload.doc, "spec.template.spec.containers").([]any)
	if !ok || len(containers) == 0 {
		t.Fatalf("%s 缺少容器", workload.name)
	}
	c, _ := containers[0].(map[string]any)
	return c
}

func assertProbePath(t *testing.T, workload docInfo, probe, want string) {
	t.Helper()
	p, ok := getPath(t, workload.doc, "spec.template.spec.containers").([]any)
	if !ok {
		t.Fatal("容器缺失")
	}
	c, _ := p[0].(map[string]any)
	pr, ok := c[probe].(map[string]any)
	if !ok {
		t.Fatalf("%s 缺少 %s", workload.name, probe)
	}
	httpGet, _ := pr["httpGet"].(map[string]any)
	if httpGet == nil || httpGet["path"] != want {
		t.Errorf("%s %s path 期望 %s 实际 %v", workload.name, probe, want, httpGet)
	}
}

func assertComposeHealthcheckMentions(t *testing.T, compose map[string]any, service, needle string) {
	t.Helper()
	svc := compose["services"].(map[string]any)[service].(map[string]any)
	hc, ok := svc["healthcheck"].(map[string]any)
	if !ok {
		t.Fatalf("compose 服务 %s 缺少 healthcheck", service)
	}
	test, _ := hc["test"].([]any)
	joined := ""
	for _, line := range test {
		s, _ := line.(string)
		joined += s + " "
	}
	if !strings.Contains(joined, needle) {
		t.Errorf("compose %s healthcheck 应包含 %q（chart 探针口径基准）: %q", service, needle, joined)
	}
}

// nginxLocationRe 提取 location 指令（含 = 修饰）。
var nginxLocationRe = regexp.MustCompile(`(?m)^\s*location\s+([^{\s]+)\s*\{`)

// assertNginxConfigAligned 把渲染出的 nginx.conf 与 frontend/nginx.conf 做结构比对：
// location 集合必须一致（除 proxy_pass 目标随 Release 名替换外不允许漂移）。
func assertNginxConfigAligned(t *testing.T, rendered, releaseName string) {
	t.Helper()
	sourceRaw, err := os.ReadFile(filepath.Join(repoRoot(t), "frontend", "nginx.conf"))
	if err != nil {
		t.Fatalf("读取 frontend/nginx.conf: %v", err)
	}
	source := string(sourceRaw)
	got := nginxLocationRe.FindAllStringSubmatch(rendered, -1)
	want := nginxLocationRe.FindAllStringSubmatch(source, -1)
	gotSet := map[string]struct{}{}
	wantSet := map[string]struct{}{}
	for _, m := range got {
		gotSet[m[1]] = struct{}{}
	}
	for _, m := range want {
		wantSet[m[1]] = struct{}{}
	}
	assertSortedStringSetEqual(t, "nginx location", gotSet, wantSet)
	if !strings.Contains(rendered, "listen       8080;") {
		t.Errorf("渲染 nginx.conf 必须保持 listen 8080（compose 容器口径）")
	}
	proxyTarget := "http://" + releaseName + "-backend:9999"
	if !strings.Contains(rendered, "proxy_pass  "+proxyTarget+";") {
		t.Errorf("渲染 nginx.conf 的 /api、/files proxy_pass 必须指向 %s", proxyTarget)
	}
	if strings.Contains(rendered, "http://backend:9999") {
		t.Errorf("渲染 nginx.conf 不得残留 compose 镜像内的 backend:9999 目标")
	}
}

// setPath 覆盖 values 嵌套键（仅测试用）。
func setPath(t *testing.T, values map[string]any, path string, value any) {
	t.Helper()
	segs := strings.Split(path, ".")
	cur := values
	for _, seg := range segs[:len(segs)-1] {
		next, ok := cur[seg].(map[string]any)
		if !ok {
			t.Fatalf("setPath: %s 中段 %s 不是映射", path, seg)
		}
		cur = next
	}
	cur[segs[len(segs)-1]] = value
}
