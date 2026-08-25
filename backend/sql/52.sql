-- 52.sql: 数据转发规则引擎（对标 ThingsPanel 数据转发 / 轻量 ThingsBoard integration）。
-- 规则把设备上行数据（遥测/属性/事件/上下线）按来源与可选模板过滤后，
-- 经可选 JS 转换脚本投递到第三方 HTTP 或 MQTT 目标。

CREATE TABLE forward_rules (
  id varchar(36) NOT NULL,
  tenant_id varchar(36) NOT NULL,
  name varchar(128) NOT NULL,
  enabled boolean NOT NULL DEFAULT false,
  source_type varchar(20) NOT NULL, -- telemetry | property | event | status
  device_template_id varchar(36) NULL,
  script text NULL,                  -- 可选 JS 转换脚本（Phase 1 直通，脚本执行挂账）
  target_type varchar(10) NOT NULL, -- http | mqtt
  http_url text NULL,
  http_method varchar(10) NULL DEFAULT 'POST',
  http_headers text NULL,            -- JSON 对象字符串
  mqtt_broker varchar(255) NULL,
  mqtt_topic varchar(255) NULL,
  mqtt_username varchar(128) NULL,
  mqtt_password varchar(255) NULL,
  remark varchar(500) NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT forward_rules_pk PRIMARY KEY (id)
);

CREATE INDEX idx_forward_rules_tenant_enabled ON forward_rules(tenant_id, enabled);
COMMENT ON TABLE forward_rules IS '数据转发规则：设备上行到第三方 HTTP/MQTT 的投递规则';
