package model

import "testing"

func TestValidateEntityRelationQuery(t *testing.T) {
	cases := []struct {
		name string
		q    EntityRelationQuery
		want error
	}{
		{
			name: "缺租户",
			q:    EntityRelationQuery{FromType: EntityTypeDevice},
			want: ErrEntityRelationMissingTenant,
		},
		{
			name: "起点类型不在白名单",
			q:    EntityRelationQuery{TenantID: "t1", FromType: "robot"},
			want: ErrEntityRelationUnknownType,
		},
		{
			name: "终点类型不在白名单",
			q:    EntityRelationQuery{TenantID: "t1", ToType: "robot"},
			want: ErrEntityRelationUnknownType,
		},
		{
			name: "只给实体类型不给 ID",
			q:    EntityRelationQuery{TenantID: "t1", EntityType: EntityTypeDevice},
			want: ErrEntityRelationIncompleteEntity,
		},
		{
			name: "只给实体 ID 不给类型",
			q:    EntityRelationQuery{TenantID: "t1", EntityID: "d1"},
			want: ErrEntityRelationIncompleteEntity,
		},
		{
			name: "实体类型不在白名单",
			q:    EntityRelationQuery{TenantID: "t1", EntityType: "robot", EntityID: "d1"},
			want: ErrEntityRelationUnknownType,
		},
		{
			name: "方向非法",
			q:    EntityRelationQuery{TenantID: "t1", EntityType: EntityTypeDevice, EntityID: "d1", Direction: "sideways"},
			want: ErrEntityRelationInvalidDirection,
		},
		{
			name: "无任何端点过滤条件",
			q:    EntityRelationQuery{TenantID: "t1"},
			want: ErrEntityRelationEmptyQuery,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.q.Validate(); err != c.want {
				t.Fatalf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

func TestValidateEntityRelationQueryAcceptsValidShapes(t *testing.T) {
	valid := []EntityRelationQuery{
		{TenantID: "t1", FromType: EntityTypeDevice, FromID: "d1"},
		{TenantID: "t1", ToType: EntityTypeAsset, ToID: "a1"},
		{TenantID: "t1", EntityType: EntityTypeGateway, EntityID: "g1"},
		{TenantID: "t1", EntityType: EntityTypeGateway, EntityID: "g1", Direction: RelationDirectionOut},
		{TenantID: "t1", EntityType: EntityTypeGateway, EntityID: "g1", Direction: RelationDirectionIn},
	}
	for i, q := range valid {
		if err := q.Validate(); err != nil {
			t.Fatalf("case %d: Validate() = %v, want nil", i, err)
		}
	}
}

func TestEntityRelationQueryNormalizedDirection(t *testing.T) {
	if got := (EntityRelationQuery{}).NormalizedDirection(); got != RelationDirectionAny {
		t.Fatalf("空方向应归一化为 any，实际 %q", got)
	}
	if got := (EntityRelationQuery{Direction: RelationDirectionIn}).NormalizedDirection(); got != RelationDirectionIn {
		t.Fatalf("显式方向不应被改写，实际 %q", got)
	}
}
