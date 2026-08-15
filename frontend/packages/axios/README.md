# Axios 工作区包

`frontend/packages/axios` 提供前端共享请求客户端抽象，供业务 service 层复用。

## 目录定位

- 封装 axios 实例、请求/响应拦截器和通用类型。
- 为业务 API wrapper 提供统一的响应 envelope、错误处理和取消/重试扩展点。
- 与 `frontend/src/service/request` 共同组成前端请求基础设施。

## 关键关系

- 这里的抽象会被多处业务 service 依赖，接口变更属于全局影响。
- 响应泛型需要和后端返回 envelope、业务 payload 类型保持一致。
- 拦截器行为要和 src 层 request client 分工清楚，避免重复处理 token 或错误提示。

## 维护注意事项

- 不要在包内写业务页面逻辑或具体接口路径。
- 修改导出类型时要检查所有 service wrapper 和调用方类型推断。
- 全局错误处理应保持可替换，便于测试和不同运行环境复用。
- 如果新增重试、取消或超时能力，需要明确默认行为，避免隐藏重复提交风险。

## 代码审查与重构建议

- 问题：共享请求抽象一旦过度绑定业务，会让所有 API 调用难以独立测试。
- 改进方案：保持包级 API 小而稳定，把业务态处理留给 src 层 request client。
- 实施步骤：先梳理当前导出面，再为 envelope/error 类型补测试，最后收敛未使用的导出。
- 预期效果：请求基础设施更可复用，API wrapper 变更风险更低。
