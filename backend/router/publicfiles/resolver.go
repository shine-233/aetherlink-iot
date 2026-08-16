// 文件用途：解析 `/files/*filepath` 请求对应的本地公开文件路径。
// 核心逻辑：清洗 URL 路径，拒绝反斜杠、盘符和目录穿越，再确认最终绝对路径仍位于 `./files` 下。
// 关键注意事项：这是文件访问安全边界，不能退化为直接字符串拼接或只做前缀判断。
// 重构建议：后续可支持注入公开文件根目录，并增加 URL 编码、符号链接和平台差异测试。
package publicfiles

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ResolvePath maps a URL path from /files/*filepath to a local file under ./files.
func ResolvePath(rawPath string) (string, error) {
	if rawPath == "" || rawPath == "/" {
		return "", errors.New("file path is required")
	}
	if strings.Contains(rawPath, `\`) || strings.Contains(rawPath, ":") || strings.HasPrefix(rawPath, "//") {
		return "", errors.New("file path must be a URL path")
	}
	for _, segment := range strings.Split(rawPath, "/") {
		if segment == ".." {
			return "", errors.New("file path escapes public directory")
		}
	}

	cleanPath := path.Clean("/" + rawPath)
	if cleanPath == "/" || strings.HasPrefix(cleanPath, "/../") || cleanPath == "/.." {
		return "", errors.New("file path escapes public directory")
	}

	relPath := strings.TrimPrefix(cleanPath, "/")
	if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "../") {
		return "", errors.New("file path escapes public directory")
	}

	baseDir, err := filepath.Abs("./files")
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(baseDir, filepath.FromSlash(relPath)))
	if err != nil {
		return "", err
	}
	relativeToBase, err := filepath.Rel(baseDir, fullPath)
	if err != nil || relativeToBase == ".." || strings.HasPrefix(relativeToBase, ".."+string(os.PathSeparator)) || filepath.IsAbs(relativeToBase) {
		return "", errors.New("file path escapes public directory")
	}
	return fullPath, nil
}
