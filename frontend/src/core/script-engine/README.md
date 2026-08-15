# 脚本引擎系统

## 模块定位

`script-engine` 为 AetherLink IoT 前端提供受控 JavaScript 脚本编辑、模板生成、上下文管理和沙箱执行能力。它面向 IoT 数据处理、规则计算、模拟数据生成和编辑器扩展场景。

## 目录结构

```text
script-engine/
├── index.ts
├── types.ts
├── script-engine.ts
├── executor.ts
├── sandbox.ts
├── context-manager.ts
├── template-manager.ts
├── templates/
│   └── built-in-templates.ts
└── components/
    └── SimpleScriptEditor.vue
```

## 核心职责

- 通过 `ScriptEngine` 组合执行器、沙箱、模板管理器和上下文管理器。
- 使用 `ScriptSandbox` 限制脚本访问的全局对象，并拦截危险语法和逃逸路径。
- 当前沙箱是同线程防护层，不是 Worker 或进程隔离边界；配置不能关闭硬安全规则，也不能可靠限制内存或抢占任意同步循环。
- 网络和文件系统能力不会由布尔配置直接放行；网络需接入可取消、可审计的宿主适配器，否则明确返回 `SCRIPT_NETWORK_EXTERNAL_BLOCKED`，文件系统在浏览器本地执行中不提供。
- 使用 `ScriptExecutor` 执行脚本、收集日志、封装结果并维护执行统计。
- 使用模板管理器提供参数化脚本生成能力，减少重复脚本编写。
- 提供轻量脚本编辑组件，服务配置面板和可视化编辑场景。

## 重构方向

- 将执行统计、日志捕获和调度控制从 `ScriptExecutor` 中拆出，便于独立测试。
- 将大段内置模板迁移为独立资源或模板工厂，降低 `built-in-templates.ts` 的维护成本。
- 将编辑器状态、模板选择和安全检查拆成 composable，减少组件内部耦合。
