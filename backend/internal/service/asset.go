// 文件用途：资产服务层（ROADMAP C2）。
// 核心逻辑：由 claims.TenantID 经 hierarchy.ScopeDown（self∪子孙，自上而下）展开为
//
//	链接推导）展开为可读租户作用域，所有 DAL 查询携带该作用域；写操作固定绑定 claims 租户，
//	并对 parent_id 做存在性 + 成环拒绝（复用 hierarchy 语义包）。
package service

import (
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/hierarchy"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service/kit"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// Asset 资产服务聚合入口。
type Asset struct{}

// assetNotFound 资产加载/删除失败映射：未命中为裸 CodeNotFound，其余一律裸 CodeDBError（不带 data）。
var assetNotFound = kit.NotFound{
	Match:   kit.IsRecordNotFound,
	OnOther: func(error) error { return errcode.New(errcode.CodeDBError) },
}

// assetWriteScope 写操作门禁：平台级（无租户）拒绝，返回 self 与可读作用域。
func assetWriteScope(claims *utils.UserClaims) (string, []string, error) {
	self, scopes := assetScope(claims)
	if self == "" {
		return "", nil, errcode.NewWithMessage(errcode.CodeParamError, "平台级（无租户）暂不支持资产")
	}
	return self, scopes, nil
}

// AssetReq 创建/更新资产入参（由 api 层绑定后透传）。
type AssetReq struct {
	ID             string  `json:"id"`
	ParentID       string  `json:"parent_id"`
	Name           string  `json:"name" binding:"required"`
	AssetType      string  `json:"asset_type"`
	Meta           string  `json:"meta"`
	ConflictPolicy *string `json:"conflict_policy" form:"conflict_policy"` // TB-15 实体名冲突策略
}

// assetScope 依据 claims 解析可读租户作用域（self∪子孙，自上而下；总部/父级可下钻）。
// 说明：SYS_ADMIN（TenantID 为空）作为平台侧暂未纳入层级资产，返回自身空作用域由调用方拒绝。
func assetScope(claims *utils.UserClaims) (self string, scopes []string) {
	if claims == nil || claims.TenantID == "" {
		return "", nil
	}
	self = claims.TenantID
	scopes = expandTenantIDScope(self)
	return self, scopes
}

// validateTree 确保 parent 属于同一租户作用域且不构成环。
// 修复（审计 2026-09-05，D1）：原实现仅当"现存映射已成环"时才报错，从不检查
// targetID 是否会因本次重挂进入 parentID 的祖先链——导致"把节点挂到自己的
// 子孙下"被接受（200），产生互为父子的死环，且环上节点因互为子节点而无法再经
// API 删除。正确语义：parentID 的祖先链中若已含 targetID（更新场景），本次写入
// 必然成环，拒绝。局限：pm 仅由本租户节点构成（nodesToHierarchy 过滤）；跨租户
// 父链场景由父节点存在性校验与创建期同租户绑定兜底，放开时需一并扩展节点来源。
func validateAssetTree(self, parentID, targetID string, scopes []string) error {
	if parentID == "" {
		return nil
	}
	if targetID != "" && parentID == targetID {
		return errcode.NewWithMessage(errcode.CodeParamError, "资产不能作为自身的父节点")
	}
	parent, err := dal.GetAsset(parentID, scopes)
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "父节点不存在或不在可读作用域内")
	}
	_ = parent
	nodes, err := dal.ListAssetNodes(scopes)
	if err != nil {
		return errcode.New(errcode.CodeDBError)
	}
	pm, err := hierarchy.BuildParentMap(nodesToHierarchy(nodes, self))
	if err != nil {
		return errcode.New(errcode.CodeDBError)
	}
	ancestors, ancErr := hierarchy.Ancestors(parentID, pm)
	if ancErr != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "资产层级存在环，拒绝写入")
	}
	if targetID != "" {
		for _, ancestor := range ancestors {
			if ancestor == targetID {
				return errcode.NewWithMessage(errcode.CodeParamError, "资产层级存在环，拒绝写入")
			}
		}
	}
	return nil
}

func nodesToHierarchy(nodes []*model.Asset, tenantID string) []hierarchy.Node {
	out := make([]hierarchy.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.TenantID != tenantID {
			continue
		}
		out = append(out, hierarchy.Node{ID: n.ID, Parent: n.ParentID})
	}
	return out
}

// Create 新建资产（name 必填；asset_type 默认 device；meta 需为合法 JSON）。
func (*Asset) Create(claims *utils.UserClaims, req *AssetReq) (*model.Asset, error) {
	self, scopes := assetScope(claims)
	if self == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "平台级（无租户）暂不支持资产；请切换至租户")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "资产名称不能为空")
	}
	if req.Meta != "" && !IsJSON(req.Meta) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "meta 必须为合法 JSON")
	}
	if err := validateAssetTree(self, strings.TrimSpace(req.ParentID), "", scopes); err != nil {
		return nil, err
	}

	// TB-15: 实体名冲突策略消解（FAIL / RENAME / IGNORE / UPDATE）
	policy := model.NormalizeConflictPolicy(req.ConflictPolicy)
	if policy != model.ConflictPolicyAllow {
		existing, err := dal.GetAssetByNameAndTenant(self, name)
		if err == nil && existing != nil {
			switch policy {
			case model.ConflictPolicyFail:
				return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("asset with name '%s' already exists", name))
			case model.ConflictPolicyIgnore:
				return existing, nil
			case model.ConflictPolicyUpdate:
				existing.ParentID = strings.TrimSpace(req.ParentID)
				if strings.TrimSpace(req.AssetType) != "" {
					existing.AssetType = strings.TrimSpace(req.AssetType)
				}
				metaStr := strings.TrimSpace(req.Meta)
				if metaStr != "" {
					existing.Meta = &metaStr
				}
				if _, err := dal.UpdateAsset(existing); err != nil {
					return nil, errcode.New(errcode.CodeDBError)
				}
				return existing, nil
			case model.ConflictPolicyRename:
				names, err := dal.GetAssetNamesMatchingBase(self, name)
				if err != nil {
					return nil, errcode.New(errcode.CodeDBError)
				}
				nameMap := make(map[string]bool, len(names))
				for _, n := range names {
					nameMap[n] = true
				}
				renamed := model.GenerateRenamedName(name, model.NameMaxLengthDefault, func(candidate string) bool {
					return nameMap[candidate]
				})
				name = renamed
			}
		}
	}
	assetType := strings.TrimSpace(req.AssetType)
	if assetType == "" {
		assetType = "device"
	}
	meta := strings.TrimSpace(req.Meta)
	asset := &model.Asset{
		ID:        kit.NewID(),
		TenantID:  self,
		ParentID:  strings.TrimSpace(req.ParentID),
		Name:      name,
		AssetType: assetType,
	}
	if meta != "" {
		asset.Meta = &meta
	}
	if err := dal.CreateAsset(asset); err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	return asset, nil
}

// Update 更新资产；只允许更新归属自身租户的记录。
func (*Asset) Update(claims *utils.UserClaims, req *AssetReq) (*model.Asset, error) {
	self, scopes, err := assetWriteScope(claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.ID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "缺少资产 ID")
	}
	exist, err := dal.GetAsset(req.ID, scopes)
	if err != nil {
		return nil, assetNotFound.Map(err)
	}
	if exist.TenantID != self {
		return nil, assetNotFound.Err()
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "资产名称不能为空")
	}
	if req.Meta != "" && !IsJSON(req.Meta) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "meta 必须为合法 JSON")
	}
	if err := validateAssetTree(self, strings.TrimSpace(req.ParentID), req.ID, scopes); err != nil {
		return nil, err
	}
	assetType := strings.TrimSpace(req.AssetType)
	if assetType == "" {
		assetType = exist.AssetType
	}
	meta := strings.TrimSpace(req.Meta)
	if meta == "" && exist.Meta != nil {
		meta = *exist.Meta
	}
	upd := &model.Asset{ID: req.ID, TenantID: self, ParentID: strings.TrimSpace(req.ParentID), Name: name, AssetType: assetType}
	if meta != "" {
		upd.Meta = &meta
	}
	ok, err := dal.UpdateAsset(upd)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	if !ok {
		return nil, assetNotFound.Err()
	}
	got, err := dal.GetAsset(req.ID, scopes)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	return got, nil
}

// Delete 删除资产；存在子节点时拒绝（需先删除子树）。
func (*Asset) Delete(claims *utils.UserClaims, id string) error {
	self, scopes, err := assetWriteScope(claims)
	if err != nil {
		return err
	}
	if _, err := dal.GetAsset(id, scopes); err != nil {
		return assetNotFound.Map(err)
	}
	children, err := dal.CountAssetChildren(id, scopes)
	if err != nil {
		return errcode.New(errcode.CodeDBError)
	}
	if children > 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "存在子节点，请先删除或迁移子树")
	}
	return assetNotFound.Map(dal.DeleteAsset(id, self))
}

// List 分页查询根/指定父节点下资产。
// TB-46 组共享可见性：普通租户用户隐藏"组绑定且非组内成员"的资产（fail-closed）；
// 管理员不受限；映射错误上抛，不得降级放行。
func (*Asset) List(claims *utils.UserClaims, parentID, keyword string, page, pageSize int) ([]*model.Asset, int64, error) {
	_, scopes := assetScope(claims)
	if len(scopes) == 0 {
		return []*model.Asset{}, 0, nil
	}
	hiddenAssetIDs, err := groupHiddenResourceIDs(scopes, claims, model.GroupElementKindAsset)
	if err != nil {
		return nil, 0, errcode.New(errcode.CodeDBError)
	}
	list, total, err := dal.ListAssetsByPageWithGroupScope(scopes, parentID, keyword, page, pageSize, hiddenAssetIDs)
	if err != nil {
		return nil, 0, errcode.New(errcode.CodeDBError)
	}
	return list, total, nil
}

// Get 读取单个资产。
func (*Asset) Get(claims *utils.UserClaims, id string) (*model.Asset, error) {
	_, scopes := assetScope(claims)
	if len(scopes) == 0 {
		return nil, assetNotFound.Err()
	}
	a, err := dal.GetAsset(id, scopes)
	if err != nil {
		return nil, assetNotFound.Map(err)
	}
	return a, nil
}

// AssetTreeNode 资产树节点（含子节点）。
type AssetTreeNode struct {
	model.Asset
	Children []*AssetTreeNode `json:"children"`
}

// Tree 返回租户作用域内完整资产树（根节点平铺）。
// TB-46 组共享可见性：普通租户用户的树中剪掉"组绑定且非组内成员"的节点及其子树
// （fail-closed：被隐藏节点下挂的内容一并不可达）；管理员不受限。
func (*Asset) Tree(claims *utils.UserClaims) ([]*AssetTreeNode, error) {
	_, scopes := assetScope(claims)
	if len(scopes) == 0 {
		return []*AssetTreeNode{}, nil
	}
	hiddenSet, err := groupHiddenResourceIDSet(scopes, claims, model.GroupElementKindAsset)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	nodes, err := dal.ListAssetNodes(scopes)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	byID := make(map[string]*AssetTreeNode, len(nodes))
	for _, n := range nodes {
		node := &AssetTreeNode{Asset: *n, Children: []*AssetTreeNode{}}
		byID[n.ID] = node
	}
	var roots []*AssetTreeNode
	for _, n := range nodes {
		if _, hidden := hiddenSet[n.ID]; hidden {
			// 被隐藏的节点不进树：作为根被剪掉（子树因父缺失同样不出现），
			// 作为他人子节点则由父节点挂载阶段的 hidden 检查剪掉。
			continue
		}
		node := byID[n.ID]
		if n.ParentID != "" {
			if p, ok := byID[n.ParentID]; ok {
				p.Children = append(p.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots, nil
}
