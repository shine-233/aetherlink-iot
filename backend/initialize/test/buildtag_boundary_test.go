// 文件用途：保持 initialize/test 包在未启用 integration 标签时仍可被 go test 识别。
// 维护提示：不要在本文件加入需要外部依赖的测试，集成测试应继续放在带 build tag 的文件中。
// 核心逻辑：仅声明 package，避免默认 go test 因 integration 文件被排除而混淆包边界。
// 关键注意事项：默认测试不能证明缓存闭环，真正集成用例仍需显式启用 integration 标签和外部服务。
// 重构建议：可抽出 Redis client 与配置加载 seam，用 fake 存储覆盖缓存索引规则的单元测试。

package test
