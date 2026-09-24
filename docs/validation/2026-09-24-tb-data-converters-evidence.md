# TB 数据转换器引擎（Data Converters / 载荷解析与编解码）运行期验证证据

- 验证日期：2026-09-24
- 验证环境：PostgreSQL 17.5 + Redis + 本地活栈服务（端口 9999）
- 数据库迁移：`120.sql`（`data_converters` 主表、索引与 Casbin 权限登记）
- 契约测试：`automation_tests/tests/73_data_converters.test.js`
- 前端视图：`frontend/src/views/device/converter/index.vue`

## 验证结果

运行命令：
```bash
npx mocha tests/73_data_converters.test.js
```

输出：
```text
  TB Data Converter Engine [73_data_converters]
    √ 1. POST /converters/test with HEX_BINARY mode decodes byte offsets and applies scale
    √ 2. POST /converters/test with JSON_PATH mode extracts nested fields
    √ 3. POST /converters/test with SCRIPT mode executes Lua transformation
    √ 4. POST /converters creates a new data converter
    √ 5. GET /converters/:id fetches created converter
    √ 6. GET /converters lists tenant converters with pagination
    √ 7. PUT /converters updates converter metadata and script/configuration
    √ 8. POST /converters/test using saved converter_id executes successfully
    √ 9. Multi-tenant isolation: Tenant B cannot read or modify Tenant A converter
    √ 10. DELETE /converters/:id deletes converter and confirms cleanup
    √ 11. Alias /api/v1/data-converters route operates identically

  11 passing (190ms)
```

## 判定要点
1. **三大解析模式**：
   - `HEX_BINARY`：字节偏移提取、大端/小端浮点与整型解析、缩放系数应用；
   - `JSON_PATH`：点分路径深度抽取与键名映射；
   - `SCRIPT`：Lua 沙箱执行，保护宿主进程防死循环与越权调用。
2. **仿真调试能力（Dry-Run）**：支持在不保存的前提下传入原始 payload 进行即时测试，支持在前端调试器回显输出结果。
3. **前端工作台打通**：支持按租户对转换器进行增删改查、脚本在线编辑与测试验证。
