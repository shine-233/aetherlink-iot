package gmqtt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMockGenerationContract keeps generated mocks inside the checkout and
// prevents local GOPATH layouts from becoming part of the build contract.
func TestMockGenerationContract(t *testing.T) {
	content, err := os.ReadFile("mock_gen.sh")
	if err != nil {
		t.Fatalf("read mock_gen.sh: %v", err)
	}

	script := string(content)
	if strings.Contains(script, "/usr/local/gopath") {
		t.Fatal("mock_gen.sh must not write to a machine-specific GOPATH")
	}
	if !strings.Contains(script, "github.com/golang/mock/mockgen@v1.6.0") {
		t.Fatal("mock_gen.sh must document the pinned mockgen v1.6.0 tool")
	}

	makefileContent, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(makefileContent)
	if !strings.Contains(makefile, "go install github.com/golang/mock/mockgen@v1.6.0") {
		t.Fatal("tools target must install the pinned mockgen v1.6.0 tool")
	}
	if strings.Contains(makefile, "go get github.com/golang/mock/mockgen") {
		t.Fatal("tools target must not mutate module dependencies while installing mockgen")
	}

	destination := "./plugin/federation/federation_grpc.pb_mock.go"
	if !strings.Contains(script, "-destination="+destination) {
		t.Fatalf("reflection mock must be generated inside the repository at %s", destination)
	}

	if filepath.IsAbs(strings.TrimPrefix(destination, "./")) {
		t.Fatalf("mock destination must remain repository-relative: %s", destination)
	}

	for _, mapping := range []struct {
		source      string
		selfPackage string
	}{
		{"persistence/queue/elem.go", "github.com/DrmagicE/gmqtt/persistence/queue"},
		{"persistence/queue/queue.go", "github.com/DrmagicE/gmqtt/persistence/queue"},
		{"persistence/session/session.go", "github.com/DrmagicE/gmqtt/persistence/session"},
		{"persistence/subscription/subscription.go", "github.com/DrmagicE/gmqtt/persistence/subscription"},
		{"persistence/unack/unack.go", "github.com/DrmagicE/gmqtt/persistence/unack"},
	} {
		expected := "-source=" + mapping.source
		if !strings.Contains(script, expected) {
			t.Errorf("mock generation source must remain declared: %s", mapping.source)
		}
		if !strings.Contains(script, expected+" ") || !strings.Contains(script, "-self_package="+mapping.selfPackage) {
			t.Errorf("mock source %s must use self package %s", mapping.source, mapping.selfPackage)
		}
	}
}

// TestProtoGenerationContract prevents maintainer-only generation targets from
// reporting success when their external toolchain is unavailable.
func TestProtoGenerationContract(t *testing.T) {
	content, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}

	makefile := string(content)
	if strings.Contains(makefile, "To Be implemented") {
		t.Fatal("proto generation targets must not be successful placeholders")
	}

	for _, tool := range []string{
		"protoc",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"protoc-gen-grpc-gateway",
		"protoc-gen-swagger",
	} {
		check := "command -v " + tool
		blocker := "external blocker: " + tool + " is required for proto regeneration"
		if !strings.Contains(makefile, check) || !strings.Contains(makefile, blocker) {
			t.Errorf("generate-proto must fail clearly when %s is unavailable", tool)
		}
	}

	for _, target := range []string{"compile-proto", "generate-grpcgw", "generate-swagger"} {
		if !strings.Contains(makefile, target) {
			t.Errorf("compatibility target %s must remain available", target)
		}
	}

	if !strings.Contains(makefile, "generate-grpc: generate-proto") {
		t.Error("generate-grpc must delegate to the guarded generate-proto target")
	}

	for _, path := range []string{
		"plugin/admin/protos/proto_gen.sh",
		"plugin/auth/protos/proto_gen.sh",
		"plugin/federation/protos/proto_gen.sh",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}

		script := string(content)
		for _, required := range []string{
			"#!/usr/bin/env sh",
			"set -eu",
			`SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)`,
			`cd "$SCRIPT_DIR"`,
			"--go-grpc_out=../",
			"--go_out=../",
			"--grpc-gateway_out=../",
			"--swagger_out=../swagger",
			"*.proto",
		} {
			if !strings.Contains(script, required) {
				t.Errorf("%s must preserve cwd-independent generation contract: %s", path, required)
			}
		}
	}
}

// TestDockerRuntimeBoundary keeps source files out of the runtime image while
// preserving the configuration referenced by the container entrypoint.
func TestDockerRuntimeBoundary(t *testing.T) {
	content, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	dockerfile := string(content)
	for _, required := range []string{
		"COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/cmd/gmqttd/gmqttd ./gmqttd",
		"COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/cmd/gmqttd/default_config.yml ./default_config.yml",
		"USER 10001:10001",
		`ENTRYPOINT ["./gmqttd", "start", "-c", "/gmqttd/default_config.yml"]`,
	} {
		if !strings.Contains(dockerfile, required) {
			t.Errorf("Dockerfile must preserve runtime contract: %s", required)
		}
	}

	if strings.Contains(dockerfile, "COPY --from=builder /go/src/app/cmd/gmqttd .") {
		t.Error("runtime image must not copy the complete broker source directory")
	}
}

// TestNormalTargetsDoNotGenerate ensures routine development commands consume
// committed generated artifacts instead of rewriting the checkout.
func TestNormalTargetsDoNotGenerate(t *testing.T) {
	content, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}

	makefile := string(content)
	for _, target := range []string{"run", "build", "test", "build-windows", "test-bench", "binary"} {
		for _, generator := range []string{"go-generate", "generate-mocks", "generate-proto"} {
			dependency := target + ": " + generator
			if strings.Contains(makefile, dependency) {
				t.Errorf("normal target %s must not depend on maintainer generator %s", target, generator)
			}
		}
	}

	for _, maintainerTarget := range []string{"generate-mocks:", "go-generate:", "generate-proto:"} {
		if !strings.Contains(makefile, maintainerTarget) {
			t.Errorf("explicit maintainer target must remain available: %s", maintainerTarget)
		}
	}
}
