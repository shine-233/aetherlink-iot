package model

import "time"

// PHASE-D-D10 BEGIN 模板市场运营化模型

// MarketCatalogEntry 行业分类目录条目（浏览页 tab 数据源）。
type MarketCatalogEntry struct {
	TypeKey       string `json:"type_key" gorm:"column:type_key"`             // 行业类型（空串=未分类）
	TemplateCount int64  `json:"template_count" gorm:"column:template_count"` // 模板数
	DownloadCount int64  `json:"download_count" gorm:"column:download_count"` // 累计导出次数
}

// BoardTemplateExport 看板/大屏模板导出描述符（ROADMAP TP-5 资源中心）。
type BoardTemplateExport struct {
	Kind        string  `json:"kind" validate:"omitempty"`                 // 固定 aetherlink-board-template，导入侧校验
	Name        string  `json:"name" validate:"required,max=255"`          // 看板模板名称
	Author      *string `json:"author" validate:"omitempty,max=99"`        // 作者
	Version     *string `json:"version" validate:"omitempty,max=36"`       // 版本号
	Description *string `json:"description" validate:"omitempty,max=500"`  // 描述
	Remark      *string `json:"remark" validate:"omitempty,max=255"`       // 备注
	Path        *string `json:"path" validate:"omitempty,max=255"`         // 封面/缩略图路径
	VisType     *string `json:"vis_type" validate:"omitempty,max=50"`      // native / thingsvis
	TypeKey     *string `json:"type_key" validate:"omitempty,max=64"`      // 行业分类
	Config      *string `json:"config" validate:"omitempty"`               // 看板配置 JSON
	HomeFlag    string  `json:"home_flag,omitempty" validate:"omitempty"`  // 首页标记
	MenuFlag    string  `json:"menu_flag,omitempty" validate:"omitempty"`  // 菜单标记
	ExportedAt  string  `json:"exported_at" validate:"omitempty"`          // 导出时间（RFC3339）
}

// ImportBoardTemplateReq 看板模板导入入参。
type ImportBoardTemplateReq = BoardTemplateExport

// MarketBundle 按行业打包的导出载荷（导入端可直接逐个回放 import 接口）。
type MarketBundle struct {
	TypeKey    string                  `json:"type_key"`              // 行业类型（空串=全量）
	ExportedAt int64                   `json:"exported_at"`           // 导出时间（unix 毫秒）
	Count      int                     `json:"count"`                 // 包含资源总数
	Templates  []*DeviceTemplateExport `json:"templates"`             // 模板导出描述符列表
	Boards     []*BoardTemplateExport  `json:"boards,omitempty"`     // 看板/大屏模板导出描述符列表（TP-5 资源中心）

	// 以下三项由签名逻辑填充，均 omitempty：老包没有这些字段时解析不受影响。
	// 但**导入侧一律要求签名**——未签名的包不得导入。
	Digest      string `json:"digest,omitempty"`        // 内容摘要（hex SHA-256），覆盖除签名三字段外的规范 JSON
	Signature   string `json:"signature,omitempty"`     // HMAC-SHA256(规范 JSON)，hex
	SignedKeyID string `json:"signed_key_id,omitempty"` // 签名所用密钥 ID，便于轮换
}

// ImportMarketBundleReq 打包载荷的导入/预览入参（ROADMAP P1.6 / TP-5）。
// Preview=true 时只读：验签与依赖检查照常执行，但不落库、不建模板。
// 预览存在覆盖项（租户内已有同名模板或看板）时，导入必须显式 ConfirmOverwrite 才会继续——
// "已存在模板必须显式列为覆盖项"是路线图门禁，静默覆盖等于让最终状态取决于操作顺序。
type ImportMarketBundleReq struct {
	Bundle           *MarketBundle `json:"bundle" validate:"required"`
	Preview          bool          `json:"preview" validate:"omitempty"`
	ConfirmOverwrite bool          `json:"confirm_overwrite" validate:"omitempty"`
}

// MarketBundleTemplateImportResult 打包导入中单个资源的结果。
type MarketBundleTemplateImportResult struct {
	Kind       string `json:"kind,omitempty"` // device_template / board_template
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	Outcome    string `json:"outcome"` // created / updated / idempotent / rejected
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
	Total             int      `json:"total"`
	Create            []string `json:"create"`              // 租户内不存在，导入即新建
	Overwrite         []string `json:"overwrite"`           // 租户内已存在，导入会覆盖（需人工确认）
	Blocking          []string `json:"blocking"`            // 阻断项：包内重名、依赖/自洽问题等。非空即不应导入。
	TemplateCreate    []string `json:"template_create"`
	TemplateOverwrite []string `json:"template_overwrite"`
	BoardCreate       []string `json:"board_create"`
	BoardOverwrite    []string `json:"board_overwrite"`
}

// HasBlocking 是否存在阻断项。
func (p MarketBundleImportPreview) HasBlocking() bool { return len(p.Blocking) > 0 }

// ResourceCenterItem 资源中心统一项（物模型 / 大屏看板统一列表）。
type ResourceCenterItem struct {
	ID            string    `json:"id"`
	ResourceType  string    `json:"resource_type"` // "device_template" | "board_template"
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	Author        string    `json:"author"`
	Description   string    `json:"description"`
	TypeKey       string    `json:"type_key"`
	Path          string    `json:"path"` // 封面或图表
	VisType       string    `json:"vis_type,omitempty"`
	DownloadCount int64     `json:"download_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ResourceCenterCatalogEntry 资源中心综合目录条目。
type ResourceCenterCatalogEntry struct {
	TypeKey       string `json:"type_key"`
	Name          string `json:"name"`
	DeviceCount   int64  `json:"device_count"`
	BoardCount    int64  `json:"board_count"`
	TotalCount    int64  `json:"total_count"`
	DownloadCount int64  `json:"download_count"`
}

// ResourceCenterListReq 资源中心综合分页查询。
type ResourceCenterListReq struct {
	Page         int    `form:"page" json:"page" validate:"required,min=1"`
	PageSize     int    `form:"page_size" json:"page_size" validate:"required,min=1,max=200"`
	ResourceType string `form:"resource_type" json:"resource_type" validate:"omitempty,oneof=all device_template board_template"`
	TypeKey      string `form:"type_key" json:"type_key" validate:"omitempty,max=64"`
	Keyword      string `form:"keyword" json:"keyword" validate:"omitempty,max=100"`
}

// ResourceCenterListRsp 资源中心综合分页响应。
type ResourceCenterListRsp struct {
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	List     []ResourceCenterItem `json:"list"`
}

// ResourceCenterApplyReq 资源中心一键应用/安装请求。
type ResourceCenterApplyReq struct {
	ResourceType string `json:"resource_type" validate:"required,oneof=device_template board_template"`
	ResourceID   string `json:"resource_id" validate:"required"`
	TargetName   string `json:"target_name" validate:"omitempty,max=255"`
}

// ResourceCenterApplyRsp 资源中心一键应用响应。
type ResourceCenterApplyRsp struct {
	ResourceType string      `json:"resource_type"`
	TargetID     string      `json:"target_id"`
	TargetName   string      `json:"target_name"`
	Message      string      `json:"message"`
	Resource     interface{} `json:"resource,omitempty"`
}

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
