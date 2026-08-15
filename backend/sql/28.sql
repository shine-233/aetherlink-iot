ALTER TABLE public."groups"
	ADD COLUMN IF NOT EXISTS owner_user_id varchar(36) NULL;

CREATE INDEX IF NOT EXISTS idx_groups_tenant_owner
	ON public."groups" USING btree (tenant_id, owner_user_id);

COMMENT ON COLUMN public."groups".owner_user_id IS '分组拥有者用户ID';
