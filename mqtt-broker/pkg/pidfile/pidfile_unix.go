//go:build !windows && !darwin
// +build !windows,!darwin

// 文件用途：Linux/Unix 平台的 PID 存活探测实现。
// 核心逻辑：通过检查 /proc/<pid> 是否存在判断进程是否仍然活跃。
// 关键注意事项：该实现依赖 procfs，不适用于 Windows 和 macOS。
// 重构建议：后续可迁移为现代 go:build 标记，并补充平台构建约束检查。
package pidfile

import (
	"os"
	"path/filepath"
	"strconv"
)

func processExists(pid int) bool {
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
		return true
	}
	return false
}
