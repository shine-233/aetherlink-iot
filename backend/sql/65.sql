-- 65.sql: 行业设备模板种子（ROADMAP 行业模板 MVP，2026-09-05）。
-- 内容：三个开箱即用的行业设备模板（工业传感器/电力监测/智能家居），
--       挂在演示租户 d616bcbb 下作为模板库起步内容（可复制/改造）。
-- 守卫：仅在目标租户存在时插入（INSERT..SELECT..WHERE EXISTS）——生产库无该
--       演示租户时零插入、零孤儿数据；重放幂等（名称唯一性由存在性守卫保证）。
-- 说明：模板的物模型（payload schema）由设备配置按 payload_schema_id 关联，
--       不随模板种子走；图表配置给最小结构，前端可按需扩展。

INSERT INTO device_templates (id, name, author, version, description, tenant_id, created_at, updated_at, flag, label, web_chart_config, remark, type_key, brand, model_number)
SELECT 'tpl-industrial-sensor', '工业传感器模板', 'AetherLink', '1.0.0',
       '面向工业现场的通用传感器接入模板：温湿度/振动/压力等点型遥测，配套 1 天分块与告警阈值示例。适用于 Modbus/SNMP/OPC UA 采集与 MQTT 直连设备。',
       t.id, now(), now(), 1, '工业', '{"charts":[]}', '行业模板种子：复制后按现场点表改造', 'industrial', '', ''
FROM tenants t
WHERE t.id = 'd616bcbb'
  AND NOT EXISTS (SELECT 1 FROM device_templates dt WHERE dt.id = 'tpl-industrial-sensor');

INSERT INTO device_templates (id, name, author, version, description, tenant_id, created_at, updated_at, flag, label, web_chart_config, remark, type_key, brand, model_number)
SELECT 'tpl-power-monitor', '电力监测模板', 'AetherLink', '1.0.0',
       '配电房/开关柜电力监测模板：三相电压电流、有功无功、功率因数、电能累计；支持越限告警与日/月电能曲线。',
       t.id, now(), now(), 1, '电力', '{"charts":[]}', '行业模板种子：复制后按现场点表改造', 'power', '', ''
FROM tenants t
WHERE t.id = 'd616bcbb'
  AND NOT EXISTS (SELECT 1 FROM device_templates dt WHERE dt.id = 'tpl-power-monitor');

INSERT INTO device_templates (id, name, author, version, description, tenant_id, created_at, updated_at, flag, label, web_chart_config, remark, type_key, brand, model_number)
SELECT 'tpl-smart-home', '智能家居模板', 'AetherLink', '1.0.0',
       '智能家居网关接入模板：温湿度、门窗、照明的状态与遥测，支持场景联动与移动端查看。',
       t.id, now(), now(), 1, '智能家居', '{"charts":[]}', '行业模板种子：复制后按现场点表改造', 'smart-home', '', ''
FROM tenants t
WHERE t.id = 'd616bcbb'
  AND NOT EXISTS (SELECT 1 FROM device_templates dt WHERE dt.id = 'tpl-smart-home');
