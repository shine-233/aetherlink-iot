# TB-17 自定义角色与细粒度权限控制体系（Custom RBAC）运行期验证证据

- 验证日期：2026-09-24
- 验证环境：PostgreSQL 17.5 + Redis + 本地活栈服务（PID 32720 / 端口 9999）
- 数据库迁移：`118.sql`（`sys_permissions`, `sys_role_permissions`，登记 Casbin 路由与初始 p 策略）
- 契约测试：`automation_tests/tests/71_custom_rbac.test.js`

## 验证结果

运行命令：
```bash
npx mocha tests/71_custom_rbac.test.js
```

输出：
```text
  TB-17 Custom RBAC & Granular Permissions [71_custom_rbac]
    √ 1. GET /permissions returns standard permission catalog with module & api_patterns
    √ 2. GET /permissions supports filtering by module
    √ 3. TENANT_ADMIN creates a custom business role
    √ 4. GET /roles/:id/permissions initially returns empty permissions
    √ 5. POST /roles/:id/permissions rejects invalid/non-existent permission codes with 100002
    √ 6. POST /roles/:id/permissions assigns valid permissions and updates Casbin policies
    √ 7. Cross-tenant defense: TENANT_B cannot view or modify TENANT_A role permissions
    √ 8. SYS_ADMIN can view role permissions across tenants
    √ 9. TENANT_ADMIN assigns users to the custom role (POST /roles/:id/users)

  9 passing (45ms)
```

## 判定要点
1. **权限字典模块化分组**：提供 `device`, `telemetry`, `alarm`, `rule_chain`, `report`, `scada`, `ota` 标准权限代码，携带 API 匹配规则与元数据。
2. **动态 Casbin 同步**：给角色赋予权限点时，后端自动解析关联 API patterns 并在内存/持久层动态生成并应用 `p, <role_id>, <pattern>, allow` 策略。
3. **租户拓扑隔离防线**：租户 B 试图查询或修改租户 A 角色的权限点时，严格返回 100003 权限拒绝。
