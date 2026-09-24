# TB-13 地理空间追踪与看板地图部件（Geospatial Map Tracking）运行期验证证据

- 验证日期：2026-09-24
- 验证环境：PostgreSQL 17.5 + Redis + 本地活栈服务（端口 9999）
- 数据库迁移：`119.sql`（登记位置拉取与轨迹回放 Casbin 路由并赋权）
- 契约测试：`automation_tests/tests/72_geospatial_map_tracking.test.js`

## 验证结果

运行命令：
```bash
npx mocha tests/72_geospatial_map_tracking.test.js
```

输出：
```text
  TB-13 Geospatial Map Tracking [72_geospatial_map_tracking]
    √ 1. GET /devices/locations/latest returns list and total structure
    √ 2. GET /device/locations/latest (singular route) returns identical contract
    √ 3. Device with static location string is resolved to numeric coordinates
    √ 4. Device reporting dynamic GPS telemetry updates latest coordinates and telemetry attributes
    √ 5. GET /device/:id/location/history returns historical breadcrumb points
    √ 6. GET /devices/:device_id/location/history (plural route) returns identical trajectory
    √ 7. Multi-tenant security: Tenant B cannot access Tenant A device location history (201001)
    √ 8. Querying non-existent device location history returns error gracefully (not 500)

  8 passing (7.2s)
```

## 判定要点
1. **静态配置坐标与动态时序遥测双轨解析**：设备既支持静态属性声明位置（`116.4074, 39.9042`），也支持实时高频 GPS 上报（latitude, longitude, speed, altitude），服务层优先提取最新遥测。
2. **历史行驶轨迹重现**：支持按设备与时间窗口检索历史点阵序列，按时间戳升序排序。
3. **租户越权拦截**：非同租户设备坐标或历史轨迹请求严格返回 201001 拒绝，防御侧漏。
