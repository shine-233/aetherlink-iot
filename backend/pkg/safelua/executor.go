// Package safelua executes tenant-provided data scripts with a bounded runtime
// and a deliberately small module surface.
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
func Execute(ctx context.Context, code string, msg []byte, topic string) (result string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	execCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	L := lua.NewState()
	defer L.Close()
	L.SetContext(execCtx)
	setupSandbox(L)

	defer func() {
		if recovered := recover(); recovered != nil {
			result = ""
			err = fmt.Errorf("lua script panic: %v", recovered)
		}
	}()

	if err := L.DoString(code); err != nil {
		if execCtx.Err() != nil {
			return "", execCtx.Err()
		}
		return "", err
	}

	entrypoint := L.GetGlobal("encodeInp")
	if entrypoint.Type() != lua.LTFunction {
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
	if value.Type() != lua.LTString {
		return "", &lua.ApiError{
			Type:   lua.ApiErrorRun,
			Object: lua.LString("script must return a string"),
		}
	}
	return value.String(), nil
}

func setupSandbox(L *lua.LState) {
	L.PreloadModule("json", luajson.Loader)
	originalRequire := L.GetGlobal("require")
	if err := L.CallByParam(lua.P{
		Fn:      originalRequire,
		NRet:    1,
		Protect: true,
	}, lua.LString("json")); err != nil {
		panic(fmt.Sprintf("preload json module: %v", err))
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
}
