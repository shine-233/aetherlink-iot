// 文件用途：维护系统用户领域的基础服务结构和共享行为。
// 核心逻辑：聚合用户认证、资料、邮箱、授权和管理能力，供 handler 层统一调用。
// 关键注意事项：用户服务横跨鉴权和租户边界，新增方法必须先明确 claims、角色和副作用语义。
// 重构建议：继续按认证、授权、资料和外部副作用拆分职责，补齐权限、事务和审计测试。
package service

type User struct {
	passwordResetStore       passwordResetStore
	passwordResetEmailSender passwordResetEmailSender
	verificationEmailAdapter verificationEmailAdapter
}
