// 文件用途：规则链草稿/发布/回滚的定向证据（ROADMAP P1.2）。
// 覆盖：published 只读、不可重复发布、回滚产生新草稿且原版本不变、双向审计、
// 内容未变不产生空版本、版本号单调递增、越权与非法流转 fail closed。
// 说明：语义层证据，不依赖数据库；持久化需新迁移，当前未接线。
package service

import (
	"testing"
	"time"
)

var ruleChainVersionNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

func ruleChainDraftFixture(chainID string, version int, hash string) RuleChainVersion {
	return RuleChainVersion{
		ID:        chainID + "-v1",
		ChainID:   chainID,
		Version:   version,
		Status:    ruleChainVersionDraft,
		GraphHash: hash,
		CreatedAt: ruleChainVersionNow,
	}
}

func TestRuleChainPublishedVersionIsReadOnly(t *testing.T) {
	draft := ruleChainDraftFixture("c1", 1, "hash-a")
	if _, err := PublishRuleChainVersion(&draft, ruleChainVersionNow); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if IsRuleChainVersionEditable(&draft) {
		t.Fatalf("published version must not be editable")
	}
}

func TestRuleChainRepublishIsRejected(t *testing.T) {
	draft := ruleChainDraftFixture("c1", 1, "hash-a")
	if _, err := PublishRuleChainVersion(&draft, ruleChainVersionNow); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if _, err := PublishRuleChainVersion(&draft, ruleChainVersionNow); err == nil {
		t.Fatalf("re-publishing a published version must fail closed")
	}
}

func TestRuleChainArchivedVersionCannotPublish(t *testing.T) {
	archived := ruleChainDraftFixture("c1", 1, "hash-a")
	archived.Status = ruleChainVersionArchived
	if _, err := PublishRuleChainVersion(&archived, ruleChainVersionNow); err == nil {
		t.Fatalf("archived version must not be publishable")
	}
}

func TestRuleChainDraftIsEditable(t *testing.T) {
	draft := ruleChainDraftFixture("c1", 1, "hash-a")
	if !IsRuleChainVersionEditable(&draft) {
		t.Fatalf("draft must be editable")
	}
}

func TestRuleChainSameGraphDoesNotCreateEmptyVersion(t *testing.T) {
	existing := []RuleChainVersion{ruleChainDraftFixture("c1", 1, "hash-a")}
	next, _, err := NewRuleChainDraftVersion("c1", "hash-a", existing, ruleChainVersionNow)
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}
	if next != nil {
		t.Fatalf("identical graph must not create another draft, got v%d", next.Version)
	}
}

func TestRuleChainNewDraftIncrementsVersion(t *testing.T) {
	existing := []RuleChainVersion{ruleChainDraftFixture("c1", 3, "hash-a")}
	next, audit, err := NewRuleChainDraftVersion("c1", "hash-b", existing, ruleChainVersionNow)
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}
	if next == nil || next.Version != 4 {
		t.Fatalf("expected version 4, got %+v", next)
	}
	if next.Status != ruleChainVersionDraft {
		t.Fatalf("new version must be draft, got %s", next.Status)
	}
	if audit == nil || audit.Action != "created" || audit.Version != 4 {
		t.Fatalf("audit mismatch: %+v", audit)
	}
}

func TestRuleChainRollbackCreatesNewDraftAndKeepsHistory(t *testing.T) {
	published := ruleChainDraftFixture("c1", 2, "hash-old")
	if _, err := PublishRuleChainVersion(&published, ruleChainVersionNow); err != nil {
		t.Fatalf("publish: %v", err)
	}
	versions := []RuleChainVersion{
		ruleChainDraftFixture("c1", 1, "hash-old"),
		published,
	}
	rolled, audits, err := RollbackRuleChainVersion(&published, versions, ruleChainVersionNow)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled.Version != 3 {
		t.Fatalf("rollback must create version 3, got %d", rolled.Version)
	}
	if rolled.Status != ruleChainVersionDraft {
		t.Fatalf("rollback result must be a draft, got %s", rolled.Status)
	}
	if rolled.RolledBackFrom == nil || *rolled.RolledBackFrom != 2 {
		t.Fatalf("rollback must record source version 2, got %v", rolled.RolledBackFrom)
	}
	if published.Status != ruleChainVersionPublished || published.GraphHash != "hash-old" {
		t.Fatalf("original published version must stay read-only and unchanged")
	}
	if len(audits) != 2 {
		t.Fatalf("rollback must emit two audit entries, got %d", len(audits))
	}
	actions := map[string]bool{}
	for _, a := range audits {
		actions[a.Action] = true
	}
	if !actions["rollback_from"] || !actions["rollback_to"] {
		t.Fatalf("audit actions must cover both directions, got %+v", audits)
	}
}

func TestRuleChainRollbackRejectsDraftTarget(t *testing.T) {
	draft := ruleChainDraftFixture("c1", 1, "hash-a")
	if _, _, err := RollbackRuleChainVersion(&draft, []RuleChainVersion{draft}, ruleChainVersionNow); err == nil {
		t.Fatalf("rollback target must be published; a draft target must fail closed")
	}
}

func TestRuleChainVersionRejectsMissingIdentity(t *testing.T) {
	if _, _, err := NewRuleChainDraftVersion("", "hash-a", nil, ruleChainVersionNow); err == nil {
		t.Fatalf("empty chain id must fail closed")
	}
	if _, _, err := NewRuleChainDraftVersion("c1", "", nil, ruleChainVersionNow); err == nil {
		t.Fatalf("empty graph hash must fail closed")
	}
	if _, err := PublishRuleChainVersion(nil, ruleChainVersionNow); err == nil {
		t.Fatalf("nil version must fail closed")
	}
	if _, _, err := RollbackRuleChainVersion(nil, nil, ruleChainVersionNow); err == nil {
		t.Fatalf("nil rollback target must fail closed")
	}
}
