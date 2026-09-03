// ota.go maintains OTA upgrade packages, task metadata, and device-side
// upgrade progress. Package paths, version compatibility, and device
// permissions must stay strict because OTA changes device firmware state.
package service

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

type OTA struct{}

func ensureOTAPackageAccess(packageID string, claims *utils.UserClaims) (*model.OtaUpgradePackage, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to access ota package")
	}
	pkg, err := dal.GetOtaUpgradePackageByID(packageID)
	if err != nil {
		return nil, err
	}
	if claims.Authority != constant.SYS_ADMIN {
		if pkg.TenantID == nil || *pkg.TenantID != claims.TenantID {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to access ota package")
		}
	}
	return pkg, nil
}

func ensureOTADeviceWriteAccess(deviceIDs []string, claims *utils.UserClaims) error {
	if len(deviceIDs) == 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "device_id_list is required")
	}
	normalizedDeviceIDs, err := normalizeOTADeviceWriteAccessIDs(deviceIDs)
	if err != nil {
		return err
	}
	if err := requireTelemetryClaims(claims, telemetryWritePermissionMessage); err != nil {
		return err
	}

	devicesByID, err := loadOTADevicesForWriteAccess(normalizedDeviceIDs, claims)
	if err != nil {
		return err
	}
	for _, deviceID := range normalizedDeviceIDs {
		deviceInfo := devicesByID[deviceID]
		if !hasTelemetryTenantAccess(deviceInfo, claims, false) {
			return errcode.NewWithMessage(errcode.CodeNoPermission, telemetryWritePermissionMessage)
		}
	}
	return nil
}

func normalizeOTADeviceWriteAccessIDs(deviceIDs []string) ([]string, error) {
	normalizedDeviceIDs := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		normalizedDeviceID, err := requireTelemetryDeviceID(deviceID)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalizedDeviceID]; ok {
			continue
		}
		seen[normalizedDeviceID] = struct{}{}
		normalizedDeviceIDs = append(normalizedDeviceIDs, normalizedDeviceID)
	}
	return normalizedDeviceIDs, nil
}

func loadOTADevicesForWriteAccess(deviceIDs []string, claims *utils.UserClaims) (map[string]*model.Device, error) {
	if claims.Authority == constant.SYS_ADMIN {
		return dal.GetDevicesByIDsUnscoped(deviceIDs)
	}
	return dal.GetDevicesByIDsForTenant(deviceIDs, claims.TenantID)
}

func ensureOTATaskAccess(taskID string, claims *utils.UserClaims) (*model.OtaUpgradeTask, error) {
	ownerUserID, err := otaTaskOwnerUserIDForClaims(claims)
	if err != nil {
		return nil, err
	}
	task, err := query.OtaUpgradeTask.Where(query.OtaUpgradeTask.ID.Eq(taskID)).First()
	if err != nil {
		return nil, err
	}
	if _, err := ensureOTAPackageAccess(task.OtaUpgradePackageID, claims); err != nil {
		return nil, err
	}
	if ownerUserID != nil {
		owned, ownershipErr := dal.OTAUpgradeTaskDevicesOwnedBy(task.ID, *ownerUserID)
		if ownershipErr != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": ownershipErr.Error(),
			})
		}
		if !owned {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to access ota task")
		}
	}
	return task, nil
}

func otaTaskOwnerUserIDForClaims(claims *utils.UserClaims) (*string, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to access ota task")
	}
	if claims.Authority == constant.TENANT_ADMIN || claims.Authority == constant.SYS_ADMIN {
		return nil, nil
	}
	if claims.Authority != constant.TENANT_USER {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to access ota task")
	}
	ownerUserID := strings.TrimSpace(claims.ID)
	if ownerUserID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to access ota task")
	}
	return &ownerUserID, nil
}

func otaPackageLocalPathFromURL(packageURL string) (string, error) {
	cleanRel, err := otaPackageRelativePathFromURL(packageURL)
	if err != nil {
		return "", err
	}
	base, err := filepath.Abs("./files/upgradePackage")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, filepath.FromSlash(cleanRel)), nil
}

func otaPackageRelativePathFromURL(packageURL string) (string, error) {
	rawPath := strings.TrimSpace(packageURL)
	if rawPath == "" {
		return "", fmt.Errorf("package_url is required")
	}
	if parsed, err := url.Parse(rawPath); err == nil && parsed.Path != "" {
		rawPath = parsed.Path
	}
	rawPath = strings.ReplaceAll(rawPath, "\\", "/")
	if unescaped, err := url.PathUnescape(rawPath); err == nil {
		rawPath = strings.ReplaceAll(unescaped, "\\", "/")
	}
	markers := []string{
		"/api/v1/ota/download/files/upgradePackage/",
		"api/v1/ota/download/files/upgradePackage/",
		"/files/upgradePackage/",
		"files/upgradePackage/",
	}
	for _, marker := range markers {
		if idx := strings.Index(rawPath, marker); idx >= 0 {
			rawPath = rawPath[idx+len(marker):]
			break
		}
	}
	for _, segment := range strings.Split(strings.TrimPrefix(rawPath, "/"), "/") {
		if segment == ".." {
			return "", fmt.Errorf("invalid ota package path")
		}
	}
	cleanRel := path.Clean(strings.TrimPrefix(rawPath, "/"))
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, "../") {
		return "", fmt.Errorf("invalid ota package path")
	}
	if strings.Contains(cleanRel, ":") || filepath.IsAbs(filepath.FromSlash(cleanRel)) {
		return "", fmt.Errorf("invalid ota package path")
	}
	return cleanRel, nil
}

func signOTAPackageFromURL(packageURL, signatureType string) (string, error) {
	cleanRel, err := otaPackageRelativePathFromURL(packageURL)
	if err != nil {
		return "", err
	}
	return utils.FileSignInRoot("./files/upgradePackage", cleanRel, signatureType)
}

func (*OTA) CreateOTAUpgradePackage(req *model.CreateOTAUpgradePackageReq, tenantID string) error {
	if req.PackageUrl == nil || strings.TrimSpace(*req.PackageUrl) == "" {
		return fmt.Errorf("package_url is required")
	}
	if req.SignatureType == nil || strings.TrimSpace(*req.SignatureType) == "" {
		defaultSignatureType := "MD5"
		req.SignatureType = &defaultSignatureType
	}

	var ota = model.OtaUpgradePackage{}
	ota.ID = uuid.New()
	ota.Name = req.Name
	ota.Version = req.Version
	ota.TargetVersion = req.TargetVersion
	ota.DeviceConfigID = req.DeviceConfigID
	ota.Module = req.Module
	ota.PackageType = *req.PackageType
	ota.SignatureType = req.SignatureType

	// 生成文件签名
	fileURL := *req.PackageUrl
	signature, err := signOTAPackageFromURL(fileURL, *req.SignatureType)
	if err != nil {
		return err
	}
	ota.Signature = &signature

	ota.AdditionalInfo = req.AdditionalInfo
	defaultAdditionalInfo := "{}"
	if req.AdditionalInfo == nil || *req.AdditionalInfo == "" {
		ota.AdditionalInfo = &defaultAdditionalInfo
	}
	ota.Description = req.Description
	ota.PackageURL = req.PackageUrl
	ota.TenantID = &tenantID

	t := time.Now().UTC()
	ota.CreatedAt = t
	ota.UpdatedAt = &t
	ota.Remark = req.Remark
	err = dal.CreateOtaUpgradePackage(&ota)
	return err
}

func (*OTA) UpdateOTAUpgradePackage(req *model.UpdateOTAUpgradePackageReq, claims *utils.UserClaims) error {
	oldota, err := ensureOTAPackageAccess(req.Id, claims)
	if err != nil {
		return err
	}

	signatureType := resolveOTAUpdateSignatureType(req, oldota)
	ota := buildOTAUpdatePackage(req, signatureType)
	if err := refreshOTAUpdateSignatureIfNeeded(&ota, req, oldota, signatureType); err != nil {
		return err
	}

	t := time.Now().UTC()
	ota.UpdatedAt = &t
	ota.Remark = req.Remark
	info, err := dal.UpdateOtaUpgradePackage(&ota)
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data updated")
	}
	return nil
}

func resolveOTAUpdateSignatureType(req *model.UpdateOTAUpgradePackageReq, oldota *model.OtaUpgradePackage) *string {
	signatureType := oldota.SignatureType
	if req.SignatureType != nil && strings.TrimSpace(*req.SignatureType) != "" {
		normalized := strings.TrimSpace(*req.SignatureType)
		signatureType = &normalized
	}
	if signatureType == nil || strings.TrimSpace(*signatureType) == "" {
		defaultSignatureType := "MD5"
		signatureType = &defaultSignatureType
	}
	return signatureType
}

func buildOTAUpdatePackage(req *model.UpdateOTAUpgradePackageReq, signatureType *string) model.OtaUpgradePackage {
	ota := model.OtaUpgradePackage{
		ID:             req.Id,
		TargetVersion:  req.TargetVersion,
		Module:         req.Module,
		SignatureType:  signatureType,
		AdditionalInfo: req.AdditionalInfo,
		Description:    req.Description,
		PackageURL:     req.PackageUrl,
	}
	if req.Name != "" {
		ota.Name = req.Name
	}
	if req.Version != "" {
		ota.Version = req.Version
	}
	if req.DeviceConfigID != "" {
		ota.DeviceConfigID = req.DeviceConfigID
	}
	if req.PackageType != nil {
		ota.PackageType = *req.PackageType
	}
	return ota
}

func refreshOTAUpdateSignatureIfNeeded(ota *model.OtaUpgradePackage, req *model.UpdateOTAUpgradePackageReq, oldota *model.OtaUpgradePackage, signatureType *string) error {
	if !otaUpdateSignatureInputsChanged(req, oldota, signatureType) {
		return nil
	}
	packageURL := otaUpdatePackageURL(req, oldota)
	if packageURL == nil || strings.TrimSpace(*packageURL) == "" {
		return fmt.Errorf("package_url is required")
	}
	signature, err := signOTAPackageFromURL(*packageURL, *signatureType)
	if err != nil {
		return err
	}
	ota.Signature = &signature
	return nil
}

func otaUpdateSignatureInputsChanged(req *model.UpdateOTAUpgradePackageReq, oldota *model.OtaUpgradePackage, signatureType *string) bool {
	packageURLChanged := req.PackageUrl != nil && (oldota.PackageURL == nil || *req.PackageUrl != *oldota.PackageURL)
	signatureTypeChanged := oldota.SignatureType == nil || strings.TrimSpace(*signatureType) != strings.TrimSpace(*oldota.SignatureType)
	return packageURLChanged || signatureTypeChanged
}

func otaUpdatePackageURL(req *model.UpdateOTAUpgradePackageReq, oldota *model.OtaUpgradePackage) *string {
	if req.PackageUrl != nil {
		return req.PackageUrl
	}
	return oldota.PackageURL
}

func (*OTA) DeleteOTAUpgradePackage(packageId string, claims *utils.UserClaims) error {
	if _, err := ensureOTAPackageAccess(packageId, claims); err != nil {
		return err
	}
	err := dal.DeleteOtaUpgradePackage(packageId)
	return err
}

// otaUpgradePackageListScopes 解析 OTA 升级包列表读作用域（ROADMAP C2 自上而下）：
// TENANT_USER 保持 self-only（升级包为租户级资源、无 per-user 维度），空租户返回 nil fail-closed；
// 空租户管理员（SYS_ADMIN 平台包，tenant_id 为空串）→ [""] 保持旧行为；
// 其余非空租户管理员 → expandTenantIDScope（self∪子孙，链接缺失回退 self-only）。
func otaUpgradePackageListScopes(claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	if claims.Authority == constant.TENANT_USER {
		if tenantID := strings.TrimSpace(claims.TenantID); tenantID != "" {
			return []string{tenantID}
		}
		return nil
	}
	if strings.TrimSpace(claims.TenantID) == "" {
		return []string{""}
	}
	return expandTenantIDScope(claims.TenantID)
}

func (*OTA) GetOTAUpgradePackageListByPage(req *model.GetOTAUpgradePackageLisyByPageReq, userClaims *utils.UserClaims) (map[string]interface{}, error) {
	if userClaims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to list ota packages")
	}
	total, list, err := dal.GetOtaUpgradePackageListByPage(req, otaUpgradePackageListScopes(userClaims))
	if err != nil {
		return nil, err
	}
	packageListRspMap := make(map[string]interface{})
	packageListRspMap["total"] = total
	packageListRspMap["list"] = list
	return packageListRspMap, nil

}
