package safelua

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// executeFreshState reproduces the pre-cache implementation (new VM + DoString
// per message). It is the benchmark baseline and the semantic oracle.
func executeFreshState(ctx context.Context, code string, msg []byte, topic string) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	L := lua.NewState()
	defer L.Close()
	L.SetContext(execCtx)
	if err := setupSandbox(L); err != nil {
		return "", err
	}
	if err := L.DoString(code); err != nil {
		return "", err
	}
	if err := L.CallByParam(lua.P{Fn: L.GetGlobal("encodeInp"), NRet: 1, Protect: true}, lua.LString(msg), lua.LString(topic)); err != nil {
		return "", err
	}
	v := L.Get(-1)
	L.Pop(1)
	return v.String(), nil
}

const leakyScript = `
counter = (counter or 0) + 1
local json = require("json")
function encodeInp(msg, topic)
  hits = (hits or 0) + 1
  string.leak = "x"
  json.leak = true
  _G.injected = "yes"
  local mt = debug.getmetatable("")
  mt.__index.evil = function() return "evil" end
  return tostring(counter) .. ":" .. tostring(hits) .. ":" .. tostring(rawlen)
end
`

func TestPooledStateDoesNotLeakGlobalsBetweenRuns(t *testing.T) {
	p, err := Compile(leakyScript)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for i := 0; i < 5; i++ {
		got, err := p.Run(context.Background(), nil, "")
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if got != "1:1:nil" {
			t.Fatalf("run %d = %q, want 1:1:nil (state leaked)", i, got)
		}
	}
	if p.IdleStates() != 1 {
		t.Fatalf("idle states = %d, want 1 (state should be reused)", p.IdleStates())
	}

	probe, err := Compile(`function encodeInp()
  local json = require("json")
  return tostring(injected) .. tostring(string.leak) .. tostring(json.leak) .. tostring(("").evil)
end`)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	// Reuse the exact state the leaky program dirtied.
	st, _ := p.acquire()
	probe.release(st, true)
	got, err := probe.Run(context.Background(), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "nilnilnilnil" {
		t.Fatalf("cross-program leak: %q", got)
	}
}

func TestPooledStateRestoresSandboxAfterTampering(t *testing.T) {
	p, err := Compile(`function encodeInp()
  require = function() return {} end
  tostring = nil
  setfenv(1, {})
  return "tampered"
end`)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Run(context.Background(), nil, ""); err != nil {
		t.Fatal(err)
	}
	st, _ := p.acquire()
	check, _ := Compile(`function encodeInp()
  local ok = pcall(require, "os")
  return tostring(ok) .. type(require("json").encode) .. tostring(os) .. tostring(load)
end`)
	defer check.Close()
	check.release(st, true)
	got, err := check.Run(context.Background(), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "falsefunctionnilnil" {
		t.Fatalf("sandbox not restored: %q", got)
	}
}

func TestFailedRunsDiscardState(t *testing.T) {
	p, _ := Compile(`function encodeInp() error("boom") end`)
	defer p.Close()
	if _, err := p.Run(context.Background(), nil, ""); err == nil {
		t.Fatal("want error")
	}
	if p.IdleStates() != 0 {
		t.Fatalf("errored state was pooled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	loop, _ := Compile(`function encodeInp() while true do end end`)
	defer loop.Close()
	if _, err := loop.Run(ctx, nil, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if loop.IdleStates() != 0 {
		t.Fatalf("timed-out state was pooled")
	}
}

func TestSyntaxErrorMatchesDoString(t *testing.T) {
	code := `function encodeInp( return end`
	_, want := executeFreshState(context.Background(), code, nil, "")
	_, got := Execute(context.Background(), code, nil, "")
	if want == nil || got == nil || got.Error() != want.Error() {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCacheKeyedReplacementAndInvalidate(t *testing.T) {
	c := NewCache(2)
	run := func(key, code string) string {
		out, err := c.Execute(context.Background(), key, code, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	v1 := `function encodeInp() return "v1" end`
	v2 := `function encodeInp() return "v2" end`
	if run("cfg:A", v1) != "v1" || run("cfg:A", v2) != "v2" {
		t.Fatal("content change not honored")
	}
	if c.Len() != 1 {
		t.Fatalf("len = %d, want old program replaced", c.Len())
	}
	run("cfg:B", v1)
	run("cfg:C", v1)
	if c.Len() != 2 {
		t.Fatalf("len = %d, want LRU bound 2", c.Len())
	}
	c.Invalidate("cfg:C")
	if c.Len() != 1 {
		t.Fatalf("len = %d after invalidate", c.Len())
	}
}

func TestKeyedFastPathReturnsSameProgramAndHonorsContentChange(t *testing.T) {
	c := NewCache(4)
	v1 := `function encodeInp() return "v1" end`
	p1 := c.Program("k", v1)
	if c.Program("k", v1) != p1 {
		t.Fatal("same key+code must reuse the compiled program")
	}
	// Same content under a new string header (not pointer-equal) still hits.
	if c.Program("k", string([]byte(v1))) != p1 {
		t.Fatal("equal content must hit the fast path")
	}
	p2 := c.Program("k", `function encodeInp() return "v2" end`)
	if p2 == p1 {
		t.Fatal("changed content must compile a new program")
	}
	if out, err := p2.Run(context.Background(), nil, ""); err != nil || out != "v2" {
		t.Fatalf("run = %q %v", out, err)
	}
}

func TestReleaseAfterCloseClosesState(t *testing.T) {
	p, _ := Compile(`function encodeInp() return "x" end`)
	st, err := p.acquire()
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
	p.release(st, true)
	if p.IdleStates() != 0 {
		t.Fatal("state released after Close must not be pooled")
	}
}

func TestConcurrentRunsAreIsolated(t *testing.T) {
	p, _ := Compile(`function encodeInp(msg) n = (n or 0) + 1; return msg .. ":" .. n end`)
	defer p.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := fmt.Sprint(i)
			got, err := p.Run(context.Background(), []byte(msg), "")
			if err != nil || got != msg+":1" {
				errs <- fmt.Errorf("run %d = %q, %v", i, got, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

const benchScript = `
local json = require("json")
function encodeInp(msg, topic)
  local payload = json.decode(msg)
  payload.value = payload.value * 2
  payload.topic = topic
  return json.encode(payload)
end
`

func BenchmarkExecuteFreshState(b *testing.B) {
	msg := []byte(`{"value":21,"unit":"C"}`)
	for i := 0; i < b.N; i++ {
		if _, err := executeFreshState(context.Background(), benchScript, msg, "t"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExecuteCached(b *testing.B) {
	msg := []byte(`{"value":21,"unit":"C"}`)
	for i := 0; i < b.N; i++ {
		if _, err := ExecuteKeyed(context.Background(), "bench:A", benchScript, msg, "t"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExecuteCachedParallel(b *testing.B) {
	msg := []byte(`{"value":21,"unit":"C"}`)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			out, err := ExecuteKeyed(context.Background(), "bench:P", benchScript, msg, "t")
			if err != nil || !strings.Contains(out, "42") {
				b.Fatal(out, err)
			}
		}
	})
}
