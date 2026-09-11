// File purpose: HTTP layer for generic entity relations (ROADMAP P1.1).
// Core logic: bind/validate requests, resolve the caller's tenant, delegate to the service.
// Key notes:
//   - Cross-tenant is expressed as 404 (CodeNotFound) by the service, never 403, because 403
//     leaks that the ID exists elsewhere. Handlers must not convert that back into 403.
//   - The query's TenantID always comes from the caller's claims, never from the request body,
//     so a client cannot ask for another tenant's relations.

package api

import (
	"strconv"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type EntityRelationApi struct{}

type CreateEntityRelationReq struct {
	FromType     string  `json:"from_type" validate:"required,max=32"`
	FromID       string  `json:"from_id" validate:"required,max=36"`
	RelationType string  `json:"relation_type" validate:"required,max=64"`
	ToType       string  `json:"to_type" validate:"required,max=32"`
	ToID         string  `json:"to_id" validate:"required,max=36"`
	Metadata     *string `json:"metadata" validate:"omitempty"`
}

type DeleteEntityRelationsReq struct {
	EntityType string `json:"entity_type" validate:"required,max=32"`
	EntityID   string `json:"entity_id" validate:"required,max=36"`
	// Policy 为 protect（默认，有关系则拒绝）或 cascade（显式级联）。
	Policy string `json:"policy" validate:"omitempty,oneof=protect cascade"`
}

// CreateEntityRelation 创建一条关系。幂等：同一组端点与类型重复创建返回既有记录。
// @Router   /api/v1/entity-relations [post]
func (*EntityRelationApi) CreateEntityRelation(c *gin.Context) {
	var req CreateEntityRelationReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	relation := &model.EntityRelation{
		TenantID:     claims.TenantID,
		FromType:     req.FromType,
		FromID:       req.FromID,
		RelationType: req.RelationType,
		ToType:       req.ToType,
		ToID:         req.ToID,
		Metadata:     req.Metadata,
	}
	if err := service.CreateRelation(c, relation); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", relation)
}

// ListEntityRelations 查询当前租户可视范围内的关系。
// @Router   /api/v1/entity-relations [get]
func (*EntityRelationApi) ListEntityRelations(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	query := model.EntityRelationQuery{
		// 目标租户恒取调用方所属租户：不接受客户端指定别的租户。
		TenantID:     claims.TenantID,
		FromType:     c.Query("from_type"),
		FromID:       c.Query("from_id"),
		ToType:       c.Query("to_type"),
		ToID:         c.Query("to_id"),
		RelationType: c.Query("relation_type"),
		EntityType:   c.Query("entity_type"),
		EntityID:     c.Query("entity_id"),
		Direction:    model.RelationDirection(c.Query("direction")),
	}
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			c.Error(errcode.WithData(errcode.CodeParamError, "limit must be a non-negative integer"))
			return
		}
		query.Limit = parsed
	}
	if raw := c.Query("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			c.Error(errcode.WithData(errcode.CodeParamError, "offset must be a non-negative integer"))
			return
		}
		query.Offset = parsed
	}
	list, total, err := service.ListRelations(c, claims.TenantID, query)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"list": list, "total": total})
}

// DeleteEntityRelation 删除单条关系。越权或不存在均返回 404。
// @Router   /api/v1/entity-relations/{id} [delete]
func (*EntityRelationApi) DeleteEntityRelation(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	affected, err := service.DeleteRelation(c, c.Param("id"), claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"deleted": affected})
}

// DeleteEntityRelationsForEntity 删除某实体挂载的全部关系。
// 默认 protect：存在关系则拒绝，必须显式声明 cascade 才删除。
// @Router   /api/v1/entity-relations/by-entity [delete]
func (*EntityRelationApi) DeleteEntityRelationsForEntity(c *gin.Context) {
	var req DeleteEntityRelationsReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	policy := service.EntityDeletionPolicy(req.Policy)
	if policy == "" {
		// 未声明即保护：级联删除必须显式，绝不默认替调用方做决定。
		policy = service.EntityDeletionProtect
	}
	affected, err := service.DeleteRelationsForEntity(c, claims.TenantID, req.EntityType, req.EntityID, policy)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"deleted": affected})
}
