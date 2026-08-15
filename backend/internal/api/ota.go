// 文件用途：提供 OTA 升级包、升级任务、分片下载等 HTTP 接口处理器。
// 核心链路：Handler 负责绑定参数与读取 claims，再委托 service 层完成业务；下载链路额外处理路径收敛、Range 解析、CRC16 计算与分片响应头设置。
// 使用注意：本文件应保持“薄 API 层”职责，不在此处新增租户绕过、鉴权兜底、复杂业务判断或存储访问逻辑。
// 静态审查建议：重点关注路径穿越、Range 边界、Header 协议兼容性、错误码一致性，以及重复的 claims/ID 获取逻辑是否需要后续抽出公共辅助函数。
package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/howeyc/crc16"
)

type OTAApi struct{}

var errInvalidRange = errors.New("invalid range")

// CreateOTAUpgradePackage 创建 OTA 升级包。
// 关键点：参数校验完成后仅透传租户身份给 service，避免在接口层拼装额外业务默认值。
// @Router   /api/v1/ota/package [post]
func (*OTAApi) CreateOTAUpgradePackage(c *gin.Context) {
	var req model.CreateOTAUpgradePackageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.OTA.CreateOTAUpgradePackage(&req, userClaims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// DeleteOTAUpgradePackage 删除 OTA 升级包。
// 静态审查重点：确认删除操作的资源归属校验仍全部在 service 层统一处理。
// @Router   /api/v1/ota/package/{id} [delete]
func (*OTAApi) DeleteOTAUpgradePackage(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.OTA.DeleteOTAUpgradePackage(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// UpdateOTAUpgradePackage 更新 OTA 升级包元数据。
// 静态审查重点：关注请求体字段与 service 更新白名单是否一致，避免接口层放宽可写字段。
// @Router   /api/v1/ota/package/ [put]
func (*OTAApi) UpdateOTAUpgradePackage(c *gin.Context) {
	var req model.UpdateOTAUpgradePackageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.OTA.UpdateOTAUpgradePackage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleOTAUpgradePackageByPage 分页查询 OTA 升级包。
// 关键点：该接口只负责请求绑定与响应挂载，分页、过滤、租户隔离均依赖 service 层。
// @Router   /api/v1/ota/package [get]
func (*OTAApi) HandleOTAUpgradePackageByPage(c *gin.Context) {
	var req model.GetOTAUpgradePackageLisyByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	list, err := service.GroupApp.OTA.GetOTAUpgradePackageListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", list)
}

// CreateOTAUpgradeTask 创建 OTA 升级任务。
// 静态审查重点：确认任务创建前置校验、设备筛选与状态初始化未在 API 层发生分叉。
// @Router   /api/v1/ota/task [post]
func (*OTAApi) CreateOTAUpgradeTask(c *gin.Context) {
	var req model.CreateOTAUpgradeTaskReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.OTA.CreateOTAUpgradeTask(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// DeleteOTAUpgradeTask 删除 OTA 升级任务。
// @Router   /api/v1/ota/task/{id} [delete]
// PreviewOTAUpgradeTask previews backend-resolved devices for a filter-based OTA task.
// @Summary Preview OTA upgrade task device scope
// @Description Resolves the backend device filter without persisting a task, returning the eligible devices and any blocking reasons.
// @Tags OTA
// @Accept json
// @Produce json
// @Param request body model.PreviewOTAUpgradeTaskReq true "OTA task preview payload"
// @Success 200 {object} model.PreviewOTAUpgradeTaskRsp "Preview result with eligible device list"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/ota/task/preview [post]
func (*OTAApi) PreviewOTAUpgradeTask(c *gin.Context) {
	var req model.PreviewOTAUpgradeTaskReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.OTA.PreviewOTAUpgradeTask(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*OTAApi) DeleteOTAUpgradeTask(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.OTA.DeleteOTAUpgradeTask(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleOTAUpgradeTaskByPage 分页查询 OTA 升级任务。
// @Router   /api/v1/ota/task [get]
func (*OTAApi) HandleOTAUpgradeTaskByPage(c *gin.Context) {
	var req model.GetOTAUpgradeTaskListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	list, err := service.GroupApp.OTA.GetOTAUpgradeTaskListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)
}

// HandleOTAUpgradeTaskDetailByPage 分页查询 OTA 升级任务明细。
// 静态审查重点：关注明细查询是否会暴露跨租户设备信息，以及筛选条件是否足够收敛。
// @Router   /api/v1/ota/task/detail [get]
func (*OTAApi) HandleOTAUpgradeTaskDetailByPage(c *gin.Context) {
	var req model.GetOTAUpgradeTaskDetailReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	list, err := service.GroupApp.OTA.GetOTAUpgradeTaskDetailListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)

}

// GetOTAUpgradeTaskSupportBundle returns a task-level troubleshooting package for OTA rollout handoff.
// @Summary Get OTA upgrade task support bundle
// @Description Returns a task-level troubleshooting package for OTA rollout handoff, including per-device status and failure groupings.
// @Tags OTA
// @Accept json
// @Produce json
// @Param id path string true "OTA upgrade task ID"
// @Success 200 {object} model.OTAUpgradeTaskSupportBundle "Support bundle payload"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/ota/task/{id}/support-bundle [get]
func (*OTAApi) GetOTAUpgradeTaskSupportBundle(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.OTA.GetOTAUpgradeTaskSupportBundle(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateOTAUpgradeTaskStatus 更新 OTA 升级任务状态。
// 静态审查重点：状态流转规则应由 service 层集中维护，避免接口层出现状态机旁路。
// @Router   /api/v1/ota/task/detail [put]
func (*OTAApi) UpdateOTAUpgradeTaskStatus(c *gin.Context) {
	var req model.UpdateOTAUpgradeTaskStatusReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.OTA.UpdateOTAUpgradeTaskStatus(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// PreviewOTARolloutGovernance 只读预览一个 OTA rollout task 的下一步治理动作。
// 静态审查重点：本接口不下发、不改任何 detail 行、不连 broker，决策由纯规划器单一来源产出。
// @Router   /api/v1/ota/task/{id}/rollout-governance [get]
func (*OTAApi) PreviewOTARolloutGovernance(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.OTA.PreviewRolloutGovernance(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DownloadOTAUpgradePackage 下载 OTA 升级包，可选支持 Range 分片。
// 核心链路：先做路径净化与存在性校验，再根据 Range 头决定走整包输出或分片输出。
// 静态审查重点：这里是安全敏感入口，应持续审查路径穿越、非法 Range、异常中断和自定义校验头兼容性。
func (*OTAApi) DownloadOTAUpgradePackage(c *gin.Context) {
	filePath, err := safeOTAUpgradePackagePath(c.Param("path"), c.Param("file"))
	if err != nil {
		otaParamError(c, err.Error())
		return
	}

	if !utils.FileExist(filePath) {
		otaParamError(c, "file not exist")
		return
	}

	rangeHeader := c.GetHeader("Range")
	crc16Method := c.GetHeader("Crc16-Method")

	if rangeHeader == "" {
		c.File(filePath)
		return
	}

	// Range 请求进入分片下载分支，并在响应头附带当前分片的 CRC16 校验值。
	serveRangeFile(filePath, rangeHeader, crc16Method, c)
}

// serveRangeFile 输出指定字节区间的数据，并补齐 206 所需响应头。
// 静态审查重点：两次 Seek 分别用于校验和发送，后续若改为流式摘要需确认不会破坏响应长度与读取位置。
func serveRangeFile(filePath, rangeHeader, crc16Method string, c *gin.Context) {
	file, err := os.Open(filePath)
	if err != nil {
		otaInternalError(c)
		return
	}
	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			log.Printf("Error closing file: %v", closeErr)
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		otaInternalError(c)
		return
	}

	fileSize := fileInfo.Size()
	start, end, err := parseByteRange(rangeHeader, fileSize)
	if err != nil {
		otaRangeError(c, err)
		return
	}

	contentLength := end - start + 1

	c.Writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Writer.Header().Set("Accept-Ranges", "bytes")
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Writer.Header().Set("Content-Type", contentType)
	c.Status(http.StatusPartialContent)

	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		otaInternalError(c)
		return
	}

	crcValue, err := rangeCRC16(file, contentLength, crc16Method)
	if err != nil {
		otaInternalError(c)
		return
	}

	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		otaInternalError(c)
		return
	}

	// 将当前分片的 CRC16 放入响应头，供设备端在断点续传时做分片校验。
	c.Writer.Header().Set("X-CRC16", fmt.Sprintf("%04x", crcValue))

	_, err = io.CopyN(c.Writer, file, contentLength)
	if err != nil {
		otaInternalError(c)
		return
	}
}

// otaParamError 统一封装 OTA 参数错误，保持业务错误码出口一致。
func otaParamError(c *gin.Context, message string) {
	c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
		"param_err": message,
	}))
}

// otaInternalError 统一返回下载链路内部错误，避免泄漏底层文件系统细节。
func otaInternalError(c *gin.Context) {
	c.AbortWithStatus(http.StatusInternalServerError)
}

// otaRangeError 将非法或不可满足的 Range 请求映射为 416。
func otaRangeError(c *gin.Context, err error) {
	c.AbortWithError(http.StatusRequestedRangeNotSatisfiable, err)
}

// rangeCRC16 计算当前分片内容的 CRC16 值。
// 静态审查重点：该函数依赖调用方保证文件游标位置正确，后续复用时需明确这一前置条件。
func rangeCRC16(file *os.File, contentLength int64, crc16Method string) (uint16, error) {
	digest := crc16Digest(crc16Method)
	if _, err := io.CopyN(digest, file, contentLength); err != nil {
		return 0, err
	}
	return digest.Sum16(), nil
}

// crc16Digest 根据请求头选择 CRC16 算法，默认回退到 IBM。
// 静态审查重点：若协议将来扩展算法枚举，建议同步收敛到常量或枚举定义，避免魔法字符串散落。
func crc16Digest(crc16Method string) crc16.Hash16 {
	switch crc16Method {
	case "CCITT":
		return crc16.NewCCITT()
	case "MODBUS":
		return crc16.New(crc16.MBusTable)
	default:
		return crc16.NewIBM()
	}
}

// parseByteRange 解析单区间 Range 头，仅支持 bytes=start-end 形式。
// 静态审查重点：当前不支持后缀区间、多区间；若网关或设备端新增协议需求，应先补测试再扩展。
func parseByteRange(rangeHeader string, fileSize int64) (int64, int64, error) {
	rangeStr, ok := strings.CutPrefix(rangeHeader, "bytes=")
	if !ok {
		return 0, 0, errInvalidRange
	}

	startRaw, endRaw, ok := strings.Cut(rangeStr, "-")
	if !ok || strings.TrimSpace(startRaw) == "" {
		return 0, 0, errInvalidRange
	}

	start, err := strconv.ParseInt(startRaw, 10, 64)
	if err != nil {
		return 0, 0, errInvalidRange
	}

	end := fileSize - 1
	if strings.TrimSpace(endRaw) != "" {
		end, err = strconv.ParseInt(endRaw, 10, 64)
		if err != nil {
			return 0, 0, errInvalidRange
		}
	}

	if fileSize <= 0 || start < 0 || end < start || start >= fileSize || end >= fileSize {
		return 0, 0, errInvalidRange
	}

	return start, end, nil
}

// safeOTAUpgradePackagePath 将 URL 参数限制在升级包目录下，防止路径穿越。
// 静态审查重点：后续若变更基础目录或运行目录，需重新确认 filepath.Abs 与前缀比较的安全边界。
func safeOTAUpgradePackagePath(pathParam, fileParam string) (string, error) {
	if strings.TrimSpace(pathParam) == "" || strings.TrimSpace(fileParam) == "" {
		return "", errors.New("invalid ota file path")
	}
	if filepath.IsAbs(pathParam) || filepath.IsAbs(fileParam) {
		return "", errors.New("invalid ota file path")
	}
	cleanPath := filepath.Clean(pathParam)
	cleanFile := filepath.Clean(fileParam)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) {
		return "", errors.New("invalid ota file path")
	}
	if cleanFile != filepath.Base(cleanFile) || cleanFile == "." || cleanFile == ".." {
		return "", errors.New("invalid ota file name")
	}
	base, err := filepath.Abs("./files/upgradePackage")
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(base, cleanPath, cleanFile))
	if err != nil {
		return "", err
	}
	baseWithSep := base + string(os.PathSeparator)
	if fullPath != base && !strings.HasPrefix(fullPath, baseWithSep) {
		return "", errors.New("invalid ota file path")
	}
	return fullPath, nil
}
