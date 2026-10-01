// Package safelua executes tenant-provided data scripts with a bounded runtime
// and a deliberately small module surface.
//
// Hot-path design: scripts are parsed and compiled once into an immutable
// *lua.FunctionProto (a Program). Each Program keeps a small bounded pool of
// sandboxed LStates. A run takes a state, executes the compiled chunk (so
// top-level code keeps the exact per-message semantics of the old
// "DoString every time" implementation) and calls encodeInp. On release the
// state is restored to its pristine post-sandbox snapshot (globals, library
// tables, metatables, thread environment), so nothing a script writes can be
// observed by a later run. States that saw any error, timeout or panic are
// closed instead of pooled.
package safelua

import (
	"context"
	"fmt"
	"time"

	luajson "github.com/layeh/gopher-json"
	lua "github.com/yuin/gopher-lua"
)

const DefaultTimeout = 3 * time.Second

// Execute runs encodeInp(msg, topic). The supplied context may impose a
// shorter deadline, while DefaultTimeout remains the hard upper bound.
// The compiled program is cached by content hash in the package default cache.
func Execute(ctx context.Context, code string, msg []byte, topic string) (string, error) {
	return defaultCache.Execute(ctx, "", code, msg, topic)
}

// ExecuteKeyed is Execute with a caller identity (for example
// "<deviceConfigID>:<scriptType>"). A key holds at most one compiled program;
// when the script content changes under the same key, the old program and its
// pooled states are dropped.
func ExecuteKeyed(ctx context.Context, key, code string, msg []byte, topic string) (string, error) {
	return defaultCache.Execute(ctx, key, code, msg, topic)
}

// Invalidate drops the compiled program cached under key (no-op when absent).
func Invalidate(key string) { defaultCache.Invalidate(key) }

// Run executes the program once. See Execute for timeout semantics.
func (p *Program) Run(ctx context.Context, msg []byte, topic string) (result string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	execCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	st, err := p.acquire()
	if err != nil {
		return "", err
	}
	healthy := false
	defer func() {
		if recovered := recover(); recovered != nil {
			result = ""
			err = fmt.Errorf("lua script panic: %v", recovered)
			healthy = false
		}
		p.release(st, healthy)
	}()

	L := st.L
	L.SetContext(execCtx)

	L.Push(L.NewFunctionFromProto(p.proto))
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		if execCtx.Err() != nil {
			return "", execCtx.Err()
		}
		return "", err
	}
	L.SetTop(0)

	entrypoint := L.GetGlobal("encodeInp")
	if entrypoint.Type() != lua.LTFunction {
		healthy = true
		return "", &lua.ApiError{
			Type:   lua.ApiErrorRun,
			Object: lua.LString("function 'encodeInp' not found in script"),
		}
	}

	if err := L.CallByParam(lua.P{
		Fn:      entrypoint,
		NRet:    1,
		Protect: true,
	}, lua.LString(msg), lua.LString(topic)); err != nil {
		if execCtx.Err() != nil {
			return "", execCtx.Err()
		}
		return "", err
	}

	value := L.Get(-1)
	L.Pop(1)
	healthy = true
	if value.Type() != lua.LTString {
		return "", &lua.ApiError{
			Type:   lua.ApiErrorRun,
			Object: lua.LString("script must return a string"),
		}
	}
	return value.String(), nil
}

func setupSandbox(L *lua.LState) error {
	L.PreloadModule("json", luajson.Loader)
	originalRequire := L.GetGlobal("require")
	if err := L.CallByParam(lua.P{
		Fn:      originalRequire,
		NRet:    1,
		Protect: true,
	}, lua.LString("json")); err != nil {
		return fmt.Errorf("preload json module: %w", err)
	}
	jsonModule := L.Get(-1)
	L.Pop(1)
	L.SetGlobal("require", L.NewFunction(func(L *lua.LState) int {
		moduleName := L.CheckString(1)
		if moduleName != "json" {
			L.RaiseError("module %q is not allowed", moduleName)
			return 0
		}
		L.Push(jsonModule)
		return 1
	}))

	for _, name := range []string{
		"os", "io", "package", "dofile", "loadfile", "load", "loadstring",
		"rawget", "rawset", "setmetatable", "getmetatable",
	} {
		L.SetGlobal(name, lua.LNil)
	}
	return nil
}
