// 文件用途：通用实体关系查询条件与校验（ROADMAP P1.1）。
// 核心逻辑：集中声明关系查询的过滤维度，避免各调用方自行拼装 WHERE 造成越权或语义漂移。
// 关键注意事项：
//  1. TenantID 是作用域租户，必填。查询永不允许省略租户条件。
//  2. 端点匹配有两种写法：显式给 From*/To*，或给 Entity* + Direction 做"不关心是哪一端"的匹配。
//     两者同时给时以 From*/To* 为准，不做隐式合并，避免条件被静默放大。
//  3. 实体类型仍走白名单；方向取值非法一律报错，不静默兜底。
package model

import (
	"errors"
	"strings"
)

// RelationDirection 端点匹配方向。
type RelationDirection string

const (
	// RelationDirectionOut 只匹配以该实体为起点（from）的关系。
	RelationDirectionOut RelationDirection = "out"
	// RelationDirectionIn 只匹配以该实体为终点（to）的关系。
	RelationDirectionIn RelationDirection = "in"
	// RelationDirectionAny 匹配任一端点为该实体的关系。
	RelationDirectionAny RelationDirection = "any"
)

var (
	ErrEntityRelationMissingTenant    = errors.New("entity relation query requires tenant")
	ErrEntityRelationInvalidDirection = errors.New("entity relation direction is invalid")
	ErrEntityRelationIncompleteEntity = errors.New("entity relation query entity requires both type and id")
	ErrEntityRelationEmptyQuery       = errors.New("entity relation query has no endpoint filter")
)

// EntityRelationQuery 关系查询条件。
type EntityRelationQuery struct {
	// TenantID 作用域租户，必填。
	TenantID string
	// FromType/FromID 起点过滤，可空。
	FromType string
	FromID   string
	// ToType/ToID 终点过滤，可空。
	ToType string
	ToID   string
	// RelationType 关系类型过滤，可空。
	RelationType string
	// EntityType/EntityID + Direction：按"任一端/起点/终点"匹配某一实体，可空。
	EntityType string
	EntityID   string
	Direction  RelationDirection
	// Limit/Offset 分页。Limit<=0 表示不限制（调用方需自行约束上限）。
	Limit  int
	Offset int
}

// Validate 校验查询条件。
// 返回 ErrEntityRelationEmptyQuery 表示没有给任何端点过滤条件——
// 这种情况若不拒绝，就等价于"列出全租户所有关系"，属于条件被静默放大。
func (q EntityRelationQuery) Validate() error {
	if strings.TrimSpace(q.TenantID) == "" {
		return ErrEntityRelationMissingTenant
	}
	if strings.TrimSpace(q.FromType) != "" && !IsAllowedEntityType(q.FromType) {
		return ErrEntityRelationUnknownType
	}
	if strings.TrimSpace(q.ToType) != "" && !IsAllowedEntityType(q.ToType) {
		return ErrEntityRelationUnknownType
	}

	usesEntity := strings.TrimSpace(q.EntityType) != "" || strings.TrimSpace(q.EntityID) != ""
	if usesEntity {
		if strings.TrimSpace(q.EntityType) == "" || strings.TrimSpace(q.EntityID) == "" {
			return ErrEntityRelationIncompleteEntity
		}
		if !IsAllowedEntityType(q.EntityType) {
			return ErrEntityRelationUnknownType
		}
		d := q.Direction
		if d == "" {
			d = RelationDirectionAny
		}
		if d != RelationDirectionOut && d != RelationDirectionIn && d != RelationDirectionAny {
			return ErrEntityRelationInvalidDirection
		}
	}

	if strings.TrimSpace(q.FromType) == "" && strings.TrimSpace(q.ToType) == "" && !usesEntity {
		return ErrEntityRelationEmptyQuery
	}
	return nil
}

// NormalizedDirection 返回归一化后的方向（空值按 any 处理）。
// 仅在 Validate 通过后使用才有意义。
func (q EntityRelationQuery) NormalizedDirection() RelationDirection {
	if q.Direction == "" {
		return RelationDirectionAny
	}
	return q.Direction
}
