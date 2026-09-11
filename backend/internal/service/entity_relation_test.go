package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

// fakeScopeProvider 用固定映射模拟租户层级，便于脱离数据库验证 Scope 守卫。
type fakeScopeProvider struct {
	scope map[string][]string
	err   error
}

func (f *fakeScopeProvider) Scope(ctx context.Context, tenantID string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.scope[tenantID], nil
}

func useFakeScope(t *testing.T, p TenantScopeProvider) {
	t.Helper()
	old := entityRelationScopeProvider
	entityRelationScopeProvider = p
	t.Cleanup(func() { entityRelationScopeProvider = old })
}

// stubRelationList 把查询落点替换为桩，使 Scope 守卫可脱离数据库验证。
// 这些用例断言的是"允不允许查"，而不是"查到了什么"；
// 让它们真的去打数据库，在无库环境下只会把结论变成"存储不可用"。
func stubRelationList(t *testing.T) *model.EntityRelationQuery {
	t.Helper()
	var captured model.EntityRelationQuery
	old := listEntityRelations
	listEntityRelations = func(q model.EntityRelationQuery) ([]*model.EntityRelation, int64, error) {
		captured = q
		return nil, 0, nil
	}
	t.Cleanup(func() { listEntityRelations = old })
	return &captured
}

func TestListRelationsAllowsOwnTenantWithoutHierarchy(t *testing.T) {
	// 自身租户应直接放行，不依赖层级可用性（层级不可用时也不应把查自己变成失败）。
	useFakeScope(t, &fakeScopeProvider{err: errors.New("hierarchy down")})
	stubRelationList(t)

	_, _, err := ListRelations(context.Background(), "t1", model.EntityRelationQuery{
		TenantID: "t1",
		FromType: model.EntityTypeDevice,
		FromID:   "d1",
	})
	if err != nil {
		t.Fatalf("查自身租户不应因层级状态失败: %v", err)
	}
}

func TestListRelationsRejectsOutOfScopeTenantAsNotFound(t *testing.T) {
	useFakeScope(t, &fakeScopeProvider{scope: map[string][]string{
		"child": {"child"},
	}})

	// 子租户试图越权看父租户：必须被拒绝，且不泄漏"该租户存在"。
	_, _, err := ListRelations(context.Background(), "child", model.EntityRelationQuery{
		TenantID: "parent",
		FromType: model.EntityTypeDevice,
		FromID:   "d1",
	})
	if err == nil {
		t.Fatal("越权访问其他租户应被拒绝")
	}
	if !strings.Contains(err.Error(), "关系不存在") {
		t.Fatalf("跨租户应表现为 not found，实际: %v", err)
	}
}

func TestListRelationsAllowsInScopeDescendant(t *testing.T) {
	useFakeScope(t, &fakeScopeProvider{scope: map[string][]string{
		"parent": {"parent", "child"},
	}})

	// 目标租户在 Scope 内 → 通过守卫。此处会因无数据库而返回 DB 错误，
	// 但关键是**不能**是 Scope 拒绝错误。
	if err := assertTenantInScope(context.Background(), "parent", "child"); err != nil {
		t.Fatalf("Scope 内的子租户不应被守卫拒绝: %v", err)
	}
}

func TestListRelationsFailsClosedWhenHierarchyErrors(t *testing.T) {
	useFakeScope(t, &fakeScopeProvider{err: errors.New("boom")})

	_, _, err := ListRelations(context.Background(), "t1", model.EntityRelationQuery{
		TenantID: "t2",
		FromType: model.EntityTypeDevice,
		FromID:   "d1",
	})
	if err == nil {
		t.Fatal("层级不可用时应 fail closed，而不是放行")
	}
	// 关键：不能静默降级成 "关系不存在"，那会把故障伪装成没有数据。
	if strings.Contains(err.Error(), "关系不存在") {
		t.Fatalf("层级故障不应伪装成 not found: %v", err)
	}
}

func TestListRelationsRejectsMalformedQuery(t *testing.T) {
	// 无任何端点过滤 → 等价于列出全租户关系，必须拒绝。
	_, _, err := ListRelations(context.Background(), "t1", model.EntityRelationQuery{TenantID: "t1"})
	if err == nil {
		t.Fatal("空过滤条件的查询应被拒绝")
	}
}

func TestCreateRelationRejectsInvalidInputBeforeTouchingDatabase(t *testing.T) {
	tooLarge := "{" + strings.Repeat("x", 9000) + "}"

	cases := []struct {
		name string
		rel  *model.EntityRelation
	}{
		{
			name: "空关系",
			rel:  nil,
		},
		{
			name: "缺租户",
			rel:  &model.EntityRelation{FromType: model.EntityTypeDevice, FromID: "d1", ToType: model.EntityTypeAsset, ToID: "a1", RelationType: "contains"},
		},
		{
			name: "自环",
			rel:  &model.EntityRelation{TenantID: "t1", FromType: model.EntityTypeDevice, FromID: "d1", ToType: model.EntityTypeDevice, ToID: "d1", RelationType: "contains"},
		},
		{
			name: "实体类型不在白名单",
			rel:  &model.EntityRelation{TenantID: "t1", FromType: "robot", FromID: "r1", ToType: model.EntityTypeAsset, ToID: "a1", RelationType: "contains"},
		},
		{
			name: "缺关系类型",
			rel:  &model.EntityRelation{TenantID: "t1", FromType: model.EntityTypeDevice, FromID: "d1", ToType: model.EntityTypeAsset, ToID: "a1"},
		},
		{
			name: "元数据超限",
			rel:  &model.EntityRelation{TenantID: "t1", FromType: model.EntityTypeDevice, FromID: "d1", ToType: model.EntityTypeAsset, ToID: "a1", RelationType: "contains", Metadata: &tooLarge},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := CreateRelation(context.Background(), c.rel); err == nil {
				t.Fatal("非法输入应被拒绝")
			}
		})
	}
}

func TestDeleteRelationsForEntityRejectsUnknownType(t *testing.T) {
	_, err := DeleteRelationsForEntity(context.Background(), "t1", "robot", "r1", EntityDeletionProtect)
	if err == nil {
		t.Fatal("未知实体类型应被拒绝")
	}
}

func TestDeleteRelationsForEntityRejectsEmptyTenantOrID(t *testing.T) {
	if _, err := DeleteRelationsForEntity(context.Background(), "", model.EntityTypeDevice, "d1", EntityDeletionProtect); err == nil {
		t.Fatal("空租户应被拒绝")
	}
	if _, err := DeleteRelationsForEntity(context.Background(), "t1", model.EntityTypeDevice, "", EntityDeletionProtect); err == nil {
		t.Fatal("空实体 ID 应被拒绝")
	}
}

func TestHasRelationPathRejectsUnknownType(t *testing.T) {
	if _, err := HasRelationPath(context.Background(), "t1", "robot", "r1", model.EntityTypeDevice, "d1", "", 0); err == nil {
		t.Fatal("未知实体类型应被拒绝")
	}
	if _, err := HasRelationPath(context.Background(), "t1", model.EntityTypeDevice, "d1", "robot", "r1", "", 0); err == nil {
		t.Fatal("未知实体类型应被拒绝")
	}
}
