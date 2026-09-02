// 文件用途：2FA DAL 层（ROADMAP C7）。
// 核心逻辑：user_totp 状态读写 + 恢复码发放/一次性消费（used_at 原子占位防重放）。
package dal

import (
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// GetUserTOTP 读取用户 TOTP 状态；未绑定返回 gorm.ErrRecordNotFound。
func GetUserTOTP(userID string) (*model.UserTOTP, error) {
	var row model.UserTOTP
	err := global.DB.Where("user_id = ?", userID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// SaveUserTOTPSecret 首次绑定：写入密文并启用；已存在则覆盖密文并保持启用。
func SaveUserTOTPSecret(userID, cipher string) error {
	return global.DB.Exec(
		`INSERT INTO user_totp (user_id, secret_cipher, enabled, last_used_step, updated_at)
		 VALUES (?, ?, TRUE, 0, ?)
		 ON CONFLICT(user_id) DO UPDATE SET secret_cipher = EXCLUDED.secret_cipher,
		   enabled = TRUE, last_used_step = 0, updated_at = EXCLUDED.updated_at`,
		userID, cipher, time.Now().UTC()).Error
}

// DisableUserTOTP 解绑：删除状态与恢复码。
func DisableUserTOTP(userID string) error {
	if err := global.DB.Where("user_id = ?", userID).Delete(&model.UserTOTP{}).Error; err != nil {
		return err
	}
	return global.DB.Where("user_id = ?", userID).Delete(&model.UserTOTPRecoveryCode{}).Error
}

// SetTOTPLastUsedStep 记录本窗口已消费的步号（防同一验证码重放）。
func SetTOTPLastUsedStep(userID string, step int64) error {
	return global.DB.Model(&model.UserTOTP{}).
		Where("user_id = ?", userID).
		Update("last_used_step", step).Error
}

// CreateUserTOTPRecoveryCodes 批量发放恢复码（hash 幂等）。
func CreateUserTOTPRecoveryCodes(codes []*model.UserTOTPRecoveryCode) error {
	if len(codes) == 0 {
		return nil
	}
	return global.DB.CreateInBatches(codes, 50).Error
}

// ConsumeUserTOTPRecoveryCode 原子消费一个恢复码：返回受影响行数（0=不存在或已用）。
func ConsumeUserTOTPRecoveryCode(userID, codeHash string) (bool, error) {
	now := time.Now().UTC()
	res := global.DB.Model(&model.UserTOTPRecoveryCode{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, strings.ToLower(codeHash)).
		Update("used_at", &now)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ListUnusedRecoveryCodeHashes 列出未用恢复码（绑定结果校验用）。
func ListUnusedRecoveryCodeHashes(userID string) ([]string, error) {
	var rows []struct {
		CodeHash string `gorm:"column:code_hash"`
	}
	err := global.DB.Model(&model.UserTOTPRecoveryCode{}).
		Select("code_hash").Where("user_id = ? AND used_at IS NULL", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.CodeHash)
	}
	return out, nil
}
