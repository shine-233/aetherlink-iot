package model

// PHASE-D-D10 BEGIN 模板市场运营化模型

// MarketCatalogEntry 行业分类目录条目（浏览页 tab 数据源）。
type MarketCatalogEntry struct {
	TypeKey       string `json:"type_key" gorm:"column:type_key"`             // 行业类型（空串=未分类）
	TemplateCount int64  `json:"template_count" gorm:"column:template_count"` // 模板数
	DownloadCount int64  `json:"download_count" gorm:"column:download_count"` // 累计导出次数
}

// MarketBundle 按行业打包的导出载荷（导入端可直接逐个回放 import 接口）。
type MarketBundle struct {
	TypeKey    string                  `json:"type_key"`    // 行业类型（空串=全量）
	ExportedAt int64                   `json:"exported_at"` // 导出时间（unix 毫秒）
	Count      int                     `json:"count"`       // 包含模板数
	Templates  []*DeviceTemplateExport `json:"templates"`   // 模板导出描述符列表

	// 以下三项由签名逻辑填充，均 omitempty：老包没有这些字段时解析不受影响。
	// 但**导入侧一律要求签名**——未签名的包不得导入。
	Digest      string `json:"digest,omitempty"`        // 内容摘要（hex SHA-256），覆盖除签名三字段外的规范 JSON
	Signature   string `json:"signature,omitempty"`     // HMAC-SHA256(规范 JSON)，hex
	SignedKeyID string `json:"signed_key_id,omitempty"` // 签名所用密钥 ID，便于轮换
}

// PHASE-D-D10 END
