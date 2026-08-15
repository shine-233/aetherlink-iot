package main

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateSyntheticIdentityIsDeterministicAndExplicitlySynthetic(t *testing.T) {
	first, err := GenerateSyntheticIdentity("deployment-prep")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}
	second, err := GenerateSyntheticIdentity("deployment-prep")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error on repeat: %v", err)
	}

	if first != second {
		t.Fatalf("identity is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.Mode != "synthetic-rdi" || first.Provenance != "synthetic-rdi" {
		t.Fatalf("identity provenance is not synthetic-rdi: %#v", first)
	}
	if !regexp.MustCompile(SyntheticPIDPattern).MatchString(first.PID) {
		t.Fatalf("synthetic PID does not satisfy the isolated SYN namespace: %q", first.PID)
	}
	if first.Voucher.Username == "" || first.Voucher.Password == "" {
		t.Fatalf("synthetic voucher is incomplete: %#v", first.Voucher)
	}
	if first.Hardware.Kind != "synthetic" || first.Hardware.Serial == "" {
		t.Fatalf("hardware identity is not explicitly synthetic: %#v", first.Hardware)
	}
}

func TestOverrideSyntheticIdentityBindsToDatabaseFixtureWithoutChangingProvenance(t *testing.T) {
	identity, err := GenerateSyntheticIdentity("fixture-binding")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}
	override, err := OverrideSyntheticIdentity(identity, "synthrdi0001", "fixture-device-123")
	if err != nil {
		t.Fatalf("OverrideSyntheticIdentity returned error: %v", err)
	}
	if override.PID != "SYNTHRDI0001" || override.DeviceID != "fixture-device-123" {
		t.Fatalf("fixture identity override mismatch: %#v", override)
	}
	if override.Mode != SyntheticMode || override.Provenance != SyntheticMode {
		t.Fatalf("fixture identity override changed synthetic provenance: %#v", override)
	}
	if _, err := OverrideSyntheticIdentity(identity, "real-rdi", "fixture-device-123"); err == nil {
		t.Fatal("OverrideSyntheticIdentity accepted an invalid PID shape")
	}
	if _, err := OverrideSyntheticIdentity(identity, "ABC123456789", "fixture-device-123"); err == nil {
		t.Fatal("OverrideSyntheticIdentity accepted a non-SYN PID namespace")
	}
}

func TestBuildObservedContractFramesUsesRepositoryTopicsAndPayloads(t *testing.T) {
	identity, err := GenerateSyntheticIdentity("frame-contract")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}

	status, err := BuildStatusFrame(identity, true, 1700000000, 1)
	if err != nil {
		t.Fatalf("BuildStatusFrame returned error: %v", err)
	}
	if status.Topic != "devices/status/"+identity.DeviceID || string(status.Payload) != "1" {
		t.Fatalf("status frame does not match the observed direct-device contract: %#v", status)
	}

	telemetry, err := BuildTelemetryFrame(identity, map[string]any{"temperature_1": 25.5}, 1700000001, 2)
	if err != nil {
		t.Fatalf("BuildTelemetryFrame returned error: %v", err)
	}
	if telemetry.Topic != "devices/telemetry" || string(telemetry.Payload) != `{"temperature_1":25.5}` {
		t.Fatalf("telemetry frame does not match the observed direct-device contract: %#v", telemetry)
	}

	command, err := BuildCommandFrame(identity, "msg-001", "identify", map[string]any{"request": true}, 1700000002, 3)
	if err != nil {
		t.Fatalf("BuildCommandFrame returned error: %v", err)
	}
	if command.Topic != "devices/command/"+identity.PID+"/msg-001" {
		t.Fatalf("command topic mismatch: %q", command.Topic)
	}

	ack, err := BuildACKFrame(identity, command, true, 1700000003, 4)
	if err != nil {
		t.Fatalf("BuildACKFrame returned error: %v", err)
	}
	if ack.Topic != "devices/command/response/msg-001" {
		t.Fatalf("ACK topic mismatch: %q", ack.Topic)
	}
	if string(ack.Payload) != `{"message":"success","method":"identify","result":0,"ts":1700000003}` {
		t.Fatalf("ACK payload mismatch: %s", ack.Payload)
	}
}

func TestACKResponderSupportsIndependentSuccessFailureAndRetryModes(t *testing.T) {
	identity, err := GenerateSyntheticIdentity("ack-contract")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}
	command, err := BuildCommandFrame(identity, "msg-ack", "identify", nil, 1700000010, 1)
	if err != nil {
		t.Fatalf("BuildCommandFrame returned error: %v", err)
	}

	responder, err := NewACKResponder(ACKFailOnce)
	if err != nil {
		t.Fatalf("NewACKResponder returned error: %v", err)
	}
	first, err := responder.Respond(identity, command, 1700000011, 2)
	if err != nil {
		t.Fatalf("first ACK returned error: %v", err)
	}
	second, err := responder.Respond(identity, command, 1700000012, 3)
	if err != nil {
		t.Fatalf("second ACK returned error: %v", err)
	}
	if string(first.Payload) != `{"message":"failed","method":"identify","result":1,"ts":1700000011}` {
		t.Fatalf("first fail-once ACK mismatch: %s", first.Payload)
	}
	if string(second.Payload) != `{"message":"success","method":"identify","result":0,"ts":1700000012}` {
		t.Fatalf("second fail-once ACK mismatch: %s", second.Payload)
	}
}

func TestACKResponderFailOnceTracksEachMessageIDIndependently(t *testing.T) {
	identity, err := GenerateSyntheticIdentity("ack-message-scope")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}
	firstCommand, err := BuildCommandFrame(identity, "msg-one", "identify", nil, 1700000013, 1)
	if err != nil {
		t.Fatalf("first command returned error: %v", err)
	}
	secondCommand, err := BuildCommandFrame(identity, "msg-two", "identify", nil, 1700000014, 2)
	if err != nil {
		t.Fatalf("second command returned error: %v", err)
	}
	responder, err := NewACKResponder(ACKFailOnce)
	if err != nil {
		t.Fatalf("NewACKResponder returned error: %v", err)
	}

	first, err := responder.Respond(identity, firstCommand, 1700000015, 3)
	if err != nil {
		t.Fatalf("first message ACK returned error: %v", err)
	}
	second, err := responder.Respond(identity, secondCommand, 1700000016, 4)
	if err != nil {
		t.Fatalf("second message ACK returned error: %v", err)
	}
	if !strings.Contains(string(first.Payload), `"result":1`) || !strings.Contains(string(second.Payload), `"result":1`) {
		t.Fatalf("fail-once must fail the first attempt of each message ID: first=%s second=%s", first.Payload, second.Payload)
	}
}

func TestReplaySessionPreservesOrderAndRejectsRealRDIProvenance(t *testing.T) {
	identity, err := GenerateSyntheticIdentity("replay-contract")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}
	status, err := BuildStatusFrame(identity, true, 1700000020, 1)
	if err != nil {
		t.Fatalf("BuildStatusFrame returned error: %v", err)
	}
	telemetry, err := BuildTelemetryFrame(identity, map[string]any{"temperature_1": 26}, 1700000021, 2)
	if err != nil {
		t.Fatalf("BuildTelemetryFrame returned error: %v", err)
	}
	session, err := NewReplaySession(identity, []Frame{status, telemetry})
	if err != nil {
		t.Fatalf("NewReplaySession returned error: %v", err)
	}

	var observed []string
	err = ReplayFrames(context.Background(), session, func(frame Frame) error {
		observed = append(observed, frame.Topic)
		return nil
	})
	if err != nil {
		t.Fatalf("ReplayFrames returned error: %v", err)
	}
	if got := len(observed); got != 2 || observed[0] != status.Topic || observed[1] != telemetry.Topic {
		t.Fatalf("replay order mismatch: %#v", observed)
	}

	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal replay session: %v", err)
	}
	var tampered ReplaySession
	if err := json.Unmarshal(encoded, &tampered); err != nil {
		t.Fatalf("unmarshal replay session: %v", err)
	}
	tampered.Frames[0].Provenance = "real-rdi"
	if _, err := NewReplaySession(identity, tampered.Frames); err == nil {
		t.Fatal("NewReplaySession accepted frames with tampered real-rdi provenance")
	}
}

func TestReplayValidationRejectsTopicAndPayloadMismatches(t *testing.T) {
	identity, err := GenerateSyntheticIdentity("replay-validation")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}
	status, err := BuildStatusFrame(identity, true, 1700000030, 1)
	if err != nil {
		t.Fatalf("BuildStatusFrame returned error: %v", err)
	}

	tests := []struct {
		name  string
		frame Frame
	}{
		{name: "status device topic", frame: func() Frame {
			copy := status
			copy.Topic = "devices/status/wrong-device"
			return copy
		}()},
		{name: "status payload", frame: func() Frame {
			copy := status
			copy.Payload = json.RawMessage(`{"online":true}`)
			return copy
		}()},
		{name: "unsupported kind", frame: func() Frame {
			copy := status
			copy.Kind = "real-rdi"
			return copy
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewReplaySession(identity, []Frame{test.frame}); err == nil {
				t.Fatalf("NewReplaySession accepted invalid %s frame: %#v", test.name, test.frame)
			}
		})
	}
}

func TestPublicManifestNeverEmitsSyntheticVoucherSecretAsEvidence(t *testing.T) {
	identity, err := GenerateSyntheticIdentity("manifest-contract")
	if err != nil {
		t.Fatalf("GenerateSyntheticIdentity returned error: %v", err)
	}
	manifest := identity.PublicManifest()
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal public manifest: %v", err)
	}
	if string(encoded) == "" || string(encoded) == identity.Voucher.Password {
		t.Fatalf("public manifest is empty or exposes the raw voucher secret: %s", encoded)
	}
	if string(encoded) == "" || strings.Contains(string(encoded), identity.Voucher.Password) {
		t.Fatalf("public manifest exposes the voucher password: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"voucher_secret_redacted":true`) {
		t.Fatalf("public manifest does not record secret redaction: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"real_rdi_status":"not-tested"`) {
		t.Fatalf("public manifest does not preserve the real-RDI boundary: %s", encoded)
	}
}

func TestDeviceSignalRegistrationIncludesSIGTERM(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var registered bool
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		function, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || function.Sel.Name != "NotifyContext" {
			return true
		}
		packageName, ok := function.X.(*ast.Ident)
		if !ok || packageName.Name != "signal" {
			return true
		}
		for _, argument := range call.Args[1:] {
			signalName, ok := argument.(*ast.SelectorExpr)
			if !ok || signalName.Sel.Name != "SIGTERM" {
				continue
			}
			packageName, ok := signalName.X.(*ast.Ident)
			if ok && packageName.Name == "syscall" {
				registered = true
				return false
			}
		}
		return true
	})

	if !registered {
		t.Fatal("device signal context must register syscall.SIGTERM so shutdown can publish synthetic-rdi offline status")
	}
}
