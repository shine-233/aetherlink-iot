// 文件用途：承载设备编解码脚本处理模块的 executor 逻辑。
// 核心逻辑：围绕脚本缓存、Lua 沙箱执行、输入输出模型和处理器接口实现上下行数据转换，主要围绕 type LuaExecutor、func NewLuaExecutor、func (e *LuaExecutor) ExecuteDecode、func (e *LuaExecutor) ExecuteEncode 等声明展开。
// 关键注意事项：脚本处理涉及超时、沙箱和错误码，修改需保持上下行方向及失败语义清晰。
// 重构建议：后续可进一步拆分执行器、缓存和领域模型，降低处理器聚合复杂度。

package processor

import (
	"context"
	"fmt"
	"sync"

	luajson "github.com/layeh/gopher-json"
	"github.com/sirupsen/logrus"
	lua "github.com/yuin/gopher-lua"
)

// LuaExecutor Lua 脚本执行器
type LuaExecutor struct {
}

// NewLuaExecutor 创建 Lua 执行器
func NewLuaExecutor() *LuaExecutor {
	return &LuaExecutor{}
}

// ExecuteDecode 执行解码脚本（上行：设备原始数据 -> JSON）
// scriptContent: 脚本内容
// rawData: 原始字节数据
func (e *LuaExecutor) ExecuteDecode(ctx context.Context, scriptContent string, rawData []byte) (string, error) {
	// 创建带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, ScriptTimeout)
	defer cancel()

	// 创建独立的 Lua 虚拟机
	L := lua.NewState()
	// 使用 sync.Once 确保虚拟机只关闭一次，防止 double-close panic
	var closeOnce sync.Once
	defer closeOnce.Do(func() { L.Close() })

	// 设置沙箱环境和加载必要的模块（必须在启动 goroutine 之前完成）
	e.setupSandbox(L)

	// 在协程中执行脚本（支持超时控制）
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		// 添加 recover 防止 goroutine 内 panic 导致进程崩溃
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("Lua 解码脚本执行 panic: %v", r)
				errChan <- NewScriptExecuteError(fmt.Errorf("script panic: %v", r))
			}
		}()
		result, err := e.executeDecodeScript(L, scriptContent, rawData)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// 等待执行完成或超时
	select {
	case <-execCtx.Done():
		closeOnce.Do(func() { L.Close() }) // 强制关闭 Lua 虚拟机以中断脚本
		return "", NewScriptTimeoutError()
	case err := <-errChan:
		return "", NewScriptExecuteError(err)
	case result := <-resultChan:
		return result, nil
	}
}

// ExecuteEncode 执行编码脚本（下行：JSON -> 设备协议数据）
// scriptContent: 脚本内容
// jsonData: JSON 格式的标准化数据
func (e *LuaExecutor) ExecuteEncode(ctx context.Context, scriptContent string, jsonData []byte) (string, error) {
	// 创建带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, ScriptTimeout)
	defer cancel()

	// 创建独立的 Lua 虚拟机
	L := lua.NewState()
	// 使用 sync.Once 确保虚拟机只关闭一次，防止 double-close panic
	var closeOnce sync.Once
	defer closeOnce.Do(func() { L.Close() })

	// 设置沙箱环境和加载必要的模块（必须在启动 goroutine 之前完成）
	e.setupSandbox(L)

	// 在协程中执行脚本（支持超时控制）
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		// 添加 recover 防止 goroutine 内 panic 导致进程崩溃
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("Lua 编码脚本执行 panic: %v", r)
				errChan <- NewScriptExecuteError(fmt.Errorf("script panic: %v", r))
			}
		}()
		result, err := e.executeEncodeScript(L, scriptContent, jsonData)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// 等待执行完成或超时
	select {
	case <-execCtx.Done():
		closeOnce.Do(func() { L.Close() }) // 强制关闭 Lua 虚拟机以中断脚本
		return "", NewScriptTimeoutError()
	case err := <-errChan:
		return "", NewScriptExecuteError(err)
	case result := <-resultChan:
		return result, nil
	}
}

// executeDecodeScript 执行解码脚本
func (e *LuaExecutor) executeDecodeScript(L *lua.LState, scriptContent string, rawData []byte) (string, error) {
	// 直接执行脚本（DoString 会加载并执行脚本定义的函数）
	if err := L.DoString(scriptContent); err != nil {
		return "", err
	}

	// 调用 encodeInp 函数（兼容现有脚本）
	// 注意：这里函数名保持 encodeInp 是为了兼容现有脚本
	encodeInp := L.GetGlobal("encodeInp")
	if encodeInp.Type() == lua.LTNil {
		return "", &lua.ApiError{
			Type:   lua.ApiErrorRun,
			Object: lua.LString("function 'encodeInp' not found in script"),
		}
	}

	// 调用脚本函数：encodeInp(msg, topic)
	// 新的设计中不使用 topic，传空字符串
	err := L.CallByParam(lua.P{
		Fn:      encodeInp,
		NRet:    1,
		Protect: true,
	}, lua.LString(rawData), lua.LString(""))

	if err != nil {
		return "", err
	}

	// 获取返回值
	result := L.Get(-1)
	L.Pop(1)

	if result.Type() != lua.LTString {
		return "", &lua.ApiError{
			Type:   lua.ApiErrorRun,
			Object: lua.LString("script must return a string"),
		}
	}

	return result.String(), nil
}

// executeEncodeScript 执行编码脚本
func (e *LuaExecutor) executeEncodeScript(L *lua.LState, scriptContent string, jsonData []byte) (string, error) {
	// 直接执行脚本（DoString 会加载并执行脚本定义的函数）
	if err := L.DoString(scriptContent); err != nil {
		return "", err
	}

	// 调用 encodeInp 函数
	encodeInp := L.GetGlobal("encodeInp")
	if encodeInp.Type() == lua.LTNil {
		return "", &lua.ApiError{
			Type:   lua.ApiErrorRun,
			Object: lua.LString("function 'encodeInp' not found in script"),
		}
	}

	// 调用脚本函数：encodeInp(msg, topic)
	err := L.CallByParam(lua.P{
		Fn:      encodeInp,
		NRet:    1,
		Protect: true,
	}, lua.LString(jsonData), lua.LString(""))

	if err != nil {
		return "", err
	}

	// 获取返回值
	result := L.Get(-1)
	L.Pop(1)

	if result.Type() != lua.LTString {
		return "", &lua.ApiError{
			Type:   lua.ApiErrorRun,
			Object: lua.LString("script must return a string"),
		}
	}

	return result.String(), nil
}

// setupSandbox 设置 Lua 沙箱环境（禁用危险函数并加载安全模块）
func (e *LuaExecutor) setupSandbox(L *lua.LState) {
	// Preload safe modules before restricting globals; PreloadModule needs package.preload.
	L.PreloadModule("json", luajson.Loader)

	// 禁用危险的标准库
	L.SetGlobal("os", lua.LNil)           // 禁用 os 库（操作系统操作）
	L.SetGlobal("io", lua.LNil)           // 禁用 io 库（文件 IO）
	L.SetGlobal("dofile", lua.LNil)       // 禁用 dofile
	L.SetGlobal("loadfile", lua.LNil)     // 禁用 loadfile
	L.SetGlobal("load", lua.LNil)         // 禁用 load
	L.SetGlobal("loadstring", lua.LNil)   // 禁用 loadstring
	L.SetGlobal("rawget", lua.LNil)       // 禁用 rawget（可绕过沙箱）
	L.SetGlobal("rawset", lua.LNil)       // 禁用 rawset（可绕过沙箱）
	L.SetGlobal("setmetatable", lua.LNil) // 禁用 setmetatable（可绕过沙箱）
	L.SetGlobal("getmetatable", lua.LNil) // 禁用 getmetatable（可绕过沙箱）

	// Keep package and require so scripts can load preloaded safe modules such as json.
	// External file loading remains unavailable because io/os/dofile/loadfile are disabled.

	// 保留安全的基础函数：
	// print, tostring, tonumber, type, pairs, ipairs, next
	// table.*, string.*, math.*
	// require（用于加载预加载的安全模块）
	// package（用于 require 的内部机制）
	// 这些函数默认保留，无需额外设置
}
