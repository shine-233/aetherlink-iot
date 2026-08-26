// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ErrRecordNotFound 记录未找到错误
var ErrRecordNotFound = gorm.ErrRecordNotFound

// HashTemplateSecret 计算设备配置密钥的 SHA-256 hex 摘要。
// 写入路径（创建/更新 template_secret）必须统一经过该函数，落库不存明文。
func HashTemplateSecret(templateSecret string) string {
	sum := sha256.Sum256([]byte(templateSecret))
	return hex.EncodeToString(sum[:])
}

// isTemplateSecretDigest 判断库中存储值是否已经是 64 位 hex 摘要格式。
// 兼容策略约定：满足 64 位 hex 即视为摘要，否则视为历史明文行。
func isTemplateSecretDigest(storedValue string) bool {
	if len(storedValue) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(storedValue)
	return err == nil
}

// GetDeviceConfigByTemplateSecret 通过设备配置密钥获取设备配置（摘要与历史明文双读兼容）。
// 库中值为 64 位 hex 时按摘要比对 sha256(呈现值)；否则按历史明文精确比对，
// 命中后惰性升级为摘要，避免破坏性迁移。查询使用两次等值条件（呈现值、其摘要），
// 与原单列等值索引兼容，无需全表扫描。
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetDeviceConfigByTemplateSecret(presentedSecret string) (*model.DeviceConfig, error) {
	if presentedSecret == "" {
		return nil, nil
	}

	presentedDigest := HashTemplateSecret(presentedSecret)
	p := query.DeviceConfig
	// 历史明文行可用呈现值直接命中；摘要行用其摘要直接命中，两条路径都走原索引。
	rows, err := p.Where(p.TemplateSecret.Eq(presentedDigest)).Or(p.TemplateSecret.Eq(presentedSecret)).Order(p.ID).Find()
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		storedValue := ""
		if row.TemplateSecret != nil {
			storedValue = *row.TemplateSecret
		}
		if isTemplateSecretDigest(storedValue) {
			// 摘要行：即使 stored == 呈现值也只认摘要比对结果，防止 64 位 hex 明文的歧义命中。
			if storedValue == presentedDigest {
				return row, nil
			}
			continue
		}
		// 历史明文行：按明文比对成功后惰性升级为摘要；升级失败不阻断本次鉴权。
		if storedValue == presentedSecret {
			upgradeTemplateSecretToDigest(row.ID, presentedDigest)
			return row, nil
		}
	}
	return nil, nil
}

// upgradeTemplateSecretToDigest 将命中的历史明文行回写为 SHA-256 摘要。
// 仅记录失败日志并保持鉴权成功语义，等待下次登录再次尝试升级。
func upgradeTemplateSecretToDigest(configID string, digest string) {
	if _, err := query.DeviceConfig.Where(query.DeviceConfig.ID.Eq(configID)).Update(query.DeviceConfig.TemplateSecret, digest); err != nil {
		logrus.Error("[DeviceAuth][DAL] upgrade template_secret to digest failed:", err)
	}
}

// GetProductByProductKey 通过产品密钥获取产品信息
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetProductByProductKey(productKey string) (*model.Product, error) {
	product, err := query.Product.Where(query.Product.ProductKey.Eq(productKey)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return product, nil
}
