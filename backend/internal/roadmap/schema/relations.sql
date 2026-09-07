-- Framework only. Do not run directly against production.
-- Add as a numbered migration after product review and tenant-scope tests.
CREATE TABLE IF NOT EXISTS entity_relations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  from_entity_type varchar(32) NOT NULL,
  from_entity_id uuid NOT NULL,
  relation_type varchar(64) NOT NULL,
  to_entity_type varchar(32) NOT NULL,
  to_entity_id uuid NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT entity_relations_no_self_relation CHECK (
    from_entity_type <> to_entity_type OR from_entity_id <> to_entity_id
  )
);

CREATE INDEX IF NOT EXISTS idx_entity_relations_from
  ON entity_relations (tenant_id, from_entity_type, from_entity_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_entity_relations_to
  ON entity_relations (tenant_id, to_entity_type, to_entity_id, relation_type);
