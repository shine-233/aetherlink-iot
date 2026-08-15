# backend/pkg/errcode

## 目录定位

`backend/pkg/errcode` 是后端统一错误码和多语言错误消息包，用于让 API 响应、业务错误和前端展示保持稳定的错误结构。

## 文件用途

- `code.go`：定义系统级、业务级和文件上传相关错误码。
- `error.go`：定义统一错误结构和构造函数，支持自定义消息、数据、格式化参数和变量。
- `language.go`：解析 `Accept-Language` 头并规范化语言标签。
- `manager.go`：从 YAML 加载错误码/字符串消息，按语言优先级查找并缓存结果。
- `error_language_manager_test.go`：覆盖错误构造、语言解析、配置加载、兜底和错误码边界。

## 依赖关系

本目录依赖 `gopkg.in/yaml.v3` 读取消息配置，依赖 `github.com/patrickmn/go-cache` 缓存消息；响应中间件和 API 错误处理会通过本包输出稳定错误信息。配置文件通常来自 `backend/configs/messages.yaml` 和 `backend/configs/messages_str.yaml`。

## 审查发现

- 错误码范围是对外契约，删除或改值会影响前端、API 自动化和第三方调用方。
- `ErrorManager` 直接读取文件路径，测试中已覆盖缺失、非法 YAML 和非法错误码，但生产路径仍依赖配置部署正确。
- 多语言查找按请求权重降级，默认语言当前为 `zh_CN`。

## 重构建议

后续可把错误码范围、模块归属和消息 YAML 做成生成校验或静态检查，避免新增错误码时漏配多语言文案；也可以把配置加载抽象为接口，便于嵌入式配置或单元测试。

## 验证建议

修改本目录后运行 `cd backend; go test ./pkg/errcode -count=1`。新增错误码时同时检查 YAML 配置、API 响应格式和前端/自动化对错误码的断言。
