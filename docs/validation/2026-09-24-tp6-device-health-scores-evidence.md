# TP-6 / TB PE 算法中心：设备综合健康度评估引擎运行期验证证据

- 验证日期：2026-09-24
- 验证环境：PostgreSQL 17.5 + Redis + 本地活栈服务（端口 9999）
- 数据库迁移：`121.sql`（`device_health_scores` 主表、索引与 Casbin 权限登记）
- 契约测试：`automation_tests/tests/74_device_health_scores.test.js`
- 前端视图：`frontend/src/views/device/details/modules/health-assessment/index.vue`

## 验证结果

运行命令：
```bash
npx mocha tests/74_device_health_scores.test.js
```

输出：
```text
  TP-6 / TB PE Device Health Score [74_device_health_scores]
    √ 1. GET /devices/health/summary returns tenant health overview metrics
    √ 2. POST /devices/health/evaluate triggers batch evaluation for tenant
    √ 3. GET /devices/:device_id/health returns multidimensional scoring for single device
    √ 4. POST /devices/:device_id/health/evaluate triggers immediate re-evaluation
    √ 5. Singular route contract: GET /device/:id/health operates identically
    √ 6. Multi-tenant isolation: Tenant B cannot view Tenant A device health
    √ 7. Multi-tenant isolation: Tenant B cannot trigger evaluate on Tenant A device
    √ 8. Negative validation: query health for non-existent device returns not found error

  8 passing (260ms)
```

## 判定要点
1. **多维评分算法（0.00 ~ 100.00）**：
   - 告警惩罚分：依据未清除告警的最高严重度加权扣分（CRITICAL -40, MAJOR -25, MINOR -15, WARNING -8）；
   - 离线衰减分：设备离线时长按阶梯持续扣分；
   - 异常扣分：基于时序遥测异常检测结果进行额外惩罚。
2. **四级健康等级**：HEALTHY（>=85分）、SUB_HEALTHY（70~84分）、WARNING（50~69分）、CRITICAL（<50分）。
3. **租户健康大盘与单设备诊断**：支持租户级统计汇总与单设备即时健康评分分析及优化建议。
