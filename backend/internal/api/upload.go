// 文件用途：提供文件上传相关的 HTTP 接口处理器，以及上传落盘所需的路径与类型校验辅助函数。
// 上传链路：UpFile 从 multipart/form-data 中读取 file 与 type，先做空值/大小校验，再清洗文件名、校验类型，
// 调用 generateFilePath 生成受限目录与随机文件名，最后由 saveFile 落盘并返回对外暴露的相对访问路径。
// 路径校验：当前实现通过拒绝 fileType 中的分隔符、调用 utils.CheckPath，以及对目录与最终文件分别执行
// filepath.Abs + filepath.Rel 包含性检查，阻止 path traversal 逃逸出 BaseUploadDir。
// 删除/列举边界：本文件只负责“写入”链路，不负责文件列举、删除、覆盖或清理；后续若新增相关 handler，
// 应复用相同的根目录包含性校验，并明确限制只能操作业务允许的逻辑路径，避免直接暴露磁盘真实路径或做宽泛前缀删除。
// 静态审查建议：重点关注扩展名校验是否足以代表真实内容、符号链接/挂载点是否可能绕过目录约束、
// OTA 返回路径与真实落盘路径是否持续一致，以及错误细节是否会向外泄露过多内部路径信息。
package api

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type UpLoadApi struct{}

const (
	BaseUploadDir           = "./files/"
	OtaPath                 = "./api/v1/ota/download/files/"
	MaxFileSize             = 1000 << 20 // 1000MB，保留 OTA 大文件接口契约
	MaxFileSizeLabel        = "1000MB"
	multipartOverheadBudget = 1 << 20 // multipart headers and form fields
	maxUploadRequestSize    = MaxFileSize + multipartOverheadBudget
)

// UpFile 处理上传入口。
// 处理顺序固定为：限制请求体 -> 读取表单 -> 校验 type、文件大小、扩展名和可识别内容签名
// -> 生成日期目录与随机文件名 -> 再次校验最终落盘路径未逃逸根目录 -> 保存文件 -> 返回相对访问路径。
// 边界说明：
// 1. 这里只接受新文件写入，不负责覆盖已有文件、列举目录内容或删除历史文件。
// 2. type 既参与业务类型校验，也参与目录拼接，因此必须同时满足路径安全和文件类型白名单约束。
// 3. 返回值是给上层使用的相对路径；调用方若要做后续删除/列举，不能把这里返回的路径直接当作任意磁盘路径使用。
// @Tags     File upload
// @Router   /api/v1/file/up [post]
func (*UpLoadApi) UpFile(c *gin.Context) {
	if uploadRequestTooLarge(c.Request.ContentLength) {
		setUploadTooLargeError(c, c.Request.ContentLength)
		return
	}
	if c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadRequestSize)
	}

	file, err := c.FormFile("file")
	if err != nil || file == nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			setUploadTooLargeError(c, maxBytesErr.Limit)
		} else {
			c.Error(errcode.New(errcode.CodeFileEmpty))
		}
		return
	}

	fileType := c.PostForm("type")
	if fileType == "" {
		c.Error(errcode.New(errcode.CodeFileEmpty))
		return
	}

	if file.Size > MaxFileSize {
		c.Error(errcode.WithVars(errcode.CodeFileTooLarge, map[string]interface{}{
			"max_size":     MaxFileSizeLabel,
			"current_size": fmt.Sprintf("%.2fMB", float64(file.Size)/(1<<20)),
		}))
		return
	}

	filename := utils.SanitizeFilename(file.Filename)
	if err := validateFileType(filename, fileType); err != nil {
		c.Error(errcode.WithVars(errcode.CodeFileTypeMismatch, map[string]interface{}{
			"expected_type": fileType,
			"actual_type":   filepath.Ext(filename),
		}))
		return
	}
	if err := validateFileSignature(file, filename, fileType); err != nil {
		c.Error(errcode.WithVars(errcode.CodeFileTypeMismatch, map[string]interface{}{
			"expected_type": fileType,
			"actual_type":   err.Error(),
		}))
		return
	}

	uploadDir, fileName, err := generateFilePath(fileType, filename)
	if err != nil {
		c.Error(errcode.WithVars(errcode.CodeFilePathGenError, map[string]interface{}{
			"error":     err.Error(),
			"file_type": fileType,
			"filename":  file.Filename,
		}))
		return
	}

	filePath, err := saveFile(c, file, uploadDir, fileName, fileType)
	if err != nil {
		c.Error(errcode.WithVars(errcode.CodeFileSaveError, map[string]interface{}{
			"error":      err.Error(),
			"upload_dir": uploadDir,
			"filename":   fileName,
		}))
		return
	}

	c.Set("data", map[string]interface{}{
		"path": filePath,
	})
}

func uploadRequestTooLarge(contentLength int64) bool {
	return contentLength > maxUploadRequestSize
}

func setUploadTooLargeError(c *gin.Context, size int64) {
	currentSize := "unknown"
	if size > 0 {
		currentSize = fmt.Sprintf("%.2fMB", float64(size)/(1<<20))
	}
	_ = c.Error(errcode.WithVars(errcode.CodeFileTooLarge, map[string]interface{}{
		"max_size":     MaxFileSizeLabel,
		"current_size": currentSize,
	}))
}

// generateFilePath 负责生成受限上传目录和最终文件名。
// 它只允许在 BaseUploadDir 下按“业务类型/日期”建目录，并用时间戳 + 随机串生成哈希文件名，
// 以减少原始文件名碰撞和信息泄露；若目录解析结果不再位于根目录下，则直接拒绝。
func generateFilePath(fileType, filename string) (string, string, error) {
	if strings.ContainsAny(fileType, "./\\") {
		return "", "", errcode.New(errcode.CodeFilePathGenError)
	}

	dateDir := time.Now().Format("2006-01-02")
	uploadDir := filepath.Clean(filepath.Join(BaseUploadDir, fileType, dateDir))
	absUploadDir, err := filepath.Abs(uploadDir)
	if err != nil {
		return "", "", errcode.WithVars(errcode.CodeFilePathGenError, map[string]interface{}{
			"error": "invalid path",
		})
	}

	absBaseDir, err := filepath.Abs(BaseUploadDir)
	if err != nil {
		return "", "", errcode.WithVars(errcode.CodeFilePathGenError, map[string]interface{}{
			"error": "invalid base path",
		})
	}

	relPath, err := filepath.Rel(absBaseDir, absUploadDir)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) || filepath.IsAbs(relPath) {
		return "", "", errcode.WithVars(errcode.CodeFilePathGenError, map[string]interface{}{
			"error": "path traversal detected",
		})
	}

	randomStr, err := common.GenerateRandomString(16)
	if err != nil {
		return "", "", errcode.WithVars(errcode.CodeFilePathGenError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	timeStr := time.Now().Format("20060102150405")
	hashStr := fmt.Sprintf("%x", md5.Sum([]byte(timeStr+randomStr)))
	fileName := hashStr + strings.ToLower(filepath.Ext(filename))

	return uploadDir, fileName, nil
}

// saveFile 在最终写盘前做最后一次根目录包含性校验，然后调用 Gin 的 SaveUploadedFile。
// upgradePackage 是特例：真实文件仍落在 BaseUploadDir 下，但返回给调用方的是 OTA 下载路由对应的访问路径，
// 维护时需要同时关注“磁盘路径”和“对外路径”两套语义，避免后续列举/删除逻辑混用。
func saveFile(c *gin.Context, file *multipart.FileHeader, uploadDir, fileName, fileType string) (string, error) {
	fullPath := filepath.Join(uploadDir, fileName)
	if err := ensureUploadPathContained(fullPath); err != nil {
		return "", err
	}

	absBaseDir, err := filepath.Abs(BaseUploadDir)
	if err != nil {
		return "", fmt.Errorf("resolve upload base: %w", err)
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve upload path: %w", err)
	}
	relativePath, err := filepath.Rel(absBaseDir, absFullPath)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) || filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("upload path escapes base directory")
	}

	root, err := os.OpenRoot(absBaseDir)
	if err != nil {
		return "", fmt.Errorf("open upload root: %w", err)
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(relativePath), 0o755); err != nil {
		return "", fmt.Errorf("create upload directory: %w", err)
	}

	source, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open uploaded file: %w", err)
	}
	defer source.Close()
	destination, err := root.OpenFile(relativePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create uploaded file: %w", err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		_ = root.Remove(relativePath)
		return "", fmt.Errorf("save uploaded file: %w", err)
	}
	if err := destination.Close(); err != nil {
		_ = root.Remove(relativePath)
		return "", fmt.Errorf("close uploaded file: %w", err)
	}

	if fileType == "upgradePackage" {
		return "./" + filepath.Join(OtaPath, fileType, time.Now().Format("2006-01-02"), fileName), nil
	}

	return "./" + fullPath, nil
}

// ensureUploadPathContained 确认最终文件绝对路径仍位于 BaseUploadDir 内。
// 这是对 generateFilePath 的兜底校验，防止后续调用方在拼接 fullPath 时引入新的目录逃逸风险。
func ensureUploadPathContained(fullPath string) error {
	absBaseDir, err := filepath.Abs(BaseUploadDir)
	if err != nil {
		return fmt.Errorf("resolve upload base: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("resolve upload path: %w", err)
	}

	relPath, err := filepath.Rel(absBaseDir, absPath)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) || filepath.IsAbs(relPath) {
		return fmt.Errorf("upload path escapes base directory")
	}

	return nil
}

// validateFileType 统一封装业务类型与文件扩展名校验。
// 先校验 fileType 不是危险路径，再校验文件名扩展名是否属于该业务类型允许集合。
func validateFileType(filename, fileType string) error {
	if err := utils.CheckPath(fileType); err != nil {
		return fmt.Errorf("invalid file type path: %w", err)
	}

	if !utils.ValidateFileType(filename, fileType) {
		return errors.New("file type is not allowed")
	}

	return nil
}

// validateFileSignature 校验可可靠识别格式的文件头；裸固件、模型、CSV 和插件继续由后续业务解析器校验。
func validateFileSignature(file *multipart.FileHeader, filename, fileType string) error {
	if file == nil {
		return errors.New("missing file")
	}
	stream, err := file.Open()
	if err != nil {
		return fmt.Errorf("open upload: %w", err)
	}
	defer stream.Close()

	header := make([]byte, 512)
	readSize, err := io.ReadFull(stream, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read upload signature: %w", err)
	}
	return validateContentSignature(filename, fileType, header[:readSize])
}

func validateContentSignature(filename, fileType string, header []byte) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if fileType == "d_plugin" || !requiresSignatureCheck(ext) {
		return nil
	}
	if len(header) == 0 {
		return errors.New("empty file")
	}

	contentType := http.DetectContentType(header)
	valid := false
	switch ext {
	case ".jpg", ".jpeg":
		valid = contentType == "image/jpeg"
	case ".png":
		valid = contentType == "image/png"
	case ".gif":
		valid = contentType == "image/gif"
	case ".ico":
		valid = contentType == "image/x-icon" || bytes.HasPrefix(header, []byte{0, 0, 1, 0})
	case ".svg":
		trimmed := bytes.TrimSpace(bytes.TrimPrefix(header, []byte{0xEF, 0xBB, 0xBF}))
		valid = bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<svg")) || bytes.Contains(bytes.ToLower(trimmed), []byte("<svg "))
	case ".xlsx", ".zip", ".apk":
		valid = bytes.HasPrefix(header, []byte("PK\x03\x04")) || bytes.HasPrefix(header, []byte("PK\x05\x06")) || bytes.HasPrefix(header, []byte("PK\x07\x08"))
	case ".xls":
		valid = bytes.HasPrefix(header, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	case ".gz", ".gzip":
		valid = bytes.HasPrefix(header, []byte{0x1F, 0x8B})
	}
	if !valid {
		return fmt.Errorf("content signature does not match %s", ext)
	}
	return nil
}

func requiresSignatureCheck(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".ico", ".svg", ".xlsx", ".xls", ".zip", ".apk", ".gz", ".gzip":
		return true
	default:
		return false
	}
}
