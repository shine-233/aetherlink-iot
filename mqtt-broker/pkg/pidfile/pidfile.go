// Package pidfile 提供创建和删除进程 PID 文件的工具。
//
// 文件用途：为 Broker 启停流程提供 PID 文件创建、冲突检测和清理能力。
// 核心逻辑：读取已有 PID 文件，确认进程是否仍存在，再写入当前进程 ID。
// 关键注意事项：旧 PID 文件如果指向仍活跃进程，必须拒绝覆盖，避免重复启动。
// 重构建议：后续可增加更多权限、损坏 PID 文件和 stale PID 场景测试。
package pidfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PIDFile is a file used to store the process ID of a running process.
type PIDFile struct {
	path string
}

func checkPIDFileAlreadyExists(path string) error {
	if pidByte, err := os.ReadFile(path); err == nil {
		pidString := strings.TrimSpace(string(pidByte))
		if pid, err := strconv.Atoi(pidString); err == nil {
			if processExists(pid) {
				return fmt.Errorf("pid file found, ensure gmqtt is not running or delete %s", path)
			}
		}
	}
	return nil
}

// New creates a PIDfile using the specified path.
func New(path string) (*PIDFile, error) {
	if err := checkPIDFileAlreadyExists(path); err != nil {
		return nil, err
	}
	// Note MkdirAll returns nil if a directory already exists
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return nil, err
	}

	return &PIDFile{path: path}, nil
}

// remove removes the PIDFile.
func (file PIDFile) Remove() error {
	return os.Remove(file.path)
}
