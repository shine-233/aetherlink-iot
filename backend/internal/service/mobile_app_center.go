// 文件用途：移动应用中心服务（mobile_app_bundles，ROADMAP TB-23）。
// 核心逻辑：应用包上传落盘（复用 ./files 存储根落 files/apps/，SHA-256 校验和流式计算）、
//
//	租户隔离的列表/详情/删除，以及发布状态机（draft→published→archived，
//	非法流转拒绝，CAS 落库防并发击穿）。
//
// 关键注意事项：
//  1. 状态机合法矩阵集中在 CanTransitionAppBundle，是唯一判定点；流转落库必须走
//     dal.UpdateAppBundleStatus 的 WHERE status=期望值 语义，CAS 未命中按"非法流转"
//     或"已删除"如实报错，绝不静默改写。
//  2. 文件路径只允许 files/apps/ 前缀（resolveAppBundleDiskRelativePath 白名单），
//     删除/下载前做 BaseUploadDir 包含性校验；文件已不存在按幂等成功处理（自愈孤儿行）。
//  3. 重复版本（同租户同平台同版本）拒绝并回滚已落盘文件——"有登记必有文件"与
//     "失败不留垃圾"两条一致性问题都显式处理。
//
// 重构建议：uniapp 对接阶段若需要"按平台取最新已发布包"的拉取面，加只读查询；
//
//	下载开放给 TENANT_USER 前需单独评审授权面，不在本服务默认放行。
package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// MobileAppBundleService 移动应用中心服务。
type MobileAppBundleService struct{}

// 应用包发布状态机的两个动作。
const (
	AppBundleActionPublish = "publish"
	AppBundleActionArchive = "archive"
)

// appBundleUploadSubDir ./files 下的应用包业务子目录（files/apps/...）。
const appBundleUploadSubDir = "apps"

// MaxAppBundleFileSize 应用包大小上限。安装包不会是日志/视频那种无界增长物，
// 500MB 覆盖 apk/ipa/h5 zip 的现实上限，同时比 OTA 的 1000MB 收得更紧（fail-closed）。
const MaxAppBundleFileSize = 500 << 20

// MaxAppBundleFileSizeLabel 上限的可读文案（错误消息模板变量）。
const MaxAppBundleFileSizeLabel = "500MB"

// appBundleVersionPattern 版本号形态约束：字母或数字开头，仅允许字母/数字/./-/_/+，
// 最长 50（与列宽一致）。拒绝路径分隔符与空白，从源头封死文件名拼接与 UI 注入。
var appBundleVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.\-+_]{0,49}$`)

// appBundlePlatformExtensions 平台→允许的文件扩展名。uniapp 三端产物形态固定：
// android 出 .apk、ios 出 .ipa、h5 构建产物打成 .zip；跨平台扩展名混传直接拒绝。
var appBundlePlatformExtensions = map[string][]string{
	model.AppBundlePlatformAndroid: {".apk"},
	model.AppBundlePlatformIOS:     {".ipa"},
	model.AppBundlePlatformH5:      {".zip"},
}

// CanTransitionAppBundle 发布状态机的唯一判定点。
// 合法矩阵：draft --publish--> published --archive--> archived；
// 其余（含 draft 直接归档、published 重复发布、archived 复活/重复归档）一律拒绝，
// 且拒绝时 next 返回空串（目标状态仅在合法流转时给出，避免调用方误读）。
func CanTransitionAppBundle(currentStatus, action string) (next string, ok bool) {
	switch action {
	case AppBundleActionPublish:
		if currentStatus != model.AppBundleStatusDraft {
			return "", false
		}
		return model.AppBundleStatusPublished, true
	case AppBundleActionArchive:
		if currentStatus != model.AppBundleStatusPublished {
			return "", false
		}
		return model.AppBundleStatusArchived, true
	default:
		return "", false
	}
}

// canDeleteAppBundle 删除资格：draft 随时可删；archived 是生命周期终点可删；
// published 不允许直接删（可能有终端正在拉取），必须先归档。
func canDeleteAppBundle(status string) bool {
	return status == model.AppBundleStatusDraft || status == model.AppBundleStatusArchived
}

// invalidTransitionError 构造"非法流转"业务错误（业务码 + 文案模板变量，响应中间件渲染）。
func invalidTransitionError(action, currentStatus string) error {
	return errcode.WithVars(errcode.CodeAppBundleInvalidTransition, map[string]interface{}{
		"operation": action,
		"status":    currentStatus,
	})
}

// appBundleDBError 数据库错误统一包装。
func appBundleDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"error": err.Error(),
	})
}

// appBundleParamError 参数错误统一包装。
func appBundleParamError(message string) error {
	return errcode.NewWithMessage(errcode.CodeParamError, message)
}

// CreateAppBundle 上传并登记一个应用包版本（状态固定落 draft）。
// 流程：租户/平台/版本/扩展名/大小 fail-closed 校验 → 重复版本预检 →
// 落盘 files/apps/<platform>/<日期>/（流式计算 SHA-256）→ 插入登记行；
// 预检漏掉的并发重复由 UNIQUE 约束兜底，冲突时回滚已落盘文件。
func (*MobileAppBundleService) CreateAppBundle(ctx context.Context, input *model.AppBundleCreateInput, file *multipart.FileHeader) (*model.MobileAppBundle, error) {
	if input == nil || strings.TrimSpace(input.TenantID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	tenantID := strings.TrimSpace(input.TenantID)
	if !model.IsValidAppBundlePlatform(input.Platform) {
		return nil, appBundleParamError("platform must be one of: android, ios, h5")
	}
	version := strings.TrimSpace(input.Version)
	if !appBundleVersionPattern.MatchString(version) {
		return nil, appBundleParamError("version must start with a letter or digit and contain only letters, digits, dot, dash, underscore or plus (max 50)")
	}
	releaseNotes := strings.TrimSpace(input.ReleaseNotes)
	if len(releaseNotes) > 2000 {
		return nil, appBundleParamError("release notes must not exceed 2000 characters")
	}
	if file == nil || file.Size == 0 {
		return nil, errcode.New(errcode.CodeFileEmpty)
	}
	if file.Size > MaxAppBundleFileSize {
		return nil, errcode.WithVars(errcode.CodeFileTooLarge, map[string]interface{}{
			"max_size":     MaxAppBundleFileSizeLabel,
			"current_size": fmt.Sprintf("%.2fMB", float64(file.Size)/(1<<20)),
		})
	}
	fileName := strings.TrimSpace(input.FileName)
	if err := validateAppBundleExtension(file, fileName, input.Platform); err != nil {
		return nil, err
	}

	// 重复版本预检（UNIQUE 约束兜底并发窗口，见下方错误映射）。
	if existing, existErr := dal.GetAppBundleByPlatformVersion(ctx, tenantID, input.Platform, version); existErr == nil && existing != nil {
		return nil, errcode.WithVars(errcode.CodeAppBundleVersionExists, map[string]interface{}{
			"platform": input.Platform,
			"version":  version,
		})
	}

	publicPath, checksum, saveErr := saveAppBundleFile(file, fileName, input.Platform)
	if saveErr != nil {
		return nil, saveErr
	}

	now := time.Now().UTC()
	record := &model.MobileAppBundle{
		ID:           strings.ReplaceAll(uuid.NewString(), "-", ""),
		TenantID:     tenantID,
		Platform:     input.Platform,
		Version:      version,
		FileName:     fileName,
		FilePath:     publicPath,
		FileSize:     file.Size,
		Checksum:     checksum,
		ReleaseNotes: releaseNotes,
		Status:       model.AppBundleStatusDraft,
		CreatedAt:    &now,
		UpdatedAt:    &now,
	}
	if err := dal.CreateAppBundle(record); err != nil {
		// 登记失败回滚已落盘文件：不让"失败路径"在 ./files/apps 留孤儿文件。
		if removeErr := removeAppBundleFileFromDisk(publicPath); removeErr != nil {
			logrus.Warnf("app bundle rollback after failed registration: id=%s path=%s err=%v",
				record.ID, publicPath, removeErr)
		}
		if dal.IsDuplicateAppBundleError(err) {
			// 并发重复版本：返回与预检一致的业务码。
			return nil, errcode.WithVars(errcode.CodeAppBundleVersionExists, map[string]interface{}{
				"platform": input.Platform,
				"version":  version,
			})
		}
		return nil, appBundleDBError(err)
	}
	return record, nil
}

// ListAppBundles 分页查询本租户应用包列表。
func (*MobileAppBundleService) ListAppBundles(ctx context.Context, req *model.GetAppBundleListReq, tenantID string) (*model.GetAppBundleListRsp, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	total, list, err := dal.ListAppBundlesForScope(ctx, req, tenantID)
	if err != nil {
		return nil, appBundleDBError(err)
	}
	return &model.GetAppBundleListRsp{Total: total, List: list}, nil
}

// GetAppBundle 查询应用包详情（租户隔离 fail-closed）。
func (*MobileAppBundleService) GetAppBundle(ctx context.Context, id, tenantID string) (*model.MobileAppBundle, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	record, err := dal.GetAppBundleForScope(ctx, id, tenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "app bundle not found")
	}
	return record, nil
}

// UpdateAppBundle 更新应用包：仅 draft 可改（发布后不可变，版本/平台/文件本体均不可换——
// 版本是唯一键，发布后换文件等于让"同一个版本号"指向不同内容，直接禁止）。
func (*MobileAppBundleService) UpdateAppBundle(ctx context.Context, id string, req *model.UpdateAppBundleReq, tenantID string) (*model.MobileAppBundle, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	record, err := dal.GetAppBundleForScope(ctx, id, tenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "app bundle not found")
	}
	if record.Status != model.AppBundleStatusDraft {
		return nil, invalidTransitionError("update", record.Status)
	}
	if req == nil || req.ReleaseNotes == nil {
		return record, nil
	}
	releaseNotes := strings.TrimSpace(*req.ReleaseNotes)
	if len(releaseNotes) > 2000 {
		return nil, appBundleParamError("release notes must not exceed 2000 characters")
	}
	if err := dal.UpdateAppBundleReleaseNotes(ctx, id, tenantID, releaseNotes); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 状态在读取后被并发推进：按非法流转如实报错。
			return nil, invalidTransitionError("update", record.Status)
		}
		return nil, appBundleDBError(err)
	}
	return dal.GetAppBundleForScope(ctx, id, tenantID)
}

// PublishAppBundle 发布：draft→published，落 published_at（UTC）。
// 先读后 CAS（WHERE status=draft），并发下 CAS 未命中时重读并如实报错。
func (*MobileAppBundleService) PublishAppBundle(ctx context.Context, id, tenantID string) (*model.MobileAppBundle, error) {
	return transitionAppBundle(ctx, id, tenantID, AppBundleActionPublish)
}

// ArchiveAppBundle 归档：published→archived。published_at 保留作发布履历。
func (*MobileAppBundleService) ArchiveAppBundle(ctx context.Context, id, tenantID string) (*model.MobileAppBundle, error) {
	return transitionAppBundle(ctx, id, tenantID, AppBundleActionArchive)
}

// transitionAppBundle 状态流转公共实现：读记录 → 纯函数判定 → CAS 落库 → 回读。
func transitionAppBundle(ctx context.Context, id, tenantID, action string) (*model.MobileAppBundle, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	record, err := dal.GetAppBundleForScope(ctx, id, tenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "app bundle not found")
	}
	next, ok := CanTransitionAppBundle(record.Status, action)
	if !ok {
		return nil, invalidTransitionError(action, record.Status)
	}
	var publishedAt *time.Time
	if next == model.AppBundleStatusPublished {
		now := time.Now().UTC()
		publishedAt = &now
	}
	if err := dal.UpdateAppBundleStatus(ctx, id, tenantID, record.Status, next, publishedAt); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// CAS 未命中：记录被并发流转或删除。重读区分两种情况，如实报错。
			reread, readErr := dal.GetAppBundleForScope(ctx, id, tenantID)
			if readErr != nil || reread == nil {
				return nil, errcode.NewWithMessage(errcode.CodeNotFound, "app bundle not found")
			}
			return nil, invalidTransitionError(action, reread.Status)
		}
		return nil, appBundleDBError(err)
	}
	return dal.GetAppBundleForScope(ctx, id, tenantID)
}

// DeleteAppBundle 删除应用包：published 拒绝（先归档）；draft/archived 删磁盘文件+删行，
// 文件已不存在按幂等成功继续删行（自愈孤儿登记）。
func (*MobileAppBundleService) DeleteAppBundle(ctx context.Context, id, tenantID string) error {
	if strings.TrimSpace(tenantID) == "" {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	record, err := dal.GetAppBundleForScope(ctx, id, tenantID)
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeNotFound, "app bundle not found")
	}
	if !canDeleteAppBundle(record.Status) {
		return invalidTransitionError("delete", record.Status)
	}
	if err := removeAppBundleFileFromDisk(record.FilePath); err != nil {
		logrus.Warnf("app bundle disk removal failed before row delete: id=%s path=%s err=%v",
			record.ID, record.FilePath, err)
		return errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	if err := dal.DeleteAppBundleForScope(ctx, id, tenantID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 并发下已被同租户请求删除：按幂等成功处理。
			return nil
		}
		return appBundleDBError(err)
	}
	return nil
}

// AppBundleDownload 下载前置查询：确认记录存在于本租户后返回记录，文件流由 API 层用
// os.OpenInRoot 打开（同租户任意状态可下载：draft 的校验者也必然是刚上传的管理员，
// 同租户内不放大可见面）。
func (*MobileAppBundleService) AppBundleDownload(ctx context.Context, id, tenantID string) (*model.MobileAppBundle, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	record, err := dal.GetAppBundleForScope(ctx, id, tenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "app bundle not found")
	}
	if _, err := resolveAppBundleDiskRelativePath(record.FilePath); err != nil {
		return nil, errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return record, nil
}

// validateAppBundleExtension 校验上传文件扩展名与平台匹配，并做 ZIP 系内容签名检查
// （apk/ipa/zip 均为 ZIP 容器，PK 魔数足以拦住改后缀的伪装文件）。
func validateAppBundleExtension(file *multipart.FileHeader, fileName, platform string) error {
	ext := strings.ToLower(filepath.Ext(fileName))
	allowed := appBundlePlatformExtensions[platform]
	matched := false
	for _, candidate := range allowed {
		if ext == candidate {
			matched = true
			break
		}
	}
	if !matched {
		return errcode.WithVars(errcode.CodeFileTypeMismatch, map[string]interface{}{
			"expected_type": strings.Join(allowed, "/"),
			"actual_type":   ext,
		})
	}

	stream, err := file.Open()
	if err != nil {
		return errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("open upload: %w", err).Error(),
		})
	}
	defer stream.Close()
	header := make([]byte, 4)
	readSize, readErr := io.ReadFull(stream, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("read upload signature: %w", readErr).Error(),
		})
	}
	if readSize < 4 || !(bytes.HasPrefix(header[:readSize], []byte("PK\x03\x04")) ||
		bytes.HasPrefix(header[:readSize], []byte("PK\x05\x06")) ||
		bytes.HasPrefix(header[:readSize], []byte("PK\x07\x08"))) {
		return errcode.NewWithMessage(errcode.CodeFileTypeMismatch, "content signature does not match a zip-based package (apk/ipa/zip)")
	}
	return nil
}

// saveAppBundleFile 把上传文件落盘到 files/apps/<platform>/<日期>/<哈希>.<ext>，
// 复制过程中流式计算 SHA-256。路径生成与包含性校验复用 upload 链路口径
// （BaseUploadDir 根 + os.OpenRoot 受控写入，拒绝目录逃逸）。
// 返回对外访问路径（./files/apps/...）与十六进制校验和。
func saveAppBundleFile(file *multipart.FileHeader, fileName, platform string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	dateDir := time.Now().Format("2006-01-02")
	uploadDir := filepath.Clean(filepath.Join(common.BaseUploadDir, appBundleUploadSubDir, platform, dateDir))
	relativeDir, err := appBundleRelativePathWithinRoot(uploadDir)
	if err != nil {
		return "", "", errcode.WithData(errcode.CodeFilePathGenError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	randomPart := strings.ReplaceAll(uuid.NewString(), "-", "")
	relativePath := path.Join(filepath.ToSlash(relativeDir), randomPart+ext)

	absBaseDir, err := filepath.Abs(common.BaseUploadDir)
	if err != nil {
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("resolve upload base: %w", err).Error(),
		})
	}
	// os.OpenRoot 要求根目录已存在：全新部署（./files 尚未生成）时先补建，避免首次上传必失败。
	if err := os.MkdirAll(absBaseDir, 0o755); err != nil {
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("create upload base: %w", err).Error(),
		})
	}
	root, err := os.OpenRoot(absBaseDir)
	if err != nil {
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("open upload root: %w", err).Error(),
		})
	}
	defer root.Close()

	if err := root.MkdirAll(filepath.Dir(relativePath), 0o755); err != nil {
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("create upload directory: %w", err).Error(),
		})
	}

	source, err := file.Open()
	if err != nil {
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("open uploaded file: %w", err).Error(),
		})
	}
	defer source.Close()

	hasher := sha256.New()
	destination, err := root.OpenFile(relativePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("create uploaded file: %w", err).Error(),
		})
	}
	if _, err := io.Copy(io.MultiWriter(destination, hasher), source); err != nil {
		_ = destination.Close()
		_ = root.Remove(relativePath)
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("save uploaded file: %w", err).Error(),
		})
	}
	if err := destination.Close(); err != nil {
		_ = root.Remove(relativePath)
		return "", "", errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": fmt.Errorf("close uploaded file: %w", err).Error(),
		})
	}

	// 对外访问路径与 upload.go 同口径：以工作目录为基准的 "./files/apps/..."。
	// 注意 relativePath 是相对存储根（files/）的路径，两套基准不能混用——
	// 混用会让登记行指向 ./apps/...，删除/下载的白名单解析随即拒绝。
	publicPath := "./" + filepath.ToSlash(filepath.Clean(filepath.Join(common.BaseUploadDir, relativePath)))
	return publicPath, hex.EncodeToString(hasher.Sum(nil)), nil
}

// appBundleRelativePathWithinRoot 校验目标目录位于 BaseUploadDir 内并返回根内相对路径。
func appBundleRelativePathWithinRoot(dir string) (string, error) {
	absBaseDir, err := filepath.Abs(common.BaseUploadDir)
	if err != nil {
		return "", fmt.Errorf("resolve upload base: %w", err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve upload dir: %w", err)
	}
	relPath, err := filepath.Rel(absBaseDir, absDir)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("upload path escapes base directory")
	}
	return relPath, nil
}

// removeAppBundleFileFromDisk 删除 ./files 存储中的应用包文件本体。
// 路径必须是 files/apps/ 前缀（白名单解析），再做包含性校验；文件已不存在按成功处理。
func removeAppBundleFileFromDisk(publicPath string) error {
	relativePath, err := resolveAppBundleDiskRelativePath(publicPath)
	if err != nil {
		return err
	}
	absBaseDir, err := filepath.Abs(common.BaseUploadDir)
	if err != nil {
		return fmt.Errorf("resolve upload base: %w", err)
	}
	root, err := os.OpenRoot(absBaseDir)
	if err != nil {
		// 存储根整体缺失：视为文件已不存在（幂等成功，登记行仍会被删除）。
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open upload root: %w", err)
	}
	defer root.Close()
	if err := root.Remove(filepath.FromSlash(relativePath)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove app bundle file: %w", err)
	}
	return nil
}

// resolveAppBundleDiskRelativePath 把对外访问路径解析为 BaseUploadDir 内的相对路径。
// 仅接受 files/apps/ 前缀（应用包专用白名单，不复用媒体库的 OTA 特例）；
// 清洗后必须仍落在 apps/ 内（防 .. 借道返回后逃出业务目录），出现 ..、空路径或盘符一律拒绝。
func resolveAppBundleDiskRelativePath(publicPath string) (string, error) {
	trimmed := strings.TrimSpace(publicPath)
	if trimmed == "" {
		return "", fmt.Errorf("empty app bundle path")
	}
	normalized := strings.TrimPrefix(filepath.ToSlash(trimmed), "./")
	if !strings.HasPrefix(normalized, "files/"+appBundleUploadSubDir+"/") {
		return "", fmt.Errorf("app bundle path is outside the apps upload area: %s", publicPath)
	}
	clean := path.Clean(strings.TrimPrefix(normalized, "files/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") ||
		!strings.HasPrefix(clean, appBundleUploadSubDir+"/") {
		return "", fmt.Errorf("app bundle path escapes base directory: %s", publicPath)
	}
	return clean, nil
}
