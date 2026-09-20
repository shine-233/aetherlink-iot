// 文件用途：规则链版本的持久化编排（ROADMAP P1.2）。
// 核心逻辑：把 rule_chain_version.go 的纯语义（状态机、回滚造新草稿、图哈希身份）
// 与 rule_chain_version DAL 接起来；本文件只负责"取数→调用语义→落库→回传审计"。
// 关键注意事项：
//  1. 语义判定一律走 rule_chain_version.go 的纯函数，本文件不重复实现状态机，
//     避免出现两份规则。
//  2. 发布用 DAL 的带条件更新（status='draft'）落库，影响行数 0 表示已被发布过，
//     必须报错而不是静默成功——静默成功会让"重复发布"看起来成功。
//  3. 新建草稿若图哈希与现有 draft 相同，语义层返回 nil，表示无需新版本，
//     本文件原样回传 nil，不伪造一个空版本。
package service

import (
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/go-basic/uuid"
)

// ruleChainVersionsToPure 把持久层行转为语义层结构，供纯函数判定。
func ruleChainVersionsToPure(rows []*model.RuleChainVersion) []RuleChainVersion {
	out := make([]RuleChainVersion, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		pure := RuleChainVersion{
			ID:        row.ID,
			ChainID:   row.ChainID,
			Version:   row.Version,
			Status:    row.Status,
			GraphHash: row.GraphHash,
		}
		if row.CreatedAt != nil {
			pure.CreatedAt = *row.CreatedAt
		}
		pure.PublishedAt = row.PublishedAt
		pure.RolledBackFrom = row.RolledBackFrom
		out = append(out, pure)
	}
	return out
}

// persistRuleChainVersion 把语义层新版本落库。
func persistRuleChainVersion(pure *RuleChainVersion, tenantID string, graph []byte) error {
	createdAt := pure.CreatedAt
	row := &model.RuleChainVersion{
		ID:             pure.ID,
		TenantID:       tenantID,
		ChainID:        pure.ChainID,
		Version:        pure.Version,
		Status:         pure.Status,
		GraphHash:      pure.GraphHash,
		Graph:          graph,
		RolledBackFrom: pure.RolledBackFrom,
		CreatedAt:      &createdAt,
		PublishedAt:    pure.PublishedAt,
	}
	return dal.CreateRuleChainVersion(row)
}

// ListRuleChainVersions 返回当前租户下某条链的版本列表。
func ListRuleChainVersions(tenantID, chainID string) ([]*model.RuleChainVersion, error) {
	if tenantID == "" || chainID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant_id and chain_id are required")
	}
	return dal.ListRuleChainVersions(tenantID, chainID)
}

// CreateRuleChainDraftVersion 基于现有版本序列创建下一个 draft。
// 图内容未变时返回 nil（不产生空版本）。
func CreateRuleChainDraftVersion(tenantID, chainID, graphHash string, graph []byte) (*model.RuleChainVersion, *RuleChainVersionAudit, error) {
	rows, err := dal.ListRuleChainVersions(tenantID, chainID)
	if err != nil {
		return nil, nil, err
	}
	next, audit, err := NewRuleChainDraftVersion(chainID, graphHash, ruleChainVersionsToPure(rows), time.Now().UTC())
	if err != nil || next == nil {
		return nil, audit, err
	}
	next.ID = uuid.New()
	if err := persistRuleChainVersion(next, tenantID, graph); err != nil {
		return nil, nil, err
	}
	return &model.RuleChainVersion{
		ID:             next.ID,
		TenantID:       tenantID,
		ChainID:        next.ChainID,
		Version:        next.Version,
		Status:         next.Status,
		GraphHash:      next.GraphHash,
		RolledBackFrom: next.RolledBackFrom,
		CreatedAt:      &next.CreatedAt,
	}, audit, nil
}

// PublishRuleChainVersionRecord 发布指定版本。已发布过、或该版本不存在/不属于本租户时报错。
// 注意：不要改名为 PublishRuleChainVersion——那是 rule_chain_version.go 里纯语义函数的名字，
// 同包同名会导致编译失败（Go 没有重载）。持久化编排一律带 Record 后缀。
func PublishRuleChainVersionRecord(tenantID, chainID string, version int) (*RuleChainVersionAudit, error) {
	row, err := dal.GetRuleChainVersion(tenantID, chainID, version)
	if err != nil {
		return nil, err
	}
	pure := ruleChainVersionsToPure([]*model.RuleChainVersion{row})[0]
	now := time.Now().UTC()
	if _, err := PublishRuleChainVersion(&pure, now); err != nil {
		return nil, err
	}
	affected, err := dal.PublishRuleChainVersion(tenantID, chainID, version, now)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		// 带条件更新没有命中，说明并发下已被发布或状态已变，不能静默成功。
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, "rule chain version already published")
	}
	return &RuleChainVersionAudit{
		ChainID: chainID,
		Action:  "published",
		Version: version,
		At:      now,
	}, nil
}

// RollbackRuleChainVersionRecord 从指定已发布版本回滚，产生一个新 draft。
// 后缀原因同 PublishRuleChainVersionRecord：避免与纯语义函数同名。
func RollbackRuleChainVersionRecord(tenantID, chainID string, targetVersion int) (*model.RuleChainVersion, []RuleChainVersionAudit, error) {
	rows, err := dal.ListRuleChainVersions(tenantID, chainID)
	if err != nil {
		return nil, nil, err
	}
	pure := ruleChainVersionsToPure(rows)
	var target *RuleChainVersion
	for i := range pure {
		if pure[i].Version == targetVersion {
			target = &pure[i]
			break
		}
	}
	if target == nil {
		return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "rollback target version not found")
	}
	next, audits, err := RollbackRuleChainVersion(target, pure, time.Now().UTC())
	if err != nil {
		return nil, nil, err
	}
	next.ID = uuid.New()
	var graph []byte
	for _, row := range rows {
		if row.Version == targetVersion {
			graph = row.Graph
			break
		}
	}
	if err := persistRuleChainVersion(next, tenantID, graph); err != nil {
		return nil, nil, err
	}
	return &model.RuleChainVersion{
		ID:             next.ID,
		TenantID:       tenantID,
		ChainID:        next.ChainID,
		Version:        next.Version,
		Status:         next.Status,
		GraphHash:      next.GraphHash,
		RolledBackFrom: next.RolledBackFrom,
		CreatedAt:      &next.CreatedAt,
	}, audits, nil
}
