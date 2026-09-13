package model

import "time"

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

// ImportMarketBundleReq 打包载荷的导入/预览入参（ROADMAP P1.6）。
// Preview=true 时只读：验签与依赖检查照常执行，但不落库、不建模板。
// 预览存在覆盖项（租户内已有同名模板）时，导入必须显式 ConfirmOverwrite 才会继续——
// "已存在模板必须显式列为覆盖项"是路线图门禁，静默覆盖等于让最终状态取决于操作顺序。
type ImportMarketBundleReq struct {
	Bundle           *MarketBundle `json:"bundle" validate:"required"`
	Preview          bool          `json:"preview" validate:"omitempty"`
	ConfirmOverwrite bool          `json:"confirm_overwrite" validate:"omitempty"`
}

// MarketBundleTemplateImportResult 打包导入中单个模板的结果。
type MarketBundleTemplateImportResult struct {
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	Outcome    string `json:"outcome"` // created / idempotent / rejected
	TemplateID string `json:"template_id,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// ImportMarketBundleRsp 打包导入/预览的响应。
type ImportMarketBundleRsp struct {
	Preview MarketBundleImportPreview          `json:"preview"`
	Applied bool                               `json:"applied"` // false=仅预览，未落库
	Results []MarketBundleTemplateImportResult `json:"results,omitempty"`
}

// MarketBundleImportPreview 导入预览：只读判定，不落库。
// 定义在 model 层以便 API 响应直接引用；service 包保留同名别名。
type MarketBundleImportPreview struct {
	Total     int      `json:"total"`
	Create    []string `json:"create"`    // 租户内不存在，导入即新建
	Overwrite []string `json:"overwrite"` // 租户内已存在，导入会覆盖（需人工确认）
	// Blocking 阻断项：包内重名、依赖/自洽问题等。非空即不应导入。
	Blocking []string `json:"blocking"`
}

// HasBlocking 是否存在阻断项。
func (p MarketBundleImportPreview) HasBlocking() bool { return len(p.Blocking) > 0 }

// PHASE-D-D10 END

// TableNameTemplateUpgradeHistory 对应迁移 99.sql 的 device_template_upgrade_history。
// 该表为手写模型（无 .gen.go 产物），常量必须在此声明，否则 TableName() 编译不过。
const TableNameTemplateUpgradeHistory = "device_template_upgrade_history"

// TemplateUpgradeHistory 模板升级历史（P1.6 升级/回滚）。
// PreviousPayload 保存升级前旧版本完整导出载荷——回滚=重放它，
// 而不是"删掉新版本"：删行不可逆，重放幂等且可审计。
type TemplateUpgradeHistory struct {
	ID              string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID        string    `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	TemplateName    string    `gorm:"column:template_name;not null" json:"template_name"`
	FromVersion     string    `gorm:"column:from_version;not null" json:"from_version"`
	ToVersion       string    `gorm:"column:to_version;not null" json:"to_version"`
	PreviousPayload string    `gorm:"column:previous_payload;type:text;not null" json:"-"`
	ActorID         string    `gorm:"column:actor_id;not null" json:"actor_id"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*TemplateUpgradeHistory) TableName() string { return TableNameTemplateUpgradeHistory }

// UpgradeDeviceTemplateReq 模板升级：携带新版本载荷（导出格式），目标版本必须新于当前。
type UpgradeDeviceTemplateReq struct {
	Payload *DeviceTemplateExport `json:"payload" validate:"required"`
}

// UpgradeDeviceTemplateRsp 升级结果。
type UpgradeDeviceTemplateRsp struct {
	HistoryID string          `json:"history_id"`
	Template  *DeviceTemplate `json:"template"`
}
