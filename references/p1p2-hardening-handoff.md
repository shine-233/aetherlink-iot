# P1/P2 加固批次 · 三车道全部完成（2026-08-26）

## PR 链（按序合并）

1. **#170** `improve/p2p3-batch` — P2/P3 收敛批（CI 已绿）
2. **#176** `improve/p1p2-hardening` — 安全加固批（17/17 SUCCESS）
   - LIKE 转义 ×15 · F1 刷新吊销 · F2 email 键摘要化 · F3 IP 防爆破 · Casbin fail-fast 审计
   - F4 *Unscoped 改名(26 点) + *ForTenant 双条件删除 · A5 http_client 死代码
   - B1 依赖归位 · vendor-three/motion 分包 · B5 表单空态/submitLoading · C1 doctor TLS 门禁
3. **#177** `improve/p2-lane-tests-visual` — 车道三（5/5 workflow SUCCESS）
   - response 中间件 8 用例契约套件（零测试目录收敛第一步）
   - Device3DPanel 接线为设备详情「3D 预览」tab（懒加载+vendor-three 按需；温度启发式驱动颜色；WebGL 降级）
   - i18n 四语言；6 用例接线单测；shared 视图只读裁剪断言更新
   - 核实更正：**JWT HttpOnly cookie 默认已启用**（auth.cookie.enabled=true），剩余仅生产 HTTPS 开 secure

## 最终未做清单（含理由，详见 VALIDATION.md）

- 巨型文件 top10 拆分：纯代码移动也污染 blame 与在途合并，需独立排期 lane
- 1014 处硬编码 hex token 化：需先定设计 token 权威值与视觉回归基线
- logic/sseapi 测试：需先抽存储接口解耦 gorm query 单例与 Redis hub
- query/cmd/gen 等：生成物或工具入口，无测试价值
- 真实 Redis/E2E 运行验证：本三批均为静态+单测证据

## 环境备忘

worktree：`C:\Users\Zz\AppData\Local\Temp\opencode\final-wt`；改文件用 edit 工具或 .NET IO UTF8NoBOM；
git commit -m 长 message 用 `-F <file>` 且文件须无 BOM。
