// 文件用途：保留网关测试目录在无 external_integration 构建标签时的包边界。
// 核心逻辑：仅声明 package，避免默认 go test 因目录内所有有效测试文件都被 build tag 排除而产生混淆。
// 关键注意事项：不包含测试逻辑，真正的网关集成用例需要 external_integration 标签和 AUTOTEST_EXTERNAL=1。
// 重构建议：若 Go 测试入口统一到脚本，可考虑用 README 说明替代该边界文件。

/*
Purpose: 保留网关测试目录在无 external_integration 构建标签时的包边界。
Core logic: 仅声明 package，避免默认 go test 因目录内所有有效测试文件都被 build tag 排除而产生混淆。
Important notes: 不包含测试逻辑；真正的网关集成用例需要使用 external_integration 标签和 AUTOTEST_EXTERNAL=1。
Refactor suggestion: 若 Go 测试入口统一到脚本，可考虑用 README 说明替代该边界文件。
*/
package tests
