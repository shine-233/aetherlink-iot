// File purpose: TB-12 设备认领（Device Claiming）服务层——签发/撤销/赎回三动作。
// Core logic: 明文 claim_key 只在签发响应出现一次；一次性与过期由 DAL 条件更新把关；
// 赎回在同一事务里「锁令牌 → 常量时间比对 → 消费令牌 → 转移租户」，任一步失败整体回滚。
// Key notes: 存在性错误一律用统一文案，避免向未授权方泄露"设备是否存在/在谁手里"。
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// DeviceClaim TB-12 服务入口（无状态，零值可用）。
type DeviceClaim struct{}

// 认领令牌 TTL 缺省 72h；上限 30 天（2592000s）。
const (
	deviceClaimDefaultTTLSeconds = 72 * 60 * 60
	deviceClaimMaxTTLSeconds     = 30 * 24 * 60 * 60
	deviceClaimKeyPrefix         = "ack_"
	// deviceClaimNotClaimable 统一拒绝文案：不区分"无此设备/无令牌/令牌已失效"，
	// 防止认领方用错误差异探测别的租户的设备清单。
	deviceClaimNotClaimable = "device is not claimable with this key"
)

// IssueClaimToken 签发一次性认领令牌（仅设备当前所属租户的管理员）。
func (*DeviceClaim) IssueClaimToken(_ context.Context, req *model.IssueDeviceClaimTokenReq, claims *utils.UserClaims) (*model.IssueDeviceClaimTokenResp, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.New(errcode.CodeUnauthorized)
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = deviceClaimDefaultTTLSeconds
	}
	if ttl > deviceClaimMaxTTLSeconds {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "ttl_seconds must not exceed 30 days")
	}

	// 设备必须属于签发方租户；不存在/不属于都按 not found 处理（不泄露存在性）。
	device, err := dal.GetDeviceClaimSourceByIDAndTenant(claims.TenantID, deviceID)
	if err != nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}

	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeDBError, "generate claim key failed")
	}
	claimKey := deviceClaimKeyPrefix + hex.EncodeToString(keyBytes)
	sum := sha256.Sum256([]byte(claimKey))
	now := time.Now()
	token := &model.DeviceClaimToken{
		ID:           newDeviceClaimID(),
		TenantID:     claims.TenantID,
		DeviceID:     device.ID,
		DeviceNumber: device.DeviceNumber,
		ClaimKeyHash: hex.EncodeToString(sum[:]),
		Status:       model.DeviceClaimStatusActive,
		ExpiresAt:    now.Add(time.Duration(ttl) * time.Second),
		CreatedAt:    now,
	}

	// 事务保证「旧 active 行标记 replaced」与「新行插入」原子生效，
	// 否则插入失败会留下一条 active 都没有的设备。
	err = dal.WithDeviceClaimTransaction(func(tx *gorm.DB) error {
		if _, err := dal.ReplaceActiveDeviceClaimTokens(tx, device.ID); err != nil {
			return err
		}
		return dal.InsertDeviceClaimToken(tx, token)
	})
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeDBError, "issue claim token failed")
	}

	logrus.WithFields(logrus.Fields{
		"module":        "device_claim",
		"action":        "issue",
		"tenant_id":     claims.TenantID,
		"device_id":     device.ID,
		"token_id":      token.ID,
		"expires_at":    token.ExpiresAt.Format(time.RFC3339),
		"audit_message": "device claim token issued",
	}).Info("device claim token issued")

	return &model.IssueDeviceClaimTokenResp{
		TokenID:      token.ID,
		DeviceID:     device.ID,
		DeviceNumber: device.DeviceNumber,
		ClaimKey:     claimKey,
		ExpiresAt:    token.ExpiresAt,
		CreatedAt:    token.CreatedAt,
	}, nil
}

// ListClaimTokens 签发方回查令牌历史（无明文无哈希；effective 反映过期语义）。
func (*DeviceClaim) ListClaimTokens(_ context.Context, deviceID string, claims *utils.UserClaims) ([]model.DeviceClaimTokenView, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.New(errcode.CodeUnauthorized)
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	rows, err := dal.ListDeviceClaimTokensByDevice(claims.TenantID, deviceID)
	if err != nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	now := time.Now()
	views := make([]model.DeviceClaimTokenView, 0, len(rows))
	for _, row := range rows {
		effective := row.Status
		if row.Status == model.DeviceClaimStatusActive && !row.ExpiresAt.After(now) {
			effective = "expired"
		}
		views = append(views, model.DeviceClaimTokenView{
			TokenID:       row.ID,
			DeviceID:      row.DeviceID,
			DeviceNumber:  row.DeviceNumber,
			Status:        row.Status,
			Effective:     effective,
			ExpiresAt:     row.ExpiresAt,
			ConsumedByTID: row.ConsumedByTenantID,
			ConsumedAt:    row.ConsumedAt,
			CreatedAt:     row.CreatedAt,
		})
	}
	return views, nil
}

// RevokeClaimToken 撤销：只有 active 可撤销；过期/已消费/已替换的行不可逆。
func (*DeviceClaim) RevokeClaimToken(_ context.Context, tokenID string, claims *utils.UserClaims) error {
	if claims == nil || claims.TenantID == "" {
		return errcode.New(errcode.CodeUnauthorized)
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "token_id is required")
	}
	err := dal.WithDeviceClaimTransaction(func(tx *gorm.DB) error {
		if _, err := dal.GetDeviceClaimTokenForUpdate(tx, claims.TenantID, tokenID); err != nil {
			return errcode.New(errcode.CodeNotFound)
		}
		if err := dal.RevokeDeviceClaimToken(tx, tokenID); err != nil {
			if errors.Is(err, dal.ErrDeviceClaimNoRow) {
				return errcode.NewWithMessage(errcode.CodeOpDenied,
					"claim token is not active and cannot be revoked")
			}
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"module": "device_claim", "action": "revoke",
		"tenant_id": claims.TenantID, "token_id": tokenID,
		"audit_message": "device claim token revoked",
	}).Info("device claim token revoked")
	return nil
}

// RedeemClaim 认领设备：凭 device_number + claim_key，把设备从签发租户转移到认领租户。
func (*DeviceClaim) RedeemClaim(_ context.Context, req *model.RedeemDeviceClaimReq, claims *utils.UserClaims) (*model.RedeemDeviceClaimResp, error) {
	if claims == nil || claims.TenantID == "" || claims.ID == "" {
		return nil, errcode.New(errcode.CodeUnauthorized)
	}
	deviceNumber := strings.TrimSpace(req.DeviceNumber)
	claimKey := strings.TrimSpace(req.ClaimKey)
	if deviceNumber == "" || claimKey == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_number and claim_key are required")
	}

	target, err := dal.GetDeviceClaimTargetByNumber(deviceNumber)
	if err != nil {
		// 统一文案：不存在的编号与"有设备但没有可用令牌"不可区分。
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, deviceClaimNotClaimable)
	}
	if target.TenantID == claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied,
			"device already belongs to your tenant")
	}

	now := time.Now()
	resp := &model.RedeemDeviceClaimResp{}
	err = dal.WithDeviceClaimTransaction(func(tx *gorm.DB) error {
		token, err := dal.GetDeviceClaimTokenActiveForDeviceByNumber(tx, target.ID)
		if err != nil {
			return errcode.NewWithMessage(errcode.CodeNotFound, deviceClaimNotClaimable)
		}
		// 常量时间比对：wrong key 与"差一位"的耗时必须一样。
		// 错 key 与"无令牌"必须同码同文案——否则认领方能探测"该设备是否有活跃令牌"。
		sum := sha256.Sum256([]byte(claimKey))
		if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(token.ClaimKeyHash)) != 1 {
			return errcode.NewWithMessage(errcode.CodeNotFound, deviceClaimNotClaimable)
		}
		// 过期判定与一次性都在 consume 的条件更新里（status='active' AND expires_at>now）。
		if err := dal.ConsumeDeviceClaimToken(tx, token.ID, claims.TenantID, claims.ID); err != nil {
			if errors.Is(err, dal.ErrDeviceClaimNoRow) {
				return errcode.NewWithMessage(errcode.CodeNotFound, deviceClaimNotClaimable)
			}
			return err
		}
		// 转移失败（设备刚被删/被并发转移）→ 整个事务回滚，令牌消费一并撤销。
		if err := dal.TransferDeviceTenant(tx, target.ID, token.TenantID, claims.TenantID); err != nil {
			if errors.Is(err, dal.ErrDeviceClaimNoRow) {
				return errcode.NewWithMessage(errcode.CodeNotFound, deviceClaimNotClaimable)
			}
			return err
		}
		resp.DeviceID = target.ID
		resp.DeviceNumber = token.DeviceNumber
		resp.TokenID = token.ID
		resp.PreviousTenID = token.TenantID
		resp.ClaimedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"module": "device_claim", "action": "redeem",
		"tenant_id": claims.TenantID, "user_id": claims.ID,
		"device_id": resp.DeviceID, "token_id": resp.TokenID,
		"previous_tenant_id": resp.PreviousTenID,
		"audit_message":      "device claimed",
	}).Info("device claimed")
	return resp, nil
}

// newDeviceClaimID 生成 varchar(36) 主键（uuid v4 文本，与 104.sql 系列表一致）。
func newDeviceClaimID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 读取失败几乎不可能；主键损坏比报错更糟，直接失败。
		panic(fmt.Sprintf("device claim: read random failed: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
