# 前端视觉与动效升级方案（基于 GitHub 高星项目调研）

更新：2026-08-23。调研范围：GitHub 高互动动画库、3D/数字孪生框架、高颜值 Vue3 后台。
结论先行：AetherLink 前端即 SoybeanAdmin 血统（elegant-router + Naive UI + pnpm monorepo 完全吻合），
升级应"顺着血统走"——优先对齐 Soybean 最新设计语言，再引入 motion-v 微交互，最后用 TresJS 做 IoT 设备三维化。

## 一、调研对象与可借鉴点

### 高颜值后台（直接同构，抄作业成本最低）
| 项目 | Stars | 技术栈 | 学什么 |
|---|---|---|---|
| soybeanjs/soybean-admin | ~14k | Vue3+NaiveUI+UnoCSS | 被评为"列表里最漂亮的后台"：排版、配色、**流畅动效**、多布局模式、主题切换 |
| vbenjs/vue-vben-admin | ~31.5k | Vue3+Shadcn 风格 | 架构标杆：UI 适配器可插拔、monorepo 严格分层 |
| pure-admin/vue-pure-admin | ~19.8k | Vue3+ElementPlus+Tailwind | 全 ESM、零 CJS 包袱的工程实践 |
| Daymychen/art-design-pro | ~5k | Vue3 | **"超越 star 数的颜值"**：有目的性的转场动效、整站视觉一致性、多套主题方向一键切换 |

### 动画库（按场景选型，不是全都要）
| 库 | Stars/热度 | 适用场景 | 对 AetherLink 的落点 |
|---|---|---|---|
| anime.js | ~72k | 轻量补间/SVG | 数字滚动（看板统计卡）|
| GSAP | ~27.8k | 时间线编排/ScrollTrigger | 大屏滚动叙事（暂无需求）|
| **motion-v** (Motion for Vue) | 2.2k★/周下载 51 万，官方 motiondivision 组织 | 声明式微交互、弹簧物理、进出场 | 列表/卡片/弹窗微交互首选，5kb 级 |
| formkit/auto-animate | 高 | 零配置 DOM 变更动画 | 表格行增删、下拉展开，一行指令接入 |
| dotLottie-web | 官方播放器 | 设计师导出的动效 | 空状态/加载态插画 |

### 3D/数字孪生（IoT 平台的差异化王牌）
| 项目 | Stars | 说明 |
|---|---|---|
| TresJS/tres | ~3.6k | **Vue3 声明式 Three.js 渲染器**（Evan You 背书）：组件即场景、响应式驱动 3D、cientos 生态（OrbitControls/useGLTF/Stars）、devtools 可视化调试 |
| hawk86104/three-vue-tres (TvT.js) | ~2.2k | 基于 TresJS 的**数字孪生/IoT 大屏**框架：插件市场、低代码 3D 编辑器、WebGL/WebGPU——与本平台场景完全对口 |

## 二、AetherLink 现状体检

- package.json 无任何动画库（gsap/lottie/motion 均未引入）
- `<Transition>` 仅 3 个文件使用；`@keyframes` 仅 6 处
- first-device 各 section 断点混用 640/900/1100px 三档（不成体系）
- ECharts 已深度使用但主题表现力一般（默认色板）
- 优势底座：soybean 血统自带主题系统与布局引擎，动效升级是"激活"而非"重造"

## 三、分阶段落地计划

### Phase 1：纯 CSS/Vue 内建（0 新依赖，本周可做）
1. **统一断点体系**：640/960/1280 三档 CSS 变量（`--bp-sm/md/lg`），替换散落的魔法数
2. **页面级转场**：路由容器加 `<Transition name="fade-slide" mode="out-in">`（soybean 同款缓动曲线 cubic-bezier(0.4,0,0.2,1)，220ms）
3. **列表交错入场**：卡片网格用 `animation-delay: calc(var(--i) * 40ms)` 交错浮现
4. **骨架屏**：n-skeleton 替换 loading 转圈（首屏感知速度提升最明显的一招）
5. **ECharts 主题**：注册 AetherLink 品牌 theme（主色渐变面积图、圆角柱、柔和网格线），统计卡数字滚动用 requestAnimationFrame 补间（不引库）

### Phase 2：motion-v 微交互（1 个新依赖，~18KB）
- 弹窗/抽屉进出弹簧动效；按钮按压回弹
- 看板卡片 hover 浮起 + 阴影过渡（对标 art-design-pro 的"有目的性转场"）
- 设备在线状态徽标的弹性脉冲

### Phase 3：TresJS 设备三维面板（差异化竞争力）
- 设备详情新增"3D 视图"页签：TresCanvas + useGLTF 加载设备模型（glb），实时遥测驱动材质颜色/旋转/数据标签
- 参照 TvT.js 的 IoT 大屏模式：厂区/机柜层级下钻
- 注意 WebGL 降级策略：不支持时回落现有 2D 视图

### Phase 4：dotLottie 空态/加载态插画包

## 四、执行约束

- 本地 Node24 pnpm 崩溃 → 新依赖安装需在 CI 或修复本地工具链后进行；Phase 1 无依赖可直接开工
- 每阶段独立 PR，附 before/after 截图与 Playwright 视觉快照
- 动效遵循 prefers-reduced-motion 媒体查询（可达性硬要求）
