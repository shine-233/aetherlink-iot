-- 60.sql: 租户客户层级（ROADMAP C2）
-- 背景：现有系统租户为"隐式维度"——users.tenant_id 及所有业务表 tenant_id 直接承载
--       8 位随机租户 ID（邮箱自助注册时生成），没有独立的租户实体表，更无层级关系。
--       本迁移引入 tenants 表（含 parent_tenant_id 自引用），把租户口径显式化，
--       为"上级租户可管理自身 + 全部后代租户"的客户层级（分销/子公司）提供数据基础。
-- 边界：
--   1. users.tenant_id 不加外键引用 tenants.id：存量/测试数据存在遗留租户 ID，
--      强 FK 会阻塞迁移；层级归属通过 tenants 表单独登记，超出登记范围的租户
--      ID 在数据范围解析时退化为"仅自身"（保持兼容）。
--   2. 存量适配：把现有 TENANT_ADMIN 用户去重后的 tenant_id 全部登记为根租户
--      （parent_tenant_id = NULL），自注册客户平滑成为根节点，无需清库。
--   3. 删除约束：仅允许删除无子租户的叶子节点（由 service 层守卫校验）。
-- 回滚：DROP TABLE public.tenants（存量数据不受影响，仅丢失层级关系登记）。

CREATE TABLE IF NOT EXISTS public.tenants (
    id varchar(36) NOT NULL,
    "name" varchar(255) NOT NULL,
    code varchar(64) NULL,
    parent_tenant_id varchar(36) NULL,
    status varchar(2) NOT NULL DEFAULT 'N', -- N-正常 F-冻结
    remark varchar(255) NULL,
    created_at timestamptz(6) NOT NULL DEFAULT now(),
    updated_at timestamptz(6) NOT NULL DEFAULT now(),
    CONSTRAINT tenants_pkey PRIMARY KEY (id),
    CONSTRAINT tenants_code_un UNIQUE (code),
    CONSTRAINT tenants_parent_fk FOREIGN KEY (parent_tenant_id) REFERENCES public.tenants(id)
);
COMMENT ON TABLE public.tenants IS '租户实体表（ROADMAP C2 客户层级）：parent_tenant_id 为 NULL 表示根租户';
COMMENT ON COLUMN public.tenants.id IS '租户 ID，与 users.tenant_id / 各业务表 tenant_id 同源';
COMMENT ON COLUMN public.tenants.parent_tenant_id IS '上级租户 ID，NULL 表示根租户；非 NULL 时为该租户的直接上级';

CREATE INDEX IF NOT EXISTS idx_tenants_parent ON public.tenants(parent_tenant_id);

-- 存量租户登记为根租户（幂等：ON CONFLICT DO NOTHING）。
-- 仅把 authority = TENANT_ADMIN 的租户纳入登记：TENANT_USER 属于其上级租户，
-- 不应单独建根；其 tenant_id 会随上级登记一并覆盖（若上级不在登记集则保持现状）。
INSERT INTO public.tenants (id, "name", code, parent_tenant_id, status, remark, created_at, updated_at)
SELECT u.tenant_id,
       COALESCE(NULLIF(TRIM(u.organization), ''), '租户 ' || u.tenant_id) AS "name",
       NULL,
       NULL,
       'N',
       'backfill root tenant from existing TENANT_ADMIN users (60.sql/C2)',
       now(),
       now()
FROM public.users u
WHERE u.authority = 'TENANT_ADMIN'
  AND u.tenant_id IS NOT NULL
  AND TRIM(u.tenant_id) <> ''
GROUP BY u.tenant_id, u.organization
ON CONFLICT (id) DO NOTHING;