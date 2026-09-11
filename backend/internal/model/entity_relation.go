// 文件用途：通用实体关系模型与合法性校验（ROADMAP P1.1）。
// 核心逻辑：定义设备/资产/客户/网关之间的有向、带类型与元数据的关系，并集中校验写入合法性。
// 关键注意事项：
//  1. 关系是有向的：relation_type 不隐含对称，反向必须显式写入，禁止由查询层"脑补"。
//  2. 自环一律拒绝，避免生成无意义的关系环。
//  3. 实体类型必须是受控白名单，防止把任意字符串写进关系图造成语义漂移。
package model

import (
	"errors"
	"strings"
	"time"
)

const TableNameEntityRelation = "entity_relations"

// 受控的实体类型白名单。
const (
	EntityTypeDevice   = "device"
	EntityTypeAsset    = "asset"
	EntityTypeCustomer = "customer"
	EntityTypeGateway  = "gateway"
)

var entityRelationAllowedTypes = map[string]bool{
	EntityTypeDevice:   true,
	EntityTypeAsset:    true,
	EntityTypeCustomer: true,
	EntityTypeGateway:  true,
}

var (
	ErrEntityRelationSelfLoop         = errors.New("entity relation cannot point to itself")
	ErrEntityRelationUnknownType      = errors.New("entity relation type is not allowed")
	ErrEntityRelationMissingField     = errors.New("entity relation requires tenant, endpoints and relation type")
	ErrEntityRelationTypeTooLong      = errors.New("entity relation type is too long")
	ErrEntityRelationMetadataTooLarge = errors.New("entity relation metadata is too large")
)

const (
	entityRelationMaxTypeLength    = 64
	entityRelationMaxMetadataBytes = 8192
)

// EntityRelation 通用实体关系。
type EntityRelation struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID     string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	FromType     string    `gorm:"column:from_type;not null" json:"from_type"`
	FromID       string    `gorm:"column:from_id;not null" json:"from_id"`
	RelationType string    `gorm:"column:relation_type;not null" json:"relation_type"`
	ToType       string    `gorm:"column:to_type;not null" json:"to_type"`
	ToID         string    `gorm:"column:to_id;not null" json:"to_id"`
	Metadata     *string   `gorm:"column:metadata;type:jsonb;not null;default:{}" json:"metadata"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName EntityRelation's table name
func (*EntityRelation) TableName() string { return TableNameEntityRelation }

// IsAllowedEntityType 判断实体类型是否在受控白名单内。
func IsAllowedEntityType(entityType string) bool {
	return entityRelationAllowedTypes[strings.TrimSpace(entityType)]
}

// AllowedEntityTypes 返回受控实体类型列表（供接口文档与校验提示使用）。
func AllowedEntityTypes() []string {
	return []string{EntityTypeDevice, EntityTypeAsset, EntityTypeCustomer, EntityTypeGateway}
}

// ValidateEntityRelation 校验关系写入合法性。
// metadataBytes 传序列化后的字节数；为 0 表示无元数据。
func ValidateEntityRelation(r *EntityRelation, metadataBytes int) error {
	if r == nil {
		return ErrEntityRelationMissingField
	}
	if strings.TrimSpace(r.TenantID) == "" ||
		strings.TrimSpace(r.FromID) == "" ||
		strings.TrimSpace(r.ToID) == "" ||
		strings.TrimSpace(r.RelationType) == "" {
		return ErrEntityRelationMissingField
	}
	if !IsAllowedEntityType(r.FromType) || !IsAllowedEntityType(r.ToType) {
		return ErrEntityRelationUnknownType
	}
	// 自环：同类型且同 ID。不同类型但同 ID 不算自环（ID 空间本身按类型隔离）。
	if r.FromType == r.ToType && r.FromID == r.ToID {
		return ErrEntityRelationSelfLoop
	}
	if len(strings.TrimSpace(r.RelationType)) > entityRelationMaxTypeLength {
		return ErrEntityRelationTypeTooLong
	}
	if metadataBytes > entityRelationMaxMetadataBytes {
		return ErrEntityRelationMetadataTooLarge
	}
	return nil
}

// IsReverseOf 判断两条关系是否互为反向（仅比较端点与类型，不比较元数据与租户）。
// 用于提示调用方：反向关系必须显式写入，不会自动生成。
func (r *EntityRelation) IsReverseOf(other *EntityRelation) bool {
	if r == nil || other == nil {
		return false
	}
	return r.FromType == other.ToType && r.FromID == other.ToID &&
		r.ToType == other.FromType && r.ToID == other.FromID &&
		r.RelationType == other.RelationType
}
