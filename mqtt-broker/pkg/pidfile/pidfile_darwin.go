//go:build darwin
// +build darwin

// 文件用途：macOS 平台的 PID 存活探测实现。
// 核心逻辑：使用 kill(pid, 0) 判断目标进程是否存在。
// 关键注意事项：该调用只做探测，不应向进程发送终止信号。
// 重构建议：后续可迁移为现代 go:build 标记，并补充权限错误语义说明。
package pidfile

import (
	"golang.org/x/sys/unix"
)

func processExists(pid int) bool {
	// OS X does not have a proc filesystem.
	// Use kill -0 pid to judge if the process exists.
	err := unix.Kill(pid, 0)
	return err == nil
}
