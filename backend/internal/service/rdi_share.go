// RDI device sharing helpers.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

func (*RDI) CreateShareToken(deviceID string, req *model.RDIShareTokenReq, claims *utils.UserClaims) (*model.RDIShareTokenResponse, error) {
	deviceID, expiresIn, err := normalizeRDIShareTokenRequest(deviceID, req, claims)
	if err != nil {
		return nil, err
	}
	token, err := generateRDIShareToken()
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "failed to generate share token")
	}
	record := buildRDIShareTokenRecord(token, claims.ID, expiresIn, time.Now().UTC().Unix())

	// RDI share metadata is stored inside device.additional_info for API
	// compatibility. Take a row lock before pruning/appending tokens so
	// concurrent read-modify-write updates cannot overwrite each other.
	result, err := withLockedRDIShareDevice(deviceID, func(tx *query.QueryTx, device *model.Device) (*lockedRDIShareResult, error) {
		accessErr := assertRDIDeviceAccess(device, claims)
		if accessErr != nil {
			return nil, accessErr
		}
		if err := updateRDIShareTokens(tx, device, record); err != nil {
			return nil, err
		}
		return newLockedRDIShareDeviceResult(device), nil
	})
	if err != nil {
		return nil, err
	}

	return &model.RDIShareTokenResponse{
		DeviceID:   result.device.ID,
		Token:      token,
		SharePath:  "/device/share?share_token=" + token,
		AcceptPath: "/rdi/share-tokens/" + token + "/accept",
		ExpiresAt:  record.ExpiresAt,
	}, nil
}

func (*RDI) AcceptSharedDevice(token string, claims *utils.UserClaims) (*model.RDIAcceptShareResponse, error) {
	if err := validateRDIAcceptShareRequest(token, claims); err != nil {
		return nil, err
	}

	token = strings.TrimSpace(token)
	device, err := getRDIDeviceByShareToken(token)
	if err != nil {
		return nil, err
	}
	if response := rdiAcceptShareFastPath(device, claims); response != nil {
		return response, nil
	}

	return acceptSharedDeviceWithLock(device.ID, token, claims)
}

// RevokeShareToken 由设备拥有者主动作废一个分享令牌。
// 令牌被移除后，凭该令牌的接受请求与匿名配置读取都会失败；此前已凭该令牌接受分享的
// 接收人也会被一并清除，避免出现“令牌已撤销但访问仍然保留”的窗口。
func (*RDI) RevokeShareToken(deviceID string, token string, claims *utils.UserClaims) (*model.RDIRevokeShareResponse, error) {
	deviceID, err := normalizeRDIShareRevokeTarget(deviceID, claims)
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "share token is required")
	}
	tokenHash := hashRDIShareToken(token)

	// 与 AcceptSharedDevice 共用同一把设备行锁，保证撤销相对并发接受是原子的：
	// 要么接受先完成随后被撤销一并清除，要么接受在撤销之后失败。
	result, err := withLockedRDIShareDevice(deviceID, func(tx *query.QueryTx, device *model.Device) (*lockedRDIShareResult, error) {
		if accessErr := assertRDIShareRevokeAccess(device, claims); accessErr != nil {
			return nil, accessErr
		}
		state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
		now := time.Now().UTC().Unix()
		// RemoveToken 只在撤销掉尚未过期的 token 时报 removed；changed 也涵盖顺带清理的失效记录。
		removedToken, _ := state.RemoveToken(tokenHash, now)
		removedRecipients := state.RemoveRecipientsByToken(tokenHash)
		// tokensChanged also becomes true when this call only prunes unrelated
		// expired/invalid records. That cleanup must not turn an unknown token
		// into a successful revoke response; success requires the requested token
		// itself or one of its accepted recipients to have been removed.
		if !removedToken && len(removedRecipients) == 0 {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "share token not found")
		}
		if err := state.Save(tx, device.ID); err != nil {
			return nil, err
		}
		revokedTokens := 0
		if removedToken {
			revokedTokens = 1
		}
		return newLockedRDIShareRevokeResult(device, &model.RDIRevokeShareResponse{
			DeviceID:          device.ID,
			RevokedTokens:     revokedTokens,
			RevokedRecipients: len(removedRecipients),
			RevokedAt:         now,
		}), nil
	})
	if err != nil {
		return nil, err
	}
	return result.revoke, nil
}

// RevokeSharedDeviceRecipient 由设备拥有者主动收回某个接收人的访问权。
// 只清除该接收人记录，令牌本身保持有效，便于拥有者继续把设备分享给其他人。
func (*RDI) RevokeSharedDeviceRecipient(deviceID string, userID string, claims *utils.UserClaims) (*model.RDIRevokeShareResponse, error) {
	deviceID, err := normalizeRDIShareRevokeTarget(deviceID, claims)
	if err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "user id is required")
	}

	result, err := withLockedRDIShareDevice(deviceID, func(tx *query.QueryTx, device *model.Device) (*lockedRDIShareResult, error) {
		return revokeSharedDeviceRecipientLocked(tx, device, userID, claims)
	})
	if err != nil {
		return nil, err
	}
	return result.revoke, nil
}

func revokeSharedDeviceRecipientLocked(tx *query.QueryTx, device *model.Device, userID string, claims *utils.UserClaims) (*lockedRDIShareResult, error) {
	if err := assertRDIShareRevokeAccess(device, claims); err != nil {
		return nil, err
	}
	state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
	if _, found := state.RemoveRecipient(userID); !found {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "share recipient not found")
	}
	if err := state.Save(tx, device.ID); err != nil {
		return nil, err
	}
	return newLockedRDIShareRevokeResult(device, &model.RDIRevokeShareResponse{
		DeviceID:          device.ID,
		RevokedRecipients: 1,
		RevokedAt:         time.Now().UTC().Unix(),
	}), nil
}

func normalizeRDIShareRevokeTarget(deviceID string, claims *utils.UserClaims) (string, error) {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return "", errcode.New(errcode.CodeNoPermission)
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}
	return deviceID, nil
}

// assertRDIShareRevokeAccess 复用 assertRDIDeviceAccess 的写权限口径：
// 仅 SYS_ADMIN、同租户 TENANT_ADMIN 以及设备拥有者可以撤销分享。
// hasTelemetryTenantAccess 在 allowSharedRead=false 时不会把分享接收人算作有权者，
// 因此仅仅是接收人的 TENANT_USER 会在这里被拒绝，撤销能力保持 fail closed。
func assertRDIShareRevokeAccess(device *model.Device, claims *utils.UserClaims) error {
	return assertRDIDeviceAccess(device, claims)
}

func validateRDIAcceptShareRequest(token string, claims *utils.UserClaims) error {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return errcode.New(errcode.CodeNoPermission)
	}
	if strings.TrimSpace(token) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "share token is required")
	}
	return nil
}

func rdiAcceptShareFastPath(device *model.Device, claims *utils.UserClaims) *model.RDIAcceptShareResponse {
	// Only callers that already have owner/admin write access should bypass the
	// recipient registration transaction. A same-tenant TENANT_USER who does not
	// own this device still needs an explicit recipient record for read-only access.
	if hasTelemetryTenantAccess(device, claims, false) {
		return rdiAcceptedShareResponse(device, claims, time.Now().UTC().Unix(), true, false)
	}

	state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
	if recipient, ok := state.FindRecipient(claims.ID); ok {
		return rdiAcceptedShareResponse(device, claims, recipient.AcceptedAt, true, true)
	}
	return nil
}

func acceptSharedDeviceWithLock(deviceID string, token string, claims *utils.UserClaims) (response *model.RDIAcceptShareResponse, err error) {
	now := time.Now().UTC().Unix()
	tokenHash := hashRDIShareToken(token)
	result, err := withLockedRDIShareDevice(deviceID, func(tx *query.QueryTx, device *model.Device) (*lockedRDIShareResult, error) {
		state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
		if recipient, ok := state.FindRecipient(claims.ID); ok {
			return newLockedRDIShareResponseResult(
				device,
				rdiAcceptedShareResponse(device, claims, recipient.AcceptedAt, true, true),
			), nil
		}
		if !state.HasActiveToken(tokenHash, now) {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "share token is invalid or expired")
		}

		state.AppendRecipient(rdiShareRecipientFromClaims(claims, tokenHash, now))
		if err := state.Save(tx, device.ID); err != nil {
			return nil, err
		}
		return newLockedRDIShareDeviceResult(device), nil
	})
	if err != nil {
		return nil, err
	}
	if result.response != nil {
		return result.response, nil
	}

	return rdiAcceptedShareResponse(result.device, claims, now, false, true), nil
}

type lockedRDIShareResult struct {
	device   *model.Device
	response *model.RDIAcceptShareResponse
	revoke   *model.RDIRevokeShareResponse
}

func newLockedRDIShareDeviceResult(device *model.Device) *lockedRDIShareResult {
	return &lockedRDIShareResult{device: device}
}

func newLockedRDIShareResponseResult(device *model.Device, response *model.RDIAcceptShareResponse) *lockedRDIShareResult {
	return &lockedRDIShareResult{
		device:   device,
		response: response,
	}
}

func newLockedRDIShareRevokeResult(device *model.Device, revoke *model.RDIRevokeShareResponse) *lockedRDIShareResult {
	return &lockedRDIShareResult{
		device: device,
		revoke: revoke,
	}
}

func withLockedRDIShareDevice(
	deviceID string,
	fn func(tx *query.QueryTx, device *model.Device) (*lockedRDIShareResult, error),
) (*lockedRDIShareResult, error) {
	tx, err := dal.StartTransaction()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	committed := false
	defer func() {
		if !committed {
			_ = dal.Rollback(tx)
		}
	}()

	device, err := loadRDIShareLockedDevice(tx, deviceID)
	if err != nil {
		return nil, err
	}

	result, err := fn(tx, device)
	if err != nil {
		return nil, err
	}

	err = dal.Commit(tx)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	committed = true
	return result, nil
}

func loadRDIShareLockedDevice(tx *query.QueryTx, deviceID string) (*model.Device, error) {
	lockedDevice, err := dal.GetDeviceByIDForUpdate(tx, deviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "device not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return lockedDevice, nil
}

func rdiShareRecipientFromClaims(claims *utils.UserClaims, tokenHash string, acceptedAt int64) model.RDIShareRecipientRecord {
	return model.RDIShareRecipientRecord{
		UserID:     claims.ID,
		Email:      claims.Email,
		TenantID:   claims.TenantID,
		TokenHash:  tokenHash,
		AcceptedAt: acceptedAt,
	}
}

func rdiAcceptedShareResponse(device *model.Device, claims *utils.UserClaims, acceptedAt int64, alreadyAccepted bool, sharedWithMe bool) *model.RDIAcceptShareResponse {
	return &model.RDIAcceptShareResponse{
		Device: *rdiDeviceConfigResponse(device, rdiDeviceConfigResponseOptions{
			ExposeAlarmEmails: rdiMayExposeAlarmEmails(device, claims),
		}),
		AcceptedAt:      acceptedAt,
		AlreadyAccepted: alreadyAccepted,
		SharedWithMe:    sharedWithMe,
	}
}
func (*RDI) SharedDevices(req *model.RDISharedDeviceListReq, claims *utils.UserClaims) (*model.RDISharedDeviceListResponse, error) {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if req == nil {
		req = &model.RDISharedDeviceListReq{}
	}
	page, pageSize := normalizeRDISharedDevicePage(req.Page, req.PageSize)

	devices, err := findRDIDevicesByAdditionalInfoFragment(claims.ID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	records := filterRDISharedDevices(devices, req, claims)
	return buildRDISharedDeviceListResponse(records, page, pageSize), nil
}

func (*RDI) SharedDeviceConfig(token string) (*model.RDIDeviceConfigResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "share token is required")
	}
	device, err := getRDIDeviceByShareToken(token)
	if err != nil {
		return nil, err
	}
	// A token proves share-read access but carries no authenticated role or
	// ownership identity, so alarm recipient addresses must stay redacted.
	return rdiDeviceConfigResponse(device, rdiDeviceConfigResponseOptions{}), nil
}

func rdiShareRecipientForUser(device *model.Device, claims *utils.UserClaims) (model.RDIShareRecipientRecord, bool) {
	if device == nil || claims == nil || strings.TrimSpace(claims.ID) == "" {
		return model.RDIShareRecipientRecord{}, false
	}
	state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
	for _, recipient := range state.Recipients() {
		if recipient.UserID == claims.ID {
			return recipient, true
		}
	}
	return model.RDIShareRecipientRecord{}, false
}

func pruneRDIShareTokens(tokens []model.RDIShareTokenRecord, now int64) []model.RDIShareTokenRecord {
	if len(tokens) == 0 {
		return nil
	}
	pruned := make([]model.RDIShareTokenRecord, 0, len(tokens))
	for _, token := range tokens {
		if token.TokenHash != "" && token.ExpiresAt > now {
			pruned = append(pruned, token)
		}
	}
	return pruned
}

func rdiShareTokenActive(additional map[string]interface{}, tokenHash string, now int64) bool {
	return newRDIShareState(additional).HasActiveToken(tokenHash, now)
}

func normalizeRDIShareExpiresIn(expiresIn int) int {
	if expiresIn <= 0 {
		return rdiShareTokenDefaultTTL
	}
	if expiresIn > rdiShareTokenMaxTTL {
		return rdiShareTokenMaxTTL
	}
	return expiresIn
}

func generateRDIShareToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashRDIShareToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func getRDIDeviceByShareToken(token string) (*model.Device, error) {
	hash := hashRDIShareToken(token)
	now := time.Now().UTC().Unix()
	devices, err := findRDIDevicesByAdditionalInfoFragment(hash)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if device := findRDIDeviceByActiveShareToken(devices, hash, now); device != nil {
		return device, nil
	}
	return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "share token is invalid or expired")
}

func findRDIDevicesByAdditionalInfoFragment(fragment string) ([]*model.Device, error) {
	var devices []*model.Device
	// This LIKE scan is a compatibility bridge for the current additional_info storage layout.
	// It is intentionally narrow, but it is not the long-term indexed query path for share-token lookup.
	err := query.Device.WithContext(context.Background()).
		UnderlyingDB().
		Model(&model.Device{}).
		Where(`additional_info::text LIKE ? ESCAPE '\'`, "%"+escapeSQLLikeFragment(fragment)+"%").
		Find(&devices).Error
	return devices, err
}

func escapeSQLLikeFragment(fragment string) string {
	var builder strings.Builder
	for _, ch := range fragment {
		switch ch {
		case '\\', '%', '_':
			builder.WriteByte('\\')
		}
		builder.WriteRune(ch)
	}
	return builder.String()
}

func normalizeRDIShareTokenRequest(deviceID string, req *model.RDIShareTokenReq, claims *utils.UserClaims) (string, int, error) {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return "", 0, errcode.New(errcode.CodeNoPermission)
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", 0, errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}
	if req == nil {
		req = &model.RDIShareTokenReq{}
	}
	return deviceID, normalizeRDIShareExpiresIn(req.ExpiresIn), nil
}

func buildRDIShareTokenRecord(token string, userID string, expiresIn int, now int64) model.RDIShareTokenRecord {
	return model.RDIShareTokenRecord{
		TokenHash: hashRDIShareToken(token),
		CreatedBy: userID,
		CreatedAt: now,
		ExpiresAt: now + int64(expiresIn),
	}
}

func updateRDIShareTokens(tx *query.QueryTx, device *model.Device, record model.RDIShareTokenRecord) error {
	now := time.Now().UTC().Unix()
	state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
	state.AppendToken(record, now)
	return state.Save(tx, device.ID)
}

func findRDIDeviceByActiveShareToken(devices []*model.Device, tokenHash string, now int64) *model.Device {
	for _, device := range devices {
		state := newRDIShareState(parseAdditionalInfo(device.AdditionalInfo))
		if state.HasActiveToken(tokenHash, now) {
			return device
		}
	}
	return nil
}

func normalizeRDISharedDevicePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func filterRDISharedDevices(devices []*model.Device, req *model.RDISharedDeviceListReq, claims *utils.UserClaims) []model.RDISharedDeviceRecord {
	filterDeviceID := strings.TrimSpace(req.DeviceID)
	filterDeviceName := strings.ToLower(strings.TrimSpace(req.DeviceName))
	records := make([]model.RDISharedDeviceRecord, 0)
	for _, device := range devices {
		recipient, ok := rdiShareRecipientForUser(device, claims)
		if !ok || !matchesRDISharedDeviceFilters(device, filterDeviceID, filterDeviceName) {
			continue
		}
		records = append(records, model.RDISharedDeviceRecord{
			Device: *rdiDeviceConfigResponse(device, rdiDeviceConfigResponseOptions{
				ExposeAlarmEmails: rdiMayExposeAlarmEmails(device, claims),
			}),
			AcceptedAt: recipient.AcceptedAt,
		})
	}
	return records
}

func matchesRDISharedDeviceFilters(device *model.Device, filterDeviceID string, filterDeviceName string) bool {
	if filterDeviceID != "" && device.ID != filterDeviceID && device.DeviceNumber != filterDeviceID {
		return false
	}
	if filterDeviceName != "" && !strings.Contains(strings.ToLower(SafeDeref(device.Name)), filterDeviceName) {
		return false
	}
	return true
}

func buildRDISharedDeviceListResponse(records []model.RDISharedDeviceRecord, page int, pageSize int) *model.RDISharedDeviceListResponse {
	total := len(records)
	start := (page - 1) * pageSize
	if start >= total {
		return &model.RDISharedDeviceListResponse{Total: total, List: []model.RDISharedDeviceRecord{}}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &model.RDISharedDeviceListResponse{Total: total, List: records[start:end]}
}
