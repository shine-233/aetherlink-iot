// 文件用途：媒体库服务（media_files，ROADMAP TB-41）。
// 核心逻辑：上传落登记（收编 ./files 存储链路）、租户隔离的列表/详情/删除，
//
//	删除前的引用扫描（看板 config、SCADA 文档、OTA 升级包）与引用计数回写。
//
// 关键注意事项：
//  1. 删除语义取"拒绝"：引用计数>0 时返回 CodeMediaReferenced 并携带引用方列表，
//     绝不级联删除引用方，也不静默放行；磁盘文件删除成功后才删登记行（幂等自愈：
//     文件已不存在的删除请求按成功处理并继续删行）。
//  2. file_path 是对外访问路径，与磁盘真实路径的映射必须走 resolveMediaDiskRelativePath
//     （含 OTA 下载前缀特例），落盘删除前再次做 BaseUploadDir 包含性校验。
//  3. 上传登记无法归属租户（claims.TenantID 为空）时跳过登记并记日志，不阻断原上传流程。
//
// 重构建议：引用扫描目前是 LIKE 包含匹配（v1 口径），看板/SCADA 部件选择器接入后
//
//	应改为写入 media_file_refs 明细表并在保存时维护引用计数。
package service

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// MediaLibraryService 媒体库服务。
type MediaLibraryService struct{}

// MediaUploadRegistration 上传登记参数（由 api.UpFile 在文件落盘成功后组装）。
type MediaUploadRegistration struct {
	FileName string // 上传时的原始文件名（已清洗）
	FilePath string // 对外访问路径（UpFile 返回的 path 值）
	FileSize int64  // 字节数
	Mime     string // MIME 类型（按扩展名推断，未知落 application/octet-stream）
}

// RegisterMediaUpload 上传成功后落一条媒体登记；同租户同路径幂等返回既有记录。
// 返回 (nil, nil) 表示因无法归属租户而跳过登记（不阻断上传主流程）。
func (*MediaLibraryService) RegisterMediaUpload(ctx context.Context, params MediaUploadRegistration, claims *utils.UserClaims) (*model.MediaFile, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		logrus.Warnf("media upload registration skipped: no tenant in claims, file=%s", params.FilePath)
		return nil, nil
	}
	if strings.TrimSpace(params.FilePath) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "media file path is required")
	}

	// 幂等：同租户同路径已登记则直接返回既有行（上传文件名随机，正常不会走到）。
	if existing, err := dal.GetMediaFileByPathInTenant(ctx, claims.TenantID, params.FilePath); err == nil && existing != nil {
		return existing, nil
	}

	mime := strings.TrimSpace(params.Mime)
	if mime == "" {
		mime = "application/octet-stream"
	}
	now := time.Now().UTC()
	record := &model.MediaFile{
		ID:        uuid.New(),
		TenantID:  claims.TenantID,
		FileName:  params.FileName,
		FilePath:  params.FilePath,
		FileSize:  params.FileSize,
		Mime:      mime,
		CreatedAt: &now,
	}
	if err := dal.CreateMediaFile(record); err != nil {
		logrus.Errorf("failed to register media file: %v", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return record, nil
}

// ListMediaFiles 分页查询本租户媒体登记。
func (*MediaLibraryService) ListMediaFiles(ctx context.Context, req *model.GetMediaFileListReq, claims *utils.UserClaims) (*model.GetMediaFileListRsp, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	total, list, err := dal.ListMediaFilesForScope(ctx, req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return &model.GetMediaFileListRsp{Total: total, List: list}, nil
}

// GetMediaFileDetail 查询媒体详情，并实时刷新引用计数（读时统计口径）。
func (*MediaLibraryService) GetMediaFileDetail(ctx context.Context, id string, claims *utils.UserClaims) (*model.MediaFileDetailRsp, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	record, err := dal.GetMediaFileForScope(ctx, id, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "media file not found")
	}
	referencers, refsErr := dal.CountMediaReferencesForPath(ctx, claims.TenantID, record.FilePath)
	if refsErr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": refsErr.Error(),
		})
	}
	if updErr := dal.UpdateMediaFileReferencedCount(ctx, record.ID, claims.TenantID, len(referencers)); updErr != nil {
		logrus.Errorf("failed to refresh media reference count: %v", updErr)
	}
	record.ReferencedCount = len(referencers)
	return &model.MediaFileDetailRsp{File: record, Referencers: referencers}, nil
}

// DeleteMediaFile 删除媒体：引用扫描>0 拒绝（携带引用方），否则删磁盘文件 + 删登记行。
func (*MediaLibraryService) DeleteMediaFile(ctx context.Context, id string, claims *utils.UserClaims) (*model.MediaFileDeleteRsp, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	record, err := dal.GetMediaFileForScope(ctx, id, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "media file not found")
	}

	// 引用扫描以删除时刻为准（fail-closed），并把最新计数回写。
	referencers, refsErr := dal.CountMediaReferencesForPath(ctx, claims.TenantID, record.FilePath)
	if refsErr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": refsErr.Error(),
		})
	}
	if updErr := dal.UpdateMediaFileReferencedCount(ctx, record.ID, claims.TenantID, len(referencers)); updErr != nil {
		logrus.Errorf("failed to refresh media reference count: %v", updErr)
	}
	if len(referencers) > 0 {
		// 拒绝语义：业务码 + 模板变量（文案渲染）+ Data（引用方列表）一起返回。
		referencedErr := errcode.WithData(errcode.CodeMediaReferenced, map[string]interface{}{
			"referenced_count": len(referencers),
			"referencers":      referencers,
		})
		referencedErr.Variables = map[string]interface{}{
			"referenced_count": len(referencers),
		}
		return nil, referencedErr
	}

	if err := removeMediaFileFromDisk(record.FilePath); err != nil {
		return nil, errcode.WithData(errcode.CodeFileSaveError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	if err := dal.DeleteMediaFileForScope(ctx, record.ID, claims.TenantID); err != nil {
		logrus.Errorf("failed to delete media row after file removal: %v", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return &model.MediaFileDeleteRsp{Deleted: true}, nil
}

// removeMediaFileFromDisk 删除 ./files 存储中的文件本体。
// 路径必须先解析为 BaseUploadDir 内的相对路径（含 OTA 前缀特例），再做包含性校验；
// 文件已不存在按成功处理（幂等，让登记行删除流程自愈）。
func removeMediaFileFromDisk(publicPath string) error {
	relativePath, err := resolveMediaDiskRelativePath(publicPath)
	if err != nil {
		return err
	}

	absBaseDir, err := filepath.Abs(common.BaseUploadDir)
	if err != nil {
		return fmt.Errorf("resolve upload base: %w", err)
	}
	absFullPath, err := filepath.Abs(filepath.Join(common.BaseUploadDir, relativePath))
	if err != nil {
		return fmt.Errorf("resolve media path: %w", err)
	}
	relCheck, err := filepath.Rel(absBaseDir, absFullPath)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(os.PathSeparator)) || filepath.IsAbs(relCheck) {
		return fmt.Errorf("media path escapes base directory")
	}

	root, err := os.OpenRoot(absBaseDir)
	if err != nil {
		// 存储根目录整体缺失：视为文件已不存在（幂等成功，登记行仍会被删除）。
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open upload root: %w", err)
	}
	defer root.Close()

	if err := root.Remove(filepath.FromSlash(relativePath)); err != nil {
		if os.IsNotExist(err) {
			// 文件已不存在：按幂等成功处理，登记行仍会被删除（自愈历史孤儿行）。
			return nil
		}
		return fmt.Errorf("remove media file: %w", err)
	}
	return nil
}

// resolveMediaDiskRelativePath 把对外访问路径解析为 BaseUploadDir 内的相对路径。
// 支持两种前缀："./files/..."（普通上传）与 "./api/v1/ota/download/files/..."（OTA
// 升级包对外走下载路由，磁盘真实位置在 files/upgradePackage/ 下）；其余路径拒绝。
func resolveMediaDiskRelativePath(publicPath string) (string, error) {
	trimmed := strings.TrimSpace(publicPath)
	if trimmed == "" {
		return "", fmt.Errorf("empty media path")
	}
	normalized := strings.TrimPrefix(filepath.ToSlash(trimmed), "./")

	otaPrefix := strings.TrimPrefix(common.OtaPath, "./") // api/v1/ota/download/files/
	switch {
	case strings.HasPrefix(normalized, otaPrefix):
		normalized = strings.TrimPrefix(normalized, otaPrefix)
	case strings.HasPrefix(normalized, "files/"):
		normalized = strings.TrimPrefix(normalized, "files/")
	default:
		return "", fmt.Errorf("media path is outside the upload root: %s", publicPath)
	}

	clean := path.Clean(normalized)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("media path escapes base directory: %s", publicPath)
	}
	return clean, nil
}
