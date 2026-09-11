// 文件用途：规则链草稿/发布版本与回滚的语义核心（ROADMAP P1.2 最后一项）。
// 核心逻辑：版本只能沿 draft -> published 单向推进；published 只读；
// 回滚**创建新草稿**而不是把已发布版本改回去，原历史保持只读。
// 关键注意事项：
//  1. published 版本不可编辑、不可重复发布——允许改已发布版本等于让"线上正在跑的图"
//     在无人知晓的情况下被替换，且无法追溯是谁改的。
//  2. 回滚借鉴本项目 OTA 批次回滚的既有约定（RollbackFleetCommandJob）：
//     产生新版本而不是回写原版本，并在两侧各留一条审计事件，双向可追溯。
//  3. 版本号单调递增，由已有最大版本推导，不接受外部传入，避免跳号或撞号。
//  4. 图以哈希参与版本身份（graph_hash）：内容未变不得产生新版本，
//     否则每次保存都会堆一个空版本，把"改了什么"稀释掉。
//
// 重构建议：接入持久化时需要新迁移（版本表），当前迁移号位被并行改动占用，故先落地语义层。
package service

import (
	"time"

	"aetherlink-iot/backend/pkg/errcode"
)

// 规则链版本状态。published 为终态：只读、不可重复发布。
const (
	ruleChainVersionDraft     = "draft"
	ruleChainVersionPublished = "published"
	ruleChainVersionArchived  = "archived"
)

// RuleChainVersion 单个规则链版本。GraphHash 参与身份判定：内容相同不产生新版本。
type RuleChainVersion struct {
	ID          string
	ChainID     string
	Version     int
	Status      string
	GraphHash   string
	CreatedAt   time.Time
	PublishedAt *time.Time
	// RolledBackFrom 非空表示本版本是由哪个已发布版本回滚而来。
	RolledBackFrom *int
}

// RuleChainVersionAudit 每次流转产生的审计事件，双向留痕。
type RuleChainVersionAudit struct {
	ChainID   string
	Action    string // created / published / rollback_from / rollback_to
	Version   int
	RelatedTo *int
	At        time.Time
}

func ruleChainVersionErrorf(msg string) error {
	return errcode.NewWithMessage(errcode.CodeOpDenied, msg)
}

// IsRuleChainVersionEditable 只有 draft 可编辑。已发布与归档版本一律只读。
func IsRuleChainVersionEditable(version *RuleChainVersion) bool {
	return version != nil && version.Status == ruleChainVersionDraft
}

// PublishRuleChainVersion draft -> published，且仅允许一次。
func PublishRuleChainVersion(version *RuleChainVersion, now time.Time) (*RuleChainVersionAudit, error) {
	if version == nil {
		return nil, ruleChainVersionErrorf("rule chain version is missing")
	}
	if version.Status == ruleChainVersionPublished {
		return nil, ruleChainVersionErrorf("rule chain version already published; published versions are read-only")
	}
	if version.Status == ruleChainVersionArchived {
		return nil, ruleChainVersionErrorf("archived rule chain version cannot be published")
	}
	version.Status = ruleChainVersionPublished
	publishedAt := now.UTC()
	version.PublishedAt = &publishedAt
	return &RuleChainVersionAudit{
		ChainID: version.ChainID,
		Action:  "published",
		Version: version.Version,
		At:      publishedAt,
	}, nil
}

// NewRuleChainDraftVersion 在已有版本序列上创建下一个 draft。
// graphHash 与当前 draft 相同时返回 (nil, nil)，表示无需新版本——避免空版本堆积。
func NewRuleChainDraftVersion(chainID string, graphHash string, versions []RuleChainVersion, now time.Time) (*RuleChainVersion, *RuleChainVersionAudit, error) {
	if chainID == "" {
		return nil, nil, ruleChainVersionErrorf("rule chain id is required")
	}
	if graphHash == "" {
		return nil, nil, ruleChainVersionErrorf("rule chain graph hash is required")
	}
	maxVersion := 0
	for _, existing := range versions {
		if existing.ChainID != chainID {
			continue
		}
		// 已存在内容相同的 draft：不重复造版本，否则"改了什么"会被空版本稀释。
		if existing.Status == ruleChainVersionDraft && existing.GraphHash == graphHash {
			return nil, nil, nil
		}
		if existing.Version > maxVersion {
			maxVersion = existing.Version
		}
	}
	createdAt := now.UTC()
	next := &RuleChainVersion{
		ID:        chainID + "-v" + itoaVersion(maxVersion+1),
		ChainID:   chainID,
		Version:   maxVersion + 1,
		Status:    ruleChainVersionDraft,
		GraphHash: graphHash,
		CreatedAt: createdAt,
	}
	return next, &RuleChainVersionAudit{
		ChainID: chainID,
		Action:  "created",
		Version: next.Version,
		At:      createdAt,
	}, nil
}

// RollbackRuleChainVersion 从已发布版本回滚：新建一个 draft（内容取目标版本），
// 原版本保持只读，并在两侧各留一条审计事件。
func RollbackRuleChainVersion(target *RuleChainVersion, versions []RuleChainVersion, now time.Time) (*RuleChainVersion, []RuleChainVersionAudit, error) {
	if target == nil {
		return nil, nil, ruleChainVersionErrorf("rollback target is missing")
	}
	if target.Status != ruleChainVersionPublished {
		return nil, nil, ruleChainVersionErrorf("rollback target must be a published version")
	}
	maxVersion := 0
	for _, existing := range versions {
		if existing.ChainID == target.ChainID && existing.Version > maxVersion {
			maxVersion = existing.Version
		}
	}
	createdAt := now.UTC()
	from := target.Version
	next := &RuleChainVersion{
		ID:             target.ChainID + "-v" + itoaVersion(maxVersion+1),
		ChainID:        target.ChainID,
		Version:        maxVersion + 1,
		Status:         ruleChainVersionDraft,
		GraphHash:      target.GraphHash,
		CreatedAt:      createdAt,
		RolledBackFrom: &from,
	}
	audits := []RuleChainVersionAudit{
		{ChainID: target.ChainID, Action: "rollback_from", Version: from, RelatedTo: &next.Version, At: createdAt},
		{ChainID: target.ChainID, Action: "rollback_to", Version: next.Version, RelatedTo: &from, At: createdAt},
	}
	return next, audits, nil
}

func itoaVersion(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
