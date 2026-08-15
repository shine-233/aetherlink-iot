# 日志输出规范

## 目的

日志输出规范是确保软件质量、提高开发效率和维护性能的关键部分。

1. **快速定位和解决问题**：提高故障诊断效率。
2. **提升代码可维护性**：便于团队成员理解和维护。
3. **确保一致性和可比较性**：统一的格式便于分析和监控。
4. **满足安全和合规要求**：记录关键操作和数据访问。
5. **监控性能和故障恢复**：辅助性能监测和系统恢复。
6. **便于数据分析**：利于生成业务和技术洞察。

## 各个日志级别说明

当前后端主日志实现是 `github.com/sirupsen/logrus`（见
`backend/initialize/log_init.go`）；启动初始化和 `cmd/virtual_sensor` 等少数工具仍使用
标准库 `log`。`log.level` 当前接受 `panic`、`fatal`、`error`、`warn`、`info`、
`debug`、`trace`，不要把日志级别描述成只有四级。文件输出由配置的 `log.adapter_type`
控制，应用日志和 SQL 日志分别写入 `app.log` 与 `sql.log`，并由 lumberjack 负责轮转。

### 1. Trace（跟踪）

- **用途**：记录比 Debug 更细的内部执行信息，仅在需要深入排障时启用。
- **内容**：非常高频的调用细节；禁止写入密码、令牌、私钥和完整敏感报文。

### 2. Debug（调试）

- **用途**：开发和故障排除。
- **内容**：详细技术信息，如变量值、系统状态、执行路径。
- **场景**：代码调试、问题跟踪。

### 3. Information（信息）

- **用途**：记录正常运行状态和重要事件。
- **内容**：关键操作和重要事件。
- **场景**：系统监控、审计。

### 4. Warning（警告）

- **用途**：标识可能影响性能或稳定性的情况。
- **内容**：潜在问题和推荐行动。
- **场景**：故障预防、性能优化。

### 5. Error（错误）

- **用途**：记录严重问题或系统错误。
- **内容**：错误描述、影响范围、出错组件。
- **场景**：错误处理、系统恢复。

### 6. Fatal/Panic（终止性错误）

- `Fatal` 记录后会退出进程，`Panic` 记录后会触发 panic；只用于无法继续运行的启动或基础设施错误。
- 普通请求失败、设备离线或可重试错误使用 `Error`/`Warn`，不要用 `Fatal`/`Panic` 代替错误返回。

### 通用原则

- **一致性**：保持格式统一。
- **简洁性**：避免冗余。
- **安全性**：不记录敏感信息。
- **性能考虑**：注意影响。

## 代码示例

下面示例使用项目实际依赖的 `logrus` API，可直接编译；标准库 `log` 只适合没有结构化 logger 的启动/工具代码。

```go
package main

import (
    "fmt"
    "math/rand"
    "time"

    "github.com/sirupsen/logrus"
)

func main() {
    logrus.Info("应用程序启动")

    result, err := performOperation()
    if err != nil {
        logrus.WithError(err).Error("操作过程中出错")
        return
    }

    if result < 0 {
        logrus.WithField("result", result).Warn("警告：结果为负数")
    } else {
        logrus.WithField("result", result).Info("操作成功完成")
    }

    logrus.Info("应用程序结束")
}

func performOperation() (int, error) {
    logrus.Debug("开始执行操作")

    rand.Seed(time.Now().UnixNano())
    num := rand.Intn(20) - 10 // 随机数在-10到9之间

    // 在10%的情况下模拟错误
    if rand.Float32() < 0.1 {
        logrus.Debug("执行操作时遇到错误")
        return 0, fmt.Errorf("发生随机错误")
    }

    logrus.WithField("result", num).Debug("操作执行完成")
    return num, nil
}

```
