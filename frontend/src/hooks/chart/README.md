# 图表 Hook

## 目录职责

提供基于 ECharts 的 Vue 组合式封装，统一图表初始化、主题切换、resize 和销毁。

## 文件关系

该目录面向通用图表组件，核心逻辑依赖主题 store、VueUse 元素引用和 ECharts 实例生命周期。

## 维护建议

后续可把 AetherLink IoT 图表配置和通用 ECharts 生命周期进一步拆开，降低复用时的隐式耦合。
