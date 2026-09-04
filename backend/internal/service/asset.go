// 文件用途：资产服务层（ROADMAP C2）。
// 核心逻辑：由 claims.TenantID 经 hierarchy.ScopeDown（self∪子孙，自上而下）展开为
//
//	链接推导）展开为可读租户作用域，所有 DAL 查询携带该作用域；写操作固定绑定 claims 租户，
//	并对 parent_id 做存在性 + 成环拒绝（复用 hierarchy 语义包）。
package service

import (
	"errors"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/hierarchy"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/go-basic/uuid"

	"gorm.io/gorm"
)

// Asset 资产服务聚合入口。
type Asset struct{}

// AssetReq 创建/更新资产入参（由 api 层绑定后透传）。
type AssetReq struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id"`
	Name      string `json:"name" binding:"required"`
	AssetType string `json:"asset_type"`
	Meta      string `json:"meta"`
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
func validateAssetTree(self, parentID, targetID string, scopes []string) error {
	if parentID == "" {
		return nil
	}
	if parentID == targetID {
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
	if _, err := hierarchy.Ancestors(parentID, pm); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "资产层级存在环，拒绝写入")
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
	assetType := strings.TrimSpace(req.AssetType)
	if assetType == "" {
		assetType = "device"
	}
	meta := strings.TrimSpace(req.Meta)
	asset := &model.Asset{
		ID:        uuid.New(),
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
	self, scopes := assetScope(claims)
	if self == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "平台级（无租户）暂不支持资产")
	}
	if strings.TrimSpace(req.ID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "缺少资产 ID")
	}
	exist, err := dal.GetAsset(req.ID, scopes)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.CodeNotFound)
		}
		return nil, errcode.New(errcode.CodeDBError)
	}
	if exist.TenantID != self {
		return nil, errcode.New(errcode.CodeNotFound)
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
		return nil, errcode.New(errcode.CodeNotFound)
	}
	got, err := dal.GetAsset(req.ID, scopes)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	return got, nil
}

// Delete 删除资产；存在子节点时拒绝（需先删除子树）。
func (*Asset) Delete(claims *utils.UserClaims, id string) error {
	self, scopes := assetScope(claims)
	if self == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "平台级（无租户）暂不支持资产")
	}
	if _, err := dal.GetAsset(id, scopes); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.CodeNotFound)
		}
		return errcode.New(errcode.CodeDBError)
	}
	children, err := dal.CountAssetChildren(id, scopes)
	if err != nil {
		return errcode.New(errcode.CodeDBError)
	}
	if children > 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "存在子节点，请先删除或迁移子树")
	}
	if err := dal.DeleteAsset(id, self); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.CodeNotFound)
		}
		return errcode.New(errcode.CodeDBError)
	}
	return nil
}

// List 分页查询根/指定父节点下资产。
func (*Asset) List(claims *utils.UserClaims, parentID, keyword string, page, pageSize int) ([]*model.Asset, int64, error) {
	_, scopes := assetScope(claims)
	if len(scopes) == 0 {
		return []*model.Asset{}, 0, nil
	}
	list, total, err := dal.ListAssetsByPage(scopes, parentID, keyword, page, pageSize)
	if err != nil {
		return nil, 0, errcode.New(errcode.CodeDBError)
	}
	return list, total, nil
}

// Get 读取单个资产。
func (*Asset) Get(claims *utils.UserClaims, id string) (*model.Asset, error) {
	_, scopes := assetScope(claims)
	if len(scopes) == 0 {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	a, err := dal.GetAsset(id, scopes)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.CodeNotFound)
		}
		return nil, errcode.New(errcode.CodeDBError)
	}
	return a, nil
}

// AssetTreeNode 资产树节点（含子节点）。
type AssetTreeNode struct {
	model.Asset
	Children []*AssetTreeNode `json:"children"`
}

// Tree 返回租户作用域内完整资产树（根节点平铺）。
func (*Asset) Tree(claims *utils.UserClaims) ([]*AssetTreeNode, error) {
	_, scopes := assetScope(claims)
	if len(scopes) == 0 {
		return []*AssetTreeNode{}, nil
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
