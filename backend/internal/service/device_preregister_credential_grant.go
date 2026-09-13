// 文件用途：预注册凭证的**一次性下载**服务（ROADMAP P0.5）。
// 核心逻辑：为批次签发下载许可 → 下载时条件更新消费 → 第二次下载必然被拒。
// 关键注意事项：
//  1. **一次性的落点在数据库的 RowsAffected，不在应用层的 if**。
//     服务层即使先查到 pending 也不能据此放行，必须消费成功（RowsAffected=1）才下发明文。
//  2. **过期先于消费判定**：过期许可即便仍是 pending 也必须拒绝，
//     否则"设了过期时间却还能下载"等于过期形同虚设。
//  3. **不提供重新签发**：一个批次只能有一个 pending 许可，消费即终结。
//     允许补发就等于把一次性凭证变成可重复获取，门禁直接失守。
//  4. 跨租户一律表现为未命中，不泄露"该批次在别处存在"。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

// 许可默认有效期。刻意给一个较短的窗口：凭证明文的下发窗口越短，
// 被截获后可用的时间越短。
const credentialGrantDefaultTTL = 30 * time.Minute

// 服务层错误。
var (
	// ErrCredentialGrantNotPending 许可已消费/已失效。
	ErrCredentialGrantNotPending = errors.New("credential download link is no longer available")
	// ErrCredentialGrantExpired 许可已过期。
	ErrCredentialGrantExpired = errors.New("credential download link has expired")
	// ErrCredentialGrantPendingExists 该批次已有待消费许可。
	ErrCredentialGrantPendingExists = errors.New("this batch already has a pending credential download")
	// ErrCredentialGrantBatchEmpty 批次下没有设备。
	ErrCredentialGrantBatchEmpty = errors.New("batch has no pre-registered devices")
)

// CredentialGrantView 签发结果：只暴露调用方需要知道的字段。
// 不回显任何凭证内容——签发这一步与凭证明文本身无关。
type CredentialGrantView struct {
	ID          string    `json:"id"`
	BatchNumber string    `json:"batch_number"`
	DeviceCount int       `json:"device_count"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// CredentialDownload 一次成功的下载结果。
type CredentialDownload struct {
	BatchNumber string          `json:"batch_number"`
	Rows        []CredentialRow `json:"rows"`
	ConsumedAt  time.Time       `json:"consumed_at"`
}

// CredentialRow 一行凭证明文：设备编号 + 名称 + 凭证。
type CredentialRow struct {
	DeviceNumber string `json:"device_number"`
	Name         string `json:"name"`
	Voucher      string `json:"voucher"`
}

// preRegisterBatchDevices 读取批次内的预注册设备（按 device_number 排序，
// 保证顺序稳定——顺序不稳会让运维无法比对两次结果）。
func preRegisterBatchDevices(tenantID, batchNumber string) ([]model.Device, error) {
	return dal.ListPreRegisterDevicesByBatch(tenantID, batchNumber)
}

// GrantPreRegisterCredentials 为一个批次签发一次性下载许可。
// 前置校验：批次非空、批次内确有设备、该批次没有待消费许可。
func GrantPreRegisterCredentials(ctx context.Context, claims *utils.UserClaims, batchNumber string) (*CredentialGrantView, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "credential grant requires authenticated claims")
	}
	if err := model.ValidateCredentialGrantBatchNumber(batchNumber); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	batch := strings.TrimSpace(batchNumber)
	tenantID := claims.TenantID

	devices, err := preRegisterBatchDevices(tenantID, batch)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if len(devices) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, ErrCredentialGrantBatchEmpty.Error())
	}

	existing, err := dal.FindPendingCredentialGrant(tenantID, batch)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if existing != nil {
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, ErrCredentialGrantPendingExists.Error())
	}

	now := time.Now().UTC()
	grant := &model.DevicePreRegisterCredentialGrant{
		ID:          uuid.New(),
		TenantID:    tenantID,
		BatchNumber: batch,
		DeviceCount: len(devices),
		Status:      model.CredentialGrantStatusPending,
		ExpiresAt:   now.Add(credentialGrantDefaultTTL),
		CreatedBy:   claims.ID,
		CreatedAt:   now,
	}
	if err := dal.CreateCredentialGrant(grant); err != nil {
		// 唯一索引冲突意味着并发签发；按"已存在待消费许可"处理，不暴露驱动原文。
		if existingAfterRace, ferr := dal.FindPendingCredentialGrant(tenantID, batch); ferr == nil && existingAfterRace != nil {
			return nil, errcode.NewWithMessage(errcode.CodeOpDenied, ErrCredentialGrantPendingExists.Error())
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return &CredentialGrantView{
		ID:          grant.ID,
		BatchNumber: grant.BatchNumber,
		DeviceCount: grant.DeviceCount,
		ExpiresAt:   grant.ExpiresAt,
	}, nil
}

// DownloadPreRegisterCredentials 消费许可并返回该批次的凭证明文。
// 只有"条件更新成功"这一个路径会走到下发明文；其余一律拒绝。
func DownloadPreRegisterCredentials(ctx context.Context, claims *utils.UserClaims, grantID string) (*CredentialDownload, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "credential download requires authenticated claims")
	}
	if strings.TrimSpace(grantID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "grant id is required")
	}
	tenantID := claims.TenantID

	grant, err := dal.GetCredentialGrantInTenant(grantID, tenantID)
	if err != nil {
		// 跨租户与不存在都表现为未命中，不区分。
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, ErrCredentialGrantNotPending.Error())
	}

	now := time.Now().UTC()
	// 过期先于一切：pending 但已过期同样拒绝，否则过期时间形同虚设。
	if !grant.ExpiresAt.After(now) {
		if _, eerr := dal.ExpireCredentialGrantByID(grantID, tenantID); eerr == nil {
			// 收口失败不阻断本次拒绝，下一次过期收口任务会处理。
			_ = eerr
		}
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, ErrCredentialGrantExpired.Error())
	}

	affected, err := dal.ConsumeCredentialGrant(grantID, tenantID, claims.ID, now)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		// 并发场景下另一个请求刚消费掉：必须拒绝，绝不能"查到 pending 就放行"。
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, ErrCredentialGrantNotPending.Error())
	}

	devices, err := preRegisterBatchDevices(tenantID, grant.BatchNumber)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	rows := make([]CredentialRow, 0, len(devices))
	for _, d := range devices {
		name := ""
		if d.Name != nil {
			name = *d.Name
		}
		rows = append(rows, CredentialRow{
			DeviceNumber: d.DeviceNumber,
			Name:         name,
			Voucher:      d.Voucher,
		})
	}
	return &CredentialDownload{
		BatchNumber: grant.BatchNumber,
		Rows:        rows,
		ConsumedAt:  now,
	}, nil
}
