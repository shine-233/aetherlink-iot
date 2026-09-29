// 文件用途：租户管理与自助开通服务层（P3 商业化闭环与 ThingsBoard 租户层级对齐）。
// 核心逻辑：
//   - 租户管理：SYS_ADMIN 平台级创建/列表；TENANT_ADMIN 仅可管理自身及子租户（ScopeDown）；
//   - 配额执法：CreateTenant 与 SelfServiceProvisionTenant 严格受 license doc.MaxTenants 配额门控；
//   - 自助开通：开箱入驻，原子创建租户、初始管理员（TENANT_ADMIN）和默认看板；
//   - 拓扑防线：利用 internal/hierarchy 严格校验父子租户关系，杜绝自环与断链。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

type TenantService struct{}

// CreateTenant 创建新租户（受商业许可证 max_tenants 配额硬性约束）。
func (s *TenantService) CreateTenant(_ context.Context, req *model.CreateTenantReq, claims *utils.UserClaims) (*model.Tenant, error) {
	if claims == nil {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if claims.Authority != "SYS_ADMIN" && claims.Authority != "TENANT_ADMIN" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}

	// 1. 商业许可证配额前置检查
	if err := enforceTenantQuota(); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant name is required")
	}
	if len(name) > 120 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant name exceeds 120 characters")
	}

	parentID := strings.TrimSpace(req.ParentTenantID)
	// TENANT_ADMIN 仅允许在自身租户下挂载子租户
	if claims.Authority == "TENANT_ADMIN" {
		if parentID == "" {
			parentID = claims.TenantID
		} else if parentID != claims.TenantID {
			// 如果指定父租户，必须是该租户或其下级租户
			if !tenantScopeContains(claims.TenantID, parentID) {
				return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "cannot attach sub-tenant outside your tenant scope")
			}
		}
	}

	// 校验 parent_tenant_id 合法性
	if parentID != "" {
		parentTenant, err := dal.GetTenantByID(parentID)
		if err != nil {
			return nil, dbError(err)
		}
		if parentTenant == nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("parent tenant %q does not exist", parentID))
		}
	}

	tenant := &model.Tenant{
		ID:             uuid.New(),
		Name:           name,
		ParentTenantID: parentID,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := dal.CreateTenant(tenant); err != nil {
		return nil, dbError(err)
	}

	return tenant, nil
}

// ListTenants 查询租户列表（按角色作用域隔离并统计设备/用户数）。
func (s *TenantService) ListTenants(_ context.Context, page, pageSize int, search string, claims *utils.UserClaims) ([]*model.TenantDetailVO, int64, error) {
	if claims == nil {
		return nil, 0, errcode.New(errcode.CodeNoPermission)
	}

	var allowedIDs []string
	if claims.Authority == "SYS_ADMIN" {
		allowedIDs = nil // 平台管理员可见全部
	} else if claims.Authority == "TENANT_ADMIN" {
		// 租户管理员下钻可见：self ∪ 全部子孙租户
		allowedIDs = tenantVisibleScope(claims.TenantID)
	} else {
		return nil, 0, errcode.New(errcode.CodeNoPermission)
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	tenants, total, err := dal.ListTenants(offset, pageSize, search, allowedIDs)
	if err != nil {
		return nil, 0, dbError(err)
	}

	tenantIDs := make([]string, 0, len(tenants))
	for _, t := range tenants {
		tenantIDs = append(tenantIDs, t.ID)
	}
	// 一页最多 100 条：两次 GROUP BY 批量统计，替代逐租户 2 次 COUNT 的 N+1。
	deviceCounts, userCounts := countTenantEntitiesByTenant(tenantIDs)

	res := make([]*model.TenantDetailVO, 0, len(tenants))
	for _, t := range tenants {
		res = append(res, &model.TenantDetailVO{
			ID:             t.ID,
			Name:           t.Name,
			ParentTenantID: t.ParentTenantID,
			DeviceCount:    deviceCounts[t.ID],
			UserCount:      userCounts[t.ID],
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
		})
	}

	return res, total, nil
}

// GetTenant 获取指定租户详情。
func (s *TenantService) GetTenant(_ context.Context, id string, claims *utils.UserClaims) (*model.TenantDetailVO, error) {
	if claims == nil {
		return nil, errcode.New(errcode.CodeNoPermission)
	}

	t, err := dal.GetTenantByID(id)
	if err != nil {
		return nil, dbError(err)
	}
	if t == nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}

	// 权限守卫：非 SYS_ADMIN 仅允许查看自身或下级租户
	if claims.Authority != "SYS_ADMIN" && !tenantScopeContains(claims.TenantID, t.ID) {
		return nil, errcode.New(errcode.CodeNotFound)
	}

	devCount, usrCount, _ := dal.GetTenantEntityCounts(t.ID)
	return &model.TenantDetailVO{
		ID:             t.ID,
		Name:           t.Name,
		ParentTenantID: t.ParentTenantID,
		DeviceCount:    devCount,
		UserCount:      usrCount,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}, nil
}

// UpdateTenant 更新租户基本信息（名称/上级租户，严格防环）。
func (s *TenantService) UpdateTenant(_ context.Context, id string, req *model.UpdateTenantReq, claims *utils.UserClaims) (*model.Tenant, error) {
	if claims == nil || claims.Authority != "SYS_ADMIN" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}

	tenant, err := dal.GetTenantByID(id)
	if err != nil {
		return nil, dbError(err)
	}
	if tenant == nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		name := strings.TrimSpace(req.Name)
		// 纯空白名称此前会被 Trim 成 "" 后照样落库，导致租户名被清空。
		if name == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant name must not be blank")
		}
		if len(name) > 120 {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant name exceeds 120 characters")
		}
		updates["name"] = name
		tenant.Name = name
	}

	if req.ParentTenantID != nil {
		newParent := strings.TrimSpace(*req.ParentTenantID)
		if newParent == tenant.ID {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant cannot be its own parent")
		}
		if newParent != "" {
			// 校验父租户存在
			pt, pErr := dal.GetTenantByID(newParent)
			if pErr != nil || pt == nil {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("parent tenant %q not found", newParent))
			}
			// 校验防成环：待更新租户不可成为其当前子孙节点的子节点
			if tenantParentWouldCycle(tenant.ID, newParent, tenantParentLinks()) {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "cycle detected: target parent is a descendant of this tenant")
			}
		}
		updates["parent_tenant_id"] = newParent
		tenant.ParentTenantID = newParent
	}

	if len(updates) > 0 {
		if err := dal.UpdateTenant(id, updates); err != nil {
			return nil, dbError(err)
		}
	}

	return tenant, nil
}

// SelfServiceProvisionTenant 客户自助开箱入驻开通（受商业许可证配额硬性约束）。
// 原子创建：租户记录、初始租户管理员（TENANT_ADMIN）和初始默认看板。
func (s *TenantService) SelfServiceProvisionTenant(_ context.Context, req *model.SelfProvisionTenantReq) (*model.SelfProvisionTenantRsp, error) {
	// 1. 商业许可证配额前置检查
	if err := enforceTenantQuota(); err != nil {
		return nil, err
	}

	tenantName := strings.TrimSpace(req.TenantName)
	if tenantName == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant_name is required")
	}
	if len(tenantName) > 120 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant_name exceeds 120 characters")
	}

	email := strings.ToLower(strings.TrimSpace(req.AdminEmail))
	if email == "" || !strings.Contains(email, "@") {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "valid admin_email is required")
	}

	phone := strings.TrimSpace(req.AdminPhone)
	if phone == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "admin_phone is required")
	}

	password := req.AdminPassword
	if len(password) < 6 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "admin_password must be at least 6 characters")
	}

	// 2. 检查邮箱与手机号唯一性
	existingUser, err := dal.GetUsersByEmail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, dbError(err)
	}
	if existingUser != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "admin email already exists")
	}

	phoneExists, pErr := dal.CheckPhoneNumberExists(phone)
	if pErr != nil {
		return nil, dbError(pErr)
	}
	if phoneExists {
		return nil, errcode.New(errcode.CodePhoneDuplicated)
	}

	// 3. 密码加盐哈希
	hashedPassword, hErr := utils.BcryptHash(password)
	if hErr != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{"error": hErr.Error()})
	}

	tenantID := uuid.New()
	adminID := uuid.New()
	adminName := strings.TrimSpace(req.AdminName)
	if adminName == "" {
		adminName = tenantName + " 管理员"
	}

	now := time.Now().UTC()
	tenant := &model.Tenant{
		ID:             tenantID,
		Name:           tenantName,
		ParentTenantID: "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	user := &model.User{
		ID:                  adminID,
		Name:                &adminName,
		PhoneNumber:         phone,
		Email:               email,
		Status:              StringPtr("N"),
		Authority:           StringPtr("TENANT_ADMIN"),
		Password:            hashedPassword,
		TenantID:            &tenantID,
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}

	// 4. 原子事务落库
	if global.DB == nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "db is not initialized"})
	}

	txErr := global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(model.TableNameTenant).Create(tenant).Error; err != nil {
			return err
		}
		if err := tx.Table(model.TableNameUser).Create(user).Error; err != nil {
			return err
		}
		// 创建默认初始看板
		board := dal.NewDefaultBoard(&tenantID)
		if err := tx.Table(model.TableNameBoard).Create(board).Error; err != nil {
			return err
		}
		// 绑定 Casbin 角色 (g, adminID, TENANT_ADMIN)
		ptype := "g"
		adminIDStr := adminID
		roleStr := "TENANT_ADMIN"
		casbinRule := &model.CasbinRule{
			Ptype: &ptype,
			V0:    &adminIDStr,
			V1:    &roleStr,
		}
		if err := tx.Table(model.TableNameCasbinRule).Create(casbinRule).Error; err != nil {
			return err
		}
		return nil
	})
	if txErr != nil {
		return nil, dbError(txErr)
	}

	if global.CasbinEnforcer != nil {
		_ = global.CasbinEnforcer.LoadPolicy()
	}

	return &model.SelfProvisionTenantRsp{
		TenantID:   tenantID,
		TenantName: tenantName,
		AdminID:    adminID,
		AdminEmail: email,
		AdminName:  adminName,
	}, nil
}

// tenantVisibleScope 返回 self ∪ 子孙租户；self 为空时返回无法匹配任何租户的哨兵作用域，
// 避免空切片被 DAL 解释为"不过滤"。
func tenantVisibleScope(self string) []string {
	if scope := expandTenantIDScope(self); len(scope) > 0 {
		return scope
	}
	return []string{self}
}

// tenantScopeContains 判断 target 是否处于 self 的自上而下可见作用域（self ∪ 子孙）。
func tenantScopeContains(self, target string) bool {
	for _, id := range tenantVisibleScope(self) {
		if id == target {
			return true
		}
	}
	return false
}

// tenantParentWouldCycle 判断把 tenantID 挂到 newParent 下是否成环：
// 从 newParent 沿父指针向上走，途经 tenantID（或链上已有环）即判定成环。
// 旧实现依赖 hierarchy.Descendants，其遍历节点数上限 64，子孙较多时会漏判；
// 父链上行只受树深约束，且对脏数据（已有环）fail-closed。
func tenantParentWouldCycle(tenantID, newParent string, parent map[string]string) bool {
	seen := map[string]bool{}
	for cur := newParent; cur != ""; cur = parent[cur] {
		if cur == tenantID || seen[cur] {
			return true
		}
		seen[cur] = true
	}
	return false
}

// countTenantEntitiesByTenant 以两次 GROUP BY 批量统计各租户设备数与用户数。
// 统计失败按 0 处理，与逐条统计时忽略错误的既有语义保持一致。
func countTenantEntitiesByTenant(tenantIDs []string) (map[string]int64, map[string]int64) {
	deviceCounts := map[string]int64{}
	userCounts := map[string]int64{}
	if len(tenantIDs) == 0 || global.DB == nil {
		return deviceCounts, userCounts
	}
	type tenantCountRow struct {
		TenantID string
		Total    int64
	}
	load := func(table string, into map[string]int64) {
		var rows []tenantCountRow
		if err := global.DB.Table(table).
			Select("tenant_id, COUNT(*) AS total").
			Where("tenant_id IN ?", tenantIDs).
			Group("tenant_id").
			Scan(&rows).Error; err != nil {
			return
		}
		for _, row := range rows {
			into[row.TenantID] = row.Total
		}
	}
	load(model.TableNameDevice, deviceCounts)
	load(model.TableNameUser, userCounts)
	return deviceCounts, userCounts
}
