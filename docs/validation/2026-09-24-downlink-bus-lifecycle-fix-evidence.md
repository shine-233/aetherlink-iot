# 下行总线取消上下文竞态缺陷修复运行期验证证据

- 验证日期：2026-09-24
- 缺陷位置：`backend/internal/downlink/bus.go`
- 修复内容：在 `beginPublish()` 中优先检测上下文取消状态（`b.ctx != nil && b.ctx.Err() != nil`），防止工作协程提前将 `b.running` 置为 false 时导致向外误抛 `ErrBusNotStarted`（正确预期应为 `ErrBusUnavailable`）。

## 验证结果

运行单测：
```bash
go test -v -count=5 ./internal/downlink/...
```

输出：
```text
=== RUN   TestPublishRejectsCanceledConsumerContext
time="2026-09-24T20:31:20+08:00" level=warning msg="downlink message dropped, total=1" module=downlink queue=command reason="downlink bus unavailable"
--- PASS: TestPublishRejectsCanceledConsumerContext (0.00s)
PASS
ok  	aetherlink-iot/backend/internal/downlink	2.358s
```

全量后端测试回归：
```bash
go test -p 1 ./...
```
- 结果：全量 60+ 包全 ok，0 FAIL，耗时约 40s。
