// 文件用途：移动应用中心（mobile_app_bundles，ROADMAP TB-23）HTTP 处理器。
// 核心逻辑：multipart 上传绑定与表单字段校验、claims 租户边界解析（SYS_ADMIN 可指定
//
//	目标租户、其余角色锁死本租户）、Service 分发与统一 API 响应包装；下载入口按
//	记录里的对外路径经 os.OpenInRoot 受控打开并 http.ServeContent 输出（支持 Range）。
//
// 关键注意事项：
//  1. 请求上下文一律透传 c.Request.Context()，不引入 context.Background()
//     （internal/api 的 context.Background() 有数量预算守卫，预算 4）。
//  2. 租户解析复用 mobileTenant 口径（api/mobile.go）：接口不接受"裸 tenant_id 就越权"，
//     非 SYS_ADMIN 传其他租户直接拒绝；id 一律取路径参数，不信任请求体标识。
//  3. 上传请求体用 MaxBytesReader 夹紧（口径同 api/upload.go），防止 multipart 解析前内存膨胀；
//     下载路径先经 service 白名单解析（files/apps/ 前缀），拒绝任何目录逃逸。
//
// 重构建议：uniapp 对接阶段开放终端拉取面时，把 DownloadAppBundle 拆成独立
//
//	"published 专用"入口并单独评审授权，不要在现有管理面下载上加角色开关。
package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// MobileAppBundleApi 移动应用中心控制器。
type MobileAppBundleApi struct{}

// appBundleUploadRequestSize 上传请求体上限（500MB 文件 + multipart 头/表单开销预算）。
const appBundleUploadRequestSize = service.MaxAppBundleFileSize + (1 << 20)

// appBundleTenant 解析当前请求的租户边界：SYS_ADMIN 可显式指定目标租户（跨租户管理），
// 其余角色锁死 claims.TenantID；缺凭证或无租户上下文一律拒绝（fail-closed）。
func appBundleTenant(c *gin.Context, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	claimsValue, _ := c.Get("claims")
	claims, _ := claimsValue.(*utils.UserClaims)
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "missing user claims")
	}
	if claims.Authority == "SYS_ADMIN" {
		if requested != "" {
			return requested, nil
		}
		return claims.TenantID, nil
	}
	if strings.TrimSpace(claims.TenantID) == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no tenant context")
	}
	if requested != "" && requested != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "cross-tenant operation is not allowed")
	}
	return claims.TenantID, nil
}

// UploadAppBundle 上传并登记应用包（multipart：file + platform + version + release_notes，
// SYS_ADMIN 另可传 tenant_id 指定目标租户）。登记成功固定落 draft 状态。
// @Summary Upload a mobile app bundle
// @Tags MobileAppBundle
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "App bundle file (apk/ipa/zip)"
// @Param platform formData string true "android / ios / h5"
// @Param version formData string true "Bundle version"
// @Param release_notes formData string false "Release notes"
// @Router /api/v1/mobile/app_bundles/upload [post]
func (*MobileAppBundleApi) UploadAppBundle(c *gin.Context) {
	if c.Request.ContentLength > appBundleUploadRequestSize {
		c.Error(errcode.WithVars(errcode.CodeFileTooLarge, map[string]interface{}{
			"max_size":     service.MaxAppBundleFileSizeLabel,
			"current_size": "unknown",
		}))
		return
	}
	if c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, appBundleUploadRequestSize)
	}

	file, err := c.FormFile("file")
	if err != nil || file == nil {
		c.Error(errcode.New(errcode.CodeFileEmpty))
		return
	}
	platform := strings.TrimSpace(c.PostForm("platform"))
	version := strings.TrimSpace(c.PostForm("version"))
	if platform == "" || version == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "platform and version are required"))
		return
	}

	tenantID, err := appBundleTenant(c, c.PostForm("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}

	// 文件名清洗复用 upload 链路：去路径分隔符与危险字符，只保留基础名。
	filename := utils.SanitizeFilename(file.Filename)
	data, err := service.GroupApp.MobileAppBundle.CreateAppBundle(c.Request.Context(), &model.AppBundleCreateInput{
		TenantID:     tenantID,
		Platform:     platform,
		Version:      version,
		ReleaseNotes: c.PostForm("release_notes"),
		FileName:     filename,
	}, file)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ListAppBundles 分页查询本租户应用包列表（platform/status 精确过滤）。
// @Summary List mobile app bundles
// @Tags MobileAppBundle
// @Produce json
// @Param request query model.GetAppBundleListReq true "Pagination and filters"
// @Success 200 {object} model.GetAppBundleListRsp "App bundle list"
// @Router /api/v1/mobile/app_bundles [get]
func (*MobileAppBundleApi) ListAppBundles(c *gin.Context) {
	var req model.GetAppBundleListReq
	if !BindAndValidate(c, &req) {
		return
	}
	tenantID, err := appBundleTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.MobileAppBundle.ListAppBundles(c.Request.Context(), &req, tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetAppBundle 查询应用包详情。
// @Summary Get a mobile app bundle
// @Tags MobileAppBundle
// @Produce json
// @Param id path string true "App bundle id"
// @Success 200 {object} model.MobileAppBundle "App bundle detail"
// @Router /api/v1/mobile/app_bundles/{id} [get]
func (*MobileAppBundleApi) GetAppBundle(c *gin.Context) {
	tenantID, err := appBundleTenant(c, "")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.MobileAppBundle.GetAppBundle(c.Request.Context(), c.Param("id"), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateAppBundle 更新应用包（仅 draft 可改 release_notes；版本/平台/文件本体发布后不可变）。
// @Summary Update a mobile app bundle (draft only)
// @Tags MobileAppBundle
// @Produce json
// @Param id path string true "App bundle id"
// @Param request body model.UpdateAppBundleReq true "Update payload"
// @Success 200 {object} model.MobileAppBundle "Updated app bundle"
// @Router /api/v1/mobile/app_bundles/{id} [put]
func (*MobileAppBundleApi) UpdateAppBundle(c *gin.Context) {
	var req model.UpdateAppBundleReq
	if !BindAndValidate(c, &req) {
		return
	}
	tenantID, err := appBundleTenant(c, "")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.MobileAppBundle.UpdateAppBundle(c.Request.Context(), c.Param("id"), &req, tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// PublishAppBundle 发布：draft→published（非法流转拒绝，重复发布也拒绝）。
// @Summary Publish a mobile app bundle
// @Tags MobileAppBundle
// @Produce json
// @Param id path string true "App bundle id"
// @Success 200 {object} model.MobileAppBundle "Published app bundle"
// @Router /api/v1/mobile/app_bundles/{id}/publish [post]
func (*MobileAppBundleApi) PublishAppBundle(c *gin.Context) {
	tenantID, err := appBundleTenant(c, "")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.MobileAppBundle.PublishAppBundle(c.Request.Context(), c.Param("id"), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ArchiveAppBundle 归档：published→archived（draft/已归档一律拒绝）。
// @Summary Archive a mobile app bundle
// @Tags MobileAppBundle
// @Produce json
// @Param id path string true "App bundle id"
// @Success 200 {object} model.MobileAppBundle "Archived app bundle"
// @Router /api/v1/mobile/app_bundles/{id}/archive [post]
func (*MobileAppBundleApi) ArchiveAppBundle(c *gin.Context) {
	tenantID, err := appBundleTenant(c, "")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.MobileAppBundle.ArchiveAppBundle(c.Request.Context(), c.Param("id"), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteAppBundle 删除应用包：仅 draft/archived；published 必须先归档。
// @Summary Delete a mobile app bundle (draft/archived only)
// @Tags MobileAppBundle
// @Produce json
// @Param id path string true "App bundle id"
// @Router /api/v1/mobile/app_bundles/{id} [delete]
func (*MobileAppBundleApi) DeleteAppBundle(c *gin.Context) {
	tenantID, err := appBundleTenant(c, "")
	if err != nil {
		c.Error(err)
		return
	}
	if err := service.GroupApp.MobileAppBundle.DeleteAppBundle(c.Request.Context(), c.Param("id"), tenantID); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"deleted": true})
}

// DownloadAppBundle 下载应用包文件流（同租户任意状态；http.ServeContent 支持 Range 断点）。
// @Summary Download a mobile app bundle file
// @Tags MobileAppBundle
// @Produce application/octet-stream
// @Param id path string true "App bundle id"
// @Router /api/v1/mobile/app_bundles/{id}/download [get]
func (*MobileAppBundleApi) DownloadAppBundle(c *gin.Context) {
	tenantID, err := appBundleTenant(c, "")
	if err != nil {
		c.Error(err)
		return
	}
	record, err := service.GroupApp.MobileAppBundle.AppBundleDownload(c.Request.Context(), c.Param("id"), tenantID)
	if err != nil {
		c.Error(err)
		return
	}

	// 对外路径 → 根内相对路径（service 已做白名单预检，此处防御性再验一次）。
	normalized := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(record.FilePath)), "./")
	relativePath := strings.TrimPrefix(normalized, "files/")
	if !strings.HasPrefix(relativePath, "apps/") {
		c.Error(errcode.NewWithMessage(errcode.CodeFileSaveError, "app bundle path is outside the apps upload area"))
		return
	}
	absBaseDir, err := filepath.Abs(common.BaseUploadDir)
	if err != nil {
		c.Error(errcode.New(errcode.CodeSystemError))
		return
	}
	file, err := os.OpenInRoot(absBaseDir, filepath.FromSlash(relativePath))
	if err != nil {
		// 登记行在而文件缺失：不伪装成功，返回 404 语义的业务码并留日志排查。
		logrus.Warnf("app bundle file missing on disk: id=%s path=%s err=%v", record.ID, record.FilePath, err)
		c.Error(errcode.NewWithMessage(errcode.CodeNotFound, "app bundle file not found on disk"))
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logrus.Warnf("close app bundle file failed: id=%s err=%v", record.ID, closeErr)
		}
	}()
	fileInfo, err := file.Stat()
	if err != nil || !fileInfo.Mode().IsRegular() {
		c.Error(errcode.New(errcode.CodeSystemError))
		return
	}

	c.Header("X-App-Bundle-Checksum", "sha256-"+record.Checksum)
	http.ServeContent(c.Writer, c.Request, record.FileName, fileInfo.ModTime(), file)
}
