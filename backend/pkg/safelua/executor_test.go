package safelua

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExecuteTransformsJsonAndPropagatesTopic(t *testing.T) {
	result, err := Execute(context.Background(), `
local json = require("json")
function encodeInp(msg, topic)
  local payload = json.decode(msg)
  payload.value = payload.value + 1
  payload.topic = topic
  return json.encode(payload)
end
`, []byte(`{"value":41}`), "devices/one/telemetry")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("result %q is not JSON: %v", result, err)
	}
	if payload["value"] != float64(42) || payload["topic"] != "devices/one/telemetry" {
		t.Fatalf("transformed payload = %#v", payload)
	}
}

func TestExecuteRejectsMissingEntrypointAndNonStringResult(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
	}{
		{name: "missing entrypoint", code: `return true`, message: "function 'encodeInp' not found in script"},
		{name: "non-string result", code: `function encodeInp() return 42 end`, message: "script must return a string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Execute(context.Background(), tt.code, nil, "")
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("error = %v, want message %q", err, tt.message)
			}
		})
	}
}

func TestExecuteSandboxBlocksDangerousGlobalsAndModules(t *testing.T) {
	result, err := Execute(context.Background(), `
function encodeInp()
  if os ~= nil or io ~= nil or package ~= nil or load ~= nil or loadfile ~= nil or dofile ~= nil then
    return "unsafe"
  end
  return "safe"
end
`, nil, "")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result != "safe" {
		t.Fatalf("sandbox result = %q, want safe", result)
	}

	_, err = Execute(context.Background(), `
local forbidden = require("local_file")
function encodeInp() return tostring(forbidden) end
`, nil, "")
	if err == nil || !strings.Contains(err.Error(), `module "local_file" is not allowed`) {
		t.Fatalf("forbidden module error = %v", err)
	}
}

func TestExecuteStopsInfiniteLoopAtContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := Execute(ctx, `function encodeInp() while true do end end`, nil, "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("infinite loop stopped after %s, want under one second", elapsed)
	}
}
