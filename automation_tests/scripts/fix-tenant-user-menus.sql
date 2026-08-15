-- MANUAL_ADMIN_ONLY：本脚本不得由自动 runner 执行，仅限经授权的开发/修复环境手工执行。
-- 目标表：sys_ui_elements；执行账号须具备该表的 UPDATE 和 SELECT 权限。
-- 执行前必须备份目标数据，并准备按备份恢复或在事务中回滚。
-- 当前 UPDATE 会跳过 authority 已含 TENANT_USER 的行，因此重复执行具有幂等性。
-- 生产环境执行必须先取得变更审批。

-- 给所有包含 TENANT_ADMIN 的菜单添加 TENANT_USER 角色
-- 这样 TENANT_USER 能登录并访问基本功能
UPDATE sys_ui_elements
SET authority = (authority::jsonb || '"TENANT_USER"'::jsonb)::json
WHERE authority::text LIKE '%TENANT_ADMIN%'
  AND authority::text NOT LIKE '%TENANT_USER%';

-- 验证结果
SELECT count(*) as total_with_tenant_user
FROM sys_ui_elements
WHERE authority::text LIKE '%TENANT_USER%';
