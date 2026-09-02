// 文件用途：租户客户层级（ROADMAP C2）业务服务。
// 核心逻辑：租户树按"可管辖子树"裁剪返回；创建/更新/删除/详情统一走
//           assertTenantManageability 守卫，确保目标租户落在当前管理员管辖区内。
// 关键注意事项：
//   1. SYS_ADMIN 可管全部租户；租户管理员/用户仅可管自身及后代（scope 来自 tenants 表）。
//   2. 新租户 ID 沿用现有 8 位随机串风格（与邮箱自助注册 GenerateRandomString(8) 一致）。
//   3. 删除守卫：仅允许删除无直接子租户的叶子节点，防止误删整棵子树。
//   4. 更新不支持修改 parent_tenant_id（层级移动涉及数据归属迁移，留待后续迭代），
//      避免跨子树搬移造成数据可见性突变。
package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	common "aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

type TenantService struct{}

// GetTenantTree 返回以当前管理员可管辖子树为根的租户树。
// SYS_ADMIN 返回完整森林；租户管理员/用户返回自身为根的子树。
func (*TenantService) GetTenantTree(claims *utils.UserClaims) ([]*model.TenantTreeNode, error) {
	all, err := dal.ListAllTenants()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "list_tenants",
			"error":     err.Error(),
		})
	}

	if claims.Authority != "SYS_ADMIN" {
		if err := requireNonEmptyTenantID(claims); err != nil {
			return nil, err
		}
		scope, scopeErr := dal.TenantScope(claims.TenantID)
		if scopeErr != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"operation": "tenant_scope",
				"error":     scopeErr.Error(),
			})
		}
		inScope := make(map[string]struct{}, len(scope))
		for _, id := range scope {
			inScope[id] = struct{}{}
		}
		filtered := make([]*model.Tenant, 0, len(scope))
		for _, t := range all {
			if _, ok := inScope[t.ID]; ok {
				filtered = append(filtered, t)
			}
		}
		all = filtered
	}

	nodes := make(map[string]*model.TenantTreeNode, len(all))
	for _, t := range all {
		nodes[t.ID] = &model.TenantTreeNode{
			ID:     t.ID,
			Name:   t.Name,
			Code:   safeTenantCode(t.Code),
			Status: t.Status,
		}
	}

	var roots []*model.TenantTreeNode
	for _, t := range all {
		node := nodes[t.ID]
		if t.ParentTenantID == nil || *t.ParentTenantID == "" {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodes[*t.ParentTenantID]
		if !ok {
			// 父租户不在可见范围内：该节点作为虚拟根展示，避免孤儿节点丢失。
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
		parent.ChildCount++
	}
	return roots, nil
}

// CreateTenant 新建租户登记并挂入层级。
// SYS_ADMIN 可创建根租户（parent 为空）或任意子树租户；
// 租户管理员仅可把自己的后代（含自身）作为父租户，禁止跨子树挂载。
func (*TenantService) CreateTenant(req *model.CreateTenantReq, claims *utils.UserClaims) (*model.Tenant, error) {
	parentID := safeTenantCode(req.ParentTenantID)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.WithVars(100005, map[string]interface{}{"field": "name"})
	}
	if parentID != "" {
		if err := assertTenantManageability(claims, parentID); err != nil {
			return nil, err
		}
	} else if claims.Authority != "SYS_ADMIN" {
		// 非超管不能凭空创建根租户：必须挂在自己的子树下。
		return nil, errcode.New(errcode.CodeNoPermission)
	}

	tenantID, genErr := common.GenerateRandomString(8)
	if genErr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "generate_tenant_id",
			"error":     genErr.Error(),
		})
	}
	tenant := &model.Tenant{
		ID:             tenantID,
		Name:           name,
		Code:           req.Code,
		ParentTenantID: req.ParentTenantID,
		Status:         "N",
		Remark:         req.Remark,
	}
	created, err := dal.CreateTenant(tenant)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "create_tenant",
			"error":     err.Error(),
		})
	}
	return created, nil
}

// UpdateTenant 更新租户名称/编码/状态/备注；不支持移动父节点。
func (*TenantService) UpdateTenant(id string, req *model.UpdateTenantReq, claims *utils.UserClaims) error {
	if req == nil {
		return errcode.WithVars(100005, map[string]interface{}{"field": "body"})
	}
	if err := assertTenantManageability(claims, id); err != nil {
		return err
	}
	existing, err := dal.GetTenantByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errcode.New(errcode.CodeNotFound)
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "load_tenant",
			"error":     err.Error(),
		})
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return errcode.WithVars(100005, map[string]interface{}{"field": "name"})
		}
		existing.Name = name
	}
	if req.Code != nil {
		existing.Code = req.Code
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if status != "N" && status != "F" {
			return errcode.WithVars(100005, map[string]interface{}{"field": "status"})
		}
		existing.Status = status
	}
	if req.Remark != nil {
		existing.Remark = req.Remark
	}
	if err := dal.UpdateTenant(existing); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "update_tenant",
			"error":     err.Error(),
		})
	}
	return nil
}

// DeleteTenant 删除叶子租户登记；非叶子（存在直接子租户）拒绝删除。
// 注意：删除登记不迁移既有数据（业务表 tenant_id 保持原值），仅回收层级归属；
// 冻结用途可改用 UpdateTenant 置 status=F。
func (*TenantService) DeleteTenant(id string, claims *utils.UserClaims) error {
	if err := assertTenantManageability(claims, id); err != nil {
		return err
	}
	if _, err := dal.GetTenantByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errcode.New(errcode.CodeNotFound)
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "load_tenant",
			"error":     err.Error(),
		})
	}
	children, err := dal.CountChildren(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "count_children",
			"error":     err.Error(),
		})
	}
	if children > 0 {
		return errcode.NewWithMessage(errcode.CodeOpDenied, "tenant has child tenants, delete children first")
	}
	if err := dal.DeleteTenant(id); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "delete_tenant",
			"error":     err.Error(),
		})
	}
	return nil
}

// GetTenantDetail 返回租户详情（含父租户名称与直接子租户数）。
func (*TenantService) GetTenantDetail(id string, claims *utils.UserClaims) (*model.TenantDetailResp, error) {
	if err := assertTenantManageability(claims, id); err != nil {
		return nil, err
	}
	tenant, err := dal.GetTenantByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.New(errcode.CodeNotFound)
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "load_tenant",
			"error":     err.Error(),
		})
	}
	children, err := dal.CountChildren(id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "count_children",
			"error":     err.Error(),
		})
	}
	resp := &model.TenantDetailResp{
		ID:             tenant.ID,
		Name:           tenant.Name,
		Code:           safeTenantCode(tenant.Code),
		Status:         tenant.Status,
		Remark:         safeTenantCode(tenant.Remark),
		ParentTenantID: tenant.ParentTenantID,
		ChildCount:     children,
		CreatedAt:      tenant.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      tenant.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if tenant.ParentTenantID != nil && *tenant.ParentTenantID != "" {
		parent, parentErr := dal.GetTenantByID(*tenant.ParentTenantID)
		if parentErr == nil {
			resp.ParentName = parent.Name
		}
	}
	return resp, nil
}

// assertTenantManageability 校验当前登录人可管辖 targetTenantID：
// SYS_ADMIN 放行；其余角色要求 targetTenantID 落在 claims.TenantID 的"自身+后代"集合内。
func assertTenantManageability(claims *utils.UserClaims, targetTenantID string) error {
	if claims == nil {
		return errcode.New(errcode.CodeNoPermission)
	}
	if claims.Authority == "SYS_ADMIN" {
		return nil
	}
	if err := requireNonEmptyTenantID(claims); err != nil {
		return err
	}
	scope, err := dal.TenantScope(claims.TenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "tenant_scope",
			"error":     err.Error(),
		})
	}
	if tenantIDInScope(targetTenantID, scope) {
		return nil
	}
	return errcode.New(errcode.CodeNoPermission)
}

// tenantIDInScope 判断 targetTenantID 是否落在 scope 集合内（纯函数，便于单测）。
func tenantIDInScope(targetTenantID string, scope []string) bool {
	for _, id := range scope {
		if id == targetTenantID {
			return true
		}
	}
	return false
}

func requireNonEmptyTenantID(claims *utils.UserClaims) error {
	if claims.TenantID == "" {
		return errcode.New(errcode.CodeNoPermission)
	}
	return nil
}

func safeTenantCode(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}