/**
 * 文件用途：集中导入应用启动所需的全局静态样式和资源。
 * 核心逻辑：加载 UnoCSS、全局样式、第三方样式或本地图标注册等副作用资源。
 * 关键注意事项：导入顺序会影响样式优先级，调整时需验证主题、图标和组件库样式。
 * 重构建议：可按样式、图标和第三方资源拆分后由当前入口聚合。
 */
import 'uno.css'
import '../styles/css/global.css'
import '../styles/scss/global.scss'
