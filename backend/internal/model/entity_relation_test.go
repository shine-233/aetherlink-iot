// 文件用途：通用实体关系校验的定向证据（ROADMAP P1.1）。
// 覆盖：必填字段、实体类型白名单、自环拒绝、关系类型长度、元数据上限、反向关系判定。
package model

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validRelation() *EntityRelation {
	meta := `{}`
	return &EntityRelation{
		TenantID:     "tenant-1",
		FromType:     EntityTypeDevice,
		FromID:       "dev-1",
		RelationType: "installed_at",
		ToType:       EntityTypeAsset,
		ToID:         "asset-1",
		Metadata:     &meta,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

func TestValidateEntityRelationAcceptsValidEdge(t *testing.T) {
	if err := ValidateEntityRelation(validRelation(), len(`{}`)); err != nil {
		t.Fatalf("valid relation must be accepted: %v", err)
	}
	// 无元数据同样合法。
	r := validRelation()
	r.Metadata = nil
	if err := ValidateEntityRelation(r, 0); err != nil {
		t.Fatalf("relation without metadata must be accepted: %v", err)
	}
}

func TestValidateEntityRelationRejectsMissingFields(t *testing.T) {
	cases := map[string]func(*EntityRelation){
		"tenant":     func(r *EntityRelation) { r.TenantID = "" },
		"from id":    func(r *EntityRelation) { r.FromID = "" },
		"to id":      func(r *EntityRelation) { r.ToID = "" },
		"rel type":   func(r *EntityRelation) { r.RelationType = "" },
		"whitespace": func(r *EntityRelation) { r.TenantID = "   " },
	}
	for name, mutate := range cases {
		r := validRelation()
		mutate(r)
		if err := ValidateEntityRelation(r, 0); !errors.Is(err, ErrEntityRelationMissingField) {
			t.Fatalf("%s must be rejected as missing field, got %v", name, err)
		}
	}
	if err := ValidateEntityRelation(nil, 0); !errors.Is(err, ErrEntityRelationMissingField) {
		t.Fatalf("nil relation must be rejected, got %v", err)
	}
}

func TestValidateEntityRelationEnforcesTypeAllowlist(t *testing.T) {
	// 白名单之外的实体类型一律拒绝，防止任意字符串污染关系图。
	for _, bad := range []string{"", "Device", "widget", "unknown", " "} {
		r := validRelation()
		r.FromType = bad
		if err := ValidateEntityRelation(r, 0); !errors.Is(err, ErrEntityRelationUnknownType) {
			t.Fatalf("from_type %q must be rejected, got %v", bad, err)
		}
		r = validRelation()
		r.ToType = bad
		if err := ValidateEntityRelation(r, 0); !errors.Is(err, ErrEntityRelationUnknownType) {
			t.Fatalf("to_type %q must be rejected, got %v", bad, err)
		}
	}
	for _, ok := range AllowedEntityTypes() {
		if !IsAllowedEntityType(ok) {
			t.Fatalf("%s must be allowed", ok)
		}
	}
}

func TestValidateEntityRelationRejectsSelfLoop(t *testing.T) {
	r := validRelation()
	r.ToType = r.FromType
	r.ToID = r.FromID
	if err := ValidateEntityRelation(r, 0); !errors.Is(err, ErrEntityRelationSelfLoop) {
		t.Fatalf("self loop must be rejected, got %v", err)
	}
	// 不同类型但同 ID 不算自环（ID 空间按类型隔离）。
	r = validRelation()
	r.ToType = EntityTypeAsset
	r.ToID = r.FromID
	if err := ValidateEntityRelation(r, 0); err != nil {
		t.Fatalf("same id across different types is not a self loop: %v", err)
	}
}

func TestValidateEntityRelationRejectsOversizedInput(t *testing.T) {
	r := validRelation()
	r.RelationType = strings.Repeat("x", 65)
	if err := ValidateEntityRelation(r, 0); !errors.Is(err, ErrEntityRelationTypeTooLong) {
		t.Fatalf("overlong relation type must be rejected, got %v", err)
	}

	r = validRelation()
	if err := ValidateEntityRelation(r, 8193); !errors.Is(err, ErrEntityRelationMetadataTooLarge) {
		t.Fatalf("oversized metadata must be rejected, got %v", err)
	}
	// 恰好在上限应通过。
	if err := ValidateEntityRelation(validRelation(), 8192); err != nil {
		t.Fatalf("metadata at the limit must be accepted: %v", err)
	}
}

func TestEntityRelationReverseDetection(t *testing.T) {
	a := validRelation()
	b := &EntityRelation{
		FromType: a.ToType, FromID: a.ToID,
		RelationType: a.RelationType,
		ToType:       a.FromType, ToID: a.FromID,
	}
	if !a.IsReverseOf(b) {
		t.Fatal("reverse relation must be detected")
	}
	if !b.IsReverseOf(a) {
		t.Fatal("reverse detection must be symmetric")
	}
	// 端点相同但关系类型不同，不算反向。
	c := *b
	c.RelationType = "owned_by"
	if a.IsReverseOf(&c) {
		t.Fatal("different relation type must not count as reverse")
	}
	// nil 安全。
	var nilRel *EntityRelation
	if nilRel.IsReverseOf(a) || a.IsReverseOf(nilRel) {
		t.Fatal("nil relation must not be reverse of anything")
	}
}
