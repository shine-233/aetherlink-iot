// 文件用途：Windows 平台的 PID 存活探测实现。
// 核心逻辑：打开目标进程并读取退出码，确认进程是否仍处于 active 状态。
// 关键注意事项：系统调用失败时按进程不存在处理，避免 PID 文件阻塞正常启动。
// 重构建议：后续可补充权限不足和进程刚退出场景的 Windows 专项测试。
package pidfile

import (
	"golang.org/x/sys/windows"
)

const (
	processQueryLimitedInformation = 0x1000

	stillActive = 259
)

func processExists(pid int) bool {
	h, err := windows.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	var c uint32
	err = windows.GetExitCodeProcess(h, &c)
	windows.Close(h)
	if err != nil {
		return c == stillActive
	}
	return true
}
